package main

import (
	"cmp"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/netip"
	"slices"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type NetworkValidationRules struct {
	IgnoreOverlapsWith []*Network
	Substitutes        map[string]*Network
}

func AddNewNetwork(cidr string) {
	newNet := &Network{
		MenuFolder: &MenuFolder{
			ID:         cidr,
			ParentPath: currentMenuItem.GetPath(),
		},
	}

	if err := newNet.Validate(); err != nil {
		statusLine.Clear()
		statusLine.SetText("Error adding new network: " + err.Error())
		return
	}

	menuItems.MustAdd(newNet)

	reloadMenu(newNet)

	statusLine.Clear()
	statusLine.SetText("Added new network: " + cidr)
}

func SplitNetwork(newPrefix int) {
	focusedNetwork, ok := currentMenuFocus.(*Network)
	if !ok {
		panic("SplitNetwork called on non-Network")
	}

	if focusedNetwork.AllocationMode != AllocationModeUnallocated {
		panic("SplitNetwork called on allocated Network")
	}

	newNetworks, err := splitNetwork(focusedNetwork.ID, newPrefix)
	if err != nil {
		statusLine.Clear()
		statusLine.SetText("Error splitting network: " + err.Error())
		return
	}

	newMenuItems := []*Network{}
	for _, newNet := range newNetworks {
		newMenuItem := &Network{
			MenuFolder: &MenuFolder{
				ID:         newNet,
				ParentPath: focusedNetwork.GetParent().GetPath(),
			},
		}
		if err := newMenuItem.ValidateWithRules(&NetworkValidationRules{IgnoreOverlapsWith: []*Network{focusedNetwork}}); err != nil {
			statusLine.Clear()
			statusLine.SetText("Error adding newly split network: " + err.Error())
			return
		}
		newMenuItems = append(newMenuItems, newMenuItem)
	}

	menuItems.Delete(focusedNetwork)

	for _, newMenuItem := range newMenuItems {
		menuItems.MustAdd(newMenuItem)
	}

	reloadMenu(focusedNetwork)

	statusLine.Clear()
	statusLine.SetText("Split network " + focusedNetwork.GetPath() + " into " + fmt.Sprintf("%d subnets", len(newNetworks)))
}

func SummarizeNetwork() {
	focusedNetwork, ok := currentMenuFocus.(*Network)
	if !ok {
		statusLine.Clear()
		statusLine.SetText("Error summarizing network: selected item is not a network")
		return
	}

	if focusedNetwork.AllocationMode != AllocationModeUnallocated {
		statusLine.Clear()
		statusLine.SetText("Error summarizing network: network is allocated")
		return
	}

	summarizeable, summarizeableNetworks, newNetwork := findSummarizableRangeForNetwork(focusedNetwork)
	if !summarizeable {
		statusLine.Clear()
		statusLine.SetText("Error summarizing network: no summarizable unallocated range found")
		return
	}

	newMenuItem := &Network{
		MenuFolder: &MenuFolder{
			ID:         newNetwork,
			ParentPath: focusedNetwork.GetParent().GetPath(),
		},
	}
	if err := newMenuItem.ValidateWithRules(&NetworkValidationRules{IgnoreOverlapsWith: summarizeableNetworks}); err != nil {
		statusLine.Clear()
		statusLine.SetText("Error validating summarized network: " + err.Error())
		return
	}

	for _, summarizeableNeighbor := range summarizeableNetworks {
		if summarizeableNeighbor == nil {
			statusLine.Clear()
			statusLine.SetText("Error summarizing network: internal mapping error")
			return
		}
		menuItems.Delete(summarizeableNeighbor)
	}

	menuItems.MustAdd(newMenuItem)

	reloadMenu(newMenuItem)

	statusLine.Clear()
	statusLine.SetText("Summarized network " + newMenuItem.GetID())
}

func SummarizeNetworkSelection(candidates []*Network, fromIndex, toIndex int) {
	if len(candidates) < 2 {
		statusLine.Clear()
		statusLine.SetText("Error summarizing network: at least two unallocated sibling networks are required")
		return
	}

	if fromIndex > toIndex {
		fromIndex, toIndex = toIndex, fromIndex
	}
	if fromIndex < 0 || toIndex >= len(candidates) {
		statusLine.Clear()
		statusLine.SetText("Error summarizing network: invalid selection range")
		return
	}
	if toIndex-fromIndex < 1 {
		statusLine.Clear()
		statusLine.SetText("Error summarizing network: select at least two networks")
		return
	}

	selected := candidates[fromIndex : toIndex+1]
	parent := selected[0].GetParent()
	selectedCIDRs := make([]string, 0, len(selected))
	for _, n := range selected {
		if n == nil {
			statusLine.Clear()
			statusLine.SetText("Error summarizing network: invalid selection")
			return
		}
		if n.GetParent() != parent {
			statusLine.Clear()
			statusLine.SetText("Error summarizing network: selected networks must share the same parent")
			return
		}
		if n.AllocationMode != AllocationModeUnallocated {
			statusLine.Clear()
			statusLine.SetText("Error summarizing network: only unallocated networks can be summarized")
			return
		}
		selectedCIDRs = append(selectedCIDRs, n.ID)
	}

	newNetwork, err := summarizeCIDRs(selectedCIDRs)
	if err != nil {
		statusLine.Clear()
		statusLine.SetText("Error summarizing network: " + err.Error())
		return
	}

	newMenuItem := &Network{
		MenuFolder: &MenuFolder{
			ID:         newNetwork,
			ParentPath: parent.GetPath(),
		},
	}
	if err := newMenuItem.ValidateWithRules(&NetworkValidationRules{IgnoreOverlapsWith: selected}); err != nil {
		statusLine.Clear()
		statusLine.SetText("Error validating summarized network: " + err.Error())
		return
	}

	for _, old := range selected {
		menuItems.Delete(old)
	}
	menuItems.MustAdd(newMenuItem)
	reloadMenu(newMenuItem)

	statusLine.Clear()
	statusLine.SetText("Summarized networks into " + newMenuItem.GetID())
}

func AllocateNetworkInSubnetsMode(displayName, description string, subnetsPrefix int) {
	focusedNetwork, ok := currentMenuFocus.(*Network)
	if !ok {
		panic("AllocateNetworkInSubnetsMode called on non-Network")
	}

	if focusedNetwork.AllocationMode != AllocationModeUnallocated {
		panic("AllocateNetworkInSubnetsMode called on already allocated Network")
	}

	mod := func(n *Network) {
		n.AllocationMode = AllocationModeSubnets
		n.DisplayName = displayName
		n.Description = description
	}

	copy := *focusedNetwork
	mod(&copy)
	if err := copy.ValidateWithRules(
		&NetworkValidationRules{
			Substitutes: map[string]*Network{
				focusedNetwork.ID: &copy,
			},
		},
	); err != nil {
		statusLine.Clear()
		statusLine.SetText("Error allocating network: " + err.Error())
		return
	}

	newNetworks, err := splitNetwork(focusedNetwork.ID, subnetsPrefix)
	if err != nil {
		statusLine.Clear()
		statusLine.SetText("Error splitting network: " + err.Error())
		return
	}

	newMenuItems := []*Network{}
	for _, newNet := range newNetworks {
		newMenuItem := &Network{
			MenuFolder: &MenuFolder{
				ID:         newNet,
				ParentPath: focusedNetwork.GetPath(),
			},
		}
		if err := newMenuItem.ValidateWithRules(
			&NetworkValidationRules{
				Substitutes: map[string]*Network{
					focusedNetwork.ID: &copy,
				},
			},
		); err != nil {
			statusLine.Clear()
			statusLine.SetText("Error validating new network: " + err.Error())
			return
		}
		newMenuItems = append(newMenuItems, newMenuItem)
	}

	mod(focusedNetwork)
	if err := focusedNetwork.Validate(); err != nil {
		// This needs to panic since we just changed state of the object in memory and now it fails validation.
		// We do not know how to recover, this would be a bug and it should never reach this brunch here - missed during pre validation above somehow.
		panic(err)
	}

	for _, newMenuItem := range newMenuItems {
		menuItems.MustAdd(newMenuItem)
	}

	reloadMenu(focusedNetwork)

	statusLine.Clear()
	statusLine.SetText("Allocated network: " + focusedNetwork.GetPath())
}

func AllocateNetworkInHostsMode(displayName, description string) {
	focusedNetwork, ok := currentMenuFocus.(*Network)
	if !ok {
		panic("AllocateNetworkInHostsMode called on non-Network")
	}

	if focusedNetwork.AllocationMode != AllocationModeUnallocated {
		panic("AllocateNetworkInHostsMode called on already allocated Network")
	}

	mod := func(n *Network) {
		n.AllocationMode = AllocationModeHosts
		n.DisplayName = displayName
		n.Description = description
	}

	copy := *focusedNetwork
	mod(&copy)
	if err := copy.ValidateWithRules(
		&NetworkValidationRules{
			Substitutes: map[string]*Network{
				focusedNetwork.ID: &copy,
			},
		},
	); err != nil {
		statusLine.Clear()
		statusLine.SetText("Error allocating network: " + err.Error())
		return
	}

	mod(focusedNetwork)
	if err := focusedNetwork.Validate(); err != nil {
		// This needs to panic since we just changed state of the object in memory and now it fails validation.
		// We do not know how to recover, this would be a bug and it should never reach this brunch here - missed during pre validation above somehow.
		panic(err)
	}

	reloadMenu(focusedNetwork)

	statusLine.Clear()
	statusLine.SetText("Allocated network: " + focusedNetwork.GetPath())
}

func UpdateNetworkAllocation(displayName, description string) {
	focusedNetwork, ok := currentMenuFocus.(*Network)
	if !ok {
		panic("UpdateNetworkAllocation called on non-Network")
	}

	if focusedNetwork.AllocationMode == AllocationModeUnallocated {
		panic("UpdateNetworkAllocation called on unallocated Network")
	}

	mod := func(n *Network) {
		n.DisplayName = displayName
		n.Description = description
	}

	copy := *focusedNetwork
	mod(&copy)
	if err := copy.ValidateWithRules(
		&NetworkValidationRules{
			Substitutes: map[string]*Network{
				focusedNetwork.ID: &copy,
			},
		},
	); err != nil {
		statusLine.Clear()
		statusLine.SetText("Error updating network allocation: " + err.Error())
		return
	}

	mod(focusedNetwork)
	if err := focusedNetwork.Validate(); err != nil {
		// This needs to panic since we just changed state of the object in memory and now it fails validation.
		// We do not know how to recover, this would be a bug and it should never reach this brunch here - missed during pre validation above somehow.
		panic(err)
	}

	reloadMenu(focusedNetwork)

	statusLine.Clear()
	statusLine.SetText("Allocated network updated: " + focusedNetwork.GetPath())
}

func DeallocateNetwork() {
	focusedNetwork, ok := currentMenuFocus.(*Network)
	if !ok {
		panic("DeallocateNetwork called on non-Network")
	}

	if focusedNetwork.AllocationMode == AllocationModeUnallocated {
		panic("DeallocateNetwork called on unallocated Network")
	}

	mod := func(n *Network) {
		n.AllocationMode = AllocationModeUnallocated
		n.DisplayName = ""
		n.Description = ""
	}

	copy := *focusedNetwork
	mod(&copy)
	if err := copy.ValidateWithRules(
		&NetworkValidationRules{
			Substitutes: map[string]*Network{
				focusedNetwork.ID: &copy,
			},
		},
	); err != nil {
		statusLine.Clear()
		statusLine.SetText("Error deallocating network: " + err.Error())
		return
	}

	mod(focusedNetwork)
	if err := focusedNetwork.Validate(); err != nil {
		// This needs to panic since we just changed state of the object in memory and now it fails validation.
		// We do not know how to recover, this would be a bug and it should never reach this brunch here - missed during pre validation above somehow.
		panic(err)
	}

	children := menuItems.GetChilds(focusedNetwork)
	for _, child := range children {
		menuItems.Delete(child)
	}

	reloadMenu(focusedNetwork)

	statusLine.Clear()
	statusLine.SetText("Deallocated network: " + focusedNetwork.GetPath())
}

func DeleteNetwork() {
	focusedNetwork, ok := currentMenuFocus.(*Network)
	if !ok {
		panic("DeleteNetwork called on non-Network")
	}

	parent := focusedNetwork.GetParent()
	_, ok = parent.(*Network)
	if ok {
		panic("DeleteNetwork called on child of Network " + focusedNetwork.GetPath())
	}

	menuItems.Delete(focusedNetwork)

	reloadMenu(focusedNetwork)

	statusLine.Clear()
	statusLine.SetText("Deleted network: " + focusedNetwork.GetPath())
}

func getUnallocatedSiblingNetworks(network *Network) []*Network {
	unallocated := []*Network{}
	for _, sibling := range menuItems.GetChilds(network.GetParent()) {
		n, ok := sibling.(*Network)
		if !ok {
			continue
		}
		if n.AllocationMode == AllocationModeUnallocated {
			unallocated = append(unallocated, n)
		}
	}
	return unallocated
}

func hasAnySummarizableRange(candidates []*Network) bool {
	if len(candidates) < 2 {
		return false
	}

	cidrs := make([]string, 0, len(candidates))
	for _, n := range candidates {
		cidrs = append(cidrs, n.ID)
	}
	for i := range cidrs {
		summarizeable, _, _ := findSummarizableRange(cidrs, i)
		if summarizeable {
			return true
		}
	}

	return false
}

type AllocationMode uint8

const (
	AllocationModeUnallocated AllocationMode = iota
	AllocationModeSubnets
	AllocationModeHosts
)

type Network struct {
	*MenuFolder
	AllocationMode AllocationMode `json:"allocation_mode"`
	DisplayName    string         `json:"display_name"`
	Description    string         `json:"description"`
}

func (n *Network) Validate() error {
	return n.ValidateWithRules(&NetworkValidationRules{})
}

func (n *Network) ValidateWithRules(rules *NetworkValidationRules) error {
	if err := n.MenuFolder.Validate(); err != nil {
		return err
	}

	ip, ipNet, err := net.ParseCIDR(n.ID)
	if err != nil {
		return fmt.Errorf("Invalid ID %s (must be a valid network CIDR): %s", n.ID, err)
	}

	if ipNet.Mask == nil {
		return fmt.Errorf("Provided CIDR does not contain a network mask: %s", n.ID)
	}

	maskBits, _ := ipNet.Mask.Size()
	if ipNet.IP.To4() != nil && maskBits == 32 {
		return fmt.Errorf("Provided CIDR is a single IPv4 address, not a network: %s", n.ID)
	} else if ipNet.IP.To4() == nil && maskBits == 128 {
		return fmt.Errorf("Provided CIDR is a single IPv6 address, not a network: %s", n.ID)
	}

	if !ip.Equal(ipNet.IP) {
		return fmt.Errorf("Provided CIDR specifies a host, not a network: %s (should be %s/%d)", n.ID, ipNet.IP, maskBits)
	}

	parent := n.GetParent()
	if parent == nil {
		return fmt.Errorf("Parent not found for Network=%s", n.GetPath())
	}

	switch p := parent.(type) {
	case *MenuStatic:
		networksFolder := menuItems.GetByParentAndID(nil, "Networks")
		if networksFolder == nil {
			panic("Networks folder not found")
		}
		if p != networksFolder {
			return fmt.Errorf("Parent must be Networks for Network=%s", n.GetPath())
		}
	case *Network:
		if substitute, ok := rules.Substitutes[p.ID]; ok {
			if substitute == nil {
				break
			}
			p = substitute
		}

		if p.AllocationMode != AllocationModeSubnets {
			return fmt.Errorf("Parent network must be allocated in subnets mode network for Network=%s", n.GetPath())
		}

		_, parentIPNet, err := net.ParseCIDR(p.ID)
		if err != nil {
			return fmt.Errorf("Error parsing parent CIDR %s: %s", p.ID, err)
		}

		if parentIPNet.String() == ipNet.String() {
			return fmt.Errorf("Network=%s cannot be the same as parent Network=%s", n.GetPath(), p.GetPath())
		}

		if !parentIPNet.Contains(ipNet.IP) {
			return fmt.Errorf("Network=%s is not within parent Network=%s", n.GetPath(), p.GetPath())
		}
	default:
		return fmt.Errorf("Parent must be Networks folder or another allocated network for Network=%s", n.GetPath())
	}

	// Check that other networks do not overlap
	otherNetworks := menuItems.GetChilds(parent)
	for _, other := range otherNetworks {
		otherNetwork, ok := other.(*Network)
		if !ok {
			panic("Non-network child found in Networks")
		}

		if substitute, ok := rules.Substitutes[otherNetwork.ID]; ok {
			if substitute == nil {
				continue
			}
			otherNetwork = substitute
		}

		if otherNetwork == n {
			continue
		}

		mustSkip := false
		for _, ignore := range rules.IgnoreOverlapsWith {
			if ignore == other {
				mustSkip = true
			}
		}
		if mustSkip {
			continue
		}

		_, otherIPNet, err := net.ParseCIDR(otherNetwork.ID)
		if err != nil {
			return fmt.Errorf("Error parsing other CIDR %s: %s", other.GetID(), err)
		}

		if otherIPNet.String() == ipNet.String() {
			return fmt.Errorf("Network=%s cannot be the same as Network=%s", n.GetPath(), other.GetPath())
		}

		if otherIPNet.Contains(ipNet.IP) || ipNet.Contains(otherIPNet.IP) {
			return fmt.Errorf("Network=%s overlaps with Network=%s", n.GetPath(), other.GetPath())
		}
	}

	if n.AllocationMode != AllocationModeUnallocated {
		if n.DisplayName == "" {
			return fmt.Errorf("DisplayName must be set for allocated Network=%s", n.GetPath())
		}
	}

	return nil
}

func (n *Network) GetID() string {
	if n.AllocationMode != AllocationModeUnallocated {
		if n.DisplayName != "" {
			return fmt.Sprintf("%s (%s)", n.ID, n.DisplayName)
		}
		return n.ID
	} else {
		return fmt.Sprintf("%s (*)", n.ID)
	}
}

func (n *Network) Compare(other MenuItem) int {
	if other == nil {
		return 1
	}

	otherMenu, ok := other.(*Network)
	if !ok {
		return cmp.Compare(n.GetID(), other.GetID())
	}

	_, ipNetLeft, err := net.ParseCIDR(n.ID)
	if err != nil {
		panic(err)
	}

	_, ipNetRight, err := net.ParseCIDR(otherMenu.ID)
	if err != nil {
		panic(err)
	}

	ipLeft := ipNetLeft.IP.Mask(ipNetLeft.Mask)
	ipRight := ipNetRight.IP.Mask(ipNetRight.Mask)

	// Ensure both IPs are in 16-byte form
	ipLeft = ipLeft.To16()
	ipRight = ipRight.To16()

	// Compare byte by byte
	for i := 0; i < net.IPv6len; i++ {
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

func (n *Network) OnChangedFunc() {
	detailsPanel.Clear()
	detailsPanel.SetText(n.RenderDetails())

	parentIsNetwork := false
	parent := n.GetParent()
	if parent != nil {
		_, parentIsNetwork = parent.(*Network)
	}

	if n.AllocationMode != AllocationModeUnallocated {
		currentFocusKeys = []string{
			"<u> Update Allocation",
			"<d> Deallocate",
		}
	} else {
		currentFocusKeys = []string{
			"<a> Allocate Subnets",
			"<A> Allocate Hosts",
			"<s> Split",
		}
		if hasAnySummarizableRange(getUnallocatedSiblingNetworks(n)) {
			currentFocusKeys = append(currentFocusKeys, "<S> Summarize Range")
		}
	}
	if !parentIsNetwork {
		currentFocusKeys = append(currentFocusKeys, "<D> Delete")
	}
}

func (n *Network) OnSelectedFunc() {
	positionLine.Clear()
	positionLine.SetText(n.GetPath())
	if n.AllocationMode == AllocationModeHosts {
		currentMenuItemKeys = []string{"<r> Reserve IP"}
	} else {
		currentMenuItemKeys = []string{}
	}
}

func (n *Network) OnDoneFunc() {
	positionLine.Clear()
	positionLine.SetText(n.GetPath())
	currentMenuItemKeys = []string{}
}

func (n *Network) CurrentMenuInputCapture(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 'r':
			if n.AllocationMode != AllocationModeHosts {
				return event
			}

			reserveIPDialog.SetTitle(fmt.Sprintf("Reserve IP in %s", n.ID))
			reserveIPDialog.SetFocus(0)
			pages.ShowPage(reserveIPPage)
			app.SetFocus(reserveIPDialog)
			return nil
		}
	}

	return event
}

func (n *Network) CurrentFocusInputCapture(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 'a':
			if n.AllocationMode != AllocationModeUnallocated {
				return event
			}

			allocateNetworkSubnetsModeDialog.SetTitle(fmt.Sprintf("Allocate Subnets for %s", n.ID))
			allocateNetworkSubnetsModeDialog.SetFocus(0)
			pages.ShowPage(allocateNetworkSubnetsModePage)
			app.SetFocus(allocateNetworkSubnetsModeDialog)
			return nil
		case 'A':
			if n.AllocationMode != AllocationModeUnallocated {
				return event
			}

			allocateNetworkHostsModeDialog.SetTitle(fmt.Sprintf("Allocate Hosts for %s", n.ID))
			allocateNetworkHostsModeDialog.SetFocus(0)
			pages.ShowPage(allocateNetworkHostsModePage)
			app.SetFocus(allocateNetworkHostsModeDialog)
			return nil
		case 'u':
			if n.AllocationMode == AllocationModeUnallocated {
				return event
			}

			updateNetworkAllocationDialog.SetTitle(fmt.Sprintf("Update Allocation for %s", n.ID))
			setTextFromInputField(updateNetworkAllocationDialog, "Name", n.DisplayName)
			setTextFromTextArea(updateNetworkAllocationDialog, "Description", n.Description)
			updateNetworkAllocationDialog.SetFocus(0)
			pages.ShowPage(updateNetworkAllocationPage)
			app.SetFocus(updateNetworkAllocationDialog)
			return nil
		case 's':
			if n.AllocationMode != AllocationModeUnallocated {
				return event
			}

			splitNetworkDialog.SetTitle(fmt.Sprintf("Split %s", n.ID))
			splitNetworkDialog.SetFocus(0)
			pages.ShowPage(splitNetworkPage)
			app.SetFocus(splitNetworkDialog)
			return nil
		case 'S':
			if n.AllocationMode != AllocationModeUnallocated {
				return event
			}

			candidates := getUnallocatedSiblingNetworks(n)
			if !hasAnySummarizableRange(candidates) {
				return event
			}

			options := make([]string, 0, len(candidates))
			focusedIndex := 0
			for i, candidate := range candidates {
				options = append(options, candidate.GetID())
				if candidate == n {
					focusedIndex = i
				}
			}

			summarizeCandidates = candidates
			summarizeFromIndex = focusedIndex
			summarizeToIndex = focusedIndex
			if focusedIndex < len(options)-1 {
				summarizeToIndex = focusedIndex + 1
			} else if focusedIndex > 0 {
				summarizeFromIndex = focusedIndex - 1
			}

			_, fromItem := getFormItemByLabel(summarizeNetworkDialog, "From")
			fromDropdown, ok := fromItem.(*tview.DropDown)
			if !ok {
				panic("Failed to cast summarize From dropdown")
			}
			fromDropdown.SetOptions(options, func(option string, optionIndex int) {
				if optionIndex >= 0 {
					summarizeFromIndex = optionIndex
				}
			}).SetCurrentOption(summarizeFromIndex)

			_, toItem := getFormItemByLabel(summarizeNetworkDialog, "To")
			toDropdown, ok := toItem.(*tview.DropDown)
			if !ok {
				panic("Failed to cast summarize To dropdown")
			}
			toDropdown.SetOptions(options, func(option string, optionIndex int) {
				if optionIndex >= 0 {
					summarizeToIndex = optionIndex
				}
			}).SetCurrentOption(summarizeToIndex)

			summarizeNetworkDialog.SetTitle(fmt.Sprintf("Summarize in %s", n.GetParent().GetID()))
			summarizeNetworkDialog.SetFocus(0)
			pages.ShowPage(summarizeNetworkPage)
			app.SetFocus(summarizeNetworkDialog)
			return nil
		case 'd':
			if n.AllocationMode == AllocationModeUnallocated {
				return event
			}

			deallocateNetworkDialog.SetText(fmt.Sprintf("Deallocate %s?\n\nAll child subnets will be removed.", n.GetID()))
			deallocateNetworkDialog.SetFocus(1)
			pages.ShowPage(deallocateNetworkPage)
			app.SetFocus(deallocateNetworkDialog)
			return nil
		case 'D':
			parentIsNetwork := false
			parent := n.GetParent()
			if parent != nil {
				_, parentIsNetwork = parent.(*Network)
			}
			if parentIsNetwork {
				return event
			}

			deleteNetworkDialog.SetText(fmt.Sprintf("Delete %s?\n\nAll child subnets will be removed.", n.GetID()))
			deleteNetworkDialog.SetFocus(1)
			pages.ShowPage(deleteNetworkPage)
			app.SetFocus(deleteNetworkDialog)
			return nil
		}
	}
	return event
}

