package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/plumber-cd/ez-ipam/internal/domain"
	"github.com/plumber-cd/ez-ipam/internal/store"
)

func addNetworkViaDialog(h *TestHarness, cidr string) {
	h.OpenAddNetworkDialog()
	h.TypeText(cidr)
	h.PressEnter()
}

func splitFocusedNetwork(h *TestHarness, newPrefix string) {
	h.PressRune('s')
	h.AssertScreenContains("Split")
	h.TypeText(newPrefix)
	h.PressEnter()
}

func allocateSubnetsFocused(h *TestHarness, name, description, vlanID, prefix string) {
	h.PressRune('a')
	h.AssertScreenContains("Subnet Container")
	h.TypeText(name)
	h.PressTab()
	h.TypeText(description)
	h.PressTab() // VLAN dropdown
	if strings.TrimSpace(vlanID) != "" {
		h.SelectDropdownOption("VLAN ID", strings.TrimSpace(vlanID))
	}
	h.PressTab() // Child Prefix Len
	h.TypeText(prefix)
	h.PressEnter()
}

func allocateHostsFocused(h *TestHarness, name, description, vlanID string) {
	h.PressRune('A')
	h.AssertScreenContains("Host Pool")
	h.TypeText(name)
	h.PressTab()
	h.TypeText(description)
	h.PressTab() // VLAN dropdown
	if strings.TrimSpace(vlanID) != "" {
		h.SelectDropdownOption("VLAN ID", strings.TrimSpace(vlanID))
	}
	h.PressTab() // Save button
	h.PressEnter()
}

func updateAllocationFocused(h *TestHarness, nameSuffix, descriptionSuffix, vlanID string) {
	h.PressRune('u')
	h.AssertScreenContains("Update Metadata")
	h.TypeText(nameSuffix)
	h.PressTab()
	h.TypeText(descriptionSuffix)
	h.PressTab() // VLAN dropdown
	if strings.TrimSpace(vlanID) != "" {
		h.SelectDropdownOption("VLAN ID", strings.TrimSpace(vlanID))
	}
	h.PressTab() // Save button
	h.PressEnter()
}

func reserveIPFromCurrentNetwork(h *TestHarness, ip, name, description string) {
	h.PressRune('r')
	h.AssertScreenContains("Reserve IP")
	h.TypeText(ip)
	h.PressTab()
	h.TypeText(name)
	h.PressTab()
	h.TypeText(description)
	h.PressTab()
	h.PressEnter()
}

func navigateToVLANs(t *testing.T, h *TestHarness) {
	t.Helper()
	h.PressBackspace()
	h.MoveFocusToID(t, "VLANs")
	h.PressEnter()
	h.AssertScreenContains("│VLANs")
}

func navigateToSSIDs(t *testing.T, h *TestHarness) {
	t.Helper()
	h.PressBackspace()
	h.MoveFocusToID(t, "WiFi SSIDs")
	h.PressEnter()
	h.AssertScreenContains("│WiFi SSIDs")
}

func navigateToNetworksRoot(t *testing.T, h *TestHarness) {
	t.Helper()
	for range 16 {
		h.PressBackspace()
	}
	h.MoveFocusToID(t, "Networks")
	h.PressEnter()
	h.AssertScreenContains("│Networks")
}

func addVLANViaDialog(h *TestHarness, id, name, description, zone string) {
	h.PressRune('v')
	h.AssertScreenContains("Add VLAN")
	h.TypeText(id)
	h.PressTab()
	h.TypeText(name)
	h.PressTab()
	h.TypeText(description)
	h.PressTab() // Zone dropdown
	if strings.TrimSpace(zone) != "" {
		h.SelectDropdownOption("Zone", strings.TrimSpace(zone))
	}
	h.PressTab() // Save button
	h.PressEnter()
}

func addSSIDViaDialog(h *TestHarness, id, description string) {
	h.PressRune('w')
	h.AssertScreenContains("Add WiFi SSID")
	h.TypeText(id)
	h.PressTab()
	h.TypeText(description)
	h.PressTab()
	h.PressEnter()
}

func navigateToZones(t *testing.T, h *TestHarness) {
	t.Helper()
	for range 16 {
		h.PressBackspace()
	}
	h.MoveFocusToID(t, "Zones")
	h.PressEnter()
	h.AssertScreenContains("│Zones")
}

func navigateToEquipmentRoot(t *testing.T, h *TestHarness) {
	t.Helper()
	for range 16 {
		h.PressBackspace()
	}
	h.MoveFocusToID(t, "Equipment")
	h.PressEnter()
	h.AssertScreenContains("│Equipment")
}

func addZoneViaDialog(h *TestHarness, name, description, vlanIDs string) {
	h.PressRune('z')
	h.AssertScreenContains("Add Zone")
	h.TypeText(name)
	h.PressTab()
	h.TypeText(description)
	requested := strings.TrimSpace(vlanIDs)
	if requested != "" {
		parts := strings.Split(requested, ",")
		for range parts {
			h.PressTab()
			h.ToggleCheckbox()
		}
	}
	for range 8 {
		h.PressTab()
		h.PressEnter()
		if !strings.Contains(h.GetScreenText(), "Add Zone") {
			return
		}
	}
}

func addEquipmentViaDialog(h *TestHarness, name, model, description string) {
	h.PressRune('e')
	h.AssertScreenContains("Add Equipment")
	h.TypeText(name)
	h.PressTab()
	h.TypeText(model)
	h.PressTab()
	h.TypeText(description)
	h.PressTab()
	h.PressEnter()
}

func addPortViaDialog(h *TestHarness, number, name, portType, speed, poe, lagGroup, lagMode, nativeVLAN, taggedMode, taggedVLANIDs, description string) {
	h.PressRune('p')
	h.AssertScreenContains("Add Port")
	h.TypeText(number)
	h.PressTab()
	h.TypeText(name)
	h.PressTab()
	h.TypeText(portType)
	h.PressTab()
	h.TypeText(speed)
	h.PressTab()
	h.TypeText(poe)
	h.PressTab() // LAG Mode dropdown
	if strings.TrimSpace(lagMode) != "" {
		// LAG mode dropdown rebuilds the dialog when changed; drive it with keys.
		h.PressEnter()
		h.PressDown() // Disabled -> 802.3ad
		h.PressEnter()
		// Focus remains on LAG Mode after rebuild; Tab moves to LAG Group.
		h.PressTab()
		h.TypeText(lagGroup)
		h.PressTab()
	}
	if strings.TrimSpace(nativeVLAN) != "" {
		h.SelectDropdownOption("Native VLAN ID", nativeVLAN)
	}
	h.PressTab() // Tagged VLAN Mode dropdown
	if strings.TrimSpace(taggedMode) != "" {
		h.PressEnter()
		switch strings.TrimSpace(taggedMode) {
		case "AllowAll":
			h.PressDown()
		case "BlockAll":
			h.PressDown()
			h.PressDown()
		case "Custom":
			h.PressDown()
			h.PressDown()
			h.PressDown()
		}
		h.PressEnter()
		if strings.EqualFold(strings.TrimSpace(taggedMode), "Custom") && strings.TrimSpace(taggedVLANIDs) != "" {
			for _, vlan := range strings.Split(taggedVLANIDs, ",") {
				if strings.TrimSpace(vlan) == "" {
					continue
				}
				h.PressTab()
				h.ToggleCheckbox()
			}
		}
	}
	h.PressTab() // Description textarea
	h.TypeText(description)
	for range 10 {
		h.PressTab()
		h.PressEnter()
		if !strings.Contains(h.GetScreenText(), "Add Port") {
			return
		}
	}
}

