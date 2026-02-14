package main

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type SSID struct {
	*MenuFolder
	Description string `json:"description"`
}

func (s *SSID) Validate() error {
	if err := s.MenuFolder.Validate(); err != nil {
		return err
	}

	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("SSID ID must be set for SSID=%s", s.GetPath())
	}

	parent := s.GetParent()
	if parent == nil {
		return fmt.Errorf("parent not found for SSID=%s", s.GetPath())
	}
	parentStatic, ok := parent.(*MenuStatic)
	if !ok || parentStatic.ID != "WiFi SSIDs" {
		return fmt.Errorf("parent must be WiFi SSIDs for SSID=%s", s.GetPath())
	}

	return nil
}

func (s *SSID) GetID() string {
	return s.ID
}

func (s *SSID) Compare(other MenuItem) int {
	if other == nil {
		return 1
	}
	otherSSID, ok := other.(*SSID)
	if !ok {
		return cmp.Compare(s.GetID(), other.GetID())
	}

	return cmp.Compare(s.ID, otherSSID.ID)
}

func (s *SSID) OnChangedFunc() {
	details := new(strings.Builder)
	details.WriteString(fmt.Sprintf("SSID                 : %s\n", s.ID))
	if strings.TrimSpace(s.Description) == "" {
		details.WriteString("Description          : <none>\n")
	} else {
		details.WriteString(fmt.Sprintf("Description          : %s\n", s.Description))
	}

	detailsPanel.Clear()
	detailsPanel.SetText(details.String())
	currentFocusKeys = []string{
		"<u> Update WiFi SSID",
		"<D> Delete WiFi SSID",
	}
}

func (s *SSID) OnSelectedFunc() {
	positionLine.Clear()
	positionLine.SetText(s.GetPath())
	currentMenuItemKeys = []string{}
}

func (s *SSID) OnDoneFunc() {
	positionLine.Clear()
	positionLine.SetText(s.GetPath())
	currentMenuItemKeys = []string{}
}

func (s *SSID) CurrentMenuInputCapture(event *tcell.EventKey) *tcell.EventKey {
	return event
}

func (s *SSID) CurrentFocusInputCapture(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 'u':
			updateSSIDDialog.SetTitle(fmt.Sprintf("Update WiFi SSID %s", s.ID))
			setTextFromInputField(updateSSIDDialog, "Description", s.Description)
			updateSSIDDialog.SetFocus(0)
			pages.ShowPage(updateSSIDPage)
			app.SetFocus(updateSSIDDialog)
			return nil
		case 'D':
			deleteSSIDDialog.SetText(fmt.Sprintf("Delete WiFi SSID %s?", s.ID))
			deleteSSIDDialog.SetFocus(1)
			pages.ShowPage(deleteSSIDPage)
			app.SetFocus(deleteSSIDDialog)
			return nil
		}
	}

	return event
}

func AddSSID(id, description string) {
	parent, ok := currentMenuItem.(*MenuStatic)
	if !ok || parent.ID != "WiFi SSIDs" {
		panic("AddSSID called with non-WiFi SSIDs current menu item")
	}

	ssid := &SSID{
		MenuFolder: &MenuFolder{
			ID:         strings.TrimSpace(id),
			ParentPath: parent.GetPath(),
		},
		Description: strings.TrimSpace(description),
	}

	if err := ssid.Validate(); err != nil {
		statusLine.Clear()
		statusLine.SetText("Error adding WiFi SSID: " + err.Error())
		return
	}

	if menuItems.GetByParentAndID(parent, ssid.GetID()) != nil {
		statusLine.Clear()
		statusLine.SetText("Error adding WiFi SSID: duplicate WiFi SSID")
		return
	}

	for _, sibling := range menuItems.GetChilds(parent) {
		other, ok := sibling.(*SSID)
		if !ok {
			continue
		}
		if other.ID == ssid.ID {
			statusLine.Clear()
			statusLine.SetText("Error adding WiFi SSID: SSID already exists")
			return
		}
	}

	menuItems.MustAdd(ssid)
	reloadMenu(ssid)

	statusLine.Clear()
	statusLine.SetText("Added WiFi SSID: " + ssid.GetPath())
}

func UpdateSSID(description string) {
	focusedSSID, ok := currentMenuFocus.(*SSID)
	if !ok {
		panic("UpdateSSID called on non-SSID")
	}

	oldDescription := focusedSSID.Description
	focusedSSID.Description = strings.TrimSpace(description)
	if err := focusedSSID.Validate(); err != nil {
		focusedSSID.Description = oldDescription
		statusLine.Clear()
		statusLine.SetText("Error updating WiFi SSID: " + err.Error())
		return
	}

	reloadMenu(focusedSSID)
	statusLine.Clear()
	statusLine.SetText("Updated WiFi SSID: " + focusedSSID.GetPath())
}

func DeleteSSID() {
	focusedSSID, ok := currentMenuFocus.(*SSID)
	if !ok {
		panic("DeleteSSID called on non-SSID")
	}

	menuItems.Delete(focusedSSID)
	reloadMenu(focusedSSID)

	statusLine.Clear()
	statusLine.SetText("Deleted WiFi SSID: " + focusedSSID.GetPath())
}
