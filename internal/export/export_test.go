package export

import (
	"strings"
	"testing"

	"github.com/plumber-cd/ez-ipam/internal/domain"
)

func TestRenderMarkdownEmpty(t *testing.T) {
	c := domain.NewCatalog()
	c.Put(&domain.StaticFolder{Base: domain.Base{ID: "Networks"}, Index: 0})
	c.Put(&domain.StaticFolder{Base: domain.Base{ID: "Zones"}, Index: 1})
	c.Put(&domain.StaticFolder{Base: domain.Base{ID: "VLANs"}, Index: 2})
	c.Put(&domain.StaticFolder{Base: domain.Base{ID: "WiFi SSIDs"}, Index: 3})
	c.Put(&domain.StaticFolder{Base: domain.Base{ID: "Equipment"}, Index: 4})
	c.Put(&domain.StaticFolder{Base: domain.Base{ID: "DNS"}, Index: 5})

	md, err := RenderMarkdown(c)
	if err != nil {
		t.Fatalf("RenderMarkdown() error: %v", err)
	}

	// Should contain the title.
	if !strings.Contains(md, "# EZ-IPAM") {
		t.Error("expected markdown to contain title")
	}

	// Always-present sections.
	for _, section := range []string{"## Overview", "## Networks", "## Detailed Networks"} {
		if !strings.Contains(md, section) {
			t.Errorf("expected markdown to contain %q", section)
		}
	}

	// Conditional sections should NOT appear when empty.
	for _, section := range []string{"## VLANs", "## DNS Records", "## WiFi SSIDs", "## Zones", "## Equipment"} {
		if strings.Contains(md, section) {
			t.Errorf("empty catalog should not contain %q", section)
		}
	}
}

func TestRenderMarkdownWithData(t *testing.T) {
	c := domain.NewCatalog()
	c.Put(&domain.StaticFolder{Base: domain.Base{ID: "Networks"}, Index: 0})
	c.Put(&domain.StaticFolder{Base: domain.Base{ID: "Zones"}, Index: 1})
	c.Put(&domain.StaticFolder{Base: domain.Base{ID: "VLANs"}, Index: 2})
	c.Put(&domain.StaticFolder{Base: domain.Base{ID: "WiFi SSIDs"}, Index: 3})
	c.Put(&domain.StaticFolder{Base: domain.Base{ID: "Equipment"}, Index: 4})
	c.Put(&domain.StaticFolder{Base: domain.Base{ID: "DNS"}, Index: 5})

	// Add a VLAN.
	c.Put(&domain.VLAN{
		Base:        domain.Base{ID: "100", ParentPath: "VLANs"},
		DisplayName: "Office",
		Description: "Main office VLAN",
	})

	// Add a network with allocation.
	net := &domain.Network{
		Base:           domain.Base{ID: "10.0.0.0/24", ParentPath: "Networks"},
		AllocationMode: domain.AllocationModeHosts,
		DisplayName:    "Office LAN",
		Description:    "Office network",
		VLANID:         100,
	}
	c.Put(net)

	// Add an IP.
	c.Put(&domain.IP{
		Base:        domain.Base{ID: "10.0.0.1", ParentPath: net.GetPath()},
		DisplayName: "Gateway",
		MACAddress:  "00:11:22:33:44:55",
		Description: "Default gateway",
	})

	// Add DNS alias record.
	c.Put(&domain.DNSRecord{
		Base:           domain.Base{ID: "gateway.example.com", ParentPath: "DNS"},
		ReservedIPPath: net.GetPath() + " -> 10.0.0.1",
		Description:    "Gateway alias",
	})
	c.Put(&domain.DNSRecord{
		Base:        domain.Base{ID: "mail.example.com", ParentPath: "DNS"},
		RecordType:  "MX",
		RecordValue: "10 mail.provider.home",
		Description: "Mail route",
	})

	// Add equipment with port.
	eq := &domain.Equipment{
		Base:        domain.Base{ID: "Switch-1", ParentPath: "Equipment"},
		DisplayName: "Switch-1",
		Model:       "SG300",
	}
	c.Put(eq)

	c.Put(&domain.Port{
		Base:     domain.Base{ID: "1", ParentPath: eq.GetPath()},
		Name:     "uplink",
		PortType: "RJ45",
		Speed:    "1G",
	})

	// Add SSID.
	c.Put(&domain.SSID{
		Base:        domain.Base{ID: "OfficeWiFi", ParentPath: "WiFi SSIDs"},
		Description: "Office wireless",
	})

	// Add zone.
	c.Put(&domain.Zone{
		Base:        domain.Base{ID: "DMZ", ParentPath: "Zones"},
		DisplayName: "DMZ",
		Description: "Demilitarized zone",
		VLANIDs:     []int{100},
	})

	md, err := RenderMarkdown(c)
	if err != nil {
		t.Fatalf("RenderMarkdown() error: %v", err)
	}

	// Check that key data appears in the output.
	checks := []string{
		"10.0.0.0/24",
		"Office LAN",
		"10.0.0.1",
		"Gateway",
		"DNS Records",
		"gateway.example.com",
		"`10.0.0.1` (Gateway `00:11:22:33:44:55`)",
		"`10 mail.provider.home`",
		"100",
		"Office",
		"Switch-1",
		"SG300",
		"OfficeWiFi",
		"DMZ",
	}
	for _, check := range checks {
		if !strings.Contains(md, check) {
			t.Errorf("expected markdown to contain %q", check)
		}
	}
}

func TestRenderMarkdownNetworkHierarchy(t *testing.T) {
	c := domain.NewCatalog()
	c.Put(&domain.StaticFolder{Base: domain.Base{ID: "Networks"}, Index: 0})
	c.Put(&domain.StaticFolder{Base: domain.Base{ID: "Zones"}, Index: 1})
	c.Put(&domain.StaticFolder{Base: domain.Base{ID: "VLANs"}, Index: 2})
	c.Put(&domain.StaticFolder{Base: domain.Base{ID: "WiFi SSIDs"}, Index: 3})
	c.Put(&domain.StaticFolder{Base: domain.Base{ID: "Equipment"}, Index: 4})
	c.Put(&domain.StaticFolder{Base: domain.Base{ID: "DNS"}, Index: 5})

	// Parent network allocated as subnets.
	parent := &domain.Network{
		Base:           domain.Base{ID: "10.0.0.0/16", ParentPath: "Networks"},
		AllocationMode: domain.AllocationModeSubnets,
		DisplayName:    "Main",
	}
	c.Put(parent)

	// Child networks.
	c.Put(&domain.Network{
		Base:           domain.Base{ID: "10.0.0.0/24", ParentPath: parent.GetPath()},
		AllocationMode: domain.AllocationModeHosts,
		DisplayName:    "Office",
	})
	c.Put(&domain.Network{
		Base: domain.Base{ID: "10.0.1.0/24", ParentPath: parent.GetPath()},
	})

	md, err := RenderMarkdown(c)
	if err != nil {
		t.Fatalf("RenderMarkdown() error: %v", err)
	}

	if !strings.Contains(md, "10.0.0.0/16") {
		t.Error("expected parent network in markdown")
	}
	if !strings.Contains(md, "10.0.0.0/24") {
		t.Error("expected child network in markdown")
	}
}
