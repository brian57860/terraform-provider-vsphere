package vsphere

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
	"github.com/terraform-providers/terraform-provider-vsphere/vsphere/internal/helper/structure"
	"github.com/terraform-providers/terraform-provider-vsphere/vsphere/internal/helper/virtualmachine"
	"github.com/terraform-providers/terraform-provider-vsphere/vsphere/internal/virtualdevice"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

func dataSourceVSphereVirtualMachine() *schema.Resource {
	r := &schema.Resource{
		Read: dataSourceVSphereVirtualMachineRead,

		Schema: map[string]*schema.Schema{
			"alternate_guest_name": {
				Type:        schema.TypeString,
				Description: "The alternate guest name of the virtual machine when guest_id is a non-specific operating system, like otherGuest.",
				Computed:    true,
			},
			"annotation": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "User-provided description of the virtual machine.",
			},
			"cpu_hot_add_enabled": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Allow CPUs to be added to this virtual machine while it is running.",
			},
			"datacenter_id": {
				Type:        schema.TypeString,
				Description: "The managed object ID of the datacenter the virtual machine is in. This is not required when using ESXi directly, or if there is only one datacenter in your infrastructure.",
				Optional:    true,
			},
			"disks": {
				Type:        schema.TypeList,
				Description: "Select configuration attributes from the disks on this virtual machine, sorted by bus and unit number.",
				Computed:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"size": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"eagerly_scrub": {
							Type:     schema.TypeBool,
							Computed: true,
						},
						"thin_provisioned": {
							Type:     schema.TypeBool,
							Computed: true,
						},
					},
				},
			},
			"enable_disk_uuid": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Expose the UUIDs of attached virtual disks to the virtual machine, allowing access to them in the guest.",
			},
			"enable_logging": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Enable logging on this virtual machine.",
			},
			"firmware": {
				Type:        schema.TypeString,
				Description: "The firmware type for this virtual machine.",
				Computed:    true,
			},
			"guest_id": {
				Type:        schema.TypeString,
				Description: "The guest ID of the virtual machine.",
				Computed:    true,
			},
			"memory": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     1024,
				Description: "The size of the virtual machine's memory, in MB.",
			},
			"memory_hot_add_enabled": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Allow memory to be added to this virtual machine while it is running.",
			},
			"name": {
				Type:        schema.TypeString,
				Description: "The name or path of the virtual machine.",
				Required:    true,
			},
			"network_interface_types": {
				Type:        schema.TypeList,
				Description: "The types of network interfaces found on the virtual machine, sorted by unit number.",
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
			"network_interfaces": {
				Type:        schema.TypeList,
				Description: "The types of network interfaces found on the virtual machine, sorted by unit number.",
				Computed:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"adapter_type": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"bandwidth_limit": {
							Type:         schema.TypeInt,
							Optional:     true,
							Default:      -1,
							Description:  "The upper bandwidth limit of this network interface, in Mbits/sec.",
							ValidateFunc: validation.IntAtLeast(-1),
						},
						"bandwidth_reservation": {
							Type:         schema.TypeInt,
							Optional:     true,
							Default:      0,
							Description:  "The bandwidth reservation of this network interface, in Mbits/sec.",
							ValidateFunc: validation.IntAtLeast(0),
						},
						"bandwidth_share_level": {
							Type:         schema.TypeString,
							Optional:     true,
							Default:      string(types.SharesLevelNormal),
							Description:  "The bandwidth share allocation level for this interface. Can be one of low, normal, high, or custom.",
							ValidateFunc: validation.StringInSlice(sharesLevelAllowedValues, false),
						},
						"bandwidth_share_count": {
							Type:         schema.TypeInt,
							Optional:     true,
							Computed:     true,
							Description:  "The share count for this network interface when the share level is custom.",
							ValidateFunc: validation.IntAtLeast(0),
						},
						"mac_address": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"network_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"num_cores_per_socket": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     1,
				Description: "The number of cores to distribute amongst the CPUs in this virtual machine. If specified, the value supplied to num_cpus must be evenly divisible by this value.",
			},
			"num_cpus": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     1,
				Description: "The number of virtual processors to assign to this virtual machine.",
			},
			"scsi_bus_sharing": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Mode for sharing the SCSI bus.",
			},
			"scsi_controller_scan_count": {
				Type:        schema.TypeInt,
				Description: "The number of SCSI controllers to scan for disk sizes and controller types on.",
				Optional:    true,
				Default:     1,
			},
			"scsi_type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The common SCSI bus type of all controllers on the virtual machine.",
			},
		},
	}
	structure.MergeSchema(r.Schema, schemaVirtualMachineResourceAllocation())

	return r
}

func dataSourceVSphereVirtualMachineRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*VSphereClient).vimClient

	name := d.Get("name").(string)
	log.Printf("[DEBUG] Looking for VM or template by name/path %q", name)
	var dc *object.Datacenter
	if dcID, ok := d.GetOk("datacenter_id"); ok {
		var err error
		dc, err = datacenterFromID(client, dcID.(string))
		if err != nil {
			return fmt.Errorf("cannot locate datacenter: %s", err)
		}
		log.Printf("[DEBUG] Datacenter for VM/template search: %s", dc.InventoryPath)
	}
	vm, err := virtualmachine.FromPath(client, name, dc)
	if err != nil {
		return fmt.Errorf("error fetching virtual machine: %s", err)
	}
	props, err := virtualmachine.Properties(vm)
	if err != nil {
		return fmt.Errorf("error fetching virtual machine properties: %s", err)
	}

	if props.Config == nil {
		return fmt.Errorf("no configuration returned for virtual machine %q", vm.InventoryPath)
	}

	if props.Config.Uuid == "" {
		return fmt.Errorf("virtual machine %q does not have a UUID", vm.InventoryPath)
	}

	d.SetId(props.Config.Uuid)
	d.Set("alternate_guest_name", props.Config.AlternateGuestName)
	d.Set("annotation", props.Config.Annotation)
	d.Set("cpu_hot_add_enabled", props.Config.CpuHotAddEnabled)
	d.Set("enable_logging", props.Config.Flags.EnableLogging)
	d.Set("enable_disk_uuid", props.Config.Flags.DiskUuidEnabled)
	d.Set("firmware", props.Config.Firmware)
	d.Set("guest_id", props.Config.GuestId)
	d.Set("memory", props.Config.Hardware.MemoryMB)
	d.Set("memory_hot_add_enabled", props.Config.MemoryHotAddEnabled)
	d.Set("num_cores_per_socket", props.Config.Hardware.NumCoresPerSocket)
	d.Set("num_cpus", props.Config.Hardware.NumCPU)
	d.Set("scsi_type", virtualdevice.ReadSCSIBusType(object.VirtualDeviceList(props.Config.Hardware.Device), d.Get("scsi_controller_scan_count").(int)))
	d.Set("scsi_bus_sharing", virtualdevice.ReadSCSIBusSharing(object.VirtualDeviceList(props.Config.Hardware.Device), d.Get("scsi_controller_scan_count").(int)))

	disks, err := virtualdevice.ReadDiskAttrsForDataSource(object.VirtualDeviceList(props.Config.Hardware.Device), d.Get("scsi_controller_scan_count").(int))
	if err != nil {
		return fmt.Errorf("error reading disk sizes: %s", err)
	}
	nics, err := virtualdevice.ReadNetworkInterfaceTypes(object.VirtualDeviceList(props.Config.Hardware.Device))
	if err != nil {
		return fmt.Errorf("error reading network interface types: %s", err)
	}
	networkInterfaces, err := virtualdevice.ReadNetworkInterfaces(object.VirtualDeviceList(props.Config.Hardware.Device))
	if err != nil {
		return fmt.Errorf("error reading network interfaces: %s", err)
	}
	if d.Set("disks", disks); err != nil {
		return fmt.Errorf("error setting disk sizes: %s", err)
	}
	if d.Set("network_interface_types", nics); err != nil {
		return fmt.Errorf("error setting network interface types: %s", err)
	}
	if d.Set("network_interfaces", networkInterfaces); err != nil {
		return fmt.Errorf("error setting network interfaces: %s", err)
	}
	if err := flattenVirtualMachineResourceAllocation(d, props.Config.CpuAllocation, "cpu"); err != nil {
		return fmt.Errorf("error setting cpu share allocation and limit: %s", err)
	}
	if err := flattenVirtualMachineResourceAllocation(d, props.Config.MemoryAllocation, "memory"); err != nil {
		return fmt.Errorf("error setting memory share allocation and limit: %s", err)
	}
	log.Printf("[DEBUG] VM search for %q completed successfully (UUID %q)", name, props.Config.Uuid)
	return nil
}
