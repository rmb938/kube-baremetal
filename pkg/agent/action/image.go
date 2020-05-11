package action

import (
	"archive/tar"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"

	"github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/partition"
	"github.com/diskfs/go-diskfs/partition/mbr"
	"github.com/go-logr/logr"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	baremetalv1alpha1 "github.com/rmb938/kube-baremetal/api/v1alpha1"
)

type blockDevicePartitions struct {
	Name string `json:"name"`
}

type blockDevices struct {
	Children []blockDevicePartitions `json:"children"`
}

type lsblk struct {
	BlockDevices []blockDevices `json:"blockdevices"`
}

type ImageRequest struct {
	Image               string `json:"image"`
	DiskPath            string `json:"disk_path"`
	MetadataContents    string `json:"metadata_contents"`
	NetworkDataContents string `json:"network_data_contents"`
	UserDataContents    string `json:"user_data_contents"`
}

type imageAction struct {
	Image               string
	DiskPath            string
	MetadataContents    string
	NetworkdataContents string
	UserDataContents    string

	status *Status

	logger logr.Logger
}

const (
	BootStartSector    uint32 = 2048
	BootPartitionLabel string = "BOOT"
	BootPartitionSize  int    = 500 * 1024 * 1024 // 500MB

	MbrBootPartitionMountPath string = "/boot"
	GptBootPartitionMountPath string = "/boot"

	RootFSPartitionSize    int = 10 * 1024 * 1024 * 1024 // 10GB
	CloudInitPartitionSize int = 64 * 1024 * 1024        // 64 MB
)

func NewImageAction(image, diskPath, metadataContents, networkdataContents, userDataContents string) *imageAction {
	action := &imageAction{
		Image:               image,
		DiskPath:            diskPath,
		MetadataContents:    metadataContents,
		NetworkdataContents: networkdataContents,
		UserDataContents:    userDataContents,

		status: &Status{
			Type:  ImagingActionType,
			Done:  false,
			Error: "",
		},

		logger: ctrllog.Log.WithName("image-action"),
	}

	return action
}

func (i *imageAction) Do(hardware *baremetalv1alpha1.BareMetalDiscoveryHardware) {
	err := i.do()
	if err != nil {
		i.status.Error = err.Error()
	}

	i.status.Done = true
}