func updateFocusedIPReservation(h *TestHarness, nameSuffix, descriptionSuffix string) {
	h.PressRune('u')
	h.AssertScreenContains("Update Reservation")
	h.TypeText(nameSuffix)
	h.PressTab()
	h.TypeText(descriptionSuffix)
	h.PressTab()
	h.PressEnter()
}

func assertPrimaryCancelVisible(t *testing.T, h *TestHarness, primary string) {
	t.Helper()
	h.AssertScreenContains(primary)
	h.AssertScreenContains("Cancel")
}

func stepSnapshot(h *TestHarness, entity string, step *int, description string) {
	clean := strings.ToLower(strings.TrimSpace(description))
	clean = strings.ReplaceAll(clean, " ", "_")
	clean = strings.ReplaceAll(clean, "/", "_")
	clean = strings.ReplaceAll(clean, "-", "_")
	clean = strings.ReplaceAll(clean, "__", "_")
	name := fmt.Sprintf("%s_%02d_%s", entity, *step, clean)
	h.AssertGoldenSnapshot(name)
	*step++
}

func TestInitialStateAndGolden(t *testing.T) {
	h := NewTestHarness(t)
	h.AssertScreenContains("Home")
	h.AssertScreenContains("Networks")
	h.AssertScreenContains("WiFi SSIDs")
	h.AssertGoldenSnapshot("g01_s01_initial_state")
}

func TestVimNavigationAndKeysLineContext(t *testing.T) {
	h := NewTestHarness(t)
	h.NavigateToNetworks()
	menuKeys, _ := h.CurrentKeys()
	if len(menuKeys) != 1 || menuKeys[0] != "<n> New Network" {
		t.Fatalf("unexpected menu keys: %#v", menuKeys)
	}

	addNetworkViaDialog(h, "10.0.0.0/24")
	addNetworkViaDialog(h, "10.0.1.0/24")

	h.MoveFocusToID(t, "10.0.0.0/24")
	_, focusKeys := h.CurrentKeys()
	if !strings.Contains(strings.Join(focusKeys, " | "), "<a> Allocate Subnet Container") {
		t.Fatalf("expected allocation keys on unallocated network, got %#v", focusKeys)
	}
	h.PressRune('j')
	if !h.FocusMatches("10.0.1.0/24") {
		t.Fatalf("expected focus on 10.0.1.0/24, got %q", h.CurrentFocusID())
	}
	h.PressRune('k')
	if !h.FocusMatches("10.0.0.0/24") {
		t.Fatalf("expected focus back on 10.0.0.0/24, got %q", h.CurrentFocusID())
	}
	h.PressRune('l')
	h.AssertScreenContains("10.0.0.0/24")
	h.PressRune('h')
	h.AssertScreenContains("│Networks")

	h.MoveFocusToID(t, "10.0.0.0/24")
	allocateHostsFocused(h, "Web", "Tier", "")
	h.AssertScreenContains("Allocated network")
	h.PressEnter()
	menuKeys, _ = h.CurrentKeys()
	if !strings.Contains(strings.Join(menuKeys, " | "), "<r> Reserve IP") {
		t.Fatalf("expected reserve key on hosts network, got %#v", menuKeys)
	}
	reserveIPFromCurrentNetwork(h, "10.0.0.1", "gw", "gateway")
	h.AssertScreenContains("Reserved IP")
	_, focusKeys = h.CurrentKeys()
	if !strings.Contains(strings.Join(focusKeys, " | "), "<u> Update Reservation") {
		t.Fatalf("expected ip reservation keys, got %#v", focusKeys)
	}

	h.MoveFocusToID(t, "10.0.0.1 (gw)")
	h.PressBackspace()
	h.AssertScreenContains("│Networks")
}

func TestNetworkValidationBranches(t *testing.T) {
	cases := []struct {
		name string
		cidr string
		ok   bool
	}{
		{name: "valid_ipv4", cidr: "10.0.0.0/8", ok: true},
		{name: "valid_ipv6", cidr: "fd00::/48", ok: true},
		{name: "invalid_cidr", cidr: "not-a-cidr"},
		{name: "host_not_network", cidr: "10.0.0.1/8"},
		{name: "single_ipv4", cidr: "10.0.0.1/32"},
		{name: "single_ipv6", cidr: "fd00::1/128"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewTestHarness(t)
			h.NavigateToNetworks()
			addNetworkViaDialog(h, tc.cidr)
			if tc.ok {
				h.AssertScreenContains(strings.Split(tc.cidr, "/")[0])
			} else {
				h.AssertStatusContains("Error adding new network")
			}
		})
	}
}

func TestAddOverlapAndDuplicate(t *testing.T) {
	h := NewTestHarness(t)
	h.NavigateToNetworks()
	addNetworkViaDialog(h, "10.0.0.0/24")
	addNetworkViaDialog(h, "10.0.0.128/25")
	h.AssertStatusContains("Error adding new network")
	addNetworkViaDialog(h, "10.0.0.0/24")
	h.AssertStatusContains("Error adding new network")
}

func TestSplitNetworkBranches(t *testing.T) {
	setup := func(t *testing.T) *TestHarness {
		t.Helper()
		h := NewTestHarness(t)
		h.NavigateToNetworks()
		addNetworkViaDialog(h, "10.0.0.0/24")
		h.MoveFocusToID(t, "10.0.0.0/24")
		return h
	}

	t.Run("invalid_prefix_input", func(t *testing.T) {
		h := setup(t)
		splitFocusedNetwork(h, "bad")
		if !strings.Contains(h.CurrentStatusText(), "Invalid prefix length") {
			t.Fatalf("expected invalid prefix error, got %q", h.CurrentStatusText())
		}
	})

	t.Run("prefix_too_small", func(t *testing.T) {
		h := setup(t)
		splitFocusedNetwork(h, "23")
		if !strings.Contains(h.CurrentStatusText(), "Error splitting network") {
			t.Fatalf("expected split error, got %q", h.CurrentStatusText())
		}
	})

	t.Run("prefix_too_large", func(t *testing.T) {
		h := setup(t)
		splitFocusedNetwork(h, "33")
		if !strings.Contains(h.CurrentStatusText(), "Error splitting network") {
			t.Fatalf("expected split error, got %q", h.CurrentStatusText())
		}
	})

	t.Run("valid_split", func(t *testing.T) {
		h := setup(t)
		splitFocusedNetwork(h, "26")
		h.AssertStatusContains("Split network")
		h.AssertScreenContains("10.0.0.0/26")
	})
}

func TestSummarizeNetwork(t *testing.T) {
	h := NewTestHarness(t)
	h.NavigateToNetworks()
	addNetworkViaDialog(h, "10.0.0.0/25")
	addNetworkViaDialog(h, "10.0.0.128/25")
	h.MoveFocusToID(t, "10.0.0.0/25")

	h.PressRune('S')
	h.AssertScreenContains("Summarize in Networks")
	h.PressTab()
	h.PressTab()
	h.PressEnter()
	h.AssertStatusContains("Summarized networks into")
	h.AssertScreenContains("10.0.0.0/24")
}

