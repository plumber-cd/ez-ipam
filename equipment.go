package main

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type Equipment struct {
	*MenuFolder
	DisplayName string `json:"display_name"`
	Model       string `json:"model"`
	Description string `json:"description"`
}

func (e *Equipment) Validate() error {
	if err := e.MenuFolder.Validate(); err != nil {
		return err
	}

	e.DisplayName = strings.TrimSpace(e.DisplayName)
	e.Model = strings.TrimSpace(e.Model)
	e.Description = strings.TrimSpace(e.Description)
	if e.DisplayName == "" {
		return fmt.Errorf("display name must be set for Equipment=%s", e.GetPath())
	}
	if e.Model == "" {
		return fmt.Errorf("model must be set for Equipment=%s", e.GetPath())
	}

	parent := e.GetParent()
	if parent == nil {
		return fmt.Errorf("parent not found for Equipment=%s", e.GetPath())
	}
	parentStatic, ok := parent.(*MenuStatic)
	if !ok || parentStatic.ID != "Equipment" {
		return fmt.Errorf("parent must be Equipment for Equipment=%s", e.GetPath())
	}

	return nil
}

func (e *Equipment) GetID() string {
	return fmt.Sprintf("%s (%s)", e.DisplayName, e.Model)
}

func (e *Equipment) Compare(other MenuItem) int {
	if other == nil {
		return 1
	}
	otherEquipment, ok := other.(*Equipment)
	if !ok {
		return cmp.Compare(e.GetID(), other.GetID())
	}
	return cmp.Compare(e.DisplayName, otherEquipment.DisplayName)
}

func (e *Equipment) OnChangedFunc() {
	details := new(strings.Builder)
	details.WriteString(fmt.Sprintf("Name                 : %s\n", e.DisplayName))
	details.WriteString(fmt.Sprintf("Model                : %s\n", e.Model))
	if e.Description == "" {
		details.WriteString("Description          : <none>\n")
	} else {
		details.WriteString(fmt.Sprintf("Description          : %s\n", e.Description))
	}

	ports := 0
	for _, child := range menuItems.GetChilds(e) {
		if _, ok := child.(*Port); ok {
			ports++
		}
	}
	details.WriteString(fmt.Sprintf("Ports                : %d\n", ports))

	detailsPanel.Clear()
	detailsPanel.SetText(details.String())
	currentFocusKeys = []string{
		"<u> Update Equipment",
		"<D> Delete Equipment",
	}
}

func (e *Equipment) OnSelectedFunc() {
	positionLine.Clear()
	positionLine.SetText(e.GetPath())
	currentMenuItemKeys = []string{
		"<p> New Port",
	}
}

func (e *Equipment) OnDoneFunc() {
	positionLine.Clear()
	positionLine.SetText(e.GetPath())
	currentMenuItemKeys = []string{}
}

func (e *Equipment) CurrentMenuInputCapture(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 'p':
			addPortDialog.SetTitle(fmt.Sprintf("Add Port in %s", e.DisplayName))
			setTextFromInputField(addPortDialog, "Port Number", "")
			setTextFromInputField(addPortDialog, "Name", "")
			setTextFromInputField(addPortDialog, "Port Type", "")
			setTextFromInputField(addPortDialog, "Speed", "")
			setTextFromInputField(addPortDialog, "PoE", "")
			setTextFromInputField(addPortDialog, "LAG Group", "")
			setTextFromInputField(addPortDialog, "LAG Mode", "")
			setTextFromInputField(addPortDialog, "Native VLAN ID", "")
			setTextFromInputField(addPortDialog, "Tagged VLAN Mode", "")
			setTextFromInputField(addPortDialog, "Tagged VLAN IDs", "")
			setTextFromInputField(addPortDialog, "Description", "")
			addPortDialog.SetFocus(0)
			pages.ShowPage(addPortPage)
			app.SetFocus(addPortDialog)
			return nil
		}
	}
	return event
}

func (e *Equipment) CurrentFocusInputCapture(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 'u':
			updateEquipmentDialog.SetTitle(fmt.Sprintf("Update Equipment %s", e.DisplayName))
			setTextFromInputField(updateEquipmentDialog, "Name", e.DisplayName)
			setTextFromInputField(updateEquipmentDialog, "Model", e.Model)
			setTextFromInputField(updateEquipmentDialog, "Description", e.Description)
			updateEquipmentDialog.SetFocus(0)
			pages.ShowPage(updateEquipmentPage)
			app.SetFocus(updateEquipmentDialog)
			return nil
		case 'D':
			deleteEquipmentDialog.SetText(fmt.Sprintf("Delete equipment %s?\n\nAll child ports will be removed.", e.GetID()))
			deleteEquipmentDialog.SetFocus(1)
			pages.ShowPage(deleteEquipmentPage)
			app.SetFocus(deleteEquipmentDialog)
			return nil
		}
	}
	return event
}

