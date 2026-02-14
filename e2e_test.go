package main

import (
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
	runSync := func(fn func()) {
		done := make(chan struct{})
		app.QueueUpdateDraw(func() {
			fn()
			close(done)
		})
		<-done
	}

	mustAddNetwork := func(cidr, parentPath, name, description string, mode AllocationMode, vlanID int) string {
		network := &Network{
			MenuFolder: &MenuFolder{
				ID:         cidr,
				ParentPath: parentPath,
			},
			AllocationMode: mode,
			DisplayName:    name,
			Description:    description,
			VLANID:         vlanID,
		}
		menuItems.MustAdd(network)
		return network.GetPath()
	}
	mustAddIP := func(address, parentPath, name, description string) {
		menuItems.MustAdd(&IP{
			MenuFolder: &MenuFolder{
				ID:         address,
				ParentPath: parentPath,
			},
			DisplayName: name,
			Description: description,
		})
	}

	runSync(func() {
		vlansRoot := menuItems.GetByParentAndID(nil, "VLANs")
		menuItems.MustAdd(&VLAN{MenuFolder: &MenuFolder{ID: "10", ParentPath: vlansRoot.GetPath()}, DisplayName: "Home-Infra", Description: "Home infrastructure"})
		menuItems.MustAdd(&VLAN{MenuFolder: &MenuFolder{ID: "20", ParentPath: vlansRoot.GetPath()}, DisplayName: "Home-Users", Description: "User devices"})
		menuItems.MustAdd(&VLAN{MenuFolder: &MenuFolder{ID: "30", ParentPath: vlansRoot.GetPath()}, DisplayName: "Home-IoT", Description: "IoT devices"})

		networksRoot := menuItems.GetByParentAndID(nil, "Networks").GetPath()
		cloudPath := mustAddNetwork("10.0.0.0/10", networksRoot, "Cloud", "Cloud supernet", AllocationModeSubnets, 0)
		homePath := mustAddNetwork("192.168.0.0/16", networksRoot, "Home", "Home supernet with VLAN segments", AllocationModeSubnets, 0)

		type cloud struct {
			ipv4 string
			name string
		}
		clouds := []cloud{
			{ipv4: "10.0.0.0/12", name: "AWS"},
			{ipv4: "10.16.0.0/12", name: "GCP"},
			{ipv4: "10.32.0.0/12", name: "Azure"},
		}

		for _, c := range clouds {
			cloudBlockPath := mustAddNetwork(c.ipv4, cloudPath, c.name, c.name+" provider block", AllocationModeSubnets, 0)

			var regionA, regionB, regionSpare string
			switch c.ipv4 {
			case "10.0.0.0/12":
				regionA, regionB, regionSpare = "10.0.0.0/14", "10.4.0.0/14", "10.8.0.0/13"
			case "10.16.0.0/12":
				regionA, regionB, regionSpare = "10.16.0.0/14", "10.20.0.0/14", "10.24.0.0/13"
			default:
				regionA, regionB, regionSpare = "10.32.0.0/14", "10.36.0.0/14", "10.40.0.0/13"
			}
			regionAPath := mustAddNetwork(regionA, cloudBlockPath, c.name+" us-east-1", "Primary region", AllocationModeSubnets, 0)
			regionBPath := mustAddNetwork(regionB, cloudBlockPath, c.name+" eu-west-1", "Secondary region", AllocationModeSubnets, 0)
			mustAddNetwork(regionSpare, cloudBlockPath, "", "", AllocationModeUnallocated, 0)

			regionPaths := []string{regionAPath, regionBPath}
			for _, regionPath := range regionPaths {
				var vpcCIDR, spare18, spare17 string
				switch {
				case strings.Contains(regionPath, "10.0.0.0/14"):
					vpcCIDR, spare18, spare17 = "10.0.0.0/18", "10.0.64.0/18", "10.0.128.0/17"
				case strings.Contains(regionPath, "10.4.0.0/14"):
					vpcCIDR, spare18, spare17 = "10.4.0.0/18", "10.4.64.0/18", "10.4.128.0/17"
				case strings.Contains(regionPath, "10.16.0.0/14"):
					vpcCIDR, spare18, spare17 = "10.16.0.0/18", "10.16.64.0/18", "10.16.128.0/17"
				case strings.Contains(regionPath, "10.20.0.0/14"):
					vpcCIDR, spare18, spare17 = "10.20.0.0/18", "10.20.64.0/18", "10.20.128.0/17"
				case strings.Contains(regionPath, "10.32.0.0/14"):
					vpcCIDR, spare18, spare17 = "10.32.0.0/18", "10.32.64.0/18", "10.32.128.0/17"
				default:
					vpcCIDR, spare18, spare17 = "10.36.0.0/18", "10.36.64.0/18", "10.36.128.0/17"
				}

				vpcPath := mustAddNetwork(vpcCIDR, regionPath, "Primary VPC", "Regional primary VPC", AllocationModeSubnets, 0)
				mustAddNetwork(spare18, regionPath, "", "", AllocationModeUnallocated, 0)
				mustAddNetwork(spare17, regionPath, "", "", AllocationModeUnallocated, 0)

				var public20, private20, backend20, spare20 string
				switch vpcCIDR {
				case "10.0.0.0/18":
					public20, private20, backend20, spare20 = "10.0.0.0/20", "10.0.16.0/20", "10.0.32.0/20", "10.0.48.0/20"
				case "10.4.0.0/18":
					public20, private20, backend20, spare20 = "10.4.0.0/20", "10.4.16.0/20", "10.4.32.0/20", "10.4.48.0/20"
				case "10.16.0.0/18":
					public20, private20, backend20, spare20 = "10.16.0.0/20", "10.16.16.0/20", "10.16.32.0/20", "10.16.48.0/20"
				case "10.20.0.0/18":
					public20, private20, backend20, spare20 = "10.20.0.0/20", "10.20.16.0/20", "10.20.32.0/20", "10.20.48.0/20"
				case "10.32.0.0/18":
					public20, private20, backend20, spare20 = "10.32.0.0/20", "10.32.16.0/20", "10.32.32.0/20", "10.32.48.0/20"
				default:
					public20, private20, backend20, spare20 = "10.36.0.0/20", "10.36.16.0/20", "10.36.32.0/20", "10.36.48.0/20"
				}

				typePath := mustAddNetwork(public20, vpcPath, "Public", "Public subnet type", AllocationModeSubnets, 0)
				mustAddNetwork(private20, vpcPath, "Private", "Private subnet type", AllocationModeSubnets, 0)
				mustAddNetwork(backend20, vpcPath, "Backend", "Backend subnet type", AllocationModeSubnets, 0)
				mustAddNetwork(spare20, vpcPath, "", "", AllocationModeUnallocated, 0)

				var azA, azB, azSpare string
				switch public20 {
				case "10.0.0.0/20":
					azA, azB, azSpare = "10.0.0.0/22", "10.0.4.0/22", "10.0.8.0/21"
				case "10.4.0.0/20":
					azA, azB, azSpare = "10.4.0.0/22", "10.4.4.0/22", "10.4.8.0/21"
				case "10.16.0.0/20":
					azA, azB, azSpare = "10.16.0.0/22", "10.16.4.0/22", "10.16.8.0/21"
				case "10.20.0.0/20":
					azA, azB, azSpare = "10.20.0.0/22", "10.20.4.0/22", "10.20.8.0/21"
				case "10.32.0.0/20":
					azA, azB, azSpare = "10.32.0.0/22", "10.32.4.0/22", "10.32.8.0/21"
				default:
					azA, azB, azSpare = "10.36.0.0/22", "10.36.4.0/22", "10.36.8.0/21"
				}
				azAPath := mustAddNetwork(azA, typePath, "AZ-a", "Availability zone A", AllocationModeHosts, 0)
				mustAddNetwork(azB, typePath, "AZ-b", "Availability zone B", AllocationModeHosts, 0)
				mustAddNetwork(azSpare, typePath, "", "", AllocationModeUnallocated, 0)
				mustAddIP(strings.Split(azA, "/")[0][:len(strings.Split(azA, "/")[0])-1]+"1", azAPath, "gateway", "Default gateway")
			}

		}

		mustAddNetwork("10.48.0.0/12", cloudPath, "", "", AllocationModeUnallocated, 0)

		homeInfra := mustAddNetwork("192.168.0.0/24", homePath, "Home Infra", "Routers and servers", AllocationModeHosts, 10)
		homeUsers := mustAddNetwork("192.168.1.0/24", homePath, "Home Users", "Laptops and phones", AllocationModeHosts, 20)
		homeIoT := mustAddNetwork("192.168.2.0/24", homePath, "Home IoT", "Cameras and sensors", AllocationModeHosts, 30)
		mustAddNetwork("192.168.4.0/22", homePath, "", "", AllocationModeUnallocated, 0)
		mustAddNetwork("192.168.8.0/21", homePath, "", "", AllocationModeUnallocated, 0)
		mustAddNetwork("192.168.16.0/20", homePath, "", "", AllocationModeUnallocated, 0)
		mustAddNetwork("192.168.32.0/19", homePath, "", "", AllocationModeUnallocated, 0)
		mustAddNetwork("192.168.64.0/18", homePath, "", "", AllocationModeUnallocated, 0)
		mustAddNetwork("192.168.128.0/17", homePath, "", "", AllocationModeUnallocated, 0)
		mustAddIP("192.168.0.1", homeInfra, "gateway", "Default gateway")
		mustAddIP("192.168.0.10", homeInfra, "nas", "NAS")
		mustAddIP("192.168.1.1", homeUsers, "gateway", "Default gateway")
		mustAddIP("192.168.1.50", homeUsers, "printer", "Office printer")
		mustAddIP("192.168.2.1", homeIoT, "gateway", "Default gateway")
		mustAddIP("192.168.2.20", homeIoT, "camera-nvr", "NVR")

		save()
	})

	navigateToNetworksRoot(t, h)

	h.AssertStatusContains("Saved to .ez-ipam/ and EZ-IPAM.md")
	h.AssertGoldenSnapshot("g03_s01_demo_state_main")
	h.SaveDemoArtifactsToRepo()
}