func TestDeleteNetworkBranches(t *testing.T) {
	t.Run("top_level_cancel_and_confirm", func(t *testing.T) {
		h := NewTestHarness(t)
		h.NavigateToNetworks()
		addNetworkViaDialog(h, "10.0.0.0/24")
		h.MoveFocusToID(t, "10.0.0.0/24")

		h.PressRune('D')
		h.AssertScreenContains("Delete 10.0.0.0/24")
		h.AssertScreenContains("(*)?")
		h.CancelModal()
		h.AssertScreenContains("10.0.0.0/24")

		h.PressRune('D')
		h.AssertScreenContains("Delete 10.0.0.0/24")
		h.AssertScreenContains("(*)?")
		h.ConfirmModal()
		h.AssertStatusContains("Deleted network")
	})

	t.Run("child_delete_blocked", func(t *testing.T) {
		h := NewTestHarness(t)
		h.NavigateToNetworks()
		addNetworkViaDialog(h, "10.0.0.0/24")
		h.MoveFocusToID(t, "10.0.0.0/24")
		splitFocusedNetwork(h, "25")
		h.MoveFocusToID(t, "10.0.0.0/25")

		h.PressRune('D')
		h.AssertScreenNotContains("Delete 10.0.0.0/25?")
		h.AssertScreenNotContains("<D> Delete")
	})
}

func TestAllocateSubnetsBranches(t *testing.T) {
	setup := func(t *testing.T) *TestHarness {
		t.Helper()
		h := NewTestHarness(t)
		h.NavigateToNetworks()
		addNetworkViaDialog(h, "10.0.0.0/24")
		h.MoveFocusToID(t, "10.0.0.0/24")
		return h
	}

	t.Run("empty_name", func(t *testing.T) {
		h := setup(t)
		allocateSubnetsFocused(h, "", "", "", "25")
		h.AssertStatusContains("Error allocating network")
	})

	t.Run("invalid_prefix", func(t *testing.T) {
		h := setup(t)
		allocateSubnetsFocused(h, "Prod", "desc", "", "bad")
		if !strings.Contains(h.CurrentStatusText(), "Invalid subnet prefix length") {
			t.Fatalf("expected invalid subnet prefix error, got %q", h.CurrentStatusText())
		}
	})

	t.Run("valid", func(t *testing.T) {
		h := setup(t)
		allocateSubnetsFocused(h, "Prod", "desc", "", "25")
		h.AssertStatusContains("Allocated network")
	})
}

func TestAllocateHostsBranches(t *testing.T) {
	h := NewTestHarness(t)
	h.NavigateToNetworks()
	addNetworkViaDialog(h, "192.168.1.0/24")
	h.MoveFocusToID(t, "192.168.1.0/24")

	allocateHostsFocused(h, "", "", "")
	h.AssertStatusContains("Error allocating network")

	allocateHostsFocused(h, "Office", "LAN", "")
	h.AssertStatusContains("Allocated network")
}

func TestUpdateAllocationAndDeallocate(t *testing.T) {
	h := NewTestHarness(t)
	h.NavigateToNetworks()
	addNetworkViaDialog(h, "10.0.0.0/24")
	h.MoveFocusToID(t, "10.0.0.0/24")
	allocateHostsFocused(h, "Web", "servers", "")

	updateAllocationFocused(h, "-2", " updated", "")
	h.AssertStatusContains("Allocated network updated")
	h.AssertScreenContains("10.0.0.0/24 (Web-2)")

	h.PressRune('d')
	h.AssertScreenContains("Deallocate 10.0.0.0/24")
	h.CancelModal()
	h.AssertScreenContains("(Web-2)")

	h.PressRune('d')
	h.AssertScreenContains("Deallocate 10.0.0.0/24")
	h.ConfirmModal()
	h.AssertStatusContains("Deallocated network")
	h.AssertScreenContains("10.0.0.0/24 (*)")
}

func TestReserveUpdateUnreserveIPBranches(t *testing.T) {
	h := NewTestHarness(t)
	h.NavigateToNetworks()
	addNetworkViaDialog(h, "192.168.1.0/24")
	h.MoveFocusToID(t, "192.168.1.0/24")
	allocateHostsFocused(h, "Office", "LAN", "")
	h.PressEnter()

	reserveIPFromCurrentNetwork(h, "192.168.1.1", "gateway", "gw")
	h.AssertStatusContains("Reserved IP")

	reserveIPFromCurrentNetwork(h, "not-an-ip", "a", "b")
	h.AssertStatusContains("Error reserving IP")
	reserveIPFromCurrentNetwork(h, "10.0.0.1", "a", "b")
	h.AssertStatusContains("Error reserving IP")
	reserveIPFromCurrentNetwork(h, "192.168.1.1", "dup", "dup")
	h.AssertStatusContains("Error reserving IP")
	reserveIPFromCurrentNetwork(h, "192.168.1.2", "", "desc")
	h.AssertStatusContains("Error reserving IP")
	reserveIPFromCurrentNetwork(h, "fd00::1", "x", "y")
	h.AssertStatusContains("Error reserving IP")

	h.MoveFocusToID(t, "192.168.1.1 (gateway)")
	updateFocusedIPReservation(h, "-1", " updated")
	h.AssertStatusContains("Updated IP reservation")
	h.AssertScreenContains("192.168.1.1 (gateway-1)")

	h.PressRune('R')
	h.AssertScreenContains("Unreserve 192.168.1.1")
	h.AssertScreenContains("(gateway-1)?")
	h.CancelModal()
	h.AssertScreenContains("192.168.1.1 (gateway-1)")

	h.PressRune('R')
	h.AssertScreenContains("Unreserve 192.168.1.1")
	h.AssertScreenContains("(gateway-1)?")
	h.ConfirmModal()
	h.AssertStatusContains("Unreserved IP")
}

func TestAddVLANBranches(t *testing.T) {
	h := NewTestHarness(t)
	navigateToVLANs(t, h)

	addVLANViaDialog(h, "100", "Management", "infra management", "")
	h.AssertStatusContains("Added VLAN")

	addVLANViaDialog(h, "0", "Invalid", "bad", "")
	h.AssertStatusContains("Error adding VLAN")
	addVLANViaDialog(h, "4095", "Invalid", "bad", "")
	h.AssertStatusContains("Error adding VLAN")
	addVLANViaDialog(h, "bad", "Invalid", "bad", "")
	h.AssertStatusContains("Error adding VLAN")
	addVLANViaDialog(h, "101", "", "missing name", "")
	h.AssertStatusContains("Error adding VLAN")
	addVLANViaDialog(h, "100", "Duplicate", "dup", "")
	h.AssertStatusContains("Error adding VLAN")
}

func TestUpdateAndDeleteVLAN(t *testing.T) {
	h := NewTestHarness(t)
	navigateToVLANs(t, h)
	addVLANViaDialog(h, "200", "Users", "user segment", "")
	h.MoveFocusToID(t, "200 (Users)")

	h.PressRune('u')
	h.AssertScreenContains("Update VLAN")
	h.TypeText("-2")
	h.PressTab()
	h.TypeText(" updated")
	h.PressTab()
	h.PressTab() // Zone dropdown -> Save button
	h.PressEnter()
	h.AssertStatusContains("Updated VLAN")
	h.AssertScreenContains("200 (Users-2)")

	h.PressRune('D')
	h.AssertScreenContains("Delete VLAN 200")
	h.CancelModal()
	h.AssertScreenContains("200 (Users-2)")

	h.PressRune('D')
	h.AssertScreenContains("Delete VLAN 200")
	h.ConfirmModal()
	h.AssertStatusContains("Deleted VLAN")
}

func TestAddSSIDBranches(t *testing.T) {
	h := NewTestHarness(t)
	navigateToSSIDs(t, h)

	addSSIDViaDialog(h, "Home-Infra-5G", "Infrastructure devices")
	h.AssertStatusContains("Added WiFi SSID")

	addSSIDViaDialog(h, "", "bad")
	h.AssertStatusContains("Error adding WiFi SSID")
	addSSIDViaDialog(h, "Home-Infra-5G", "dup")
	h.AssertStatusContains("Error adding WiFi SSID")
}

