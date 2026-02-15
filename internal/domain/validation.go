package domain

import (
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
)

// Validate for StaticFolder is always valid.
func (m *StaticFolder) Validate(_ *Catalog) error { return nil }

// Validate checks that the network has a valid CIDR.
func (n *Network) Validate(c *Catalog) error {
	if n.ID == "" {
		return fmt.Errorf("CIDR must be set for network")
	}
	_, ipNet, err := net.ParseCIDR(n.ID)
	if err != nil {
		return fmt.Errorf("invalid CIDR %q: %w", n.ID, err)
	}
	if ipNet.IP.String() != strings.Split(n.ID, "/")[0] {
		return fmt.Errorf("CIDR %q is a host address, not a network address (should be %s)", n.ID, ipNet.String())
	}
	prefix, err := netip.ParsePrefix(n.ID)
	if err != nil {
		return fmt.Errorf("invalid CIDR %q: %w", n.ID, err)
	}
	maxBits := prefix.Addr().BitLen()
	if prefix.Bits() >= maxBits {
		return fmt.Errorf("CIDR %q is a single address and cannot be used as a network", n.ID)
	}
	if n.ParentPath == "" {
		return fmt.Errorf("parent path must be set for network %s", n.ID)
	}

	if n.VLANID > 0 && c != nil {
		vlan := c.FindVLANByID(n.VLANID)
		if vlan == nil {
			return fmt.Errorf("VLAN ID %d not found for network %s", n.VLANID, n.ID)
		}
	}

	if n.AllocationMode != AllocationModeUnallocated {
		if strings.TrimSpace(n.DisplayName) == "" {
			return fmt.Errorf("display name must be set for allocated network %s", n.ID)
		}
	}

	return nil
}

// Validate checks IP invariants.
func (i *IP) Validate(c *Catalog) error {
	if i.ID == "" {
		return fmt.Errorf("IP address must be set")
	}
	if _, err := netip.ParseAddr(i.ID); err != nil {
		return fmt.Errorf("invalid IP address %q: %w", i.ID, err)
	}
	if err := ValidateHostname(i.DisplayName); err != nil {
		return fmt.Errorf("invalid display name for IP %s: %w", i.ID, err)
	}
	if i.ParentPath == "" {
		return fmt.Errorf("parent path must be set for IP %s", i.ID)
	}
	if strings.TrimSpace(i.MACAddress) != "" {
		if _, err := net.ParseMAC(i.MACAddress); err != nil {
			return fmt.Errorf("invalid MAC address %q for IP %s: %w", i.MACAddress, i.ID, err)
		}
	}
	return nil
}

// Validate checks DNSRecord invariants.
func (d *DNSRecord) Validate(c *Catalog) error {
	if err := ValidateHostname(d.ID); err != nil {
		return fmt.Errorf("invalid FQDN %q: %w", d.ID, err)
	}
	if d.ParentPath == "" {
		return fmt.Errorf("parent path must be set for DNS record %s", d.ID)
	}
	if c != nil {
		parent := c.Get(d.GetParentPath())
		if parent == nil {
			return fmt.Errorf("parent not found for DNS record %s", d.ID)
		}
		parentStatic, ok := parent.(*StaticFolder)
		if !ok || parentStatic.ID != FolderDNS {
			return fmt.Errorf("parent must be DNS for DNS record %s", d.ID)
		}
	}

	recordType := strings.TrimSpace(d.RecordType)
	recordValue := strings.TrimSpace(d.RecordValue)
	reservedIPPath := strings.TrimSpace(d.ReservedIPPath)
	hasRecord := recordType != "" || recordValue != ""
	hasAlias := reservedIPPath != ""
	if hasRecord == hasAlias {
		return fmt.Errorf("DNS record %s must be either a record (type+value) or an alias", d.ID)
	}
	if hasRecord {
		if recordType == "" || recordValue == "" {
			return fmt.Errorf("DNS record %s must set both record type and value", d.ID)
		}
		return nil
	}

	if c != nil {
		item := c.Get(reservedIPPath)
		if item == nil {
			return fmt.Errorf("reserved IP reference %q not found for DNS record %s", reservedIPPath, d.ID)
		}
		if _, ok := item.(*IP); !ok {
			return fmt.Errorf("reserved IP reference %q is not an IP for DNS record %s", reservedIPPath, d.ID)
		}
	}
	return nil
}

