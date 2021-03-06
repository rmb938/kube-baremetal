package action

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/partition/gpt"
	"github.com/diskfs/go-diskfs/partition/mbr"
	"github.com/go-logr/logr"
	"github.com/mholt/archiver"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	baremetalv1alpha1 "github.com/rmb938/kube-baremetal/api/v1alpha1"
)

type ImageRequest struct {
	ImageURL            string `json:"image_url"`
	DiskPath            string `json:"disk_path"`
	MetadataContents    string `json:"metadata_contents"`
	NetworkDataContents string `json:"network_data_contents"`
	UserDataContents    string `json:"user_data_contents"`
}

type imageAction struct {
	ImageURL            string
	DiskPath            string
	MetadataContents    string
	NetworkdataContents string
	UserDataContents    string

	status *Status

	logger logr.Logger
}

func NewImageAction(imageURL, diskPath, metadataContents, networkdataContents, userDataContents string) *imageAction {
	action := &imageAction{
		ImageURL:            imageURL,
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

	copyIO := func(dst io.Writer, src io.Reader, size int64) error {
		_, err := io.Copy(dst, src)
		return err
	}

	if filepath.Ext(i.ImageURL) != ".raw" {
		archive, err := archiver.ByExtension(i.ImageURL)
		if err != nil {
			i.logger.Error(err, "error creating archiver from image url")
			return fmt.Errorf("error creating archiver from image url: %v", err)
		}

		switch archive.(type) {
		case archiver.Decompressor:
			decompressor := archive.(archiver.Decompressor)
			copyIO = func(dst io.Writer, src io.Reader, size int64) error {
				return decompressor.Decompress(src, dst)
			}
		case archiver.Reader:
			reader := archive.(archiver.Reader)
			copyIO = func(dst io.Writer, src io.Reader, size int64) error {
				err := reader.Open(src, size)
				if err != nil {
					return err
				}

				f, err := reader.Read()
				if err != nil {
					return err
				}
				defer f.Close()

				_, err = io.Copy(dst, f)
				return err
			}
		default:
			i.logger.Error(nil, "image url is not a compression or archive format")
			return fmt.Errorf("image url is not a compression or archive format")
		}
	}

	i.logger.Info("Downloading image", "url", i.ImageURL)
	resp, err := http.Get(i.ImageURL)
	if err != nil {
		i.logger.Error(err, "error downloading image", "image_url", i.ImageURL)
		return fmt.Errorf("error downloading image: %v", err)
	}
	defer resp.Body.Close()

	i.logger.Info("Writing image to disk", "disk", i.DiskPath)
	diskFile, err := os.OpenFile(i.DiskPath, os.O_RDWR|os.O_EXCL, 0600)
	if err != nil {
		i.logger.Error(err, "error opening disk file", "disk", i.DiskPath)
		return fmt.Errorf("error opening disk file %s: %v", i.DiskPath, err)
	}
	defer diskFile.Close()

	err = copyIO(diskFile, resp.Body, resp.ContentLength)
	if err != nil {
		i.logger.Error(err, "error copying image to disk", "disk", i.DiskPath)
		return fmt.Errorf("error copying image to disk %s: %v", i.DiskPath, err)
	}

	err = diskFile.Close()
	if err != nil {
		i.logger.Error(err, "error closing disk file", "disk", i.DiskPath)
		return fmt.Errorf("error closing disk file %s: %v", i.DiskPath, err)
	}

	i.logger.Info("Opening drive", "disk", i.DiskPath)
	destDisk, err := diskfs.Open(i.DiskPath)
	if err != nil {
		i.logger.Error(err, "error opening disk", "disk", i.DiskPath)
		return fmt.Errorf("error opening disk %s: %v", i.DiskPath, err)
	}
	defer destDisk.File.Close()

	i.logger.Info("Reading disk partitions", "disk", i.DiskPath)
	rawTable, err := destDisk.GetPartitionTable()
	if err != nil {
		i.logger.Error(err, "error reading partition table from drive", "disk", i.DiskPath)
		return fmt.Errorf("error reading partition table from drive %s: %v", i.DiskPath, err)
	}

	i.logger.Info("Found partition table", "type", rawTable.Type())
	cloudInitPartitionNumber := -1

	if rawTable.Type() == "gpt" {
		destDisk.File.Close()

		i.logger.Info("Running sgdisk to move the GPT partition table")
		sgdiskCommand := exec.Command("sgdisk", "-e", i.DiskPath)
		out, err := sgdiskCommand.CombinedOutput()
		if err != nil {
			i.logger.Error(err, "error running sgdisk to move the GPT partition table", "disk", i.DiskPath, "stdout/err", string(out))
			return fmt.Errorf("error running sgdisk to move the GPT partition table %s: %v: %v", i.DiskPath, err, string(out))
		}

		i.logger.Info("Reopening drive")
		destDisk, err = diskfs.Open(i.DiskPath)
		if err != nil {
			i.logger.Error(err, "error opening disk", "disk", i.DiskPath)
			return fmt.Errorf("error opening disk %s: %v", i.DiskPath, err)
		}
		defer destDisk.File.Close()

		i.logger.Info("Rereading partition table")
		rawTable, err = destDisk.GetPartitionTable()
		if err != nil {
			i.logger.Error(err, "error reading partition table from drive", "disk", i.DiskPath)
			return fmt.Errorf("error reading partition table from drive %s: %v", i.DiskPath, err)
		}

		table := rawTable.(*gpt.Table)

		cloudInitSize := 64 * 1024 * 1024 // 64 MB
		cloudInitSectors := uint64(cloudInitSize / table.LogicalSectorSize)
		// we want to create it at the end of the disk
		// so find the disk sector count and minus the cloudinit sectors
		// minus 33 to leave room for gpt partition table at the end of the disk
		cloudInitStart := uint64(int(destDisk.Size)/table.LogicalSectorSize) - cloudInitSectors - 33

		lastUsedParition := -1
		for partIndex, part := range table.Partitions {
			if part.Type != gpt.Unused {
				lastUsedParition = partIndex
			}
		}

		if lastUsedParition >= len(table.Partitions) {
			i.logger.Error(nil, "gpt partition table is full, there is no room for cloud-init", "disk", i.DiskPath)
			return fmt.Errorf("gpt partition table is full, there is no room for cloud-init on drive %s", i.DiskPath)
		}

		cloudInitPartitionIndex := lastUsedParition + 1
		table.Partitions[cloudInitPartitionIndex] = &gpt.Partition{
			Type:  gpt.LinuxFilesystem,
			Start: cloudInitStart,
			Size:  uint64(cloudInitSize),
		}
		cloudInitPartitionNumber = cloudInitPartitionIndex + 1

		i.logger.Info("Writing gpt partition table to disk")
		err = destDisk.Partition(table)
		if err != nil {
			i.logger.Error(err, "error writing gpt partition table to drive", "disk", i.DiskPath)
			return fmt.Errorf("error writing gpt partition table to drive %s: %v", i.DiskPath, err)
		}
	} else {
		table := rawTable.(*mbr.Table)

		cloudInitSize := 64 * 1024 * 1024 // 64 MB
		cloudInitSectors := uint32(cloudInitSize / table.LogicalSectorSize)
		// we want to create it at the end of the disk
		// so find the disk sector count and minus the cloudinit sectors
		cloudInitStart := uint32(int(destDisk.Size)/table.LogicalSectorSize) - cloudInitSectors

		partitions := make([]*mbr.Partition, 0)
		for _, part := range table.Partitions {
			if part.Type == mbr.Empty {
				continue
			}
			partitions = append(partitions, part)
		}

		if len(partitions) >= 4 {
			i.logger.Error(err, "mbr partition table already has 4 partitions, there is no room for cloud-init", "disk", i.DiskPath)
			return fmt.Errorf("mbr partition table already has 4 partitions, there is no room for cloud-init on drive %s: %v", i.DiskPath, err)
		}

		// add cloud-init partition
		table.Partitions = append(partitions, &mbr.Partition{
			Bootable: false,
			Type:     mbr.Linux,
			Start:    cloudInitStart,
			Size:     cloudInitSectors,
		})
		cloudInitPartitionNumber = len(table.Partitions)

		// write partition table to disk
		i.logger.Info("Writing mbr partition table to disk")
		err = destDisk.Partition(table)
		if err != nil {
			i.logger.Error(err, "error writing mbr partition table to drive", "disk", i.DiskPath)
			return fmt.Errorf("error writing mbr partition table to drive %s: %v", i.DiskPath, err)
		}
	}

	i.logger.Info("Creating cloud init filesystem")
	cloudInitFS, err := destDisk.CreateFilesystem(disk.FilesystemSpec{
		Partition:   cloudInitPartitionNumber,
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
