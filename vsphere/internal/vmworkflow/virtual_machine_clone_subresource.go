package vmworkflow

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
	"github.com/terraform-providers/terraform-provider-vsphere/vsphere/internal/helper/datastore"
	"github.com/terraform-providers/terraform-provider-vsphere/vsphere/internal/helper/hostsystem"
	"github.com/terraform-providers/terraform-provider-vsphere/vsphere/internal/helper/network"
	"github.com/terraform-providers/terraform-provider-vsphere/vsphere/internal/helper/provider"
	"github.com/terraform-providers/terraform-provider-vsphere/vsphere/internal/helper/resourcepool"
	"github.com/terraform-providers/terraform-provider-vsphere/vsphere/internal/helper/virtualmachine"
	"github.com/terraform-providers/terraform-provider-vsphere/vsphere/internal/virtualdevice"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// VirtualMachineCloneSchema represents the schema for the VM clone sub-resource.
//
// This is a workflow for vsphere_virtual_machine that facilitates the creation
// of a virtual machine through cloning from an existing template.
// Customization is nested here, even though it exists in its own workflow.
func VirtualMachineCloneSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"template_uuid": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The UUID of the source virtual machine or template.",
		},
		"linked_clone": {
			Type:        schema.TypeBool,
			Optional:    true,
			Description: "Whether or not to create a linked clone when cloning. When this option is used, the source VM must have a single snapshot associated with it.",
		},
		"timeout": {
			Type:         schema.TypeInt,
			Optional:     true,
			Default:      30,
			Description:  "The timeout, in minutes, to wait for the virtual machine clone to complete.",
			ValidateFunc: validation.IntAtLeast(10),
		},
		"customize": {
			Type:        schema.TypeList,
			Optional:    true,
			MaxItems:    1,
			Description: "The customization spec for this clone. This allows the user to configure the virtual machine post-clone.",
			Elem:        &schema.Resource{Schema: VirtualMachineCustomizeSchema()},
		},
	}
}

// VirtualMachineInstantCloneSchema represents the schema for the VM instant clone sub-resource.
//
// This is a workflow for vsphere_virtual_machine that facilitates the creation
// of a virtual machine through instant cloning an existing powered on virtual machine.
func VirtualMachineInstantCloneSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"source_uuid": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The UUID of the source virtual machine.",
		},
		"timeout": {
			Type:         schema.TypeInt,
			Optional:     true,
			Default:      30,
			Description:  "The timeout, in minutes, to wait for the virtual machine clone to complete.",
			ValidateFunc: validation.IntAtLeast(10),
		},
	}
}

