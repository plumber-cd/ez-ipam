package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/plumber-cd/ez-ipam/internal/domain"
)

func Test_safeFileNameSegment(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"Hello World", "Hello_World"},
		{"a/b\\c", "a_b_c"},
		{"  spaces  ", "spaces"},
		{"", "item"},
		{"   ", "item"},
		{"my-router_1", "my-router_1"},
		{"special!@#$%chars", "special_____chars"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := safeFileNameSegment(tt.input)
			if got != tt.want {
				t.Errorf("safeFileNameSegment(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	// Build a catalog.
	catalog := domain.NewCatalog()
	catalog.Put(&domain.StaticFolder{Base: domain.Base{ID: "Networks"}, Index: 0})
	catalog.Put(&domain.StaticFolder{Base: domain.Base{ID: "Zones"}, Index: 1})
	catalog.Put(&domain.StaticFolder{Base: domain.Base{ID: "VLANs"}, Index: 2})
	catalog.Put(&domain.StaticFolder{Base: domain.Base{ID: "WiFi SSIDs"}, Index: 3})
	catalog.Put(&domain.StaticFolder{Base: domain.Base{ID: "Equipment"}, Index: 4})

	// Add a network.
	n := &domain.Network{
		Base:           domain.Base{ID: "10.0.0.0/24", ParentPath: "Networks"},
		AllocationMode: domain.AllocationModeHosts,
		DisplayName:    "Office LAN",
		Description:    "Main office network",
		VLANID:         100,
	}
	catalog.Put(n)

	// Add a VLAN.
	v := &domain.VLAN{
		Base:        domain.Base{ID: "100", ParentPath: "VLANs"},
		DisplayName: "Office",
		Description: "Office VLAN",
	}
	catalog.Put(v)

	// Add an IP.
	ip := &domain.IP{
		Base:        domain.Base{ID: "10.0.0.1", ParentPath: n.GetPath()},
		DisplayName: "Gateway",
		Description: "Default gateway",
	}
	catalog.Put(ip)

	// Add an SSID.
	ssid := &domain.SSID{
		Base:        domain.Base{ID: "OfficeWiFi", ParentPath: "WiFi SSIDs"},
		Description: "Office wireless",
	}
	catalog.Put(ssid)

	// Add a zone.
	zone := &domain.Zone{
		Base:        domain.Base{ID: "DMZ", ParentPath: "Zones"},
		DisplayName: "DMZ",
		Description: "Demilitarized zone",
		VLANIDs:     []int{100},
	}
	catalog.Put(zone)

	// Add equipment and port.
	eq := &domain.Equipment{
		Base:        domain.Base{ID: "Switch-1", ParentPath: "Equipment"},
		DisplayName: "Switch-1",
		Model:       "SG300",
		Description: "Core switch",
	}
	catalog.Put(eq)

	port := &domain.Port{
		Base:     domain.Base{ID: "1", ParentPath: eq.GetPath()},
		PortType: "RJ45",
		Speed:    "1G",
	}
	catalog.Put(port)

	// Save.
	if err := Save(dir, catalog); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify data dir exists.
	dataDir := filepath.Join(dir, DataDirName)
	if _, err := os.Stat(dataDir); err != nil {
		t.Fatalf("data dir missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, networksDirName)); err != nil {
		t.Fatalf("networks dir missing: %v", err)
	}

	// Load back.
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Verify static folders exist.
	if loaded.GetByParentAndDisplayID(nil, "Networks") == nil {
		t.Error("Networks folder not loaded")
	}
	if loaded.GetByParentAndDisplayID(nil, "VLANs") == nil {
		t.Error("VLANs folder not loaded")
	}

	// Verify network loaded.
	loadedNet := loaded.Get(n.GetPath())
	if loadedNet == nil {
		t.Fatal("network not loaded")
	}
	loadedNetwork, ok := loadedNet.(*domain.Network)
	if !ok {
		t.Fatal("expected *domain.Network")
	}
	if loadedNetwork.DisplayName != "Office LAN" {
		t.Errorf("network display name = %q, want \"Office LAN\"", loadedNetwork.DisplayName)
	}
	if loadedNetwork.VLANID != 100 {
		t.Errorf("network VLAN ID = %d, want 100", loadedNetwork.VLANID)
	}

	// Verify VLAN loaded.
	loadedVLAN := loaded.FindVLANByID(100)
	if loadedVLAN == nil {
		t.Fatal("VLAN 100 not loaded")
	}

	// Verify IP loaded.
	loadedIP := loaded.Get(ip.GetPath())
	if loadedIP == nil {
		t.Fatal("IP not loaded")
	}

	// Verify SSID loaded.
	ssidFolder := loaded.GetByParentAndDisplayID(nil, "WiFi SSIDs")
	ssidChildren := loaded.GetChildren(ssidFolder)
	if len(ssidChildren) != 1 {
		t.Fatalf("expected 1 SSID, got %d", len(ssidChildren))
	}

	// Verify zone loaded.
	zoneFolder := loaded.GetByParentAndDisplayID(nil, "Zones")
	zoneChildren := loaded.GetChildren(zoneFolder)
	if len(zoneChildren) != 1 {
		t.Fatalf("expected 1 zone, got %d", len(zoneChildren))
	}

	// Verify equipment loaded.
	eqFolder := loaded.GetByParentAndDisplayID(nil, "Equipment")
	eqChildren := loaded.GetChildren(eqFolder)
	if len(eqChildren) != 1 {
		t.Fatalf("expected 1 equipment, got %d", len(eqChildren))
	}

	// Verify port loaded.
	loadedEq := eqChildren[0]
	portChildren := loaded.GetChildren(loadedEq)
	if len(portChildren) != 1 {
		t.Fatalf("expected 1 port, got %d", len(portChildren))
	}
}

func TestLoadEmptyDir(t *testing.T) {
	dir := t.TempDir()

	// Load from empty dir should create directories and return catalog with static folders.
	catalog, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Should have 5 static folders.
	topLevel := catalog.GetChildren(nil)
	if len(topLevel) != 5 {
		t.Fatalf("expected 5 static folders, got %d", len(topLevel))
	}

	// Verify directories were created.
	dataDir := filepath.Join(dir, DataDirName)
	for _, subDir := range []string{networksDirName, ipsDirName, vlansDirName, ssidsDirName, zonesDirName, equipmentDirName, portsDirName} {
		if _, err := os.Stat(filepath.Join(dataDir, subDir)); err != nil {
			t.Errorf("expected %s directory to be created: %v", subDir, err)
		}
	}
}

func TestSaveAtomicRollback(t *testing.T) {
	dir := t.TempDir()

	// First save to create the data dir.
	catalog := domain.NewCatalog()
	catalog.Put(&domain.StaticFolder{Base: domain.Base{ID: "Networks"}, Index: 0})
	catalog.Put(&domain.StaticFolder{Base: domain.Base{ID: "Zones"}, Index: 1})
	catalog.Put(&domain.StaticFolder{Base: domain.Base{ID: "VLANs"}, Index: 2})
	catalog.Put(&domain.StaticFolder{Base: domain.Base{ID: "WiFi SSIDs"}, Index: 3})
	catalog.Put(&domain.StaticFolder{Base: domain.Base{ID: "Equipment"}, Index: 4})

	if err := Save(dir, catalog); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Add a network and save again.
	catalog.Put(&domain.Network{
		Base:           domain.Base{ID: "10.0.0.0/24", ParentPath: "Networks"},
		AllocationMode: domain.AllocationModeHosts,
		DisplayName:    "Test",
	})
	if err := Save(dir, catalog); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify .old directory is cleaned up.
	dataOldDir := filepath.Join(dir, DataDirName+".old")
	if _, err := os.Stat(dataOldDir); !os.IsNotExist(err) {
		t.Error("old directory should be removed after save")
	}

	// Verify .tmp directory is cleaned up.
	dataTmpDir := filepath.Join(dir, DataDirName+".tmp")
	if _, err := os.Stat(dataTmpDir); !os.IsNotExist(err) {
		t.Error("tmp directory should be removed after save")
	}
}