func TestUpdateAndDeleteSSID(t *testing.T) {
	h := NewTestHarness(t)
	navigateToSSIDs(t, h)
	addSSIDViaDialog(h, "Home-IoT", "IoT devices")
	h.MoveFocusToID(t, "Home-IoT")

	h.PressRune('u')
	h.AssertScreenContains("Update WiFi SSID")
	h.TypeText(" updated")
	h.PressTab()
	h.PressEnter()
	h.AssertStatusContains("Updated WiFi SSID")
	h.AssertScreenContains("IoT devices updated")

	h.PressRune('D')
	h.AssertScreenContains("Delete WiFi SSID Home-IoT?")
	h.CancelModal()
	h.AssertScreenContains("Home-IoT")

	h.PressRune('D')
	h.AssertScreenContains("Delete WiFi SSID Home-IoT?")
	h.ConfirmModal()
	h.AssertStatusContains("Deleted WiFi SSID")
}

func TestNetworkVLANAssignment(t *testing.T) {
	h := NewTestHarness(t)
	navigateToVLANs(t, h)
	addVLANViaDialog(h, "300", "Servers", "servers vlan", "")

	navigateToNetworksRoot(t, h)
	addNetworkViaDialog(h, "10.50.0.0/24")
	h.MoveFocusToID(t, "10.50.0.0/24")
	allocateHostsFocused(h, "Servers", "pool", "300")
	h.AssertStatusContains("Allocated network")
	h.AssertScreenContains("300 (Servers)")

	addNetworkViaDialog(h, "10.51.0.0/24")
	h.MoveFocusToID(t, "10.51.0.0/24")
	allocateHostsFocused(h, "NoVLAN", "pool", "")
	h.AssertStatusContains("Allocated network")
}

func TestVLANCrossReference(t *testing.T) {
	h := NewTestHarness(t)
	navigateToVLANs(t, h)
	addVLANViaDialog(h, "400", "Apps", "application vlan", "")

	navigateToNetworksRoot(t, h)
	addNetworkViaDialog(h, "10.60.0.0/24")
	h.MoveFocusToID(t, "10.60.0.0/24")
	allocateHostsFocused(h, "Apps", "pool", "400")
	h.AssertStatusContains("Allocated network")

	h.PressBackspace()
	navigateToVLANs(t, h)
	h.MoveFocusToID(t, "400 (Apps)")
	h.AssertScreenContains("10.60.0.0/24")
}

func TestZonesAndVLANCascade(t *testing.T) {
	h := NewTestHarness(t)
	navigateToVLANs(t, h)
	addVLANViaDialog(h, "101", "Main", "main vlan", "")
	h.AssertStatusContains("Added VLAN")

	navigateToZones(t, h)
	addZoneViaDialog(h, "Main", "trusted clients", "101")
	h.AssertStatusContains("Added Zone")
	h.MoveFocusToID(t, "Main")
	h.AssertScreenContains("101 (Main)")

	navigateToVLANs(t, h)
	h.MoveFocusToID(t, "101 (Main)")
	h.PressRune('D')
	h.ConfirmModal()
	h.AssertStatusContains("Deleted VLAN")

	navigateToZones(t, h)
	h.MoveFocusToID(t, "Main")
	h.AssertScreenContains("Associated VLANs")
	h.AssertScreenContains("<none>")
}

func TestEquipmentPortAndConnections(t *testing.T) {
	h := NewTestHarness(t)
	navigateToEquipmentRoot(t, h)
	addEquipmentViaDialog(h, "Gateway", "UCG-Fiber", "edge")
	addEquipmentViaDialog(h, "Lab", "USW-Enterprise-8-PoE", "lab switch")
	h.AssertStatusContains("Added Equipment")

	h.MoveFocusToID(t, "Gateway (UCG-Fiber)")
	h.PressEnter()
	addPortViaDialog(h, "1", "WAN1", "RJ45", "2.5GbE", "", "", "", "", "", "", "uplink")
	h.AssertStatusContains("Added Port")
	addPortViaDialog(h, "2", "", "RJ45", "2.5GbE", "", "", "", "", "", "", "")
	h.AssertStatusContains("Added Port")

	h.PressBackspace()
	h.MoveFocusToID(t, "Lab (USW-Enterprise-8-PoE)")
	h.PressEnter()
	addPortViaDialog(h, "1", "SFP+ 1", "SFP+", "10GbE", "", "", "", "", "", "", "")
	h.AssertStatusContains("Added Port")
	addPortViaDialog(h, "4", "Port 4", "RJ45", "2.5GbE", "", "3", "802.3ad", "", "", "", "")
	h.AssertStatusContains("Error adding Port")

	h.PressBackspace()
	h.MoveFocusToID(t, "Gateway (UCG-Fiber)")
	h.PressEnter()
	h.MoveFocusToID(t, "1: WAN1")
	h.PressRune('c')
	h.AssertScreenContains("Connect")
	h.PressTab()
	h.PressEnter()
	h.AssertStatusContains("Connected Port")

	h.PressRune('x')
	h.AssertScreenContains("Disconnect")
	h.ConfirmModal()
	h.AssertStatusContains("Disconnected Port")
}

func TestMarkdownIncludesZonesAndEquipment(t *testing.T) {
	h := NewTestHarness(t)
	navigateToVLANs(t, h)
	addVLANViaDialog(h, "104", "Servers", "servers vlan", "")

	navigateToZones(t, h)
	addZoneViaDialog(h, "Servers", "server zone", "104")

	navigateToEquipmentRoot(t, h)
	addEquipmentViaDialog(h, "Servers", "USW-Aggregation", "agg switch")
	h.MoveFocusToID(t, "Servers (USW-Aggregation)")
	h.PressEnter()
	addPortViaDialog(h, "1", "SFP+ 1", "SFP+", "10GbE", "", "", "", "104", "BlockAll", "", "proxmox uplink")

	h.PressCtrl('s')
	h.AssertStatusContains("Saved to .ez-ipam/ and EZ-IPAM.md")

	mdBytes, err := os.ReadFile(filepath.Join(h.workDir, store.MarkdownFileName))
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	md := string(mdBytes)
	if !strings.Contains(md, "## Zones") {
		t.Fatalf("markdown missing zones section")
	}
	if !strings.Contains(md, "## Equipment") {
		t.Fatalf("markdown missing equipment section")
	}
	if !strings.Contains(md, "Servers (USW-Aggregation)") {
		t.Fatalf("markdown missing equipment title")
	}
	if !strings.Contains(md, "SFP+ 1") {
		t.Fatalf("markdown missing port row")
	}
}

