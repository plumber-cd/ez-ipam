package main

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type VLAN struct {
	*MenuFolder
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

func (v *VLAN) Validate() error {
	if err := v.MenuFolder.Validate(); err != nil {
		return err
	}

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

	parent := v.GetParent()
	if parent == nil {
		return fmt.Errorf("parent not found for VLAN=%s", v.GetPath())
	}
	parentStatic, ok := parent.(*MenuStatic)
	if !ok || parentStatic.ID != "VLANs" {
		return fmt.Errorf("parent must be VLANs for VLAN=%s", v.GetPath())
	}

	return nil
}

func (v *VLAN) GetID() string {
	return fmt.Sprintf("%s (%s)", v.ID, v.DisplayName)
}

func (v *VLAN) Compare(other MenuItem) int {
	if other == nil {
		return 1
	}
	otherVLAN, ok := other.(*VLAN)
	if !ok {
		return cmp.Compare(v.GetID(), other.GetID())
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

func (v *VLAN) OnChangedFunc() {
	details := new(strings.Builder)
	details.WriteString(fmt.Sprintf("VLAN ID              : %s\n", v.ID))
	details.WriteString(fmt.Sprintf("Display Name         : %s\n", v.DisplayName))
	if strings.TrimSpace(v.Description) == "" {
		details.WriteString("Description          : <none>\n")
	} else {
		details.WriteString(fmt.Sprintf("Description          : %s\n", v.Description))
	}

	associated := []string{}
	for _, menuItem := range menuItems {
		network, ok := menuItem.(*Network)
		if !ok {
			continue
		}
		if network.VLANID > 0 && strconv.Itoa(network.VLANID) == v.ID {
			associated = append(associated, network.GetPath())
		}
	}

	details.WriteString("\nAssociated Networks  :")
	if len(associated) == 0 {
		details.WriteString(" <none>")
	} else {
		for _, path := range associated {
			details.WriteString("\n- " + path)
		}
	}

	detailsPanel.Clear()
	detailsPanel.SetText(details.String())
	currentFocusKeys = []string{
		"<u> Update VLAN",
		"<D> Delete VLAN",
	}
}

func (v *VLAN) OnSelectedFunc() {
	positionLine.Clear()
	positionLine.SetText(v.GetPath())
	currentMenuItemKeys = []string{}
}

func (v *VLAN) OnDoneFunc() {
	positionLine.Clear()
	positionLine.SetText(v.GetPath())
	currentMenuItemKeys = []string{}
}

func (v *VLAN) CurrentMenuInputCapture(event *tcell.EventKey) *tcell.EventKey {
	return event
}

func (v *VLAN) CurrentFocusInputCapture(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 'u':
			updateVLANDialog.SetTitle(fmt.Sprintf("Update VLAN %s", v.ID))
			setTextFromInputField(updateVLANDialog, "Name", v.DisplayName)
			setTextFromInputField(updateVLANDialog, "Description", v.Description)
			updateVLANDialog.SetFocus(0)
			pages.ShowPage(updateVLANPage)
			app.SetFocus(updateVLANDialog)
			return nil
		case 'D':
			deleteVLANDialog.SetText(fmt.Sprintf("Delete VLAN %s (%s)?\n\nNetwork VLAN references will be cleared.", v.ID, v.DisplayName))
			deleteVLANDialog.SetFocus(1)
			pages.ShowPage(deleteVLANPage)
			app.SetFocus(deleteVLANDialog)
			return nil
		}
	}

	return event
}

func AddVLAN(id, displayName, description string) {
	parent, ok := currentMenuItem.(*MenuStatic)
	if !ok || parent.ID != "VLANs" {
		panic("AddVLAN called with non-VLANs current menu item")
	}

	vlan := &VLAN{
		MenuFolder: &MenuFolder{
			ID:         strings.TrimSpace(id),
			ParentPath: parent.GetPath(),
		},
		DisplayName: strings.TrimSpace(displayName),
		Description: strings.TrimSpace(description),
	}

	if err := vlan.Validate(); err != nil {
		statusLine.Clear()
		statusLine.SetText("Error adding VLAN: " + err.Error())
		return
	}

	if menuItems.GetByParentAndID(parent, vlan.GetID()) != nil {
		statusLine.Clear()
		statusLine.SetText("Error adding VLAN: duplicate VLAN")
		return
	}

	for _, sibling := range menuItems.GetChilds(parent) {
		other, ok := sibling.(*VLAN)
		if !ok {
			continue
		}
		if other.ID == vlan.ID {
			statusLine.Clear()
			statusLine.SetText("Error adding VLAN: VLAN ID already exists")
			return
		}
	}

	menuItems.MustAdd(vlan)
	reloadMenu(vlan)

	statusLine.Clear()
	statusLine.SetText("Added VLAN: " + vlan.GetPath())
}

func UpdateVLAN(displayName, description string) {
	focusedVLAN, ok := currentMenuFocus.(*VLAN)
	if !ok {
		panic("UpdateVLAN called on non-VLAN")
	}

	oldDisplayName := focusedVLAN.DisplayName
	oldDescription := focusedVLAN.Description
	focusedVLAN.DisplayName = strings.TrimSpace(displayName)
	focusedVLAN.Description = strings.TrimSpace(description)
	if err := focusedVLAN.Validate(); err != nil {
		focusedVLAN.DisplayName = oldDisplayName
		focusedVLAN.Description = oldDescription
		statusLine.Clear()
		statusLine.SetText("Error updating VLAN: " + err.Error())
		return
	}

	reloadMenu(focusedVLAN)
	statusLine.Clear()
	statusLine.SetText("Updated VLAN: " + focusedVLAN.GetPath())
}

func DeleteVLAN() {
	focusedVLAN, ok := currentMenuFocus.(*VLAN)
	if !ok {
		panic("DeleteVLAN called on non-VLAN")
	}

	vlanID, err := strconv.Atoi(focusedVLAN.ID)
	if err != nil {
		panic(err)
	}

	for _, menuItem := range menuItems {
		network, ok := menuItem.(*Network)
		if !ok {
			continue
		}
		if network.VLANID == vlanID {
			network.VLANID = 0
		}
	}

	menuItems.Delete(focusedVLAN)
	reloadMenu(focusedVLAN)

	statusLine.Clear()
	statusLine.SetText("Deleted VLAN: " + focusedVLAN.GetPath())
}
