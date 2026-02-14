package main

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type Zone struct {
	*MenuFolder
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	VLANIDs     []int  `json:"vlan_ids,omitempty"`
}

func (z *Zone) Validate() error {
	if err := z.MenuFolder.Validate(); err != nil {
		return err
	}

	z.DisplayName = strings.TrimSpace(z.DisplayName)
	if z.DisplayName == "" {
		return fmt.Errorf("display name must be set for Zone=%s", z.GetPath())
	}

	parent := z.GetParent()
	if parent == nil {
		return fmt.Errorf("parent not found for Zone=%s", z.GetPath())
	}
	parentStatic, ok := parent.(*MenuStatic)
	if !ok || parentStatic.ID != "Zones" {
		return fmt.Errorf("parent must be Zones for Zone=%s", z.GetPath())
	}

	slices.Sort(z.VLANIDs)
	z.VLANIDs = slices.Compact(z.VLANIDs)
	for _, vlanID := range z.VLANIDs {
		if vlanID < 1 || vlanID > 4094 {
			return fmt.Errorf("invalid VLAN ID %d for Zone=%s", vlanID, z.GetPath())
		}
		if findVLANByID(vlanID) == nil {
			return fmt.Errorf("VLAN ID %d not found for Zone=%s", vlanID, z.GetPath())
		}
	}

	return nil
}

func (z *Zone) GetID() string {
	return z.DisplayName
}

func (z *Zone) Compare(other MenuItem) int {
	if other == nil {
		return 1
	}
	otherZone, ok := other.(*Zone)
	if !ok {
		return cmp.Compare(z.GetID(), other.GetID())
	}
	return cmp.Compare(z.DisplayName, otherZone.DisplayName)
}

func (z *Zone) OnChangedFunc() {
	details := new(strings.Builder)
	details.WriteString(fmt.Sprintf("Zone                 : %s\n", z.DisplayName))
	if strings.TrimSpace(z.Description) == "" {
		details.WriteString("Description          : <none>\n")
	} else {
		details.WriteString(fmt.Sprintf("Description          : %s\n", z.Description))
	}

	details.WriteString("Associated VLANs     :")
	if len(z.VLANIDs) == 0 {
		details.WriteString(" <none>\n")
	} else {
		for _, vlanID := range z.VLANIDs {
			details.WriteString("\n- " + renderVLANID(vlanID))
		}
		details.WriteString("\n")
	}

	detailsPanel.Clear()
	detailsPanel.SetText(details.String())
	currentFocusKeys = []string{
		"<u> Update Zone",
		"<D> Delete Zone",
	}
}

func (z *Zone) OnSelectedFunc() {
	positionLine.Clear()
	positionLine.SetText(z.GetPath())
	currentMenuItemKeys = []string{}
}

func (z *Zone) OnDoneFunc() {
	positionLine.Clear()
	positionLine.SetText(z.GetPath())
	currentMenuItemKeys = []string{}
}

func (z *Zone) CurrentMenuInputCapture(event *tcell.EventKey) *tcell.EventKey {
	return event
}

func (z *Zone) CurrentFocusInputCapture(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 'u':
			updateZoneDialog.SetTitle(fmt.Sprintf("Update Zone %s", z.DisplayName))
			setTextFromInputField(updateZoneDialog, "Name", z.DisplayName)
			setTextFromInputField(updateZoneDialog, "Description", z.Description)
			zoneVLANs := make([]string, 0, len(z.VLANIDs))
			for _, vlanID := range z.VLANIDs {
				zoneVLANs = append(zoneVLANs, fmt.Sprintf("%d", vlanID))
			}
			setTextFromInputField(updateZoneDialog, "VLAN IDs", strings.Join(zoneVLANs, ","))
			updateZoneDialog.SetFocus(0)
			pages.ShowPage(updateZonePage)
			app.SetFocus(updateZoneDialog)
			return nil
		case 'D':
			deleteZoneDialog.SetText(fmt.Sprintf("Delete zone %s?", z.DisplayName))
			deleteZoneDialog.SetFocus(1)
			pages.ShowPage(deleteZonePage)
			app.SetFocus(deleteZoneDialog)
			return nil
		}
	}
	return event
}

func AddZone(displayName, description, vlanIDsText string) {
	parent, ok := currentMenuItem.(*MenuStatic)
	if !ok || parent.ID != "Zones" {
		panic("AddZone called with non-Zones current menu item")
	}

	vlanIDs, err := parseVLANListCSV(vlanIDsText)
	if err != nil {
		statusLine.Clear()
		statusLine.SetText("Error adding Zone: " + err.Error())
		return
	}

	name := strings.TrimSpace(displayName)
	zone := &Zone{
		MenuFolder: &MenuFolder{
			ID:         name,
			ParentPath: parent.GetPath(),
		},
		DisplayName: name,
		Description: strings.TrimSpace(description),
		VLANIDs:     vlanIDs,
	}
	if err := zone.Validate(); err != nil {
		statusLine.Clear()
		statusLine.SetText("Error adding Zone: " + err.Error())
		return
	}

	for _, sibling := range menuItems.GetChilds(parent) {
		other, ok := sibling.(*Zone)
		if !ok {
			continue
		}
		if strings.EqualFold(other.DisplayName, zone.DisplayName) {
			statusLine.Clear()
			statusLine.SetText("Error adding Zone: zone already exists")
			return
		}
	}

	menuItems.MustAdd(zone)
	reloadMenu(zone)
	statusLine.Clear()
	statusLine.SetText("Added Zone: " + zone.GetPath())
}

func UpdateZone(displayName, description, vlanIDsText string) {
	focusedZone, ok := currentMenuFocus.(*Zone)
	if !ok {
		panic("UpdateZone called on non-Zone")
	}

	vlanIDs, err := parseVLANListCSV(vlanIDsText)
	if err != nil {
		statusLine.Clear()
		statusLine.SetText("Error updating Zone: " + err.Error())
		return
	}

	name := strings.TrimSpace(displayName)
	updated := &Zone{
		MenuFolder: &MenuFolder{
			ID:         name,
			ParentPath: focusedZone.ParentPath,
		},
		DisplayName: name,
		Description: strings.TrimSpace(description),
		VLANIDs:     vlanIDs,
	}
	if err := updated.Validate(); err != nil {
		statusLine.Clear()
		statusLine.SetText("Error updating Zone: " + err.Error())
		return
	}

	for _, sibling := range menuItems.GetChilds(focusedZone.GetParent()) {
		other, ok := sibling.(*Zone)
		if !ok || other == focusedZone {
			continue
		}
		if strings.EqualFold(other.DisplayName, updated.DisplayName) {
			statusLine.Clear()
			statusLine.SetText("Error updating Zone: zone already exists")
			return
		}
	}

	delete(menuItems, focusedZone.GetPath())
	menuItems.MustAdd(updated)
	currentMenuFocus = updated
	reloadMenu(updated)
	statusLine.Clear()
	statusLine.SetText("Updated Zone: " + updated.GetPath())
}

func DeleteZone() {
	focusedZone, ok := currentMenuFocus.(*Zone)
	if !ok {
		panic("DeleteZone called on non-Zone")
	}

	menuItems.Delete(focusedZone)
	reloadMenu(focusedZone)
	statusLine.Clear()
	statusLine.SetText("Deleted Zone: " + focusedZone.GetPath())
}