func TestSaveLoadAndQuit(t *testing.T) {
	t.Run("save_and_load", func(t *testing.T) {
		h := NewTestHarness(t)
		h.NavigateToNetworks()
		addNetworkViaDialog(h, "10.0.0.0/24")
		h.PressCtrl('s')
		h.AssertStatusContains("Saved to .ez-ipam/ and EZ-IPAM.md")

		if _, err := os.Stat(filepath.Join(h.workDir, store.DataDirName, "networks")); err != nil {
			t.Fatalf("networks dir missing: %v", err)
		}
		mdBytes, err := os.ReadFile(filepath.Join(h.workDir, store.MarkdownFileName))
		if err != nil {
			t.Fatalf("read markdown: %v", err)
		}
		if !strings.Contains(string(mdBytes), "10.0.0.0/24") {
			t.Fatalf("markdown missing network")
		}

		dir := h.workDir
		h.Close()

		h2 := NewTestHarnessInDir(t, dir)
		h2.NavigateToNetworks()
		h2.AssertScreenContains("10.0.0.0/24")
	})

	t.Run("quit_q_cancel", func(t *testing.T) {
		h := NewTestHarness(t)
		h.NavigateToNetworks()
		h.PressRune('q')
		h.AssertScreenContains("Do you want to quit?")
		h.CancelModal()
		h.AssertScreenContains("│Networks")
	})

	t.Run("quit_q_confirm", func(t *testing.T) {
		h := NewTestHarness(t)
		h.NavigateToNetworks()
		h.PressRune('q')
		h.AssertScreenContains("Do you want to quit?")
		h.ConfirmModal()
		if err := h.WaitForExit(2 * time.Second); err != nil {
			t.Fatalf("app did not exit: %v", err)
		}
	})

	t.Run("quit_ctrl_c", func(t *testing.T) {
		h := NewTestHarness(t)
		h.PressCtrl('c')
		h.AssertScreenContains("Do you want to quit?")
		h.CancelModal()
		h.AssertScreenContains("Home")
	})

	t.Run("quit_ctrl_q", func(t *testing.T) {
		h := NewTestHarness(t)
		h.InjectKeyNoWait(tcell.KeyCtrlQ, 0, tcell.ModNone)
		if err := h.WaitForExit(2 * time.Second); err != nil {
			t.Fatalf("app did not exit: %v", err)
		}
	})
}

func TestHomeNavigation(t *testing.T) {
	h := NewTestHarness(t)
	step := 1
	stepSnapshot(h, "home", &step, "initial_home")

	h.NavigateToNetworks()
	stepSnapshot(h, "home", &step, "networks_folder")
	h.PressBackspace()

	h.MoveFocusToID(t, "VLANs")
	h.PressEnter()
	stepSnapshot(h, "home", &step, "vlans_folder")
	h.PressBackspace()

	h.MoveFocusToID(t, "WiFi SSIDs")
	h.PressEnter()
	stepSnapshot(h, "home", &step, "ssids_folder")
	h.PressBackspace()

	h.MoveFocusToID(t, "Zones")
	h.PressEnter()
	stepSnapshot(h, "home", &step, "zones_folder")
	h.PressBackspace()

	h.MoveFocusToID(t, "Equipment")
	h.PressEnter()
	stepSnapshot(h, "home", &step, "equipment_folder")
	h.PressBackspace()
}

func TestNetworksLifecycle(t *testing.T) {
	h := NewTestHarness(t)
	step := 1
	h.NavigateToNetworks()
	stepSnapshot(h, "networks", &step, "folder_empty")

	h.PressRune('n')
	stepSnapshot(h, "networks", &step, "add_dialog")
	h.PressEscape()

	addNetworkViaDialog(h, "10.0.0.0/24")
	stepSnapshot(h, "networks", &step, "added_ipv4")
	addNetworkViaDialog(h, "fd00::/64")
	stepSnapshot(h, "networks", &step, "added_ipv6")

	h.MoveFocusToID(t, "10.0.0.0/24")
	h.PressRune('s')
	stepSnapshot(h, "networks", &step, "split_dialog")
	h.PressEscape()
	splitFocusedNetwork(h, "25")
	stepSnapshot(h, "networks", &step, "split_into_25")

	h.MoveFocusToID(t, "10.0.0.0/25")
	h.PressRune('S')
	stepSnapshot(h, "networks", &step, "summarize_dialog")
	h.PressTab()
	h.PressTab()
	h.PressEnter()
	stepSnapshot(h, "networks", &step, "summarized_to_24")

	h.MoveFocusToID(t, "10.0.0.0/24")
	h.PressRune('a')
	stepSnapshot(h, "networks", &step, "allocate_subnets_dialog")
	h.PressEscape()
	allocateSubnetsFocused(h, "Prod", "subnets", "", "25")
	stepSnapshot(h, "networks", &step, "allocated_subnets")

	h.PressRune('u')
	stepSnapshot(h, "networks", &step, "update_dialog")
	h.PressEscape()
	updateAllocationFocused(h, "-new", " metadata", "")
	stepSnapshot(h, "networks", &step, "updated_metadata")

	h.PressRune('d')
	stepSnapshot(h, "networks", &step, "deallocate_confirm")
	h.CancelModal()
	stepSnapshot(h, "networks", &step, "deallocate_cancel")
	h.PressRune('d')
	h.ConfirmModal()
	stepSnapshot(h, "networks", &step, "deallocated")

	allocateHostsFocused(h, "Hosts", "pool", "")
	stepSnapshot(h, "networks", &step, "allocated_hosts")
	h.PressEnter()
	h.PressRune('r')
	stepSnapshot(h, "networks", &step, "reserve_ip_dialog")
	h.PressEscape()
	reserveIPFromCurrentNetwork(h, "10.0.0.1", "gw", "gateway")
	stepSnapshot(h, "networks", &step, "ip_reserved")

	h.MoveFocusToID(t, "10.0.0.1 (gw)")
	h.PressRune('u')
	stepSnapshot(h, "networks", &step, "update_ip_dialog")
	h.PressEscape()
	updateFocusedIPReservation(h, "-x", " updated")
	stepSnapshot(h, "networks", &step, "ip_updated")

	h.PressRune('R')
	stepSnapshot(h, "networks", &step, "unreserve_confirm")
	h.CancelModal()
	stepSnapshot(h, "networks", &step, "unreserve_cancel")
	h.PressRune('R')
	h.ConfirmModal()
	stepSnapshot(h, "networks", &step, "ip_unreserved")
}

func TestNetworksNegative(t *testing.T) {
	h := NewTestHarness(t)
	step := 1
	h.NavigateToNetworks()

	addNetworkViaDialog(h, "not-a-cidr")
	stepSnapshot(h, "networks_negative", &step, "invalid_cidr")
	addNetworkViaDialog(h, "10.0.0.1/24")
	stepSnapshot(h, "networks_negative", &step, "host_not_network")
	addNetworkViaDialog(h, "10.0.0.0/24")
	addNetworkViaDialog(h, "10.0.0.0/24")
	stepSnapshot(h, "networks_negative", &step, "duplicate_network")
	addNetworkViaDialog(h, "10.0.0.128/25")
	stepSnapshot(h, "networks_negative", &step, "overlap_network")

	h.MoveFocusToID(t, "10.0.0.0/24")
	splitFocusedNetwork(h, "bad")
	stepSnapshot(h, "networks_negative", &step, "split_invalid_prefix")
	h.PressEscape()
	allocateSubnetsFocused(h, "", "", "", "25")
	stepSnapshot(h, "networks_negative", &step, "allocate_empty_name")
	allocateHostsFocused(h, "", "", "")
	stepSnapshot(h, "networks_negative", &step, "allocate_hosts_empty_name")
}