// ValidateVirtualMachineClone does pre-creation validation of a virtual
// machine's configuration to make sure it's suitable for use in cloning.
// This includes, but is not limited to checking to make sure that the disks in
// the new VM configuration line up with the configuration in the existing
// template, and checking to make sure that the VM has a single snapshot we can
// use in the even that linked clones are enabled.
func ValidateVirtualMachineClone(d *schema.ResourceDiff, c *govmomi.Client) error {
	tUUID := d.Get("clone.0.template_uuid").(string)
	if d.NewValueKnown("clone.0.template_uuid") {
		log.Printf("[DEBUG] ValidateVirtualMachineClone: Validating fitness of source VM/template %s", tUUID)
		vm, err := virtualmachine.FromUUID(c, tUUID)
		if err != nil {
			return fmt.Errorf("cannot locate virtual machine or template with UUID %q: %s", tUUID, err)
		}
		vprops, err := virtualmachine.Properties(vm)
		if err != nil {
			return fmt.Errorf("error fetching virtual machine or template properties: %s", err)
		}
		// Check to see if our guest IDs match.
		eGuestID := vprops.Config.GuestId
		aGuestID := d.Get("guest_id").(string)
		if eGuestID != aGuestID {
			return fmt.Errorf("invalid guest ID %q for clone. Please set it to %q", aGuestID, eGuestID)
		}
		// If linked clone is enabled, check to see if we have a snapshot. There need
		// to be a single snapshot on the template for it to be eligible.
		linked := d.Get("clone.0.linked_clone").(bool)
		if linked {
			log.Printf("[DEBUG] ValidateVirtualMachineClone: Checking snapshots on %s for linked clone eligibility", tUUID)
			if err := validateCloneSnapshots(vprops); err != nil {
				return err
			}
		}
		// Check to make sure the disks for this VM/template line up with the disks
		// in the configuration. This is in the virtual device package, so pass off
		// to that now.
		l := object.VirtualDeviceList(vprops.Config.Hardware.Device)
		if err := virtualdevice.DiskCloneValidateOperation(d, c, l, linked); err != nil {
			return err
		}
		vconfig := vprops.Config.VAppConfig
		if vconfig != nil {
			// We need to set the vApp transport types here so that it is available
			// later in CustomizeDiff where transport requirements are validated in
			// ValidateVAppTransport
			d.SetNew("vapp_transport", vconfig.GetVmConfigInfo().OvfEnvironmentTransport)
		}
	} else {
		log.Printf("[DEBUG] ValidateVirtualMachineClone: template_uuid is not available. Skipping template validation.")
	}

	// If a customization spec was defined, we need to check some items in it as well.
	if len(d.Get("clone.0.customize").([]interface{})) > 0 {
		if poolID, ok := d.GetOk("resource_pool_id"); ok {
			pool, err := resourcepool.FromID(c, poolID.(string))
			if err != nil {
				return fmt.Errorf("could not find resource pool ID %q: %s", poolID, err)
			}
			family, err := resourcepool.OSFamily(c, pool, d.Get("guest_id").(string))
			if err != nil {
				return fmt.Errorf("cannot find OS family for guest ID %q: %s", d.Get("guest_id").(string), err)
			}
			if err := ValidateCustomizationSpec(d, family); err != nil {
				return err
			}
		} else {
			log.Printf("[DEBUG] ValidateVirtualMachineClone: resource_pool_id is not available. Skipping OS family check.")
		}
	}
	log.Printf("[DEBUG] ValidateVirtualMachineClone: Source VM/template %s is a suitable source for cloning", tUUID)
	return nil
}

// validateCloneSnapshots checks a VM to make sure it has a single snapshot
// with no children, to make sure there is no ambiguity when selecting a
// snapshot for linked clones.
func validateCloneSnapshots(props *mo.VirtualMachine) error {
	if props.Snapshot == nil {
		return fmt.Errorf("virtual machine or template %s must have a snapshot to be used as a linked clone", props.Config.Uuid)
	}
	// Root snapshot list can only have a singular element
	if len(props.Snapshot.RootSnapshotList) != 1 {
		return fmt.Errorf("virtual machine or template %s must have exactly one root snapshot (has: %d)", props.Config.Uuid, len(props.Snapshot.RootSnapshotList))
	}
	// Check to make sure the root snapshot has no children
	if len(props.Snapshot.RootSnapshotList[0].ChildSnapshotList) > 0 {
		return fmt.Errorf("virtual machine or template %s's root snapshot must not have children", props.Config.Uuid)
	}
	// Current snapshot must match root snapshot (this should be the case anyway)
	if props.Snapshot.CurrentSnapshot.Value != props.Snapshot.RootSnapshotList[0].Snapshot.Value {
		return fmt.Errorf("virtual machine or template %s's current snapshot must match root snapshot", props.Config.Uuid)
	}
	return nil
}

