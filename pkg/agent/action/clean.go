package action

import (
	"fmt"
	"os/exec"
	"sync"

	"github.com/diskfs/go-diskfs"
	"github.com/go-logr/logr"
	"go.uber.org/multierr"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	baremetalv1alpha1 "github.com/rmb938/kube-baremetal/api/v1alpha1"
)

type cleanAction struct {
	hardware *baremetalv1alpha1.BareMetalDiscoverySpec

	status *Status

	logger logr.Logger
}

func NewCleanAction() *cleanAction {
	action := &cleanAction{

		status: &Status{
			Type:  CleaningActionType,
			Done:  false,
			Error: "",
		},

		logger: ctrllog.Log.WithName("clean-action"),
	}

	return action
}

func (c *cleanAction) Do(hardware *baremetalv1alpha1.BareMetalDiscoverySpec) {
	c.hardware = hardware
	err := c.do()
	if err != nil {
		c.status.Error = err.Error()
	}

	c.status.Done = true
}

func (c *cleanAction) do() error {
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(c.hardware.Hardware.Storage))

	errorChan := make(chan error, len(c.hardware.Hardware.Storage))

	c.logger.Info("Starting clean action")
	for _, storage := range c.hardware.Hardware.Storage {
		go func() {
			defer waitGroup.Done()
			errorChan <- c.cleanDrive(storage)
		}()
	}

	var err error
	waitGroup.Wait()
	c.logger.Info("Clean action has finished")
	close(errorChan)
	for cleanError := range errorChan {
		err = multierr.Append(err, cleanError)
	}

	return err
}

func (c *cleanAction) cleanDrive(storage baremetalv1alpha1.BareMetalDiscoveryHardwareStorage) error {
	diskPath := "/dev/" + storage.Name

	if storage.Trim == true {
		// when the device supports trim just use blkdiscard
		// try secure, otherwise do non-secure
		c.logger.Info("Trying secure blkdiscard", "disk", diskPath)
		secureDiscardCmd := exec.Command("blkdiscard", "-s", diskPath)
		_, err := secureDiscardCmd.CombinedOutput()
		if err != nil {
			c.logger.Info("Running unsecure blkdiscard", "disk", diskPath)
			unsecureDiscardCmd := exec.Command("blkdiscard", diskPath)
			output, err := unsecureDiscardCmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("error running blkdisard on %s: output: %s error: %v", diskPath, string(output), err)
			}
		}
	} else {
		// device doesn't support trim so do the following
		// 1. wipefs --force --all $diskPath  # wipe filesystem metadata
		// 2. dd bs=512 if=/dev/zero $diskPath count=33  # wipe partition metadata
		// 3. dd bs=512 if=/dev/zero $diskPath seek=$sectorCount-33 count=33 # wipe gpt partition metadata
		// 4. sgdisk -Z $diskPath # zap GPT and MBR structures

		disk, err := diskfs.Open(diskPath)
		if err != nil {
			return fmt.Errorf("error opening disk to calculate sector count: %v", err)
		}
		disk.File.Close()

		c.logger.Info("Wiping filesystem metadata", "disk", diskPath)
		wipefsCmd := exec.Command("wipefs", "--force", "--all", diskPath)
		output, err := wipefsCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error running wipefs on %s: output: %s error: %v", diskPath, string(output), err)
		}

		c.logger.Info("Wiping partition metadata", "disk", diskPath)
		wipePartitionMetadataCmd := exec.Command("dd", "bs=512", "if=/dev/zero", fmt.Sprintf("of=%s", diskPath), "count=33")
		output, err = wipePartitionMetadataCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error running dd to wipe metadata on %s: output: %s error: %v", diskPath, string(output), err)
		}

		c.logger.Info("Wiping gpt partition metadata", "disk", diskPath)
		sectorCount := disk.Size / disk.LogicalBlocksize
		gptBackupTableStart := sectorCount - 33
		wipeGPTPartitionMetadataCmd := exec.Command("dd", "bs=512", "if=/dev/zero", fmt.Sprintf("of=%s", diskPath), fmt.Sprintf("seek=%d", gptBackupTableStart), "count=33")
		output, err = wipeGPTPartitionMetadataCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error running dd to wipe gpt metadata on %s: output: %s error: %v", diskPath, string(output), err)
		}

		c.logger.Info("zap gpt and mbr structures", "disk", diskPath)
		sgDiskCmd := exec.Command("sgdisk", "-Z", diskPath)
		output, err = sgDiskCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error running sgdisk on %s: output: %s error: %v", diskPath, string(output), err)
		}
	}

	return nil
}

func (c *cleanAction) Status() (*Status, error) {
	return c.status, nil
}