func TestVLANsLifecycleAndNegative(t *testing.T) {
	h := NewTestHarness(t)
	step := 1
	navigateToVLANs(t, h)
	stepSnapshot(h, "vlans", &step, "folder_empty")

	h.PressRune('v')
	stepSnapshot(h, "vlans", &step, "add_dialog")
	h.PressEscape()

	addVLANViaDialog(h, "100", "Management", "infra", "")
	stepSnapshot(h, "vlans", &step, "added_100")
	addVLANViaDialog(h, "200", "Users", "clients", "")
	stepSnapshot(h, "vlans", &step, "added_200")

	h.MoveFocusToID(t, "200 (Users)")
	h.PressRune('u')
	stepSnapshot(h, "vlans", &step, "update_dialog")
	h.PressEscape()
	h.PressRune('u')
	h.TypeText("-new")
	h.PressTab()
	h.TypeText(" updated")
	h.PressTab()
	h.PressTab() // Zone dropdown -> Save button
	h.PressEnter()
	stepSnapshot(h, "vlans", &step, "updated")

	h.PressRune('D')
	stepSnapshot(h, "vlans", &step, "delete_confirm")
	h.CancelModal()
	stepSnapshot(h, "vlans", &step, "delete_cancel")

	addVLANViaDialog(h, "0", "Invalid", "bad", "")
	stepSnapshot(h, "vlans_negative", &step, "id_0")
	addVLANViaDialog(h, "4095", "Invalid", "bad", "")
	stepSnapshot(h, "vlans_negative", &step, "id_4095")
	addVLANViaDialog(h, "bad", "Invalid", "bad", "")
	stepSnapshot(h, "vlans_negative", &step, "id_non_numeric")
	addVLANViaDialog(h, "100", "Duplicate", "bad", "")
	stepSnapshot(h, "vlans_negative", &step, "duplicate_id")
}

func TestSSIDsLifecycleAndNegative(t *testing.T) {
	h := NewTestHarness(t)
	step := 1
	navigateToSSIDs(t, h)
	stepSnapshot(h, "ssids", &step, "folder_empty")

	h.PressRune('w')
	stepSnapshot(h, "ssids", &step, "add_dialog")
	h.PressEscape()

	addSSIDViaDialog(h, "Home-IoT", "devices")
	stepSnapshot(h, "ssids", &step, "added")

	h.MoveFocusToID(t, "Home-IoT")
	h.PressRune('u')
	stepSnapshot(h, "ssids", &step, "update_dialog")
	h.PressEscape()
	h.PressRune('u')
	h.TypeText(" updated")
	h.PressTab()
	h.PressEnter()
	stepSnapshot(h, "ssids", &step, "updated")

	h.PressRune('D')
	stepSnapshot(h, "ssids", &step, "delete_confirm")
	h.CancelModal()
	stepSnapshot(h, "ssids", &step, "delete_cancel")

	addSSIDViaDialog(h, "", "bad")
	stepSnapshot(h, "ssids_negative", &step, "empty_id")
	addSSIDViaDialog(h, "Home-IoT", "dup")
	stepSnapshot(h, "ssids_negative", &step, "duplicate_id")
}

func TestZonesEquipmentPortsLifecycleAndNegative(t *testing.T) {
	h := NewTestHarness(t)
	step := 1
	navigateToVLANs(t, h)
	addVLANViaDialog(h, "101", "Main", "main", "")
	addVLANViaDialog(h, "102", "Guest", "guest", "")

	navigateToZones(t, h)
	stepSnapshot(h, "zones", &step, "folder_empty")
	addZoneViaDialog(h, "Trusted", "trusted clients", "101")
	stepSnapshot(h, "zones", &step, "added_with_vlan")
	addZoneViaDialog(h, "", "invalid", "")
	stepSnapshot(h, "zones_negative", &step, "empty_name")

	navigateToEquipmentRoot(t, h)
	stepSnapshot(h, "equipment", &step, "folder_empty")
	addEquipmentViaDialog(h, "Gateway", "UCG", "edge")
	stepSnapshot(h, "equipment", &step, "added_gateway")
	addEquipmentViaDialog(h, "", "Model", "bad")
	stepSnapshot(h, "equipment_negative", &step, "empty_name")
	addEquipmentViaDialog(h, "NoModel", "", "bad")
	stepSnapshot(h, "equipment_negative", &step, "empty_model")

	addEquipmentViaDialog(h, "Switch", "USW", "agg")
	h.MoveFocusToID(t, "Gateway (UCG)")
	h.PressEnter()
	addPortViaDialog(h, "1", "WAN1", "RJ45", "2.5GbE", "", "", "", "", "", "", "uplink")
	stepSnapshot(h, "ports", &step, "added_port")
	addPortViaDialog(h, "1", "WAN1-dup", "RJ45", "2.5GbE", "", "", "", "", "", "", "dup")
	stepSnapshot(h, "ports_negative", &step, "duplicate_port_number")
}

func TestCrossReferencesAndPersistence(t *testing.T) {
	h := NewTestHarness(t)
	step := 1
	navigateToVLANs(t, h)
	addVLANViaDialog(h, "300", "Servers", "srv", "")
	stepSnapshot(h, "crossref", &step, "vlan_added")

	navigateToNetworksRoot(t, h)
	addNetworkViaDialog(h, "10.60.0.0/24")
	h.MoveFocusToID(t, "10.60.0.0/24")
	allocateHostsFocused(h, "Servers", "pool", "300")
	stepSnapshot(h, "crossref", &step, "network_allocated_with_vlan")

	navigateToVLANs(t, h)
	h.MoveFocusToID(t, "300 (Servers)")
	stepSnapshot(h, "crossref", &step, "vlan_shows_network")

	h.PressCtrl('s')
	stepSnapshot(h, "persistence", &step, "saved_status")

	h.Close()
	h2 := NewTestHarnessInDir(t, h.workDir)
	navigateToVLANs(t, h2)
	h2.MoveFocusToID(t, "300 (Servers)")
	stepSnapshot(h2, "persistence", &step, "loaded_state")
}

func TestQuitAndDialogEscape(t *testing.T) {
	h := NewTestHarness(t)
	step := 1
	h.NavigateToNetworks()
	addNetworkViaDialog(h, "10.0.0.0/24")
	h.MoveFocusToID(t, "10.0.0.0/24")

	h.PressRune('n')
	stepSnapshot(h, "escape", &step, "add_network_dialog")
	h.PressEscape()
	stepSnapshot(h, "escape", &step, "after_add_network_escape")

	h.PressRune('a')
	stepSnapshot(h, "escape", &step, "allocate_subnets_dialog")
	h.PressEscape()
	stepSnapshot(h, "escape", &step, "after_allocate_escape")

	h.PressRune('A')
	stepSnapshot(h, "escape", &step, "allocate_hosts_dialog")
	h.PressEscape()
	stepSnapshot(h, "escape", &step, "after_hosts_escape")

	h.PressRune('q')
	stepSnapshot(h, "quit", &step, "quit_modal")
	h.CancelModal()
	stepSnapshot(h, "quit", &step, "quit_cancel")
}

