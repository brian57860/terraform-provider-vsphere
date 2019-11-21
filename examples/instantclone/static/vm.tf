resource "vsphere_virtual_machine" "vm" {
    annotation = "${data.vsphere_virtual_machine.source.annotation}"
    count = "${var.number_vms_required}"
    cpu_share_count = "${data.vsphere_virtual_machine.source.cpu_share_count}"
    cpu_share_level = "${data.vsphere_virtual_machine.source.cpu_share_level}"
    cpu_hot_add_enabled = "${data.vsphere_virtual_machine.source.cpu_hot_add_enabled}"
    datastore_id = "${data.vsphere_datastore.datastore.id}"

    disk {
        label = "disk0"
        size = "${data.vsphere_virtual_machine.source.disks.0.size}"
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

    network_interface {
        adapter_type = "${data.vsphere_virtual_machine.source.network_interfaces.0.adapter_type}"
        bandwidth_limit = "${data.vsphere_virtual_machine.source.network_interfaces.0.bandwidth_limit}"
        bandwidth_reservation = "${data.vsphere_virtual_machine.source.network_interfaces.0.bandwidth_reservation}"
        bandwidth_share_count = "${data.vsphere_virtual_machine.source.network_interfaces.0.bandwidth_share_count}"
        bandwidth_share_level = "${data.vsphere_virtual_machine.source.network_interfaces.0.bandwidth_share_level}"
        mac_address = "${data.vsphere_virtual_machine.source.network_interfaces.0.mac_address}"
        network_id = "${data.vsphere_virtual_machine.source.network_interfaces.0.network_id}"
        use_static_mac = true
    }

    num_cores_per_socket = "${data.vsphere_virtual_machine.source.num_cores_per_socket}"
    num_cpus = "${data.vsphere_virtual_machine.source.num_cpus}"
    resource_pool_id = "${data.vsphere_resource_pool.resource_pool.id}"
    scsi_type = "${data.vsphere_virtual_machine.source.scsi_type}"
    wait_for_guest_net_timeout = "0"
}