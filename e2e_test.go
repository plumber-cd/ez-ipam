package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func currentFocusID() string {
	done := make(chan string, 1)
	app.QueueUpdateDraw(func() {
		if currentMenuFocus == nil {
			done <- ""
			return
		}
		done <- currentMenuFocus.GetID()
	})
	return <-done
}

func currentKeys() ([]string, []string) {
	done := make(chan struct {
		menu  []string
		focus []string
	}, 1)
	app.QueueUpdateDraw(func() {
		menuCopy := append([]string{}, currentMenuItemKeys...)
		focusCopy := append([]string{}, currentFocusKeys...)
		done <- struct {
			menu  []string
			focus []string
		}{menu: menuCopy, focus: focusCopy}
	})
	v := <-done
	return v.menu, v.focus
}

func currentStatusText() string {
	done := make(chan string, 1)
	app.QueueUpdateDraw(func() {
		done <- statusLine.GetText(true)
	})
	return <-done
}

func focusMatches(id string) bool {
	return strings.HasPrefix(currentFocusID(), id)
}

func moveFocusToID(t *testing.T, h *TestHarness, id string) {
	t.Helper()
	for i := 0; i < 40; i++ {
		h.PressRune('k')
	}
	if focusMatches(id) {
		return
	}
	for i := 0; i < 200; i++ {
		if focusMatches(id) {
			return
		}
		h.PressRune('j')
	}
	t.Fatalf("could not focus item %q; current=%q", id, currentFocusID())
}

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
	h.PressTab()
	h.TypeText(vlanID)
	h.PressTab()
	h.TypeText(prefix)
	h.PressEnter()
}

func allocateHostsFocused(h *TestHarness, name, description, vlanID string) {
	h.PressRune('A')
	h.AssertScreenContains("Host Pool")
	h.TypeText(name)
	h.PressTab()
	h.TypeText(description)
	h.PressTab()
	h.TypeText(vlanID)
	h.PressTab()
	h.PressEnter()
}

func updateAllocationFocused(h *TestHarness, nameSuffix, descriptionSuffix, vlanID string) {
	h.PressRune('u')
	h.AssertScreenContains("Update Metadata")
	h.TypeText(nameSuffix)
	h.PressTab()
	h.TypeText(descriptionSuffix)
	h.PressTab()
	h.TypeText(vlanID)
	h.PressTab()
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
	moveFocusToID(t, h, "VLANs")
	h.PressEnter()
	h.AssertScreenContains("│VLANs")
}

func navigateToSSIDs(t *testing.T, h *TestHarness) {
	t.Helper()
	h.PressBackspace()
	moveFocusToID(t, h, "WiFi SSIDs")
	h.PressEnter()
	h.AssertScreenContains("│WiFi SSIDs")
}

func navigateToNetworksRoot(t *testing.T, h *TestHarness) {
	t.Helper()
	for i := 0; i < 16; i++ {
		h.PressBackspace()
	}
	moveFocusToID(t, h, "Networks")
	h.PressEnter()
	h.AssertScreenContains("│Networks")
}

func addVLANViaDialog(h *TestHarness, id, name, description string) {
	h.PressRune('v')
	h.AssertScreenContains("Add VLAN")
	h.TypeText(id)
	h.PressTab()
	h.TypeText(name)
	h.PressTab()
	h.TypeText(description)
	h.PressTab()
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
	for i := 0; i < 16; i++ {
		h.PressBackspace()
	}
	moveFocusToID(t, h, "Zones")
	h.PressEnter()
	h.AssertScreenContains("│Zones")
}

func navigateToEquipmentRoot(t *testing.T, h *TestHarness) {
	t.Helper()
	for i := 0; i < 16; i++ {
		h.PressBackspace()
	}
	moveFocusToID(t, h, "Equipment")
	h.PressEnter()
	h.AssertScreenContains("│Equipment")
}

