# vSphere Instant Clone Example

The following examples demonstrate integration of the Instant Clone technology introduced in vSphere 6.7 with the [Terraform vSphere Provider][ref-tf-vsphere]. 

The purpose of this technology is to create virtual machines which are cloned from the running state of another virtual machine and are therefore identical to the source machine. 

A significant benefit of which is that they share memory and disk space with the virtual machine from which they are cloned; with the potential to rapidly deploy significantly higher numbers of virtual machines given available hardware resources.

However, there are caveats; specifically, any operation on an Instant Clone which results in a reboot will result in the loss of the shared memory benefits. Therefore, in order to preserve shared memory observe the following recommendations:

•	Do not reboot the Instant Clone.
•	Ensure that mechanisms such as the Distributed Resource Scheduler (DRS) do not automatically migrate the Instant Clone to an alternate host.
•	Ensure that Terraform does not invoke an operation that requires a reboot.

Unfortunately, most operations in the Terraform vSphere Provider which reconfigure a virtual machine will also reboot it. However, there are a number of operations which you can employ that will not result in a reboot, as detailed below:

•	Adding additional hardware such as disks or network interface cards.
•	Adding additional CPUs if the ‘Enable CPU Hot Add’ setting is enabled.
•	Adding additional Memory if the ‘Memory Hot Plug’ setting is enabled.
•	Changing the properties of a Network Interface including the network to which it is connected.
•	Configuring the Annotation.

The goal is therefore to replicate the configuration of the source virtual machine from which we are cloning in the Terraform plan as closely as possible and the examples provided detail how to achieve this.

## Source Virtual Machine preparation 

An instant clone can be created either from a source virtual machine in a frozen state, or from the current running point of a source virtual machine. 

The principal advantage of cloning from the current running point of a source virtual machine is that guest tools are not required. However, a significant disadvantage is that each clone operation creates a new delta disk on the source virtual machine. Not only can this affect performance, but vSphere only supports a disk chain length of 255; thereafter cloning operations of the source virtual machine will fail.

It is therefore recommended that the source virtual machine is frozen prior to any instant clone operations by issuing the operating system specific VMware Tools command in the guest operating system, i.e. rpctool.exe “instantclone.freeze”

## Examples

Two examples are provided, one that employs dynamic blocks and a second which uses static properties. Dynamic blocks are advantageous in that we can clone source virtual machines with different hardware layouts using a single generic plan; whereas the static example details how to uniquely reference properties of the source vm.

## Requirements

* Instant clone depends on functionality available only in vCenter Server.
* A running virtual machine from which to clone from.

## Usage Details

1. Configure the vCenter endpoint and credentials by either adding them to the `provider.tf` file or by using the appropriate environment variables.

2. Edit the `terraform.tfvars` file and populate the given fields with relevant values.

3. Execute the plan by invoking the command `terraform apply`