// Validate checks VLAN invariants.
func (v *VLAN) Validate(c *Catalog) error {
	id, err := strconv.Atoi(v.ID)
	if err != nil {
		return fmt.Errorf("invalid VLAN ID %q: must be an integer", v.ID)
	}
	if id < 1 || id > 4094 {
		return fmt.Errorf("invalid VLAN ID %d: must be in range 1-4094", id)
	}
	if strings.TrimSpace(v.DisplayName) == "" {
		return fmt.Errorf("display name must be set for VLAN=%s", v.GetPath())
	}
	if v.ParentPath == "" {
		return fmt.Errorf("parent path must be set for VLAN=%s", v.GetPath())
	}
	if c != nil {
		parent := c.Get(v.GetParentPath())
		if parent == nil {
			return fmt.Errorf("parent not found for VLAN=%s", v.GetPath())
		}
		parentStatic, ok := parent.(*StaticFolder)
		if !ok || parentStatic.ID != FolderVLANs {
			return fmt.Errorf("parent must be VLANs for VLAN=%s", v.GetPath())
		}
	}
	return nil
}

// Validate checks SSID invariants.
func (s *SSID) Validate(c *Catalog) error {
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("SSID ID must be set for SSID=%s", s.GetPath())
	}
	if s.ParentPath == "" {
		return fmt.Errorf("parent path must be set for SSID=%s", s.GetPath())
	}
	if c != nil {
		parent := c.Get(s.GetParentPath())
		if parent == nil {
			return fmt.Errorf("parent not found for SSID=%s", s.GetPath())
		}
		parentStatic, ok := parent.(*StaticFolder)
		if !ok || parentStatic.ID != FolderSSIDs {
			return fmt.Errorf("parent must be WiFi SSIDs for SSID=%s", s.GetPath())
		}
	}
	return nil
}

// Normalize trims and deduplicates Zone fields in place.
// Call before Validate to ensure consistent state.
func (z *Zone) Normalize() {
	z.DisplayName = strings.TrimSpace(z.DisplayName)
	slices.Sort(z.VLANIDs)
	z.VLANIDs = slices.Compact(z.VLANIDs)
}

// Validate checks Zone invariants. Call Normalize first.
func (z *Zone) Validate(c *Catalog) error {
	if strings.TrimSpace(z.DisplayName) == "" {
		return fmt.Errorf("display name must be set for Zone=%s", z.GetPath())
	}
	if z.ParentPath == "" {
		return fmt.Errorf("parent path must be set for Zone=%s", z.GetPath())
	}
	if c != nil {
		parent := c.Get(z.GetParentPath())
		if parent == nil {
			return fmt.Errorf("parent not found for Zone=%s", z.GetPath())
		}
		parentStatic, ok := parent.(*StaticFolder)
		if !ok || parentStatic.ID != FolderZones {
			return fmt.Errorf("parent must be Zones for Zone=%s", z.GetPath())
		}
	}

	for _, vlanID := range z.VLANIDs {
		if vlanID < 1 || vlanID > 4094 {
			return fmt.Errorf("invalid VLAN ID %d for Zone=%s", vlanID, z.GetPath())
		}
		if c != nil && c.FindVLANByID(vlanID) == nil {
			return fmt.Errorf("VLAN ID %d not found for Zone=%s", vlanID, z.GetPath())
		}
	}

	return nil
}

// Validate checks Equipment invariants.
func (e *Equipment) Validate(c *Catalog) error {
	if strings.TrimSpace(e.DisplayName) == "" {
		return fmt.Errorf("display name must be set for equipment")
	}
	if strings.TrimSpace(e.Model) == "" {
		return fmt.Errorf("model must be set for equipment %s", e.DisplayName)
	}
	if e.ParentPath == "" {
		return fmt.Errorf("parent path must be set for equipment %s", e.DisplayName)
	}
	if c != nil {
		parent := c.Get(e.GetParentPath())
		if parent == nil {
			return fmt.Errorf("parent not found for equipment %s", e.DisplayName)
		}
		parentStatic, ok := parent.(*StaticFolder)
		if !ok || parentStatic.ID != FolderEquipment {
			return fmt.Errorf("parent must be Equipment for equipment %s", e.DisplayName)
		}
	}
	return nil
}