func (n *Network) RenderDetailsMap() ([]string, map[string]string, error) {
	index := []string{}
	result := map[string]string{}

	p := message.NewPrinter(language.English) // sorry, rest of the world

	index = append(index, "CIDR")
	result["CIDR"] = n.ID

	_, ipnet, err := net.ParseCIDR(n.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("Error parsing CIDR %s: %s", n.ID, err)
	}

	firstIP := ipnet.IP
	index = append(index, "Network Address")
	result["Network Address"] = firstIP.String()

	maskBits, _ := ipnet.Mask.Size()
	index = append(index, "Mask Bits")
	result["Mask Bits"] = p.Sprintf("%d", maskBits)

	subnetMask := make(net.IP, len(ipnet.Mask))
	copy(subnetMask, ipnet.Mask)
	subnetMaskStr := subnetMask.String()
	index = append(index, "Subnet Mask")
	result["Subnet Mask"] = subnetMaskStr

	lastIP := make(net.IP, len(firstIP))
	copy(lastIP, firstIP)
	for i := range lastIP {
		lastIP[i] = firstIP[i] | ^ipnet.Mask[i]
	}
	if ipnet.IP.To4() != nil {
		index = append(index, "Broadcast Address")
		result["Broadcast Address"] = lastIP.String()
	}
	index = append(index, "Range")
	result["Range"] = p.Sprintf("%s - %s", firstIP, lastIP)

	var totalHosts big.Int
	totalHosts.SetUint64(1)
	var usableHosts big.Int
	if ipnet.IP.To4() == nil { // IPv6
		totalHosts.Lsh(&totalHosts, uint(128-maskBits))
		usableHosts.Set(&totalHosts)
		usableHosts.Sub(&usableHosts, big.NewInt(1))

		if maskBits <= 64 {
			totalNetworks := 1 << (64 - maskBits)
			index = append(index, "Total /64 Networks")
			result["Total /64 Networks"] = p.Sprintf("%d", totalNetworks)
		} else {
			index = append(index, "Total Hosts")
			result["Total Hosts"] = p.Sprintf("%d", totalHosts.Uint64())
		}
	} else { // IPv4
		totalHosts.Lsh(&totalHosts, uint(32-maskBits))
		usableHosts.Set(&totalHosts)
		usableHosts.Sub(&usableHosts, big.NewInt(2))
		index = append(index, "Total Hosts")
		result["Total Hosts"] = p.Sprintf("%d", totalHosts.Uint64())

		usableFirstIP := make(net.IP, len(firstIP))
		copy(usableFirstIP, firstIP)
		usableFirstIP[len(usableFirstIP)-1]++
		usableLastIP := make(net.IP, len(lastIP))
		copy(usableLastIP, lastIP)
		if ipnet.IP.To4() != nil {
			usableLastIP[len(usableLastIP)-1]--
		}
		index = append(index, "Usable Range")
		result["Usable Range"] = p.Sprintf("%s - %s", usableFirstIP, usableLastIP)
		index = append(index, "Usable Hosts")
		result["Usable Hosts"] = p.Sprintf("%d", usableHosts.Uint64())
	}

	index = append(index, "Allocation Mode")
	switch n.AllocationMode {
	case AllocationModeUnallocated:
		result["Allocation Mode"] = "Unallocated"
	case AllocationModeSubnets:
		result["Allocation Mode"] = "Subnets"
	case AllocationModeHosts:
		result["Allocation Mode"] = "Hosts"
	default:
		panic("Unknown AllocationMode")
	}
	if n.AllocationMode != AllocationModeUnallocated {
		result["Description"] = n.Description
	}

	return index, result, nil
}