// ExpandVirtualMachineCloneSpec creates a clone spec for an existing virtual machine.
//
// The clone spec built by this function for the clone contains the target
// datastore, the source snapshot in the event of linked clones, and a relocate
// spec that contains the new locations and configuration details of the new
// virtual disks.
func ExpandVirtualMachineCloneSpec(d *schema.ResourceData, c *govmomi.Client) (types.VirtualMachineCloneSpec, *object.VirtualMachine, error) {
	var spec types.VirtualMachineCloneSpec
	log.Printf("[DEBUG] ExpandVirtualMachineCloneSpec: Preparing clone spec for VM")

	// Populate the datastore only if we have a datastore ID. The ID may not be
	// specified in the event a datastore cluster is specified instead.
	if dsID, ok := d.GetOk("datastore_id"); ok {
		ds, err := datastore.FromID(c, dsID.(string))
		if err != nil {
			return spec, nil, fmt.Errorf("error locating datastore for VM: %s", err)
		}
		spec.Location.Datastore = types.NewReference(ds.Reference())
	}

	tUUID := d.Get("clone.0.template_uuid").(string)
	log.Printf("[DEBUG] ExpandVirtualMachineCloneSpec: Cloning from UUID: %s", tUUID)
	vm, err := virtualmachine.FromUUID(c, tUUID)
	if err != nil {
		return spec, nil, fmt.Errorf("cannot locate virtual machine or template with UUID %q: %s", tUUID, err)
	}
	vprops, err := virtualmachine.Properties(vm)
	if err != nil {
		return spec, nil, fmt.Errorf("error fetching virtual machine or template properties: %s", err)
	}
	// If we are creating a linked clone, grab the current snapshot of the
	// source, and populate the appropriate field. This should have already been
	// validated, but just in case, validate it again here.
	if d.Get("clone.0.linked_clone").(bool) {
		log.Printf("[DEBUG] ExpandVirtualMachineCloneSpec: Clone type is a linked clone")
		log.Printf("[DEBUG] ExpandVirtualMachineCloneSpec: Fetching snapshot for VM/template UUID %s", tUUID)
		if err := validateCloneSnapshots(vprops); err != nil {
			return spec, nil, err
		}
		spec.Snapshot = vprops.Snapshot.CurrentSnapshot
		spec.Location.DiskMoveType = string(types.VirtualMachineRelocateDiskMoveOptionsCreateNewChildDiskBacking)
		log.Printf("[DEBUG] ExpandVirtualMachineCloneSpec: Snapshot for clone: %s", vprops.Snapshot.CurrentSnapshot.Value)
	}

	// Set the target host system and resource pool.
	poolID := d.Get("resource_pool_id").(string)
	pool, err := resourcepool.FromID(c, poolID)
	if err != nil {
		return spec, nil, fmt.Errorf("could not find resource pool ID %q: %s", poolID, err)
	}
	var hs *object.HostSystem
	if v, ok := d.GetOk("host_system_id"); ok {
		hsID := v.(string)
		var err error
		if hs, err = hostsystem.FromID(c, hsID); err != nil {
			return spec, nil, fmt.Errorf("error locating host system at ID %q: %s", hsID, err)
		}
	}
	// Validate that the host is part of the resource pool before proceeding
	if err := resourcepool.ValidateHost(c, pool, hs); err != nil {
		return spec, nil, err
	}
	poolRef := pool.Reference()
	spec.Location.Pool = &poolRef
	if hs != nil {
		hsRef := hs.Reference()
		spec.Location.Host = &hsRef
	}

	// Grab the relocate spec for the disks.
	l := object.VirtualDeviceList(vprops.Config.Hardware.Device)
	relocators, err := virtualdevice.DiskCloneRelocateOperation(d, c, l)
	if err != nil {
		return spec, nil, err
	}
	spec.Location.Disk = relocators
	log.Printf("[DEBUG] ExpandVirtualMachineCloneSpec: Clone spec prep complete")
	return spec, vm, nil
}

