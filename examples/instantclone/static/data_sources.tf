data "vsphere_datacenter" "dc" {
    name            = var.datacenter
}

data "vsphere_datastore" "datastore" {
    name            = var.datastore
    datacenter_id   = data.vsphere_datacenter.dc.id
}

data "vsphere_resource_pool" "resource_pool" {
    name            = var.resource_pool
    datacenter_id   = data.vsphere_datacenter.dc.id
}

data "vsphere_virtual_machine" "source" {
    name            = var.source_name
    datacenter_id   = data.vsphere_datacenter.dc.id
}