func (n *Network) RenderDetails() string {
	stringWriter := new(strings.Builder)
	template := "%-20s: %s\n"

	index, data, err := n.RenderDetailsMap()
	if err != nil {
		return fmt.Sprintf("Error rendering details: %v", err)
	}

	for _, key := range index {
		if key == "Description" {
			continue
		}
		stringWriter.WriteString(fmt.Sprintf(template, key, data[key]))
	}

	if n.AllocationMode != AllocationModeUnallocated {
		stringWriter.WriteString("\n\n\n")
		stringWriter.WriteString(n.Description)
	}

	return stringWriter.String()
}

func ipToBigInt(addr netip.Addr) *big.Int {
	ip := addr.AsSlice()
	return new(big.Int).SetBytes(ip)
}

func bigIntToAddr(i *big.Int, isIPv4 bool) netip.Addr {
	var ipLen int
	if isIPv4 {
		ipLen = net.IPv4len
	} else {
		ipLen = net.IPv6len
	}
	ipBytes := i.Bytes()
	if len(ipBytes) < ipLen {
		padding := make([]byte, ipLen-len(ipBytes))
		ipBytes = append(padding, ipBytes...)
	} else if len(ipBytes) > ipLen {
		ipBytes = ipBytes[len(ipBytes)-ipLen:]
	}
	addr, ok := netip.AddrFromSlice(ipBytes)
	if !ok {
		return netip.Addr{}
	}
	return addr
}

