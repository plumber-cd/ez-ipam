package domain

import (
	"math/big"
	"net/netip"
	"testing"
)

// ---------- helpers.go ----------

func TestParsePositiveIntID(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"1", 1, false},
		{"42", 42, false},
		{" 7 ", 7, false},
		{"0", 0, true},
		{"-1", 0, true},
		{"abc", 0, true},
		{"", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParsePositiveIntID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePositiveIntID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParsePositiveIntID(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseVLANListCSV(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []int
		wantErr bool
	}{
		{"empty", "", nil, false},
		{"single", "100", []int{100}, false},
		{"multiple", "10,20,30", []int{10, 20, 30}, false},
		{"whitespace", " 10 , 20 , 30 ", []int{10, 20, 30}, false},
		{"trailing_comma", "10,20,", []int{10, 20}, false},
		{"out_of_range_high", "5000", nil, true},
		{"out_of_range_low", "0", nil, true},
		{"non_numeric", "abc", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVLANListCSV(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVLANListCSV(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != len(tt.want) {
				t.Errorf("ParseVLANListCSV(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseVLANListCSV(%q)[%d] = %d, want %d", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseOptionalVLANID(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"", 0, false},
		{" ", 0, false},
		{"100", 100, false},
		{"4094", 4094, false},
		{"0", 0, true},
		{"4095", 0, true},
		{"abc", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseOptionalVLANID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseOptionalVLANID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseOptionalVLANID(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseOptionalIntField(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"", 0, false},
		{" ", 0, false},
		{"42", 42, false},
		{"-1", -1, false},
		{"abc", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseOptionalIntField(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseOptionalIntField(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseOptionalIntField(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTaggedMode(t *testing.T) {
	tests := []struct {
		input string
		want  TaggedVLANMode
	}{
		{"AllowAll", TaggedVLANModeAllowAll},
		{"allowall", TaggedVLANModeAllowAll},
		{"BlockAll", TaggedVLANModeBlockAll},
		{"Custom", TaggedVLANModeCustom},
		{"", TaggedVLANModeNone},
		{"None", TaggedVLANModeNone},
		{"garbage", TaggedVLANModeNone},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseTaggedMode(tt.input)
			if got != tt.want {
				t.Errorf("ParseTaggedMode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCompareNaturalNumberOrder(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"abc", "abc", 0},
		{"a1", "a2", -1},
		{"a2", "a10", -1},
		{"a10", "a2", 1},
		{"port1", "port10", -1},
		{"port9", "port10", -1},
		{"port10", "port10", 0},
		{"", "", 0},
		{"a", "b", -1},
		{"b", "a", 1},
		{"file001", "file01", 1}, // more leading zeros = larger by tie-breaking rule
	}
	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := CompareNaturalNumberOrder(tt.a, tt.b)
			if (got < 0 && tt.want >= 0) || (got > 0 && tt.want <= 0) || (got == 0 && tt.want != 0) {
				t.Errorf("CompareNaturalNumberOrder(%q, %q) = %d, want sign of %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCIDRToIdentifier(t *testing.T) {
	tests := []struct {
		cidr    string
		want    string
		wantErr bool
	}{
		{"10.0.0.0/24", "0a000000_24", false},
		{"192.168.1.0/24", "c0a80100_24", false},
		{"invalid", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.cidr, func(t *testing.T) {
			got, err := CIDRToIdentifier(tt.cidr)
			if (err != nil) != tt.wantErr {
				t.Errorf("CIDRToIdentifier(%q) error = %v, wantErr %v", tt.cidr, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("CIDRToIdentifier(%q) = %q, want %q", tt.cidr, got, tt.want)
			}
		})
	}
}

func TestIPToIdentifier(t *testing.T) {
	tests := []struct {
		ip      string
		want    string
		wantErr bool
	}{
		{"10.0.0.1", "0a000001", false},
		{"192.168.1.1", "c0a80101", false},
		{"invalid", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			got, err := IPToIdentifier(tt.ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("IPToIdentifier(%q) error = %v, wantErr %v", tt.ip, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IPToIdentifier(%q) = %q, want %q", tt.ip, got, tt.want)
			}
		})
	}
}

// ---------- network_ops.go ----------

func TestSplitNetwork(t *testing.T) {
	tests := []struct {
		name      string
		cidr      string
		newPrefix int
		wantCount int
		wantErr   bool
	}{
		{"split_/24_to_/25", "10.0.0.0/24", 25, 2, false},
		{"split_/24_to_/26", "10.0.0.0/24", 26, 4, false},
		{"split_/16_to_/24", "10.0.0.0/16", 24, 256, false},
		{"prefix_too_small", "10.0.0.0/24", 24, 0, true},
		{"prefix_too_large", "10.0.0.0/24", 33, 0, true},
		{"auto_prefix_zero", "10.0.0.0/24", 0, 2, false},
		{"invalid_cidr", "invalid", 25, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SplitNetwork(tt.cidr, tt.newPrefix)
			if (err != nil) != tt.wantErr {
				t.Errorf("SplitNetwork(%q, %d) error = %v, wantErr %v", tt.cidr, tt.newPrefix, err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != tt.wantCount {
				t.Errorf("SplitNetwork(%q, %d) returned %d subnets, want %d", tt.cidr, tt.newPrefix, len(got), tt.wantCount)
			}
		})
	}
}

func TestSummarizeCIDRs(t *testing.T) {
	tests := []struct {
		name    string
		cidrs   []string
		want    string
		wantErr bool
	}{
		{"two_halves", []string{"10.0.0.0/25", "10.0.0.128/25"}, "10.0.0.0/24", false},
		{"four_quarters", []string{"10.0.0.0/26", "10.0.0.64/26", "10.0.0.128/26", "10.0.0.192/26"}, "10.0.0.0/24", false},
		{"empty", nil, "", true},
		{"non_contiguous", []string{"10.0.0.0/25", "10.0.1.0/25"}, "", true},
		{"not_power_of_two", []string{"10.0.0.0/26", "10.0.0.64/26", "10.0.0.128/26"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SummarizeCIDRs(tt.cidrs)
			if (err != nil) != tt.wantErr {
				t.Errorf("SummarizeCIDRs(%v) error = %v, wantErr %v", tt.cidrs, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SummarizeCIDRs(%v) = %q, want %q", tt.cidrs, got, tt.want)
			}
		})
	}
}

func TestFindSummarizableRange(t *testing.T) {
	cidrs := []string{"10.0.0.0/26", "10.0.0.64/26", "10.0.0.128/26", "10.0.0.192/26"}

	ok, sub, summary := FindSummarizableRange(cidrs, 0)
	if !ok {
		t.Fatal("expected summarizable range")
	}
	if summary != "10.0.0.0/24" {
		t.Errorf("expected 10.0.0.0/24, got %s", summary)
	}
	if len(sub) != 4 {
		t.Errorf("expected 4 subnets, got %d", len(sub))
	}

	// Try with a pair from the middle.
	ok2, sub2, summary2 := FindSummarizableRange(cidrs, 1)
	if !ok2 {
		t.Fatal("expected summarizable range from index 1")
	}
	_ = sub2
	_ = summary2
}

func TestIPToBigIntAndBack(t *testing.T) {
	addr := netip.MustParseAddr("192.168.1.1")
	bigI := IPToBigInt(addr)
	back := BigIntToAddr(bigI, true)
	if back != addr {
		t.Errorf("round-trip failed: got %s, want %s", back, addr)
	}

	addr6 := netip.MustParseAddr("::1")
	bigI6 := IPToBigInt(addr6)
	if bigI6.Cmp(big.NewInt(1)) != 0 {
		t.Errorf("::1 should be 1, got %s", bigI6.String())
	}
	back6 := BigIntToAddr(bigI6, false)
	if back6 != addr6 {
		t.Errorf("round-trip IPv6 failed: got %s, want %s", back6, addr6)
	}
}

func TestLastAddr(t *testing.T) {
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	last := LastAddr(prefix)
	if last != netip.MustParseAddr("10.0.0.255") {
		t.Errorf("LastAddr(10.0.0.0/24) = %s, want 10.0.0.255", last)
	}

	prefix16 := netip.MustParsePrefix("10.0.0.0/16")
	last16 := LastAddr(prefix16)
	if last16 != netip.MustParseAddr("10.0.255.255") {
		t.Errorf("LastAddr(10.0.0.0/16) = %s, want 10.0.255.255", last16)
	}
}

// ---------- catalog.go ----------

func TestCatalogBasics(t *testing.T) {
	c := NewCatalog()
	if len(c.All()) != 0 {
		t.Fatalf("new catalog should be empty, got %d", len(c.All()))
	}

	sf := &StaticFolder{Base: Base{ID: "Networks"}, Index: 0}
	c.Put(sf)
	if len(c.All()) != 1 {
		t.Fatalf("expected 1 item, got %d", len(c.All()))
	}
	if c.Get("Networks") == nil {
		t.Fatal("expected to find Networks")
	}
}

func TestCatalogGetChildren(t *testing.T) {
	c := NewCatalog()
	sf := &StaticFolder{Base: Base{ID: "Networks"}, Index: 0}
	c.Put(sf)

	n1 := &Network{Base: Base{ID: "10.0.0.0/24", ParentPath: "Networks"}}
	n2 := &Network{Base: Base{ID: "192.168.1.0/24", ParentPath: "Networks"}}
	c.Put(n1)
	c.Put(n2)

	children := c.GetChildren(sf)
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
	// Children should be sorted by CIDR. Unallocated networks display as "CIDR (*)".
	if children[0].RawID() != "10.0.0.0/24" {
		t.Errorf("expected first child to be 10.0.0.0/24, got %s", children[0].RawID())
	}
}

func TestCatalogDelete(t *testing.T) {
	c := NewCatalog()
	sf := &StaticFolder{Base: Base{ID: "Networks"}, Index: 0}
	c.Put(sf)

	parent := &Network{Base: Base{ID: "10.0.0.0/24", ParentPath: "Networks"}, AllocationMode: AllocationModeSubnets, DisplayName: "Test"}
	c.Put(parent)

	child := &Network{Base: Base{ID: "10.0.0.0/25", ParentPath: parent.GetPath()}}
	c.Put(child)

	if len(c.All()) != 3 {
		t.Fatalf("expected 3 items, got %d", len(c.All()))
	}

	// Deleting parent should cascade to child.
	c.Delete(parent)
	if len(c.All()) != 1 {
		t.Fatalf("expected 1 item after cascade delete, got %d", len(c.All()))
	}
	if c.Get(parent.GetPath()) != nil {
		t.Error("parent should be deleted")
	}
	if c.Get(child.GetPath()) != nil {
		t.Error("child should be cascade deleted")
	}
}

func TestCatalogFindVLANByID(t *testing.T) {
	c := NewCatalog()
	sf := &StaticFolder{Base: Base{ID: "VLANs"}, Index: 2}
	c.Put(sf)

	v := &VLAN{Base: Base{ID: "100", ParentPath: "VLANs"}, DisplayName: "Test VLAN"}
	c.Put(v)

	found := c.FindVLANByID(100)
	if found == nil {
		t.Fatal("expected to find VLAN 100")
	}
	if found.ID != "100" {
		t.Errorf("expected ID=100, got %s", found.ID)
	}

	if c.FindVLANByID(200) != nil {
		t.Error("should not find non-existent VLAN")
	}
}

func TestCatalogRenderVLANID(t *testing.T) {
	c := NewCatalog()
	sf := &StaticFolder{Base: Base{ID: "VLANs"}, Index: 2}
	c.Put(sf)

	v := &VLAN{Base: Base{ID: "100", ParentPath: "VLANs"}, DisplayName: "Office"}
	c.Put(v)

	if got := c.RenderVLANID(0); got != "<none>" {
		t.Errorf("RenderVLANID(0) = %q, want <none>", got)
	}
	if got := c.RenderVLANID(100); got != "100 (Office)" {
		t.Errorf("RenderVLANID(100) = %q, want \"100 (Office)\"", got)
	}
	if got := c.RenderVLANID(200); got != "200" {
		t.Errorf("RenderVLANID(200) = %q, want \"200\"", got)
	}
}

func TestCatalogGetByParentAndDisplayID(t *testing.T) {
	c := NewCatalog()
	sf := &StaticFolder{Base: Base{ID: "Networks"}, Index: 0}
	c.Put(sf)

	n := &Network{Base: Base{ID: "10.0.0.0/24", ParentPath: "Networks"}}
	c.Put(n)

	// Unallocated networks display as "10.0.0.0/24 (*)".
	found := c.GetByParentAndDisplayID(sf, "10.0.0.0/24 (*)")
	if found == nil {
		t.Fatal("expected to find network by display ID")
	}

	// Top-level search (parent nil).
	foundTop := c.GetByParentAndDisplayID(nil, "Networks")
	if foundTop == nil {
		t.Fatal("expected to find Networks at top level")
	}
}

// ---------- validation.go ----------

func TestNetworkValidate(t *testing.T) {
	c := NewCatalog()
	sf := &StaticFolder{Base: Base{ID: "Networks"}, Index: 0}
	c.Put(sf)

	tests := []struct {
		name    string
		network *Network
		wantErr bool
	}{
		{"valid", &Network{Base: Base{ID: "10.0.0.0/24", ParentPath: "Networks"}}, false},
		{"empty_id", &Network{Base: Base{ID: "", ParentPath: "Networks"}}, true},
		{"invalid_cidr", &Network{Base: Base{ID: "garbage", ParentPath: "Networks"}}, true},
		{"host_address", &Network{Base: Base{ID: "10.0.0.1/24", ParentPath: "Networks"}}, true},
		{"single_address_v4", &Network{Base: Base{ID: "10.0.0.0/32", ParentPath: "Networks"}}, true},
		{"no_parent", &Network{Base: Base{ID: "10.0.0.0/24"}}, true},
		{"allocated_no_name", &Network{Base: Base{ID: "10.0.0.0/24", ParentPath: "Networks"}, AllocationMode: AllocationModeHosts}, true},
		{"allocated_with_name", &Network{Base: Base{ID: "10.0.0.0/24", ParentPath: "Networks"}, AllocationMode: AllocationModeHosts, DisplayName: "Test"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.network.Validate(c)
			if (err != nil) != tt.wantErr {
				t.Errorf("Network.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIPValidate(t *testing.T) {
	tests := []struct {
		name    string
		ip      *IP
		wantErr bool
	}{
		{"valid", &IP{Base: Base{ID: "10.0.0.1", ParentPath: "p"}, DisplayName: "Host"}, false},
		{"empty_id", &IP{Base: Base{ID: "", ParentPath: "p"}, DisplayName: "Host"}, true},
		{"invalid_ip", &IP{Base: Base{ID: "garbage", ParentPath: "p"}, DisplayName: "Host"}, true},
		{"no_name", &IP{Base: Base{ID: "10.0.0.1", ParentPath: "p"}}, true},
		{"no_parent", &IP{Base: Base{ID: "10.0.0.1"}, DisplayName: "Host"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ip.Validate(nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("IP.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVLANValidate(t *testing.T) {
	c := NewCatalog()
	sf := &StaticFolder{Base: Base{ID: "VLANs"}, Index: 2}
	c.Put(sf)

	tests := []struct {
		name    string
		vlan    *VLAN
		wantErr bool
	}{
		{"valid", &VLAN{Base: Base{ID: "100", ParentPath: "VLANs"}, DisplayName: "Test"}, false},
		{"invalid_id", &VLAN{Base: Base{ID: "abc", ParentPath: "VLANs"}, DisplayName: "Test"}, true},
		{"out_of_range", &VLAN{Base: Base{ID: "5000", ParentPath: "VLANs"}, DisplayName: "Test"}, true},
		{"no_name", &VLAN{Base: Base{ID: "100", ParentPath: "VLANs"}}, true},
		{"no_parent", &VLAN{Base: Base{ID: "100"}, DisplayName: "Test"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.vlan.Validate(c)
			if (err != nil) != tt.wantErr {
				t.Errorf("VLAN.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEquipmentValidate(t *testing.T) {
	c := NewCatalog()
	sf := &StaticFolder{Base: Base{ID: "Equipment"}, Index: 4}
	c.Put(sf)

	tests := []struct {
		name    string
		equip   *Equipment
		wantErr bool
	}{
		{"valid", &Equipment{Base: Base{ID: "Switch", ParentPath: "Equipment"}, DisplayName: "Switch", Model: "SG300"}, false},
		{"no_name", &Equipment{Base: Base{ID: "Switch", ParentPath: "Equipment"}, Model: "SG300"}, true},
		{"no_model", &Equipment{Base: Base{ID: "Switch", ParentPath: "Equipment"}, DisplayName: "Switch"}, true},
		{"no_parent", &Equipment{Base: Base{ID: "Switch"}, DisplayName: "Switch", Model: "SG300"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.equip.Validate(c)
			if (err != nil) != tt.wantErr {
				t.Errorf("Equipment.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPortValidate(t *testing.T) {
	c := NewCatalog()
	sf := &StaticFolder{Base: Base{ID: "Equipment"}, Index: 4}
	c.Put(sf)
	eq := &Equipment{Base: Base{ID: "Switch", ParentPath: "Equipment"}, DisplayName: "Switch", Model: "SG300"}
	c.Put(eq)

	tests := []struct {
		name    string
		port    *Port
		wantErr bool
	}{
		{"valid", &Port{Base: Base{ID: "1", ParentPath: eq.GetPath()}, PortType: "RJ45", Speed: "1G"}, false},
		{"valid_disabled_physical_only", &Port{Base: Base{ID: "2", ParentPath: eq.GetPath()}, Disabled: true, PortType: "RJ45", Speed: "1G", PoE: "PoE+", DestinationNotes: "catalogued only"}, false},
		{"disabled_with_name", &Port{Base: Base{ID: "3", ParentPath: eq.GetPath()}, Disabled: true, Name: "uplink", PortType: "RJ45", Speed: "1G"}, true},
		{"disabled_with_tagged_mode", &Port{Base: Base{ID: "4", ParentPath: eq.GetPath()}, Disabled: true, PortType: "RJ45", Speed: "1G", TaggedVLANMode: TaggedVLANModeAllowAll}, true},
		{"invalid_id", &Port{Base: Base{ID: "abc", ParentPath: eq.GetPath()}, PortType: "RJ45", Speed: "1G"}, true},
		{"zero_id", &Port{Base: Base{ID: "0", ParentPath: eq.GetPath()}, PortType: "RJ45", Speed: "1G"}, true},
		{"no_type", &Port{Base: Base{ID: "1", ParentPath: eq.GetPath()}, Speed: "1G"}, true},
		{"no_speed", &Port{Base: Base{ID: "1", ParentPath: eq.GetPath()}, PortType: "RJ45"}, true},
		{"lag_no_mode", &Port{Base: Base{ID: "1", ParentPath: eq.GetPath()}, PortType: "RJ45", Speed: "1G", LAGGroup: 1}, true},
		{"mode_no_lag", &Port{Base: Base{ID: "1", ParentPath: eq.GetPath()}, PortType: "RJ45", Speed: "1G", LAGMode: "802.3ad"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.port.Validate(c)
			if (err != nil) != tt.wantErr {
				t.Errorf("Port.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ---------- types.go ----------

func TestNetworkCompare(t *testing.T) {
	n1 := &Network{Base: Base{ID: "10.0.0.0/24"}}
	n2 := &Network{Base: Base{ID: "10.0.1.0/24"}}
	n3 := &Network{Base: Base{ID: "10.0.0.0/24"}}

	if n1.Compare(n2) >= 0 {
		t.Error("10.0.0.0/24 should sort before 10.0.1.0/24")
	}
	if n2.Compare(n1) <= 0 {
		t.Error("10.0.1.0/24 should sort after 10.0.0.0/24")
	}
	if n1.Compare(n3) != 0 {
		t.Error("same CIDRs should compare equal")
	}
}

func TestPortDisplayID(t *testing.T) {
	// Port DisplayID format: "ID (type poe speed)" or "ID: name (type poe speed)".
	p := &Port{Base: Base{ID: "1"}, PortType: "RJ45", Speed: "1G"}
	if got := p.DisplayID(); got != "1 (RJ45 1G)" {
		t.Errorf("DisplayID() = %q, want \"1 (RJ45 1G)\"", got)
	}

	p2 := &Port{Base: Base{ID: "1"}, Name: "uplink", PortType: "RJ45", Speed: "1G"}
	if got := p2.DisplayID(); got != "1: uplink (RJ45 1G)" {
		t.Errorf("DisplayID() = %q, want \"1: uplink (RJ45 1G)\"", got)
	}

	p3 := &Port{Base: Base{ID: "2"}, PortType: "SFP+", PoE: "PoE+", Speed: "10G"}
	if got := p3.DisplayID(); got != "2 (SFP+ PoE+ 10G)" {
		t.Errorf("DisplayID() = %q, want \"2 (SFP+ PoE+ 10G)\"", got)
	}

	p4 := &Port{Base: Base{ID: "3"}, PortType: "RJ45", Speed: "1G", Disabled: true}
	if got := p4.DisplayID(); got != "3 (disabled) (RJ45 1G)" {
		t.Errorf("DisplayID() = %q, want \"3 (disabled) (RJ45 1G)\"", got)
	}
}

func TestBasePath(t *testing.T) {
	b := Base{ID: "child", ParentPath: "parent"}
	if b.GetPath() != "parent -> child" {
		t.Errorf("GetPath() = %q, want \"parent -> child\"", b.GetPath())
	}

	root := Base{ID: "root"}
	if root.GetPath() != "root" {
		t.Errorf("GetPath() = %q, want \"root\"", root.GetPath())
	}
}
