# Images

The Linux distributions mentioned bellow are not an exhaustive list, just ones that have been tested by the maintainers.

## Supported Cloud Images

The following images have been tested and known to work.

* CentOS 7 - https://cloud.centos.org/centos/7/images/
    * Filename: `CentOS-7-x86_64-GenericCloud-${VERSION}.raw.tar.gz`
* Fedora 32 - https://download.fedoraproject.org/pub/fedora/linux/releases/32/Cloud/x86_64/images/
    * Filename: `Fedora-Cloud-Base-32-${VERSION}.x86_64.raw.xz`
* Debian 10 - https://cloud.debian.org/images/cloud/OpenStack/current-10/
    * Filename: `debian-10-openstack-amd64.raw`

### Image Requirements

For an image to work the following must be met:

* Legacy BIOS Support
* Room for the Config Drive partition at the end of the drive
    * 64MB or 131072 512-byte sectors
* Cloud-Init with Config Drive enabled

#### Example Disk

##### Before Imaging

```shell script
# fdisk -lu /dev/vda 

Disk /dev/vda: 20 GiB, 21474836480 bytes, 41943040 sectors
Units: sectors of 1 * 512 = 512 bytes
Sector size (logical/physical): 512 bytes / 512 bytes
I/O size (minimum/optimal): 512 bytes / 512 bytes


# lsblk
NAME MAJ:MIN RM SIZE RO TYPE MOUNTPOINT
vda  254:0    0  20G  0 disk 
```

##### After Imaging

```shell script
# fdisk -lu /dev/vda

Disk /dev/vda: 21.5 GB, 21474836480 bytes, 41943040 sectors
Units = sectors of 1 * 512 = 512 bytes
Sector size (logical/physical): 512 bytes / 512 bytes
I/O size (minimum/optimal): 512 bytes / 512 bytes
Disk label type: dos
Disk identifier: 0x000940fd

   Device Boot      Start         End      Blocks   Id  System
/dev/vda1   *        2048    41811967    20904960   83  Linux
/dev/vda2        41811968    41943039       65536   83  Linux

# lsblk -o 'NAME,MAJ:MIN,RM,SIZE,RO,TYPE,MOUNTPOINT,LABEL'
NAME   MAJ:MIN RM SIZE RO TYPE MOUNTPOINT LABEL
vda    253:0    0  20G  0 disk            
├─vda1 253:1    0  20G  0 part /          
└─vda2 253:2    0  64M  0 part            config-2
```

##### After Cleaning

```shell script
# fdisk -lu /dev/vda 

Disk /dev/vda: 20 GiB, 21474836480 bytes, 41943040 sectors
Units: sectors of 1 * 512 = 512 bytes
Sector size (logical/physical): 512 bytes / 512 bytes
I/O size (minimum/optimal): 512 bytes / 512 bytes


# lsblk
NAME MAJ:MIN RM SIZE RO TYPE MOUNTPOINT
vda  254:0    0  20G  0 disk 
```

### Non-raw Disk Images

Non-raw disk images will never be supported. There is no native Golang libraries to convert qcow2 images without calling the CLI.
Calling the CLI is unreliable and can cause issues on low-memory instances. Converting QCOW2 to raw requires random seeks 
so the whole image has to live in memory, this is incompatible with Golang's Readers and Writers.

If you want to use a disk image that is in any other format, convert the image to raw and then place it somewhere to be 
downloaded.

### GPT Partition Tables

Images that use GPT Partition tables must support booting with `CSM Compatibility` set to `Legacy Only`. This typically
requires a `BIOS boot` partition.

```shell script
$ fdisk -lu bionic-server-cloudimg-amd64.raw                                                        
Disk bionic-server-cloudimg-amd64.raw: 2.2 GiB, 2361393152 bytes, 4612096 sectors
Units: sectors of 1 * 512 = 512 bytes
Sector size (logical/physical): 512 bytes / 512 bytes
I/O size (minimum/optimal): 512 bytes / 512 bytes
Disklabel type: gpt
Disk identifier: 7BB93E78-4787-4CA7-9293-56F2B1DD8764

Device                              Start     End Sectors  Size Type
bionic-server-cloudimg-amd64.raw1  227328 4612062 4384735  2.1G Linux filesystem
bionic-server-cloudimg-amd64.raw14   2048   10239    8192    4M BIOS boot
bionic-server-cloudimg-amd64.raw15  10240  227327  217088  106M EFI System
```

### UEFI

UEFI only images are not supported. Motherboards have inconsistent behavior when devices are imaged or cleaned with UEFI 
enabled, this can cause unexpected boot behavior. It is highly recommended to set `CSM Compatibility` to `Legacy Only` 
in your motherboard's bios, without this setting boot order cannot be guaranteed to stay consistent.

There is probably a solution to easily solve this, however I do not have the resources to test on a wide range of systems.

## Semi-Supported Cloud Images

The following images are supported but require modification to work.

* Ubuntu Focal (18.04) - https://cloud-images.ubuntu.com/bionic/20200507/
    * Filename: `bionic-server-cloudimg-amd64.img`
    * Needs to be converted to a raw image
        ```shell script
        qemu-img convert -f qcow2 -O raw bionic-server-cloudimg-amd64.img bionic-server-cloudimg-amd64.raw
        ```

## Not recommended Cloud Images

The following cloud images are not recommended due to various bugs or issues.

* Ubuntu Focal (20.04) - https://cloud-images.ubuntu.com/focal/20200506/
    * Filename: `focal-server-cloudimg-amd64.img`
    * Needs to be converted to a raw image
        ```shell script
        qemu-img convert -f qcow2 -O raw focal-server-cloudimg-amd64.img focal-server-cloudimg-amd64.raw
        ```
    * When booting the machine, it always reboots twice and takes a long time to boot.
    * Constantly complains about `blk_update_request: operation not supported (WRITE_ZEROS)` on nvme boot drive

## Unsupported Cloud Images

The following images are not compatible and are not supported.

* CentOS 8 - https://cloud.centos.org/centos/8/x86_64/images/
    * Filename: `CentOS-8-GenericCloud-${VERSION}.x86_64.qcow2`
    * Needs to be converted to a raw image
        ```shell script
        qemu-img convert -f qcow2 -O raw CentOS-8-GenericCloud-${VERSION}.x86_64.qcow2 CentOS-8-GenericCloud-${VERSION}.x86_64.raw
        ```
    * Kernel does not log to tty0: https://bugs.centos.org/view.php?id=17343
    * Boot seems to hang on physical nodes
* Fedora CoreOS - https://getfedora.org/en/coreos/download?tab=metal_virtualized&stream=stable
    * No Cloud-Init support