func lastAddr(prefix netip.Prefix) netip.Addr {
	addr := prefix.Masked().Addr()
	addrInt := ipToBigInt(addr)
	var size big.Int
	if addr.Is4() {
		size.Exp(big.NewInt(2), big.NewInt(32-int64(prefix.Bits())), nil)
	} else {
		size.Exp(big.NewInt(2), big.NewInt(128-int64(prefix.Bits())), nil)
	}
	size.Sub(&size, big.NewInt(1))
	endInt := new(big.Int).Add(addrInt, &size)
	return bigIntToAddr(endInt, addr.Is4())
}

func log2BigInt(n *big.Int) int {
	bits := n.BitLen()
	if bits == 0 {
		return 0
	}
	return bits - 1
}

func isPowerOfTwo(n *big.Int) bool {
	if n.Sign() <= 0 {
		return false
	}
	one := big.NewInt(1)
	tmp := new(big.Int).Sub(n, one)
	return new(big.Int).And(n, tmp).Cmp(big.NewInt(0)) == 0
}

func CIDRToIdentifier(cidr string) (string, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR: %v", err)
	}

	var ipBytes []byte
	if ip4 := ip.To4(); ip4 != nil {
		// IPv4 address
		ipBytes = ip4
	} else if ip16 := ip.To16(); ip16 != nil {
		// IPv6 address
		ipBytes = ip16
	} else {
		return "", fmt.Errorf("invalid IP address in CIDR")
	}

	ipHex := hex.EncodeToString(ipBytes)
	prefixLen, _ := ipNet.Mask.Size()
	identifier := fmt.Sprintf("%s_%d", ipHex, prefixLen)
	return identifier, nil
}

