resource "vsphere_virtual_machine" "vm" {
    count = "${var.number_vms_required}"
    cpu_share_count = "${data.vsphere_virtual_machine.source.cpu_share_count}"
    cpu_share_level = "${data.vsphere_virtual_machine.source.cpu_share_level}"
    cpu_hot_add_enabled = "${data.vsphere_virtual_machine.source.cpu_hot_add_enabled}"
    datastore_id = "${data.vsphere_datastore.datastore.id}"

    dynamic "disk" {
        for_each = "${data.vsphere_virtual_machine.source.disks}"
        content {
            eagerly_scrub = disk.value.eagerly_scrub
            label = format("disk%d", disk.key)
            size = disk.value.size
            thin_provisioned = disk.value.thin_provisioned
            unit_number = disk.key
        }
    }

    enable_logging = "${data.vsphere_virtual_machine.source.enable_logging}"

    extra_config = {
        "guestinfo.ipaddress" = "192.168.0.${count.index+1}"
        "guestinfo.netmask" = "255.255.255.0"
    }

    folder = "${var.folder}"
    guest_id = "${data.vsphere_virtual_machine.source.guest_id}"
    
    instantclone {
        source_uuid = "${data.vsphere_virtual_machine.source.id}"
    }

    memory = "${data.vsphere_virtual_machine.source.memory}"
    memory_hot_add_enabled = "${data.vsphere_virtual_machine.source.memory_hot_add_enabled}"
    memory_share_count = "${data.vsphere_virtual_machine.source.memory_share_count}"
    memory_share_level = "${data.vsphere_virtual_machine.source.memory_share_level}"
    name = "${var.target_vm_prefix}${count.index}"

    dynamic "network_interface" {
        for_each = data.vsphere_virtual_machine.source.network_interfaces
        content {
            adapter_type = network_interface.value.adapter_type
            bandwidth_limit = network_interface.value.bandwidth_limit
            bandwidth_reservation = network_interface.value.bandwidth_reservation
            bandwidth_share_count = network_interface.value.bandwidth_share_count
            bandwidth_share_level = network_interface.value.bandwidth_share_level            
            mac_address = network_interface.value.mac_address
            network_id = network_interface.value.network_id
            use_static_mac = true
        }
    }

    num_cores_per_socket = "${data.vsphere_virtual_machine.source.num_cores_per_socket}"
    num_cpus = "${data.vsphere_virtual_machine.source.num_cpus}"
    resource_pool_id = "${data.vsphere_resource_pool.resource_pool.id}"
    scsi_type = "${data.vsphere_virtual_machine.source.scsi_type}"
    wait_for_guest_net_timeout = "0"
}