// The datacenter the resources will be created in.
variable "datacenter" {
  type = "string"
}

// The name of the datastore the instant clone will be placed on
variable "datastore" {
  type = "string"
}

// The path to the folder to put this virtual machine in, relative to the datacenter that the resource pool is in
variable "folder" {
  type = "string"
}

// The number of instant clone child virtual machines to create
variable "number_vms_required" {
  type = "string"
}

// The resource pool the virtual machines will be placed in.
variable "resource_pool" {
  type = "string"
}

// The name of the source virtual machine to use when cloning.
variable "source_name" {
  type = "string"
}

// The number of instant clone child virtual machines to create
variable "target_vm_prefix" {
  type = "string"
}