func (i *imageAction) do() error {
	metadataContents, err := base64.StdEncoding.DecodeString(i.MetadataContents)
	if err != nil {
		i.logger.Error(err, "error base64 decoding metadata")
		return fmt.Errorf("error base64 decoding metadata: %v", err)
	}

	networkDataContents, err := base64.StdEncoding.DecodeString(i.NetworkdataContents)
	if err != nil {
		i.logger.Error(err, "error base64 decoding network data")
		return fmt.Errorf("error base64 decoding network data: %v", err)
	}

	userDataContents, err := base64.StdEncoding.DecodeString(i.UserDataContents)
	if err != nil {
		i.logger.Error(err, "error base64 decoding user data")
		return fmt.Errorf("error base64 decoding user data: %v", err)
	}

	i.logger.Info("Opening drive", "disk", i.DiskPath)
	destDisk, err := diskfs.Open(i.DiskPath)
	if err != nil {
		i.logger.Error(err, "error opening disk", "disk", i.DiskPath)
		return fmt.Errorf("error opening disk %s: %v", i.DiskPath, err)
	}
	defer destDisk.File.Close()

	// mbr partitions
	//  /boot - xfs - label BOOT
	//  / - xfs - label rootfs
	//  none - vfat - label config-2
	// gpt partitions
	//  /boot/efi - vfat - label BOOT
	//  / - xfs - label rootfs
	//  none - vfat - label config-2

	bootPartitionSectors := uint32(BootPartitionSize) / uint32(destDisk.LogicalBlocksize)
	rootFSPartitionSectors := uint32(RootFSPartitionSize / int(destDisk.LogicalBlocksize))
	cloudInitSectors := uint32(CloudInitPartitionSize) / uint32(destDisk.LogicalBlocksize)
	// we want to create it at the end of the disk
	// so find the disk sector count and minus the cloudinit sectors
	cloudInitStart := uint32(destDisk.Size/destDisk.LogicalBlocksize) - cloudInitSectors

	var partitionTable partition.Table

	// TODO: if booted via efi use gpt partition table

	partitionTable = &mbr.Table{
		Partitions: []*mbr.Partition{
			{
				Bootable: true,
				Type:     mbr.Linux,
				Start:    BootStartSector,
				Size:     bootPartitionSectors,
			},
			{
				Bootable: false,
				Type:     mbr.Linux,
				Start:    BootStartSector + uint32(BootPartitionSize/int(destDisk.LogicalBlocksize)),
				Size:     rootFSPartitionSectors,
			},
			{
				Bootable: false,
				Type:     mbr.Linux,
				Start:    cloudInitStart,
				Size:     cloudInitSectors,
			},
		},
		LogicalSectorSize:  int(destDisk.LogicalBlocksize),
		PhysicalSectorSize: int(destDisk.PhysicalBlocksize),
	}

	i.logger.Info("Creating disk partitions")
	err = destDisk.Partition(partitionTable)
	if err != nil {
		i.logger.Error(err, "error creating disk partitions", "disk", i.DiskPath)
		return fmt.Errorf("error creating disk partitions %s: %v", i.DiskPath, err)
	}

	i.logger.Info("Creating cloud init filesystem")
	cloudInitFS, err := destDisk.CreateFilesystem(disk.FilesystemSpec{
		Partition:   3,
		FSType:      filesystem.TypeFat32,
		VolumeLabel: "config-2",
	})
	if err != nil {
		i.logger.Error(err, "error creating cloud-init filesystem", "disk", i.DiskPath)
		return fmt.Errorf("error creating cloud-init filesystem on %s: %v", i.DiskPath, err)
	}

	cloudInitPrefix := path.Join("/", "openstack", "latest")
	// place down cloud-init info
	i.logger.Info("Creating cloud init directory structure")
	err = cloudInitFS.Mkdir(cloudInitPrefix)
	if err != nil {
		i.logger.Error(err, "error creating cloud-init directory structure")
		return fmt.Errorf("error creating cloud-init directory structure: %v", err)
	}

	metadataPath := path.Join(cloudInitPrefix, "meta_data.json")
	i.logger.Info("Writing metadata file", "path", metadataPath)
	err = i.writeFile(cloudInitFS, metadataPath, metadataContents)
	if err != nil {
		i.logger.Error(err, "error writing metadata")
		return fmt.Errorf("error writing metadata: %v", err)
	}

	networkdataPath := path.Join(cloudInitPrefix, "network_data.json")
	i.logger.Info("Writing network data file", "path", networkdataPath)
	err = i.writeFile(cloudInitFS, networkdataPath, networkDataContents)
	if err != nil {
		i.logger.Error(err, "error writing network data")
		return fmt.Errorf("error writing network data: %v", err)
	}

	userDataPath := path.Join(cloudInitPrefix, "user_data")
	i.logger.Info("Writing user data file", "path", userDataPath)
	err = i.writeFile(cloudInitFS, userDataPath, userDataContents)
	if err != nil {
		i.logger.Error(err, "error writing user data")
		return fmt.Errorf("error writing user data: %v", err)
	}

	err = destDisk.File.Close()
	if err != nil {
		i.logger.Error(err, "error closing disk", "disk", i.DiskPath)
		return fmt.Errorf("error closing disk %s: %v", i.DiskPath, err)
	}

	i.logger.Info("Re-reading the partition table")
	partProbeOutput, err := exec.Command("partprobe", i.DiskPath).CombinedOutput()
	if err != nil {
		i.logger.Error(err, "error running partprobe", "disk", i.DiskPath, "output", string(partProbeOutput))
		return fmt.Errorf("error running partprobe %s: %v: %s", i.DiskPath, err, string(partProbeOutput))
	}

	lsblkCmd := exec.Command("lsblk", i.DiskPath, "--json", "-o", "name")
	lsblkOutput, err := lsblkCmd.CombinedOutput()
	if err != nil {
		i.logger.Error(err, "error running lsblk", "disk", i.DiskPath, "output", string(lsblkOutput))
		return fmt.Errorf("error running lsblk %s: %v: %s", i.DiskPath, err, string(lsblkOutput))
	}

	lsblk := &lsblk{}
	err = json.Unmarshal(lsblkOutput, lsblk)
	if err != nil {
		i.logger.Error(err, "error parsing lsblk", "disk", i.DiskPath, "output", string(lsblkOutput))
		return fmt.Errorf("error parsing lsblk %s: %v: %s", i.DiskPath, err, string(lsblkOutput))
	}

	// TODO: if efi this needs to be fat32
	i.logger.Info("Creating filesystem on boot partition")
	bootPartitionPath := fmt.Sprintf("/dev/%s", lsblk.BlockDevices[0].Children[0].Name)
	// mkfsOutput, err := exec.Command("mkfs.vfat", "-F32", "-n", BootPartitionLabel, bootPartitionPath).CombinedOutput()
	mkfsOutput, err := exec.Command("mkfs.xfs", "-f", "-L", BootPartitionLabel, bootPartitionPath).CombinedOutput()
	if err != nil {
		i.logger.Error(err, "error creating filesystem on boot partition", "partition", bootPartitionPath, "output", string(mkfsOutput))
		return fmt.Errorf("error creating filesystem on boot partition %s: %v: %s", bootPartitionPath, err, string(mkfsOutput))
	}

	i.logger.Info("Creating filesystem on rootfs partition")
	imagePartitionPath := fmt.Sprintf("/dev/%s", lsblk.BlockDevices[0].Children[1].Name)
	mkfsOutput, err = exec.Command("mkfs.xfs", "-f", "-L", "rootfs", imagePartitionPath).CombinedOutput()
	if err != nil {
		i.logger.Error(err, "error creating filesystem on rootfs partition", "partition", imagePartitionPath, "output", string(mkfsOutput))
		return fmt.Errorf("error creating filesystem on rootfs partition %s: %v: %s", imagePartitionPath, err, string(mkfsOutput))
	}

	i.logger.Info("Mounting image partition")
	err = syscall.Mount(imagePartitionPath, "/mnt", "xfs", 0, "")
	if err != nil {
		i.logger.Error(err, "error mounting image partition")
		return fmt.Errorf("error mounting image partition %v", err)
	}

	i.logger.Info("Mounting boot partition")
	// TODO: if efi mount in GptBootPartitionMountPath and fstype vfat
	bootPartitionMountPath := fmt.Sprintf("/mnt%s", MbrBootPartitionMountPath)
	err = os.MkdirAll(bootPartitionMountPath, 0555)
	if err != nil {
		i.logger.Error(err, "error making boot mount directory")
		return fmt.Errorf("error making boot mount directory %v", err)
	}

	// err = syscall.Mount(imagePartitionPath, fmt.Sprintf("/mnt%s", GptBootPartitionMountPath), "vfat", 0, "")
	err = syscall.Mount(bootPartitionPath, bootPartitionMountPath, "xfs", 0, "")
	if err != nil {
		i.logger.Error(err, "error mounting boot partition")
		return fmt.Errorf("error mounting boot partition %v", err)
	}

	// TODO: do some partition magic so we don't need a ton of memory for podman
	// podman --root /some/place/on/disk $COMMAND

	i.logger.Info("Creating podman container", "image", i.Image)
	createContainerCmd := exec.Command("podman", "create", "--name", "image", i.Image)
	createContainerOutput, err := createContainerCmd.CombinedOutput()
	if err != nil {
		i.logger.Error(err, "error creating podman container from image", "stderr", string(createContainerOutput))
		return fmt.Errorf("error creating podman container from image: %v: %s", err, string(createContainerOutput))
	}

	exportContainerCmd := exec.Command("podman", "export", "image")
	exportContainerCmdBuffer := &bytes.Buffer{}
	exportContainerCmd.Stderr = exportContainerCmdBuffer
	stdout, err := exportContainerCmd.StdoutPipe()
	if err != nil {
		i.logger.Error(err, "error create stdout pipe for podman export")
		return fmt.Errorf("error create stdout pipe for podman export: %v", err)
	}

	i.logger.Info("Starting podman export")
	err = exportContainerCmd.Start()
	if err != nil {
		i.logger.Error(err, "error starting podman export")
		return fmt.Errorf("error starting podman export: %v", err)
	}

	i.logger.Info("Copying podman export output to root partition")
	tarReader := tar.NewReader(stdout)
	for {
		hdr, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			i.logger.Error(err, "error reading next from podman export tar")
			return fmt.Errorf("error reading next from podman export tar: %v", err)
		}

		to := filepath.Join("/mnt", hdr.Name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			err := os.MkdirAll(to, os.FileMode(hdr.FileInfo().Mode()))
			if err != nil {
				i.logger.Error(err, "error creating directory", "directory", hdr.Name)
				return fmt.Errorf("error creating directory %s: %v", hdr.Name, err)
			}
		case tar.TypeReg, tar.TypeRegA, tar.TypeChar, tar.TypeBlock, tar.TypeFifo, tar.TypeGNUSparse:
			f, err := os.OpenFile(to, os.O_CREATE|os.O_RDWR, os.FileMode(hdr.FileInfo().Mode()))
			if err != nil {
				i.logger.Error(err, "error creating file", "file", hdr.Name)
				return fmt.Errorf("error creating file %s: %v", hdr.Name, err)
			}

			_, err = io.Copy(f, tarReader)
			if err != nil {
				i.logger.Error(err, "error copying file", "file", hdr.Name)
				return fmt.Errorf("error copying file %s: %v", hdr.Name, err)
			}

			err = f.Close()
			if err != nil {
				i.logger.Error(err, "error closing file", "file", hdr.Name)
				return fmt.Errorf("error closing file %s: %v", hdr.Name, err)
			}
		case tar.TypeSymlink:
			err := os.MkdirAll(filepath.Dir(to), 0755)
			if err != nil {
				i.logger.Error(err, "error making directory for symlink", "symlink", hdr.Name)
				return fmt.Errorf("error making directory file for symlink %s: %v", hdr.Name, err)
			}

			_, err = os.Lstat(to)
			if err == nil {
				err = os.Remove(to)
				if err != nil {
					i.logger.Error(err, "error removing file for symlink", "symlink", hdr.Name)
					return fmt.Errorf("error removing file for symlink %s: %v", hdr.Name, err)
				}
			}

			err = os.Symlink(hdr.Linkname, to)
			if err != nil {
				i.logger.Error(err, "error creating symlink", "symlink", hdr.Name)
				return fmt.Errorf("error creating symlink %s: %v", hdr.Name, err)
			}
		case tar.TypeLink:
			err := os.MkdirAll(filepath.Dir(to), 0755)
			if err != nil {
				i.logger.Error(err, "error making directory for hardlink", "hardlink", hdr.Name)
				return fmt.Errorf("error making directory file for hardlink %s: %v", hdr.Name, err)
			}

			_, err = os.Lstat(to)
			if err == nil {
				err = os.Remove(to)
				if err != nil {
					i.logger.Error(err, "error removing file for hardlink", "hardlink", hdr.Name)
					return fmt.Errorf("error removing file for hardlink %s: %v", hdr.Name, err)
				}
			}

			err = os.Link(filepath.Join("/mnt", hdr.Linkname), to)
			if err != nil {
				i.logger.Error(err, "error creating hardlink", "symlink", hdr.Name)
				return fmt.Errorf("error creating hardlink %s: %v", hdr.Name, err)
			}
		default:
			return fmt.Errorf("error reading header in tar: unknown type flag: %s: %c", hdr.Name, hdr.Typeflag)
		}
	}

	i.logger.Info("Waiting for podman export to finish")
	err = exportContainerCmd.Wait()
	if err != nil {
		i.logger.Error(err, "error exporting podman container", "stderr", string(exportContainerCmdBuffer.Bytes()))
		return fmt.Errorf("error exporting podman container: %v: %s", err, string(exportContainerCmdBuffer.Bytes()))
	}

	i.logger.Info("Podman export finished")

	i.logger.Info("Writing fstab")

	// TODO: change this for efi /boot/efi and vfat
	fstab := fmt.Sprintf(`
LABEL=rootfs / xfs defaults 1 1
LABEL=%s %s xfs defaults 1 2
`, BootPartitionLabel, MbrBootPartitionMountPath)

	err = ioutil.WriteFile("/mnt/etc/fstab", []byte(fstab), 0664)
	if err != nil {
		i.logger.Error(err, "error writting fstab")
		return fmt.Errorf("error writting fstab: %v", err)
	}

	i.logger.Info("Writing grub device map")
	err = os.MkdirAll("/mnt/boot/grub/", 0755)
	if err != nil {
		i.logger.Error(err, "error making grub directory")
		return fmt.Errorf("error making grub directory: %v", err)
	}

	deviceMap := fmt.Sprintf("(hd0) %s", i.DiskPath)
	err = ioutil.WriteFile("/mnt/boot/grub/device.map", []byte(deviceMap), 0700)
	if err != nil {
		i.logger.Error(err, "error grub device map")
		return fmt.Errorf("error grub device map: %v", err)
	}

	i.logger.Info("Doing bind mounts")

	bindMountCommands := []*exec.Cmd{
		exec.Command("mount", "-t", "proc", "proc", "/mnt/proc"),
		exec.Command("mount", "--rbind", "/sys", "/mnt/sys"),
		exec.Command("mount", "--rbind", "/dev", "/mnt/dev"),
		exec.Command("mount", "--rbind", "/run", "/mnt/run"),
	}

	for _, bindMountCmd := range bindMountCommands {
		bindMountCmdOutput, err := bindMountCmd.CombinedOutput()
		if err != nil {
			i.logger.Error(err, "error running bind mount command", "output", string(bindMountCmdOutput))
			return fmt.Errorf("error running bind mount command: %v: %s", err, string(bindMountCmdOutput))
		}
	}

	i.logger.Info("Doing grub stuffs")
	// TODO: this may change with efi

	grubCommands := []*exec.Cmd{
		exec.Command("chroot", "/mnt", "update-grub"),
		exec.Command("chroot", "/mnt", "grub-install", "--force", i.DiskPath),
		exec.Command("chroot", "/mnt", "grub-mkdevicemap"),
	}

	for _, grubCmd := range grubCommands {
		grubCmd.Env = append(grubCmd.Env, "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
		grubCmdOutput, err := grubCmd.CombinedOutput()
		if err != nil {
			i.logger.Error(err, "error running grub command", "output", string(grubCmdOutput))
			return fmt.Errorf("error running grub command: %v: %s", err, string(grubCmdOutput))
		}
	}

	i.logger.Info("Imaging has finished")

	return nil
}

func (i *imageAction) writeFile(fs filesystem.FileSystem, path string, contents []byte) error {
	f, err := fs.OpenFile(path, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return fmt.Errorf("error opening file %s: %v", path, err)
	}

	_, err = f.Write(contents)
	if err != nil {
		return fmt.Errorf("error writting file %s: %v", path, err)
	}

	return nil
}

func (i *imageAction) Status() (*Status, error) {
	return i.status, nil
}