func AddEquipment(displayName, model, description string) {
	parent, ok := currentMenuItem.(*MenuStatic)
	if !ok || parent.ID != "Equipment" {
		panic("AddEquipment called with non-Equipment current menu item")
	}

	equipment := &Equipment{
		MenuFolder: &MenuFolder{
			ID:         strings.TrimSpace(displayName),
			ParentPath: parent.GetPath(),
		},
		DisplayName: strings.TrimSpace(displayName),
		Model:       strings.TrimSpace(model),
		Description: strings.TrimSpace(description),
	}
	if err := equipment.Validate(); err != nil {
		statusLine.Clear()
		statusLine.SetText("Error adding Equipment: " + err.Error())
		return
	}

	for _, sibling := range menuItems.GetChilds(parent) {
		other, ok := sibling.(*Equipment)
		if !ok {
			continue
		}
		if strings.EqualFold(other.DisplayName, equipment.DisplayName) {
			statusLine.Clear()
			statusLine.SetText("Error adding Equipment: duplicate equipment")
			return
		}
	}

	menuItems.MustAdd(equipment)
	reloadMenu(equipment)
	statusLine.Clear()
	statusLine.SetText("Added Equipment: " + equipment.GetPath())
}

func UpdateEquipment(displayName, model, description string) {
	focusedEquipment, ok := currentMenuFocus.(*Equipment)
	if !ok {
		panic("UpdateEquipment called on non-Equipment")
	}

	newName := strings.TrimSpace(displayName)
	newModel := strings.TrimSpace(model)
	newDescription := strings.TrimSpace(description)
	newPath := focusedEquipment.ParentPath + " -> " + newName
	oldPath := focusedEquipment.GetPath()

	if strings.EqualFold(newName, focusedEquipment.DisplayName) {
		// keep exact casing stable when names match ignoring case
		newName = focusedEquipment.DisplayName
		newPath = oldPath
	}

	candidate := &Equipment{
		MenuFolder: &MenuFolder{
			ID:         newName,
			ParentPath: focusedEquipment.ParentPath,
		},
		DisplayName: newName,
		Model:       newModel,
		Description: newDescription,
	}
	if err := candidate.Validate(); err != nil {
		statusLine.Clear()
		statusLine.SetText("Error updating Equipment: " + err.Error())
		return
	}

	for _, sibling := range menuItems.GetChilds(focusedEquipment.GetParent()) {
		other, ok := sibling.(*Equipment)
		if !ok || other == focusedEquipment {
			continue
		}
		if strings.EqualFold(other.DisplayName, candidate.DisplayName) {
			statusLine.Clear()
			statusLine.SetText("Error updating Equipment: duplicate equipment")
			return
		}
	}

	children := menuItems.GetChilds(focusedEquipment)
	ports := []*Port{}
	for _, child := range children {
		port, ok := child.(*Port)
		if !ok {
			continue
		}
		ports = append(ports, port)
	}

	delete(menuItems, oldPath)
	menuItems.MustAdd(candidate)

	for _, port := range ports {
		oldPortPath := port.GetPath()
		delete(menuItems, oldPortPath)
		port.ParentPath = newPath
		menuItems.MustAdd(port)
	}

	oldPrefix := oldPath + " -> "
	newPrefix := newPath + " -> "
	for _, menuItem := range menuItems {
		port, ok := menuItem.(*Port)
		if !ok || port.ConnectedTo == "" {
			continue
		}
		if strings.HasPrefix(port.ConnectedTo, oldPrefix) {
			port.ConnectedTo = newPrefix + strings.TrimPrefix(port.ConnectedTo, oldPrefix)
		}
	}

	currentMenuFocus = candidate
	reloadMenu(candidate)
	statusLine.Clear()
	statusLine.SetText("Updated Equipment: " + candidate.GetPath())
}

func DeleteEquipment() {
	focusedEquipment, ok := currentMenuFocus.(*Equipment)
	if !ok {
		panic("DeleteEquipment called on non-Equipment")
	}

	children := menuItems.GetChilds(focusedEquipment)
	portPaths := map[string]struct{}{}
	for _, child := range children {
		port, ok := child.(*Port)
		if !ok {
			continue
		}
		portPaths[port.GetPath()] = struct{}{}
	}
	for _, menuItem := range menuItems {
		port, ok := menuItem.(*Port)
		if !ok || port.ConnectedTo == "" {
			continue
		}
		if _, removing := portPaths[port.ConnectedTo]; removing {
			port.ConnectedTo = ""
		}
	}

	menuItems.Delete(focusedEquipment)
	reloadMenu(focusedEquipment)
	statusLine.Clear()
	statusLine.SetText("Deleted Equipment: " + focusedEquipment.GetPath())
}

func equipmentAndPortFromPath(path string) (*Equipment, *Port) {
	menuItem := menuItems[path]
	if menuItem == nil {
		return nil, nil
	}
	port, ok := menuItem.(*Port)
	if !ok {
		return nil, nil
	}
	equipment, ok := port.GetParent().(*Equipment)
	if !ok {
		return nil, nil
	}
	return equipment, port
}

func renderPortLink(path string) string {
	equipment, port := equipmentAndPortFromPath(path)
	if equipment == nil || port == nil {
		return path
	}
	number, _ := strconv.Atoi(port.ID)
	if strings.TrimSpace(port.Name) != "" {
		return fmt.Sprintf("%s Port %d (%s)", equipment.DisplayName, number, port.Name)
	}
	return fmt.Sprintf("%s Port %d", equipment.DisplayName, number)
}