func IPToIdentifier(ipStr string) (string, error) {
	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return "", fmt.Errorf("invalid IP: %w", err)
	}

	return hex.EncodeToString(addr.AsSlice()), nil
}

func splitNetwork(cidr string, newSize int) ([]string, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %w", err)
	}

	origSize := prefix.Bits()
	addrLen := prefix.Addr().BitLen()

	if newSize < 1 {
		newSize = origSize + 1
	}

	if newSize <= origSize || newSize > addrLen {
		return nil, fmt.Errorf("invalid new size: must be larger than original but not exceed address bit length")
	}

	// Calculate number of subnets and prevent splitting into too many subnets
	numSubnets := 1 << (newSize - origSize)
	maxSubnets := 1024
	if numSubnets > maxSubnets {
		return nil, fmt.Errorf("splitting would create %d subnets, current limit is %d due to performance issues", numSubnets, maxSubnets)
	}

	var subnets []string
	addr := prefix.Addr()
	step := big.NewInt(1)
	step.Lsh(step, uint(addrLen-newSize))

	currIP := ipToBigInt(addr)
	isIPv4 := addr.Is4()

	for i := 0; i < (1 << (newSize - origSize)); i++ {
		nextPrefix := netip.PrefixFrom(addr, newSize)
		subnets = append(subnets, nextPrefix.String())

		currIP.Add(currIP, step)

		nextAddr := bigIntToAddr(currIP, isIPv4)
		if !nextAddr.IsValid() {
			return nil, fmt.Errorf("failed to convert bigInt %d to IP", currIP)
		}
		addr = nextAddr
	}

	return subnets, nil
}

