package domain

import (
	"cmp"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

// AllocationMode determines how a network is used.
type AllocationMode uint8

const (
	AllocationModeUnallocated AllocationMode = iota
	AllocationModeSubnets
	AllocationModeHosts
)

// TaggedVLANMode controls tagged VLAN behaviour on a port.
type TaggedVLANMode string

const (
	TaggedVLANModeNone     TaggedVLANMode = ""
	TaggedVLANModeAllowAll TaggedVLANMode = "AllowAll"
	TaggedVLANModeBlockAll TaggedVLANMode = "BlockAll"
	TaggedVLANModeCustom   TaggedVLANMode = "Custom"
)

// Static folder identifiers used as root-level catalog entries.
const (
	FolderNetworks  = "Networks"
	FolderZones     = "Zones"
	FolderVLANs     = "VLANs"
	FolderSSIDs     = "WiFi SSIDs"
	FolderEquipment = "Equipment"
)

// Item is the interface implemented by every catalog entity.
type Item interface {
	RawID() string
	DisplayID() string
	GetParentPath() string
	GetPath() string
	Compare(other Item) int
	Validate(c *Catalog) error
}

// Base holds the common identity fields shared by all catalog items.
type Base struct {
	ID         string `json:"id"`
	ParentPath string `json:"parent"`
}

func (b *Base) RawID() string         { return b.ID }
func (b *Base) GetParentPath() string { return b.ParentPath }
func (b *Base) GetPath() string {
	if b.ParentPath == "" {
		return b.ID
	}
	return b.ParentPath + " -> " + b.ID
}

// ---------- StaticFolder ----------

// StaticFolder represents a top-level menu category (Networks, VLANs, etc.).
type StaticFolder struct {
	Base
	Index       int
	Description string
}

func (m *StaticFolder) DisplayID() string { return m.ID }

func (m *StaticFolder) Compare(other Item) int {
	if other == nil {
		return 1
	}
	otherMenu, ok := other.(*StaticFolder)
	if !ok {
		return cmp.Compare(m.DisplayID(), other.DisplayID())
	}
	return cmp.Compare(m.Index, otherMenu.Index)
}

// ---------- Network ----------

// Network represents an IP network (CIDR block).
type Network struct {
	Base
	AllocationMode AllocationMode `json:"allocation_mode"`
	DisplayName    string         `json:"display_name"`
	Description    string         `json:"description"`
	VLANID         int            `json:"vlan_id,omitempty"`
}

func (n *Network) DisplayID() string {
	if n.AllocationMode != AllocationModeUnallocated {
		if n.DisplayName != "" {
			return fmt.Sprintf("%s (%s)", n.ID, n.DisplayName)
		}
		return n.ID
	}
	return fmt.Sprintf("%s (*)", n.ID)
}

func (n *Network) Compare(other Item) int {
	if other == nil {
		return 1
	}
	otherNet, ok := other.(*Network)
	if !ok {
		return cmp.Compare(n.DisplayID(), other.DisplayID())
	}

	_, ipNetLeft, err := net.ParseCIDR(n.ID)
	if err != nil {
		panic(err)
	}
	_, ipNetRight, err := net.ParseCIDR(otherNet.ID)
	if err != nil {
		panic(err)
	}

	ipLeft := ipNetLeft.IP.Mask(ipNetLeft.Mask).To16()
	ipRight := ipNetRight.IP.Mask(ipNetRight.Mask).To16()

	for i := range net.IPv6len {
		if ipLeft[i] < ipRight[i] {
			return -1
		} else if ipLeft[i] > ipRight[i] {
			return 1
		}
	}

	maskSizeLeft, _ := ipNetLeft.Mask.Size()
	maskSizeRight, _ := ipNetRight.Mask.Size()
	return maskSizeRight - maskSizeLeft
}

// ---------- IP ----------

// IP represents a reserved IP address within a host-pool network.
type IP struct {
	Base
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

func (i *IP) DisplayID() string {
	return fmt.Sprintf("%s (%s)", i.ID, i.DisplayName)
}

func (i *IP) Compare(other Item) int {
	if other == nil {
		return 1
	}
	otherIP, ok := other.(*IP)
	if !ok {
		return cmp.Compare(i.DisplayID(), other.DisplayID())
	}
	left, err := netip.ParseAddr(i.ID)
	if err != nil {
		panic(err)
	}
	right, err := netip.ParseAddr(otherIP.ID)
	if err != nil {
		panic(err)
	}
	return left.Compare(right)
}

// ---------- VLAN ----------

// VLAN represents a VLAN entry.
type VLAN struct {
	Base
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

func (v *VLAN) DisplayID() string {
	return fmt.Sprintf("%s (%s)", v.ID, v.DisplayName)
}

func (v *VLAN) Compare(other Item) int {
	if other == nil {
		return 1
	}
	otherVLAN, ok := other.(*VLAN)
	if !ok {
		return cmp.Compare(v.DisplayID(), other.DisplayID())
	}
	left, err := strconv.Atoi(v.ID)
	if err != nil {
		panic(err)
	}
	right, err := strconv.Atoi(otherVLAN.ID)
	if err != nil {
		panic(err)
	}
	return cmp.Compare(left, right)
}

// ---------- SSID ----------

// SSID represents a WiFi SSID entry.
type SSID struct {
	Base
	Description string `json:"description"`
}

func (s *SSID) DisplayID() string { return s.ID }

func (s *SSID) Compare(other Item) int {
	if other == nil {
		return 1
	}
	otherSSID, ok := other.(*SSID)
	if !ok {
		return cmp.Compare(s.DisplayID(), other.DisplayID())
	}
	return cmp.Compare(s.ID, otherSSID.ID)
}

// ---------- Zone ----------

// Zone represents a network security zone.
type Zone struct {
	Base
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	VLANIDs     []int  `json:"vlan_ids,omitempty"`
}

func (z *Zone) DisplayID() string { return z.DisplayName }

func (z *Zone) Compare(other Item) int {
	if other == nil {
		return 1
	}
	otherZone, ok := other.(*Zone)
	if !ok {
		return cmp.Compare(z.DisplayID(), other.DisplayID())
	}
	return CompareNaturalNumberOrder(z.DisplayName, otherZone.DisplayName)
}

// ---------- Equipment ----------

// Equipment represents a piece of network equipment.
type Equipment struct {
	Base
	DisplayName string `json:"display_name"`
	Model       string `json:"model"`
	Description string `json:"description"`
}

func (e *Equipment) DisplayID() string {
	return fmt.Sprintf("%s (%s)", e.DisplayName, e.Model)
}

func (e *Equipment) Compare(other Item) int {
	if other == nil {
		return 1
	}
	otherEquipment, ok := other.(*Equipment)
	if !ok {
		return cmp.Compare(e.DisplayID(), other.DisplayID())
	}
	return cmp.Compare(e.DisplayName, otherEquipment.DisplayName)
}

// ---------- Port ----------

// Port represents a port on a piece of equipment.
type Port struct {
	Base
	Name           string         `json:"name,omitempty"`
	PortType       string         `json:"port_type"`
	Speed          string         `json:"speed"`
	PoE            string         `json:"poe,omitempty"`
	LAGGroup       int            `json:"lag_group,omitempty"`
	LAGMode        string         `json:"lag_mode,omitempty"`
	NativeVLANID   int            `json:"native_vlan_id,omitempty"`
	TaggedVLANMode TaggedVLANMode `json:"tagged_vlan_mode,omitempty"`
	TaggedVLANIDs  []int          `json:"tagged_vlan_ids,omitempty"`
	ConnectedTo    string         `json:"connected_to,omitempty"`
	Description    string         `json:"description,omitempty"`
}

// Number returns the numeric port number from the ID.
func (p *Port) Number() int {
	value, err := strconv.Atoi(p.ID)
	if err != nil {
		panic(err)
	}
	return value
}

func (p *Port) DisplayID() string {
	typeSpeed := strings.TrimSpace(strings.Join([]string{p.PortType, p.PoE, p.Speed}, " "))
	typeSpeed = strings.Join(strings.Fields(typeSpeed), " ")
	if p.Name != "" {
		return fmt.Sprintf("%s: %s (%s)", p.ID, p.Name, typeSpeed)
	}
	return fmt.Sprintf("%s (%s)", p.ID, typeSpeed)
}

func (p *Port) Compare(other Item) int {
	if other == nil {
		return 1
	}
	otherPort, ok := other.(*Port)
	if !ok {
		return cmp.Compare(p.DisplayID(), other.DisplayID())
	}
	return cmp.Compare(p.Number(), otherPort.Number())
}
