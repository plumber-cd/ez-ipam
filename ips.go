package main

import (
	"cmp"
	"fmt"
	"net/netip"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type IP struct {
	*MenuFolder
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

func (i *IP) Validate() error {
	if err := i.MenuFolder.Validate(); err != nil {
		return err
	}

	addr, err := netip.ParseAddr(i.ID)
	if err != nil {
		return fmt.Errorf("invalid IP ID %q: %w", i.ID, err)
	}

	parent := i.GetParent()
	if parent == nil {
		return fmt.Errorf("parent not found for IP=%s", i.GetPath())
	}

	network, ok := parent.(*Network)
	if !ok {
		return fmt.Errorf("parent must be a network for IP=%s", i.GetPath())
	}
	if network.AllocationMode != AllocationModeHosts {
		return fmt.Errorf("parent network must be allocated in hosts mode for IP=%s", i.GetPath())
	}

	prefix, err := netip.ParsePrefix(network.ID)
	if err != nil {
		return fmt.Errorf("failed to parse parent network CIDR %q: %w", network.ID, err)
	}
	if prefix.Addr().Is4() != addr.Is4() {
		return fmt.Errorf("IP family mismatch between IP=%s and parent=%s", i.GetPath(), network.GetPath())
	}
	if !prefix.Contains(addr) {
		return fmt.Errorf("IP=%s is not within parent network=%s", i.GetPath(), network.GetPath())
	}

	for _, sibling := range menuItems.GetChilds(parent) {
		other, ok := sibling.(*IP)
		if !ok || other == i {
			continue
		}
		if other.ID == i.ID {
			return fmt.Errorf("IP=%s already reserved in %s", i.ID, network.GetPath())
		}
	}

	if strings.TrimSpace(i.DisplayName) == "" {
		return fmt.Errorf("display name must be set for reserved IP=%s", i.GetPath())
	}

	return nil
}

func (i *IP) GetID() string {
	return fmt.Sprintf("%s (%s)", i.ID, i.DisplayName)
}

func (i *IP) Compare(other MenuItem) int {
	if other == nil {
		return 1
	}

	otherIP, ok := other.(*IP)
	if !ok {
		return cmp.Compare(i.GetID(), other.GetID())
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

func (i *IP) OnChangedFunc() {
	detailsPanel.Clear()
	description := i.Description
	if description == "" {
		description = "<none>"
	}
	detailsPanel.SetText(fmt.Sprintf(
		"IP Address           : %s\nDisplay Name         : %s\nDescription          : %s\nParent Network       : %s\n",
		i.ID,
		i.DisplayName,
		description,
		i.GetParent().GetPath(),
	))
	currentFocusKeys = []string{
		"<u> Update Reservation",
		"<R> Unreserve",
	}
}

func (i *IP) OnSelectedFunc() {
	positionLine.Clear()
	positionLine.SetText(i.GetPath())
	currentMenuItemKeys = []string{}
}

func (i *IP) OnDoneFunc() {
	positionLine.Clear()
	positionLine.SetText(i.GetPath())
	currentMenuItemKeys = []string{}
}

func (i *IP) CurrentMenuInputCapture(event *tcell.EventKey) *tcell.EventKey {
	return event
}

func (i *IP) CurrentFocusInputCapture(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 'u':
			updateIPReservationDialog.SetTitle(fmt.Sprintf("Update Reservation for %s", i.ID))
			setTextFromInputField(updateIPReservationDialog, "Name", i.DisplayName)
			setTextFromInputField(updateIPReservationDialog, "Description", i.Description)
			updateIPReservationDialog.SetFocus(0)
			pages.ShowPage(updateIPReservationPage)
			app.SetFocus(updateIPReservationDialog)
			return nil
		case 'R':
			unreserveIPDialog.SetText(fmt.Sprintf("Unreserve %s?", i.GetID()))
			unreserveIPDialog.SetFocus(1)
			pages.ShowPage(unreserveIPPage)
			app.SetFocus(unreserveIPDialog)
			return nil
		}
	}

	return event
}

func ReserveIP(address, displayName, description string) {
	parent, ok := currentMenuItem.(*Network)
	if !ok {
		panic("ReserveIP called with non-network current menu item")
	}

	reserved := &IP{
		MenuFolder: &MenuFolder{
			ID:         strings.TrimSpace(address),
			ParentPath: parent.GetPath(),
		},
		DisplayName: strings.TrimSpace(displayName),
		Description: strings.TrimSpace(description),
	}
	if err := reserved.Validate(); err != nil {
		statusLine.Clear()
		statusLine.SetText("Error reserving IP: " + err.Error())
		return
	}

	menuItems.MustAdd(reserved)
	reloadMenu(reserved)

	statusLine.Clear()
	statusLine.SetText("Reserved IP: " + reserved.GetPath())
}

func UpdateIPReservation(displayName, description string) {
	focusedIP, ok := currentMenuFocus.(*IP)
	if !ok {
		panic("UpdateIPReservation called on non-IP")
	}

	oldDisplayName := focusedIP.DisplayName
	oldDescription := focusedIP.Description
	focusedIP.DisplayName = strings.TrimSpace(displayName)
	focusedIP.Description = strings.TrimSpace(description)
	if err := focusedIP.Validate(); err != nil {
		focusedIP.DisplayName = oldDisplayName
		focusedIP.Description = oldDescription
		statusLine.Clear()
		statusLine.SetText("Error updating IP reservation: " + err.Error())
		return
	}

	reloadMenu(focusedIP)

	statusLine.Clear()
	statusLine.SetText("Updated IP reservation: " + focusedIP.GetPath())
}

func UnreserveIP() {
	focusedIP, ok := currentMenuFocus.(*IP)
	if !ok {
		panic("UnreserveIP called on non-IP")
	}

	menuItems.Delete(focusedIP)
	reloadMenu(focusedIP)

	statusLine.Clear()
	statusLine.SetText("Unreserved IP: " + focusedIP.GetPath())
}
