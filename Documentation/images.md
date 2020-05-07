# Images

## Supported Cloud Images

The following images have been tested and known to work.

* CentOS 7 - https://cloud.centos.org/centos/7/images/
    * Format: `raw.tar.gz`
* Fedora 32 - https://download.fedoraproject.org/pub/fedora/linux/releases/32/Cloud/x86_64/images/
    * Format: `raw.xz`
* Debian 10 - https://cloud.debian.org/images/cloud/OpenStack/current-10/
    * Format: `raw`

### Image Requirements

For an image to work the following must be met:

* MBR Partition Table
    * Recommended 1 partition, at most 3 partitions
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

### GPT and UEFI

GPT partitioned disk images are currently not supported. When trying to boot GPT grub returns an error of 
`error: file '/grub/i386-pc/normal.mod' not found`. There does not seem to be a way to fix it reliably without enabling 
UEFI. Motherboards have inconsistent behavior when devices are imaged or cleaned with UEFI enabled, this can cause 
unexpected boot behavior. It is highly recommended to set `CSM Compatibility` to `Legacy Only` in your motherboard's 
bios, without this setting boot order cannot be guaranteed to stay consistent.

There is probably a solution to easily solve this, however I do not have the resources to test on a wide range of systems.

## Unsupported Cloud Images

The following images are not compatible and are not supported.

* Ubuntu - https://cloud-images.ubuntu.com/focal/current/
    * Uses a GPT partition table
    * Does not release raw disk images
* CentOS 8 - https://cloud.centos.org/centos/8/x86_64/images/
    * Does not release raw disk images, probably will work if converted to a raw image
        * We should poke the CentOS devs to release raw images
* Fedora CoreOS - https://getfedora.org/en/coreos/download?tab=metal_virtualized&stream=stable
    * No Cloud-Init support
* OpenSUSE - https://download.opensuse.org/repositories/Cloud:/Images:/
    * Uses a GPT partition table
