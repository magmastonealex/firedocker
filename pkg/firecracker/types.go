package firecracker

// Contains structs based on API documentation for Firecracker.

type machineConfiguration struct {
	HTEnabled     bool `json:"ht_enabled"`
	MemorySizeMiB int  `json:"mem_size_mib"`
	NumVCPUs      int  `json:"vcpu_count"` // NumVCPUs is the number of virtual CPUs (FC threads) presented to a guest. Must be 1 or even.
}

type networkInterface struct {
	AllowMMDS     bool   `json:"allow_mmds_requests"`
	GuestMAC      string `json:"guest_mac"`
	IfceID        string `json:"iface_id"`      // Guest-side iface name
	HostInterface string `json:"host_dev_name"` // Host-side TAP device name
}

type bootSource struct {
	BootArgs   string `json:"boot_args"`
	InitRDPath string `json:"initrd_path"`
	KernelImg  string `json:"kernel_image_path"`
}

type drive struct {
	DriveID  string `json:"drive_id"` // will show up in guest as /dev/<driveID>
	ReadOnly bool   `json:"is_read_only"`
	RootDev  bool   `json:"is_root_device"`
	Path     string `json:"path_on_host"` // path to the drive image
}

type action struct {
	Type string `json:"action_type"`
}

// This struct isn't defined by Firecracker - it's the format the init & containers will expect to be available over MMDS.
type mmdsRoute struct {
	Gw      string `json:"gw"`
	Network string `json:"network"`
}
type mmdsIPConfig struct {
	IPCIDR       string      `json:"ip_cidr"`
	PrimaryDNS   string      `json:"primary_dns"`
	SecondaryDNS string      `json:"secondary_dns"`
	Routes       []mmdsRoute `json:"routes"`
}

type mmdsInfo struct {
	IPConfig      string `json:"ipconfig"` // JSON-serialized mmdsIpConfig
	RuntimeConfig string `json:"runtimeConfig"`
}