func summarizeCIDRs(cidrs []string) (string, error) {
	if len(cidrs) == 0 {
		return "", errors.New("no CIDRs provided")
	}

	var startIPs []*big.Int
	var endIPs []*big.Int
	var minIP, maxIP *big.Int
	var isIPv4 bool

	for i, cidrStr := range cidrs {
		prefix, err := netip.ParsePrefix(cidrStr)
		if err != nil {
			return "", fmt.Errorf("invalid CIDR %s: %v", cidrStr, err)
		}

		if i == 0 {
			isIPv4 = prefix.Addr().Is4()
		} else {
			if isIPv4 != prefix.Addr().Is4() {
				return "", errors.New("mixed IPv4 and IPv6 addresses are not supported")
			}
		}

		startIP := ipToBigInt(prefix.Masked().Addr())
		endIP := ipToBigInt(lastAddr(prefix))

		startIPs = append(startIPs, startIP)
		endIPs = append(endIPs, endIP)

		if i == 0 || startIP.Cmp(minIP) < 0 {
			minIP = startIP
		}
		if i == 0 || endIP.Cmp(maxIP) > 0 {
			maxIP = endIP
		}
	}

	indices := make([]int, len(startIPs))
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool {
		return startIPs[indices[i]].Cmp(startIPs[indices[j]]) < 0
	})

	sortedStartIPs := make([]*big.Int, len(startIPs))
	sortedEndIPs := make([]*big.Int, len(endIPs))
	for i, idx := range indices {
		sortedStartIPs[i] = startIPs[idx]
		sortedEndIPs[i] = endIPs[idx]
	}

	currentIP := new(big.Int).Set(minIP)
	one := big.NewInt(1)

	for i := 0; i < len(sortedStartIPs); i++ {
		if sortedStartIPs[i].Cmp(currentIP) > 0 {
			return "", errors.New("CIDRs are not contiguous; gaps detected")
		}
		if sortedStartIPs[i].Cmp(currentIP) <= 0 && sortedEndIPs[i].Cmp(currentIP) >= 0 {
			currentIP = new(big.Int).Add(sortedEndIPs[i], one)
		}
	}
	if new(big.Int).Sub(currentIP, one).Cmp(maxIP) != 0 {
		return "", errors.New("CIDRs do not cover a contiguous range")
	}

	totalAddresses := new(big.Int).Add(new(big.Int).Sub(maxIP, minIP), one)
	if !isPowerOfTwo(totalAddresses) {
		return "", errors.New("total number of addresses is not a power of two; cannot summarize into a single CIDR without including extra addresses")
	}

	if new(big.Int).Mod(minIP, totalAddresses).Cmp(big.NewInt(0)) != 0 {
		return "", errors.New("network address is not aligned; cannot summarize into a single CIDR without including extra addresses")
	}

	var addressBits int
	if isIPv4 {
		addressBits = 32
	} else {
		addressBits = 128
	}
	prefixLength := addressBits - log2BigInt(totalAddresses)

	summarizedIP := bigIntToAddr(minIP, isIPv4)

	summarizedPrefix := netip.PrefixFrom(summarizedIP, int(prefixLength))
	return summarizedPrefix.String(), nil
}