func addZoneViaDialog(h *TestHarness, name, description, vlanIDs string) {
	h.PressRune('z')
	h.AssertScreenContains("Add Zone")
	h.TypeText(name)
	h.PressTab()
	h.TypeText(description)
	h.PressTab()
	h.TypeText(vlanIDs)
	h.PressTab()
	h.PressEnter()
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
	h.PressTab()
	h.TypeText(lagGroup)
	h.PressTab()
	h.TypeText(lagMode)
	h.PressTab()
	h.TypeText(nativeVLAN)
	h.PressTab()
	h.TypeText(taggedMode)
	h.PressTab()
	h.TypeText(taggedVLANIDs)
	h.PressTab()
	h.TypeText(description)
	h.PressTab()
	h.PressEnter()
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
	menuKeys, _ := currentKeys()
	if len(menuKeys) != 1 || menuKeys[0] != "<n> New Network" {
		t.Fatalf("unexpected menu keys: %#v", menuKeys)
	}

	addNetworkViaDialog(h, "10.0.0.0/24")
	addNetworkViaDialog(h, "10.0.1.0/24")

	moveFocusToID(t, h, "10.0.0.0/24")
	_, focusKeys := currentKeys()
	if !strings.Contains(strings.Join(focusKeys, " | "), "<a> Allocate Subnet Container") {
		t.Fatalf("expected allocation keys on unallocated network, got %#v", focusKeys)
	}
	h.PressRune('j')
	if !focusMatches("10.0.1.0/24") {
		t.Fatalf("expected focus on 10.0.1.0/24, got %q", currentFocusID())
	}
	h.PressRune('k')
	if !focusMatches("10.0.0.0/24") {
		t.Fatalf("expected focus back on 10.0.0.0/24, got %q", currentFocusID())
	}
	h.PressRune('l')
	h.AssertScreenContains("10.0.0.0/24")
	h.PressRune('h')
	h.AssertScreenContains("│Networks")

	moveFocusToID(t, h, "10.0.0.0/24")
	allocateHostsFocused(h, "Web", "Tier", "")
	h.AssertScreenContains("Allocated network")
	h.PressEnter()
	menuKeys, _ = currentKeys()
	if !strings.Contains(strings.Join(menuKeys, " | "), "<r> Reserve IP") {
		t.Fatalf("expected reserve key on hosts network, got %#v", menuKeys)
	}
	reserveIPFromCurrentNetwork(h, "10.0.0.1", "gw", "gateway")
	h.AssertScreenContains("Reserved IP")
	_, focusKeys = currentKeys()
	if !strings.Contains(strings.Join(focusKeys, " | "), "<u> Update Reservation") {
		t.Fatalf("expected ip reservation keys, got %#v", focusKeys)
	}

	moveFocusToID(t, h, "10.0.0.1 (gw)")
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
		moveFocusToID(t, h, "10.0.0.0/24")
		return h
	}

	t.Run("invalid_prefix_input", func(t *testing.T) {
		h := setup(t)
		splitFocusedNetwork(h, "bad")
		if !strings.Contains(currentStatusText(), "Invalid prefix length") {
			t.Fatalf("expected invalid prefix error, got %q", currentStatusText())
		}
	})

	t.Run("prefix_too_small", func(t *testing.T) {
		h := setup(t)
		splitFocusedNetwork(h, "23")
		if !strings.Contains(currentStatusText(), "Error splitting network") {
			t.Fatalf("expected split error, got %q", currentStatusText())
		}
	})

	t.Run("prefix_too_large", func(t *testing.T) {
		h := setup(t)
		splitFocusedNetwork(h, "33")
		if !strings.Contains(currentStatusText(), "Error splitting network") {
			t.Fatalf("expected split error, got %q", currentStatusText())
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
	moveFocusToID(t, h, "10.0.0.0/25")

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
		moveFocusToID(t, h, "10.0.0.0/24")

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
		moveFocusToID(t, h, "10.0.0.0/24")
		splitFocusedNetwork(h, "25")
		moveFocusToID(t, h, "10.0.0.0/25")

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
		moveFocusToID(t, h, "10.0.0.0/24")
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
		if !strings.Contains(currentStatusText(), "Invalid subnet prefix length") {
			t.Fatalf("expected invalid subnet prefix error, got %q", currentStatusText())
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
	moveFocusToID(t, h, "192.168.1.0/24")

	allocateHostsFocused(h, "", "", "")
	h.AssertStatusContains("Error allocating network")

	allocateHostsFocused(h, "Office", "LAN", "")
	h.AssertStatusContains("Allocated network")
}

func TestUpdateAllocationAndDeallocate(t *testing.T) {
	h := NewTestHarness(t)
	h.NavigateToNetworks()
	addNetworkViaDialog(h, "10.0.0.0/24")
	moveFocusToID(t, h, "10.0.0.0/24")
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
	moveFocusToID(t, h, "192.168.1.0/24")
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

	moveFocusToID(t, h, "192.168.1.1 (gateway)")
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

	addVLANViaDialog(h, "100", "Management", "infra management")
	h.AssertStatusContains("Added VLAN")

	addVLANViaDialog(h, "0", "Invalid", "bad")
	h.AssertStatusContains("Error adding VLAN")
	addVLANViaDialog(h, "4095", "Invalid", "bad")
	h.AssertStatusContains("Error adding VLAN")
	addVLANViaDialog(h, "bad", "Invalid", "bad")
	h.AssertStatusContains("Error adding VLAN")
	addVLANViaDialog(h, "101", "", "missing name")
	h.AssertStatusContains("Error adding VLAN")
	addVLANViaDialog(h, "100", "Duplicate", "dup")
	h.AssertStatusContains("Error adding VLAN")
}

func TestUpdateAndDeleteVLAN(t *testing.T) {
	h := NewTestHarness(t)
	navigateToVLANs(t, h)
	addVLANViaDialog(h, "200", "Users", "user segment")
	moveFocusToID(t, h, "200 (Users)")

	h.PressRune('u')
	h.AssertScreenContains("Update VLAN")
	h.TypeText("-2")
	h.PressTab()
	h.TypeText(" updated")
	h.PressTab()
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
	moveFocusToID(t, h, "Home-IoT")

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
	addVLANViaDialog(h, "300", "Servers", "servers vlan")

	navigateToNetworksRoot(t, h)
	addNetworkViaDialog(h, "10.50.0.0/24")
	moveFocusToID(t, h, "10.50.0.0/24")
	allocateHostsFocused(h, "Servers", "pool", "300")
	h.AssertStatusContains("Allocated network")
	h.AssertScreenContains("300 (Servers)")

	addNetworkViaDialog(h, "10.51.0.0/24")
	moveFocusToID(t, h, "10.51.0.0/24")
	allocateHostsFocused(h, "Invalid", "pool", "999")
	h.AssertStatusContains("Error allocating network")
}

func TestVLANCrossReference(t *testing.T) {
	h := NewTestHarness(t)
	navigateToVLANs(t, h)
	addVLANViaDialog(h, "400", "Apps", "application vlan")

	navigateToNetworksRoot(t, h)
	addNetworkViaDialog(h, "10.60.0.0/24")
	moveFocusToID(t, h, "10.60.0.0/24")
	allocateHostsFocused(h, "Apps", "pool", "400")
	h.AssertStatusContains("Allocated network")

	h.PressBackspace()
	navigateToVLANs(t, h)
	moveFocusToID(t, h, "400 (Apps)")
	h.AssertScreenContains("10.60.0.0/24")
}

func TestZonesAndVLANCascade(t *testing.T) {
	h := NewTestHarness(t)
	navigateToVLANs(t, h)
	addVLANViaDialog(h, "101", "Main", "main vlan")
	h.AssertStatusContains("Added VLAN")

	navigateToZones(t, h)
	addZoneViaDialog(h, "Main", "trusted clients", "101")
	h.AssertStatusContains("Added Zone")
	moveFocusToID(t, h, "Main")
	h.AssertScreenContains("101 (Main)")

	navigateToVLANs(t, h)
	moveFocusToID(t, h, "101 (Main)")
	h.PressRune('D')
	h.ConfirmModal()
	h.AssertStatusContains("Deleted VLAN")

	navigateToZones(t, h)
	moveFocusToID(t, h, "Main")
	h.AssertScreenContains("Associated VLANs")
	h.AssertScreenContains("<none>")
}

func TestEquipmentPortAndConnections(t *testing.T) {
	h := NewTestHarness(t)
	navigateToEquipmentRoot(t, h)
	addEquipmentViaDialog(h, "Gateway", "UCG-Fiber", "edge")
	addEquipmentViaDialog(h, "Lab", "USW-Enterprise-8-PoE", "lab switch")
	h.AssertStatusContains("Added Equipment")

	moveFocusToID(t, h, "Gateway (UCG-Fiber)")
	h.PressEnter()
	addPortViaDialog(h, "1", "WAN1", "RJ45", "2.5GbE", "", "", "", "", "", "", "uplink")
	h.AssertStatusContains("Added Port")
	addPortViaDialog(h, "2", "", "RJ45", "2.5GbE", "", "", "", "", "", "", "")
	h.AssertStatusContains("Added Port")

	h.PressBackspace()
	moveFocusToID(t, h, "Lab (USW-Enterprise-8-PoE)")
	h.PressEnter()
	addPortViaDialog(h, "1", "SFP+ 1", "SFP+", "10GbE", "", "", "", "", "", "", "")
	h.AssertStatusContains("Added Port")
	addPortViaDialog(h, "4", "Port 4", "RJ45", "2.5GbE", "", "3", "802.3ad", "", "", "", "")
	h.AssertStatusContains("Error adding Port")

	h.PressBackspace()
	moveFocusToID(t, h, "Gateway (UCG-Fiber)")
	h.PressEnter()
	moveFocusToID(t, h, "1: WAN1")
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
	addVLANViaDialog(h, "104", "Servers", "servers vlan")

	navigateToZones(t, h)
	addZoneViaDialog(h, "Servers", "server zone", "104")

	navigateToEquipmentRoot(t, h)
	addEquipmentViaDialog(h, "Servers", "USW-Aggregation", "agg switch")
	moveFocusToID(t, h, "Servers (USW-Aggregation)")
	h.PressEnter()
	addPortViaDialog(h, "1", "SFP+ 1", "SFP+", "10GbE", "", "", "", "104", "BlockAll", "", "proxmox uplink")

	h.PressCtrl('s')
	h.AssertStatusContains("Saved to .ez-ipam/ and EZ-IPAM.md")

	mdBytes, err := os.ReadFile(filepath.Join(h.workDir, markdownFileName))
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

		if _, err := os.Stat(filepath.Join(h.workDir, dataDirName, networksDirName)); err != nil {
			t.Fatalf("networks dir missing: %v", err)
		}
		mdBytes, err := os.ReadFile(filepath.Join(h.workDir, markdownFileName))
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

func TestGoldenDialogsAndScreens(t *testing.T) {
	h := NewTestHarness(t)
	h.NavigateToNetworks()
	addNetworkViaDialog(h, "10.0.0.0/24")
	addNetworkViaDialog(h, "fd00::/64")

	moveFocusToID(t, h, "10.0.0.0/24")
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
	moveFocusToID(t, h, "10.0.0.0/24")
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
	moveFocusToID(t, h, "10.0.0.1 (gw)")
	h.AssertGoldenSnapshot("g02_s12_ip_details")

	h.PressRune('u')
	assertPrimaryCancelVisible(t, h, "Save")
	h.AssertGoldenSnapshot("g02_s13_update_ip_dialog")
	h.PressEscape()

	h.PressRune('R')
	h.AssertGoldenSnapshot("g02_s14_unreserve_confirm_modal")
	h.CancelModal()

	h.PressBackspace()
	moveFocusToID(t, h, "fd00::/64")
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
	addVLANViaDialog(h, fmt.Sprintf("%d", vlanA), "Segment-Alpha", "Synthetic demo segment alpha")
	addVLANViaDialog(h, fmt.Sprintf("%d", vlanB), "Segment-Beta", "Synthetic demo segment beta")
	addVLANViaDialog(h, fmt.Sprintf("%d", vlanC), "Segment-Gamma", "Synthetic demo segment gamma")

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

	moveFocusToID(t, h, routerName)
	h.PressEnter()
	addPortViaDialog(h, "1", "WAN", "RJ45", "2.5GbE", "", "", "", "", "BlockAll", "", "Uplink")
	addPortViaDialog(h, "2", "LAN-A", "RJ45", "2.5GbE", "", "", "", fmt.Sprintf("%d", vlanA), "AllowAll", "", "Clients")
	addPortViaDialog(h, "3", "", "RJ45", "2.5GbE", "", "", "", "", "", "", "")
	h.PressBackspace()

	moveFocusToID(t, h, switchName)
	h.PressEnter()
	addPortViaDialog(h, "1", "SFP+ 1", "SFP+", "10GbE", "", "", "", "", "AllowAll", "", "Trunk uplink")
	addPortViaDialog(h, "2", "SFP+ 2", "SFP+", "10GbE", "", "", "", "", "AllowAll", "", "Trunk peer")
	addPortViaDialog(h, "3", "Port 3", "RJ45", "2.5GbE", "PoE+", "3", "802.3ad", fmt.Sprintf("%d", vlanB), "BlockAll", "", "Endpoint A")
	addPortViaDialog(h, "4", "Port 4", "RJ45", "2.5GbE", "PoE+", "3", "802.3ad", fmt.Sprintf("%d", vlanB), "BlockAll", "", "Endpoint B")
	addPortViaDialog(h, "5", "", "RJ45", "2.5GbE", "", "", "", "", "", "", "")
	h.PressBackspace()

	navigateToNetworksRoot(t, h)

	addNetworkViaDialog(h, "10.0.0.0/10")
	moveFocusToID(t, h, "10.0.0.0/10")
	allocateSubnetsFocused(h, "Cloud", "Cloud supernet", "", "11")
	h.PressEnter()

	cloudHalves, err := splitNetwork("10.0.0.0/10", 11)
	if err != nil || len(cloudHalves) != 2 {
		t.Fatalf("split cloud /10 into /11 failed: %v", err)
	}
	moveFocusToID(t, h, cloudHalves[0])
	splitFocusedNetwork(h, "12")
	moveFocusToID(t, h, cloudHalves[1])
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
		moveFocusToID(t, h, provider.cidr)
		allocateSubnetsFocused(h, provider.name, provider.name+" provider block", "", "13")
		h.PressEnter()

		provider13s, err := splitNetwork(provider.cidr, 13)
		if err != nil || len(provider13s) != 2 {
			t.Fatalf("split %s into /13 failed: %v", provider.cidr, err)
		}
		moveFocusToID(t, h, provider13s[0])
		splitFocusedNetwork(h, "14")

		regions, err := splitNetwork(provider13s[0], 14)
		if err != nil || len(regions) != 2 {
			t.Fatalf("split %s into /14 failed: %v", provider13s[0], err)
		}
		for regionIdx, region := range regions {
			regionName := fmt.Sprintf("%s region-%d", provider.name, regionIdx+1)
			moveFocusToID(t, h, region)
			allocateSubnetsFocused(h, regionName, "Regional address space", "", "15")
			h.PressEnter()

			region15s, err := splitNetwork(region, 15)
			if err != nil || len(region15s) != 2 {
				t.Fatalf("split %s into /15 failed: %v", region, err)
			}
			moveFocusToID(t, h, region15s[0])
			splitFocusedNetwork(h, "16")

			vpcs, err := splitNetwork(region15s[0], 16)
			if err != nil || len(vpcs) != 2 {
				t.Fatalf("split %s into /16 failed: %v", region15s[0], err)
			}
			moveFocusToID(t, h, vpcs[0])
			allocateSubnetsFocused(h, "Primary VPC", "Regional primary VPC", "", "17")
			h.PressEnter()

			vpc17s, err := splitNetwork(vpcs[0], 17)
			if err != nil || len(vpc17s) != 2 {
				t.Fatalf("split %s into /17 failed: %v", vpcs[0], err)
			}
			moveFocusToID(t, h, vpc17s[0])
			splitFocusedNetwork(h, "18")
			moveFocusToID(t, h, vpc17s[1])
			splitFocusedNetwork(h, "18")

			left18s, err := splitNetwork(vpc17s[0], 18)
			if err != nil || len(left18s) != 2 {
				t.Fatalf("split %s into /18 failed: %v", vpc17s[0], err)
			}
			right18s, err := splitNetwork(vpc17s[1], 18)
			if err != nil || len(right18s) != 2 {
				t.Fatalf("split %s into /18 failed: %v", vpc17s[1], err)
			}
			subnetTypes := append(left18s, right18s...)
			if len(subnetTypes) < 3 {
				t.Fatalf("expected at least 3 subnet type blocks in %s", vpcs[0])
			}
			typeNames := []string{"Public", "Private", "Backend"}
			for i := 0; i < 3; i++ {
				moveFocusToID(t, h, subnetTypes[i])
				allocateSubnetsFocused(h, typeNames[i], typeNames[i]+" subnet type", "", "19")
				h.PressEnter()

				azs, err := splitNetwork(subnetTypes[i], 19)
				if err != nil || len(azs) != 2 {
					t.Fatalf("split %s into /19 failed: %v", subnetTypes[i], err)
				}
				moveFocusToID(t, h, azs[0])
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
				moveFocusToID(t, h, azs[1])
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
	moveFocusToID(t, h, "192.168.0.0/16")
	allocateSubnetsFocused(h, "Home", "Home supernet with VLAN segments", "", "17")
	h.PressEnter()

	activeCIDR := "192.168.0.0/17"
	for prefix := 18; prefix <= 22; prefix++ {
		moveFocusToID(t, h, activeCIDR)
		splitFocusedNetwork(h, fmt.Sprintf("%d", prefix))
		childCIDRs, err := splitNetwork(activeCIDR, prefix)
		if err != nil {
			t.Fatalf("split %s into /%d: %v", activeCIDR, prefix, err)
		}
		activeCIDR = childCIDRs[0]
	}
	moveFocusToID(t, h, activeCIDR)
	splitFocusedNetwork(h, "24")

	homeCIDRs, err := splitNetwork(activeCIDR, 24)
	if err != nil || len(homeCIDRs) < 3 {
		t.Fatalf("split home into /24 failed: %v", err)
	}
	moveFocusToID(t, h, homeCIDRs[0])
	allocateHostsFocused(h, "Home Infra", "Routers and servers", fmt.Sprintf("%d", vlanA))
	h.PressEnter()
	reserveIPFromCurrentNetwork(h, "192.168.0.1", "gateway", "Default gateway")
	reserveIPFromCurrentNetwork(h, "192.168.0.10", "nas", "NAS")
	h.PressBackspace()
	moveFocusToID(t, h, homeCIDRs[1])
	allocateHostsFocused(h, "Home Users", "Laptops and phones", fmt.Sprintf("%d", vlanB))
	h.PressEnter()
	reserveIPFromCurrentNetwork(h, "192.168.1.1", "gateway", "Default gateway")
	reserveIPFromCurrentNetwork(h, "192.168.1.50", "printer", "Office printer")
	h.PressBackspace()
	moveFocusToID(t, h, homeCIDRs[2])
	allocateHostsFocused(h, "Home IoT", "Cameras and sensors", fmt.Sprintf("%d", vlanC))
	h.PressEnter()
	reserveIPFromCurrentNetwork(h, "192.168.2.1", "gateway", "Default gateway")
	reserveIPFromCurrentNetwork(h, "192.168.2.20", "camera-nvr", "NVR")
	h.PressBackspace()

	navigateToNetworksRoot(t, h)
	addNetworkViaDialog(h, "fd42::/56")
	moveFocusToID(t, h, "fd42::/56")
	allocateSubnetsFocused(h, "Home IPv6", "Home IPv6 supernet", "", "57")
	h.PressEnter()

	activeV6CIDR := "fd42::/57"
	for prefix := 58; prefix <= 63; prefix++ {
		moveFocusToID(t, h, activeV6CIDR)
		splitFocusedNetwork(h, fmt.Sprintf("%d", prefix))
		childCIDRs, err := splitNetwork(activeV6CIDR, prefix)
		if err != nil {
			t.Fatalf("split %s into /%d: %v", activeV6CIDR, prefix, err)
		}
		activeV6CIDR = childCIDRs[0]
	}
	moveFocusToID(t, h, activeV6CIDR)
	splitFocusedNetwork(h, "64")

	homeV6CIDRs, err := splitNetwork(activeV6CIDR, 64)
	if err != nil || len(homeV6CIDRs) < 2 {
		t.Fatalf("split home IPv6 into /64 failed: %v", err)
	}
	moveFocusToID(t, h, homeV6CIDRs[0])
	allocateHostsFocused(h, "Home Infra v6", "Routers and servers v6", "")
	h.PressEnter()
	base0 := strings.Split(homeV6CIDRs[0], "/")[0]
	reserveIPFromCurrentNetwork(h, strings.Replace(base0, "::", "::1", 1), "gateway-v6", "Default gateway IPv6")
	reserveIPFromCurrentNetwork(h, strings.Replace(base0, "::", "::53", 1), "dns-v6", "Resolver IPv6")
	h.PressBackspace()
	moveFocusToID(t, h, homeV6CIDRs[1])
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