// Validate checks Port invariants.
func (p *Port) Validate(c *Catalog) error {
	if _, err := strconv.Atoi(p.ID); err != nil {
		return fmt.Errorf("port number must be a positive integer, got %q", p.ID)
	}
	if p.Number() < 1 {
		return fmt.Errorf("port number must be a positive integer, got %d", p.Number())
	}
	if p.ParentPath == "" {
		return fmt.Errorf("parent path must be set for port %s", p.ID)
	}
	if c != nil {
		parent := c.Get(p.GetParentPath())
		if parent == nil {
			return fmt.Errorf("parent not found for port %s", p.ID)
		}
		_, ok := parent.(*Equipment)
		if !ok {
			return fmt.Errorf("parent must be Equipment for port %s", p.ID)
		}
	}
	if strings.TrimSpace(p.PortType) == "" {
		return fmt.Errorf("port type must be set for port %s", p.ID)
	}
	if strings.TrimSpace(p.Speed) == "" {
		return fmt.Errorf("speed must be set for port %s", p.ID)
	}
	if p.Disabled {
		if strings.TrimSpace(p.Name) != "" {
			return fmt.Errorf("name must be empty for disabled port %s", p.ID)
		}
		if p.LAGGroup != 0 {
			return fmt.Errorf("LAG group must be empty for disabled port %s", p.ID)
		}
		if strings.TrimSpace(p.LAGMode) != "" {
			return fmt.Errorf("LAG mode must be empty for disabled port %s", p.ID)
		}
		if p.NativeVLANID != 0 {
			return fmt.Errorf("native VLAN ID must be empty for disabled port %s", p.ID)
		}
		if p.TaggedVLANMode != TaggedVLANModeNone {
			return fmt.Errorf("tagged VLAN mode must be empty for disabled port %s", p.ID)
		}
		if len(p.TaggedVLANIDs) > 0 {
			return fmt.Errorf("tagged VLAN IDs must be empty for disabled port %s", p.ID)
		}
		if strings.TrimSpace(p.ConnectedTo) != "" {
			return fmt.Errorf("connected_to must be empty for disabled port %s", p.ID)
		}
		return nil
	}

	if p.LAGGroup > 0 && strings.TrimSpace(p.LAGMode) == "" {
		return fmt.Errorf("LAG mode must be set when LAG group is specified for port %s", p.ID)
	}
	if strings.TrimSpace(p.LAGMode) != "" && p.LAGGroup < 1 {
		return fmt.Errorf("LAG group must be set when LAG mode is specified for port %s", p.ID)
	}

	if p.NativeVLANID > 0 && c != nil {
		if c.FindVLANByID(p.NativeVLANID) == nil {
			return fmt.Errorf("native VLAN ID %d not found for port %s", p.NativeVLANID, p.ID)
		}
	}

	switch p.TaggedVLANMode {
	case TaggedVLANModeNone, TaggedVLANModeAllowAll, TaggedVLANModeBlockAll:
		if len(p.TaggedVLANIDs) > 0 {
			return fmt.Errorf("tagged VLAN IDs must be empty when mode is %q for port %s", p.TaggedVLANMode, p.ID)
		}
	case TaggedVLANModeCustom:
		if len(p.TaggedVLANIDs) == 0 {
			return fmt.Errorf("tagged VLAN IDs must be set when mode is Custom for port %s", p.ID)
		}
		for _, vlanID := range p.TaggedVLANIDs {
			if vlanID < 1 || vlanID > 4094 {
				return fmt.Errorf("invalid tagged VLAN ID %d for port %s", vlanID, p.ID)
			}
			if c != nil && c.FindVLANByID(vlanID) == nil {
				return fmt.Errorf("tagged VLAN ID %d not found for port %s", vlanID, p.ID)
			}
		}
	default:
		return fmt.Errorf("invalid tagged VLAN mode %q for port %s", p.TaggedVLANMode, p.ID)
	}

	if p.ConnectedTo != "" && c != nil {
		target := c.Get(p.ConnectedTo)
		if target == nil {
			return fmt.Errorf("connected_to target %q not found for port %s", p.ConnectedTo, p.ID)
		}
		if _, ok := target.(*Port); !ok {
			return fmt.Errorf("connected_to target %q is not a port for port %s", p.ConnectedTo, p.ID)
		}
	}

	return nil
}