func findSummarizableRange(cidrs []string, index int) (bool, []string, string) {
	maxSize := len(cidrs)
	for size := maxSize; size >= 2; size-- {
		for start := index - size + 1; start <= index; start++ {
			end := start + size - 1
			if start < 0 || end >= len(cidrs) {
				continue
			}
			if start <= index && index <= end {
				subCIDRs := cidrs[start : end+1]
				summarizedCIDR, err := summarizeCIDRs(subCIDRs)
				if err == nil {
					return true, subCIDRs, summarizedCIDR
				}
			}
		}
	}
	return false, nil, ""
}

func findSummarizableRangeForNetwork(network *Network) (bool, []*Network, string) {
	neighbors := menuItems.GetChilds(network.GetParent())

	unallocatedNeighbors := []*Network{}
	unallocatedNeighborsCIDRs := []string{}
	indexAmongAllNeighbors := -1
	for i, neighbor := range neighbors {
		if neighbor == network {
			indexAmongAllNeighbors = i
			break
		}
	}
	if indexAmongAllNeighbors < 0 || indexAmongAllNeighbors >= len(neighbors) {
		panic("index out of range")
	}
	for i := indexAmongAllNeighbors - 1; i >= 0; i-- {
		if n, ok := neighbors[i].(*Network); ok {
			if n.AllocationMode != AllocationModeUnallocated {
				break
			}

			unallocatedNeighbors = append(unallocatedNeighbors, n)
			unallocatedNeighborsCIDRs = append(unallocatedNeighborsCIDRs, n.ID)
		}
	}
	slices.Reverse(unallocatedNeighbors)
	slices.Reverse(unallocatedNeighborsCIDRs)
	index := len(unallocatedNeighborsCIDRs)
	unallocatedNeighbors = append(unallocatedNeighbors, network)
	unallocatedNeighborsCIDRs = append(unallocatedNeighborsCIDRs, network.ID)
	for i := indexAmongAllNeighbors + 1; i < len(neighbors); i++ {
		if n, ok := neighbors[i].(*Network); ok {
			if n.AllocationMode != AllocationModeUnallocated {
				break
			}

			unallocatedNeighbors = append(unallocatedNeighbors, n)
			unallocatedNeighborsCIDRs = append(unallocatedNeighborsCIDRs, n.ID)
		}
	}

	summarizeable, summarizeableCIDRs, newNetwork := findSummarizableRange(unallocatedNeighborsCIDRs, index)
	_, parentIsNetwork := network.GetParent().(*Network)
	if parentIsNetwork && len(neighbors) == len(summarizeableCIDRs) {
		originalUnallocatedNeighbors := unallocatedNeighbors
		originalUnallocatedNeighborsCIDRs := unallocatedNeighborsCIDRs
		originalIndex := index

		isFirst := index == 0

		// Unless this is the first one itself, try without the first.
		if !isFirst {
			unallocatedNeighbors = unallocatedNeighbors[1:]
			unallocatedNeighborsCIDRs = unallocatedNeighborsCIDRs[1:]
			index--
			summarizeable, summarizeableCIDRs, newNetwork = findSummarizableRange(unallocatedNeighborsCIDRs, index)
		}

		// If still no result (or we started on first), try without the last.
		if isFirst || !summarizeable {
			unallocatedNeighbors = originalUnallocatedNeighbors[:len(originalUnallocatedNeighbors)-1]
			unallocatedNeighborsCIDRs = originalUnallocatedNeighborsCIDRs[:len(originalUnallocatedNeighborsCIDRs)-1]
			index = originalIndex
			summarizeable, summarizeableCIDRs, newNetwork = findSummarizableRange(unallocatedNeighborsCIDRs, index)
		}
	}

	summarizeableNetworks := make([]*Network, len(summarizeableCIDRs))
	for i, cidr := range summarizeableCIDRs {
		for _, neighbor := range unallocatedNeighbors {
			if neighbor.ID == cidr {
				summarizeableNetworks[i] = neighbor
			}
		}
		if summarizeableNetworks[i] == nil {
			return false, nil, ""
		}
	}

	return summarizeable, summarizeableNetworks, newNetwork
}