func TestGoldenDialogsAndScreens(t *testing.T) {
	h := NewTestHarness(t)
	h.NavigateToNetworks()
	addNetworkViaDialog(h, "10.0.0.0/24")
	addNetworkViaDialog(h, "fd00::/64")

	h.MoveFocusToID(t, "10.0.0.0/24")
	h.AssertGoldenSnapshot("g02_s01_network_details_ipv4")

	h.PressRune('n')
	assertPrimaryCancelVisible(t, h, "Save")
	h.AssertGoldenSnapshot("g02_s02_new_network_dialog")
	h.PressEscape()

	h.PressRune('s')
	assertPrimaryCancelVisible(t, h, "Save")
	h.AssertGoldenSnapshot("g02_s03_split_network_dialog")
	h.PressEscape()

	addNetworkViaDialog(h, "10.0.1.0/24")
	h.MoveFocusToID(t, "10.0.0.0/24")
	h.PressRune('S')
	assertPrimaryCancelVisible(t, h, "Summarize")
	h.AssertGoldenSnapshot("g02_s04_summarize_network_dialog")
	h.PressEscape()

	h.PressRune('a')
	assertPrimaryCancelVisible(t, h, "Save")
	h.AssertGoldenSnapshot("g02_s05_allocate_subnets_dialog")
	h.PressEscape()

	h.PressRune('A')
	assertPrimaryCancelVisible(t, h, "Save")
	h.AssertGoldenSnapshot("g02_s06_allocate_hosts_dialog")
	h.PressEscape()

	allocateHostsFocused(h, "Hosts", "desc", "")
	h.AssertGoldenSnapshot("g02_s07_allocated_network_hosts")

	h.PressRune('u')
	assertPrimaryCancelVisible(t, h, "Save")
	h.AssertGoldenSnapshot("g02_s08_update_allocation_dialog")
	h.PressEscape()

	h.PressRune('d')
	h.AssertGoldenSnapshot("g02_s09_deallocate_confirm_modal")
	h.CancelModal()

	h.PressRune('D')
	h.AssertGoldenSnapshot("g02_s10_delete_confirm_modal")
	h.CancelModal()

	h.PressEnter()
	h.PressRune('r')
	assertPrimaryCancelVisible(t, h, "Save")
	h.AssertGoldenSnapshot("g02_s11_reserve_ip_dialog")
	h.PressEscape()

	reserveIPFromCurrentNetwork(h, "10.0.0.1", "gw", "gateway")
	h.MoveFocusToID(t, "10.0.0.1 (gw)")
	h.AssertGoldenSnapshot("g02_s12_ip_details")

	h.PressRune('u')
	assertPrimaryCancelVisible(t, h, "Save")
	h.AssertGoldenSnapshot("g02_s13_update_ip_dialog")
	h.PressEscape()

	h.PressRune('R')
	h.AssertGoldenSnapshot("g02_s14_unreserve_confirm_modal")
	h.CancelModal()

	h.PressBackspace()
	h.MoveFocusToID(t, "fd00::/64")
	h.AssertGoldenSnapshot("g02_s15_network_details_ipv6")

	h.PressRune('a')
	h.TypeText("IPv6-Lab")
	h.PressTab()
	h.TypeText("subnets")
	h.PressTab()
	h.PressTab()
	h.TypeText("80")
	h.PressEnter()
	h.AssertGoldenSnapshot("g02_s16_allocated_network_subnets")

	h.PressRune('q')
	h.AssertGoldenSnapshot("g02_s17_quit_modal")
	h.CancelModal()
}