// ExpandVirtualMachineInstantCloneSpec creates an instant clone spec for an existing virtual machine.
//
// The instant clone spec built by this function specifies the target datastore,
// the resource pool, the folder, extra config and the networks that back the
// respective network devices that are cloned from the source vm
func ExpandVirtualMachineInstantCloneSpec(d *schema.ResourceData, c *govmomi.Client, fo *object.Folder) (types.VirtualMachineInstantCloneSpec, *object.VirtualMachine, error) {
	var spec types.VirtualMachineInstantCloneSpec
	log.Printf("[DEBUG] ExpandVirtualMachineInstantCloneSpec: Preparing instant clone spec for VM")

	// Populate the datastore only if we have a datastore ID. The ID may not be
	// specified in the event a datastore cluster is specified instead.
	if dsID, ok := d.GetOk("datastore_id"); ok {
		ds, err := datastore.FromID(c, dsID.(string))
		if err != nil {
			return spec, nil, fmt.Errorf("error locating datastore for VM: %s", err)
		}
		spec.Location.Datastore = types.NewReference(ds.Reference())
	}

	spec.Name = d.Get("name").(string)
	spec.Location.Folder = types.NewReference(fo.Reference())

	//Set extra configuration data which can be used to supply advanced parameters
	ec := d.Get("extra_config").(map[string]interface{})

	for k, v := range ec {
		spec.Config = append(spec.Config, &types.OptionValue{Key: k, Value: v})
	}

	// prepare virtual device config spec for network card
	configSpecs := []types.BaseVirtualDeviceConfigSpec{}

	// Get the hardware devices associated with source vm
	srcUUID := d.Get("instantclone.0.source_uuid").(string)
	log.Printf("[DEBUG] ExpandVirtualMachineInstantCloneSpec: Cloning from UUID: %s", srcUUID)
	srcVM, err := virtualmachine.FromUUID(c, srcUUID)
	if err != nil {
		return spec, nil, fmt.Errorf("cannot locate virtual machine or template with UUID %q: %s", srcUUID, err)
	}
	vprops, err := virtualmachine.Properties(srcVM)
	if err != nil {
		return spec, nil, fmt.Errorf("error fetching virtual machine or template properties: %s", err)
	}

	// Filter devices of type BaseVirtualEthernetCard
	devices := object.VirtualDeviceList(vprops.Config.Hardware.Device)
	devices = devices.Select(func(device types.BaseVirtualDevice) bool {
		if _, ok := device.(types.BaseVirtualEthernetCard); ok {
			return true
		}
		return false
	})

	//Get network interfaces from resource
	n := d.Get("network_interface").([]interface{})

	// Iterate through network devices and update the backing devices
	for index, device := range devices {
		if index < len(n) {

			// Get backing device for network_interface from resource
			backingMoid := n[index].(map[string]interface{})["network_id"]

			net, err := network.FromID(c, backingMoid.(string))
			if err != nil {
				return spec, nil, err
			}
			bctx, bcancel := context.WithTimeout(context.Background(), provider.DefaultAPITimeout)
			defer bcancel()
			backing, err := net.EthernetCardBackingInfo(bctx)
			if err != nil {
				return spec, nil, err
			}

			// Configure the network device with the backing device previously determined from resource
			virtualEthernetCard := device.(types.BaseVirtualEthernetCard).GetVirtualEthernetCard()
			virtualEthernetCard.Backing = backing

			bandwidthLimit := int64(n[index].(map[string]interface{})["bandwidth_limit"].(int))
			bandwidthReservation := int64(n[index].(map[string]interface{})["bandwidth_reservation"].(int))

			virtualEthernetCard.ResourceAllocation.Limit = &bandwidthLimit
			virtualEthernetCard.ResourceAllocation.Reservation = &bandwidthReservation
			virtualEthernetCard.ResourceAllocation.Share.Level = types.SharesLevel(n[index].(map[string]interface{})["bandwidth_share_level"].(string))

			if virtualEthernetCard.ResourceAllocation.Share.Level == types.SharesLevelCustom {
				virtualEthernetCard.ResourceAllocation.Share.Shares = int32(n[index].(map[string]interface{})["bandwidth_share_count"].(int))
			}

			// If required then configure a static mac
			if n[index].(map[string]interface{})["use_static_mac"].(bool) {
				virtualEthernetCard.AddressType = string(types.VirtualEthernetCardMacTypeManual)
				virtualEthernetCard.MacAddress = n[index].(map[string]interface{})["mac_address"].(string)
			} else {
				virtualEthernetCard.AddressType = ""
				virtualEthernetCard.MacAddress = ""
			}

			configSpecs = append(configSpecs, &types.VirtualDeviceConfigSpec{
				Operation: types.VirtualDeviceConfigSpecOperationEdit,
				Device:    device,
			})
		}

		spec.Location.DeviceChange = configSpecs
	}

	// Set the target host system and resource pool.
	poolID := d.Get("resource_pool_id").(string)
	pool, err := resourcepool.FromID(c, poolID)
	if err != nil {
		return spec, nil, fmt.Errorf("could not find resource pool ID %q: %s", poolID, err)
	}

	poolRef := pool.Reference()
	spec.Location.Pool = &poolRef

	// Grab the relocate spec for the disks.
	l := object.VirtualDeviceList(vprops.Config.Hardware.Device)
	relocators, err := virtualdevice.DiskCloneRelocateOperation(d, c, l)
	if err != nil {
		return spec, nil, err
	}
	spec.Location.Disk = relocators
	log.Printf("[DEBUG] ExpandVirtualMachineInstantCloneSpec: Instant Clone spec prep complete")
	return spec, srcVM, nil
}
