# Kube BareMetal

**Project status: *alpha*** Not all features are completed. The API, spec, status and other user facing objects 
may change.

Kube BareMetal provides a provisioning solution for bare metal hardware in a cloud-like experience. Built with extensibility
in mind Kube BareMetal can be easily adapted and changed to fit any datacenter architecture.

## Kube BareMetal vs Other Solutions

### Other Kubernetes Operators

#### [Metal3](https://metal3.io/)

The idea of Kube BareMetal and the initial internal POCs started before Metal3 was announced. Kube BareMetal was initially
started to support a [Home Lab](https://www.reddit.com/r/homelab/) environment. This type of environment tends to have 
mixed hardware and sometimes no BMC support. 

Metal3 and Kube BareMetal aim to achieve similar goals with slightly different implementations. It is up to you to decide
which one is better for your environment.

### External Kubernetes Solutions

#### Unattended Install

* [Foreman](https://theforeman.org/)
* [Cobbler](https://cobbler.github.io/)
* ect..

While unattended install solutions are a great first step for automating operating system installation they have a few
main problems, they lack immutability and speed. Having to re-install the whole operating system takes a significant 
amount of time even with fast networking and disk. Immutability is also not guaranteed, a lot of work needs to be done
to ensure every installation gets the same version of packages installed.

## Development

### Operator Setup

#### Requirements

* [Go 1.13+](https://golang.org/)
* [Kube Builder](https://github.com/kubernetes-sigs/kubebuilder)
* [kind](https://github.com/kubernetes-sigs/kind)
* [tilt.dev](https://tilt.dev/)
* [linuxkit](https://github.com/linuxkit/linuxkit)

#### Steps

1. Setup the project in your favorite IDE
1. Run `make linuxkit`
1. Run `make kind`
1. Run `make tilt`

### Local VM Setup

If you want to test using a local virtual machine follow the steps bellow.

#### Requirements

* Libvirt 6.1.0+
* Ansible
    * python-libvirt
    * python-lxml

#### Steps

1. Run `make ansible`
1. Follow the Operator Setup
1. Start the VM
    1. Run `virt-manager --connect qemu:///session --show-domain-console kube-baremetal-0`
    1. Click the play button

## Deployment

### Requirements

* DHCP configured for IPXE Booting
* Kubernetes Cluster (tested on 1.17.0)
* Servers (bare metal or vms) 
    * UEFI booting is not supported, hardware must be configured to boot in CSM Legacy Only mode
    * Primary boot device set to PXE on the first NIC
    * Secondary boot device set to the drive where the OS will be installed

### Installation

TODO

## Usage

TODO