func TestDemoState(t *testing.T) {
	h := NewTestHarness(t)
	// Intentionally fixed, fictional showcase data (not user topology, deterministic for tests).
	vlanA := 147
	vlanB := 233
	vlanC := 318
	switchName := "Switch-Cobalt"
	routerName := "Router-Topaz"

	navigateToVLANs(t, h)
	addVLANViaDialog(h, fmt.Sprintf("%d", vlanA), "Segment-Alpha", "Synthetic demo segment alpha", "")
	addVLANViaDialog(h, fmt.Sprintf("%d", vlanB), "Segment-Beta", "Synthetic demo segment beta", "")
	addVLANViaDialog(h, fmt.Sprintf("%d", vlanC), "Segment-Gamma", "Synthetic demo segment gamma", "")

	navigateToZones(t, h)
	addZoneViaDialog(h, "Zone-Red", "Demo trust zone red", fmt.Sprintf("%d,%d", vlanA, vlanB))
	addZoneViaDialog(h, "Zone-Blue", "Demo trust zone blue", fmt.Sprintf("%d", vlanC))

	navigateToSSIDs(t, h)
	addSSIDViaDialog(h, "Mesh-Blue", "Synthetic wireless profile blue")
	addSSIDViaDialog(h, "Mesh-Green", "Synthetic wireless profile green")
	addSSIDViaDialog(h, "Mesh-Red", "Synthetic wireless profile red")

	navigateToEquipmentRoot(t, h)
	addEquipmentViaDialog(h, switchName, "Model-X", "Synthetic demo switch")
	addEquipmentViaDialog(h, routerName, "Model-Y", "Synthetic demo router")

	h.MoveFocusToID(t, routerName)
	h.PressEnter()
	addPortViaDialog(h, "1", "WAN", "RJ45", "2.5GbE", "", "", "", "", "BlockAll", "", "Uplink")
	addPortViaDialog(h, "2", "LAN-A", "RJ45", "2.5GbE", "", "", "", fmt.Sprintf("%d", vlanA), "AllowAll", "", "Clients")
	addPortViaDialog(h, "3", "", "RJ45", "2.5GbE", "", "", "", "", "", "", "")
	h.PressBackspace()

	h.MoveFocusToID(t, switchName)
	h.PressEnter()
	addPortViaDialog(h, "1", "SFP+ 1", "SFP+", "10GbE", "", "", "", "", "AllowAll", "", "Trunk uplink")
	addPortViaDialog(h, "2", "SFP+ 2", "SFP+", "10GbE", "", "", "", "", "AllowAll", "", "Trunk peer")
	addPortViaDialog(h, "3", "Port 3", "RJ45", "2.5GbE", "PoE+", "3", "802.3ad", fmt.Sprintf("%d", vlanB), "BlockAll", "", "Endpoint A")
	addPortViaDialog(h, "4", "Port 4", "RJ45", "2.5GbE", "PoE+", "3", "802.3ad", fmt.Sprintf("%d", vlanB), "BlockAll", "", "Endpoint B")
	addPortViaDialog(h, "5", "", "RJ45", "2.5GbE", "", "", "", "", "", "", "")
	h.PressBackspace()

	navigateToNetworksRoot(t, h)

	addNetworkViaDialog(h, "10.0.0.0/10")
	h.MoveFocusToID(t, "10.0.0.0/10")
	allocateSubnetsFocused(h, "Cloud", "Cloud supernet", "", "11")
	h.PressEnter()

	cloudHalves, err := domain.SplitNetwork("10.0.0.0/10", 11)
	if err != nil || len(cloudHalves) != 2 {
		t.Fatalf("split cloud /10 into /11 failed: %v", err)
	}
	h.MoveFocusToID(t, cloudHalves[0])
	splitFocusedNetwork(h, "12")
	h.MoveFocusToID(t, cloudHalves[1])
	splitFocusedNetwork(h, "12")

	providers := []struct {
		cidr string
		name string
	}{
		{cidr: "10.0.0.0/12", name: "AWS"},
		{cidr: "10.16.0.0/12", name: "GCP"},
		{cidr: "10.32.0.0/12", name: "Azure"},
	}

	for _, provider := range providers {
		h.MoveFocusToID(t, provider.cidr)
		allocateSubnetsFocused(h, provider.name, provider.name+" provider block", "", "13")
		h.PressEnter()

		provider13s, err := domain.SplitNetwork(provider.cidr, 13)
		if err != nil || len(provider13s) != 2 {
			t.Fatalf("split %s into /13 failed: %v", provider.cidr, err)
		}
		h.MoveFocusToID(t, provider13s[0])
		splitFocusedNetwork(h, "14")

		regions, err := domain.SplitNetwork(provider13s[0], 14)
		if err != nil || len(regions) != 2 {
			t.Fatalf("split %s into /14 failed: %v", provider13s[0], err)
		}
		for regionIdx, region := range regions {
			regionName := fmt.Sprintf("%s region-%d", provider.name, regionIdx+1)
			h.MoveFocusToID(t, region)
			allocateSubnetsFocused(h, regionName, "Regional address space", "", "15")
			h.PressEnter()

			region15s, err := domain.SplitNetwork(region, 15)
			if err != nil || len(region15s) != 2 {
				t.Fatalf("split %s into /15 failed: %v", region, err)
			}
			h.MoveFocusToID(t, region15s[0])
			splitFocusedNetwork(h, "16")

			vpcs, err := domain.SplitNetwork(region15s[0], 16)
			if err != nil || len(vpcs) != 2 {
				t.Fatalf("split %s into /16 failed: %v", region15s[0], err)
			}
			h.MoveFocusToID(t, vpcs[0])
			allocateSubnetsFocused(h, "Primary VPC", "Regional primary VPC", "", "17")
			h.PressEnter()

			vpc17s, err := domain.SplitNetwork(vpcs[0], 17)
			if err != nil || len(vpc17s) != 2 {
				t.Fatalf("split %s into /17 failed: %v", vpcs[0], err)
			}
			h.MoveFocusToID(t, vpc17s[0])
			splitFocusedNetwork(h, "18")
			h.MoveFocusToID(t, vpc17s[1])
			splitFocusedNetwork(h, "18")

			left18s, err := domain.SplitNetwork(vpc17s[0], 18)
			if err != nil || len(left18s) != 2 {
				t.Fatalf("split %s into /18 failed: %v", vpc17s[0], err)
			}
			right18s, err := domain.SplitNetwork(vpc17s[1], 18)
			if err != nil || len(right18s) != 2 {
				t.Fatalf("split %s into /18 failed: %v", vpc17s[1], err)
			}
			subnetTypes := slices.Concat(left18s, right18s)
			if len(subnetTypes) < 3 {
				t.Fatalf("expected at least 3 subnet type blocks in %s", vpcs[0])
			}
			typeNames := []string{"Public", "Private", "Backend"}
			for i := range 3 {
				h.MoveFocusToID(t, subnetTypes[i])
				allocateSubnetsFocused(h, typeNames[i], typeNames[i]+" subnet type", "", "19")
				h.PressEnter()

				azs, err := domain.SplitNetwork(subnetTypes[i], 19)
				if err != nil || len(azs) != 2 {
					t.Fatalf("split %s into /19 failed: %v", subnetTypes[i], err)
				}
				h.MoveFocusToID(t, azs[0])
				allocateHostsFocused(h, "AZ-a", "Availability zone A", "")
				h.PressEnter()
				base := strings.Split(azs[0], "/")[0]
				octets := strings.Split(base, ".")
				if len(octets) != 4 {
					t.Fatalf("expected IPv4 subnet for AZ-a, got %s", azs[0])
				}
				prefix3 := strings.Join(octets[:3], ".")
				reserveIPFromCurrentNetwork(h, prefix3+".1", "gateway", "Default gateway")
				h.PressBackspace()
				h.MoveFocusToID(t, azs[1])
				allocateHostsFocused(h, "AZ-b", "Availability zone B", "")
				h.PressBackspace()
			}
			h.PressBackspace()
			h.PressBackspace()
		}
		h.PressBackspace()
	}

	navigateToNetworksRoot(t, h)
	addNetworkViaDialog(h, "192.168.0.0/16")
	h.MoveFocusToID(t, "192.168.0.0/16")
	allocateSubnetsFocused(h, "Home", "Home supernet with VLAN segments", "", "17")
	h.PressEnter()

	activeCIDR := "192.168.0.0/17"
	for prefix := 18; prefix <= 22; prefix++ {
		h.MoveFocusToID(t, activeCIDR)
		splitFocusedNetwork(h, fmt.Sprintf("%d", prefix))
		childCIDRs, err := domain.SplitNetwork(activeCIDR, prefix)
		if err != nil {
			t.Fatalf("split %s into /%d: %v", activeCIDR, prefix, err)
		}
		activeCIDR = childCIDRs[0]
	}
	h.MoveFocusToID(t, activeCIDR)
	splitFocusedNetwork(h, "24")

	homeCIDRs, err := domain.SplitNetwork(activeCIDR, 24)
	if err != nil || len(homeCIDRs) < 3 {
		t.Fatalf("split home into /24 failed: %v", err)
	}
	h.MoveFocusToID(t, homeCIDRs[0])
	allocateHostsFocused(h, "Home Infra", "Routers and servers", fmt.Sprintf("%d", vlanA))
	h.PressEnter()
	reserveIPFromCurrentNetwork(h, "192.168.0.1", "gateway", "Default gateway")
	reserveIPFromCurrentNetwork(h, "192.168.0.10", "nas", "NAS")
	h.PressBackspace()
	h.MoveFocusToID(t, homeCIDRs[1])
	allocateHostsFocused(h, "Home Users", "Laptops and phones", fmt.Sprintf("%d", vlanB))
	h.PressEnter()
	reserveIPFromCurrentNetwork(h, "192.168.1.1", "gateway", "Default gateway")
	reserveIPFromCurrentNetwork(h, "192.168.1.50", "printer", "Office printer")
	h.PressBackspace()
	h.MoveFocusToID(t, homeCIDRs[2])
	allocateHostsFocused(h, "Home IoT", "Cameras and sensors", fmt.Sprintf("%d", vlanC))
	h.PressEnter()
	reserveIPFromCurrentNetwork(h, "192.168.2.1", "gateway", "Default gateway")
	reserveIPFromCurrentNetwork(h, "192.168.2.20", "camera-nvr", "NVR")
	h.PressBackspace()

	navigateToNetworksRoot(t, h)
	addNetworkViaDialog(h, "fd42::/56")
	h.MoveFocusToID(t, "fd42::/56")
	allocateSubnetsFocused(h, "Home IPv6", "Home IPv6 supernet", "", "57")
	h.PressEnter()

	activeV6CIDR := "fd42::/57"
	for prefix := 58; prefix <= 63; prefix++ {
		h.MoveFocusToID(t, activeV6CIDR)
		splitFocusedNetwork(h, fmt.Sprintf("%d", prefix))
		childCIDRs, err := domain.SplitNetwork(activeV6CIDR, prefix)
		if err != nil {
			t.Fatalf("split %s into /%d: %v", activeV6CIDR, prefix, err)
		}
		activeV6CIDR = childCIDRs[0]
	}
	h.MoveFocusToID(t, activeV6CIDR)
	splitFocusedNetwork(h, "64")

	homeV6CIDRs, err := domain.SplitNetwork(activeV6CIDR, 64)
	if err != nil || len(homeV6CIDRs) < 2 {
		t.Fatalf("split home IPv6 into /64 failed: %v", err)
	}
	h.MoveFocusToID(t, homeV6CIDRs[0])
	allocateHostsFocused(h, "Home Infra v6", "Routers and servers v6", "")
	h.PressEnter()
	base0 := strings.Split(homeV6CIDRs[0], "/")[0]
	reserveIPFromCurrentNetwork(h, strings.Replace(base0, "::", "::1", 1), "gateway-v6", "Default gateway IPv6")
	reserveIPFromCurrentNetwork(h, strings.Replace(base0, "::", "::53", 1), "dns-v6", "Resolver IPv6")
	h.PressBackspace()
	h.MoveFocusToID(t, homeV6CIDRs[1])
	allocateHostsFocused(h, "Home Users v6", "Laptops and phones v6", "")
	h.PressEnter()
	base1 := strings.Split(homeV6CIDRs[1], "/")[0]
	reserveIPFromCurrentNetwork(h, strings.Replace(base1, "::", "::1", 1), "gateway-v6", "Default gateway IPv6")
	navigateToNetworksRoot(t, h)

	h.PressCtrl('s')
	h.AssertStatusContains("Saved to .ez-ipam/ and EZ-IPAM.md")
	h.AssertGoldenSnapshot("g03_s01_demo_state_main")
	h.SaveDemoArtifactsToRepo()
}
