package main

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type TaggedVLANMode string

const (
	TaggedVLANModeNone     TaggedVLANMode = ""
	TaggedVLANModeAllowAll TaggedVLANMode = "AllowAll"
	TaggedVLANModeBlockAll TaggedVLANMode = "BlockAll"
	TaggedVLANModeCustom   TaggedVLANMode = "Custom"
)

type Port struct {
	*MenuFolder
	Name           string         `json:"name,omitempty"`
	PortType       string         `json:"port_type"`
	Speed          string         `json:"speed"`
	PoE            string         `json:"poe,omitempty"`
	LAGGroup       int            `json:"lag_group,omitempty"`
	LAGMode        string         `json:"lag_mode,omitempty"`
	NativeVLANID   int            `json:"native_vlan_id,omitempty"`
	TaggedVLANMode TaggedVLANMode `json:"tagged_vlan_mode,omitempty"`
	TaggedVLANIDs  []int          `json:"tagged_vlan_ids,omitempty"`
	ConnectedTo    string         `json:"connected_to,omitempty"`
	Description    string         `json:"description,omitempty"`
}

func (p *Port) Number() int {
	value, err := strconv.Atoi(p.ID)
	if err != nil {
		panic(err)
	}
	return value
}

func (p *Port) Validate() error {
	if err := p.MenuFolder.Validate(); err != nil {
		return err
	}

	if _, err := parsePositiveIntID(p.ID); err != nil {
		return fmt.Errorf("invalid port number %q: %w", p.ID, err)
	}

	parent := p.GetParent()
	if parent == nil {
		return fmt.Errorf("parent not found for Port=%s", p.GetPath())
	}
	if _, ok := parent.(*Equipment); !ok {
		return fmt.Errorf("parent must be equipment for Port=%s", p.GetPath())
	}

	p.Name = strings.TrimSpace(p.Name)
	p.PortType = strings.TrimSpace(p.PortType)
	p.Speed = strings.TrimSpace(p.Speed)
	p.PoE = strings.TrimSpace(p.PoE)
	p.LAGMode = strings.TrimSpace(p.LAGMode)
	p.Description = strings.TrimSpace(p.Description)
	p.ConnectedTo = strings.TrimSpace(p.ConnectedTo)
	slices.Sort(p.TaggedVLANIDs)
	p.TaggedVLANIDs = slices.Compact(p.TaggedVLANIDs)

	if p.PortType == "" {
		return fmt.Errorf("port type must be set for Port=%s", p.GetPath())
	}
	if p.Speed == "" {
		return fmt.Errorf("speed must be set for Port=%s", p.GetPath())
	}

	for _, sibling := range menuItems.GetChilds(parent) {
		other, ok := sibling.(*Port)
		if !ok || other == p {
			continue
		}
		if other.ID == p.ID {
			return fmt.Errorf("port number %s already exists in %s", p.ID, parent.GetPath())
		}
		if p.Name != "" && strings.EqualFold(other.Name, p.Name) {
			return fmt.Errorf("port name %q already exists in %s", p.Name, parent.GetPath())
		}
	}

	if p.NativeVLANID > 0 && findVLANByID(p.NativeVLANID) == nil {
		return fmt.Errorf("native VLAN ID %d not found for Port=%s", p.NativeVLANID, p.GetPath())
	}

	switch p.TaggedVLANMode {
	case TaggedVLANModeNone, TaggedVLANModeAllowAll, TaggedVLANModeBlockAll, TaggedVLANModeCustom:
	default:
		return fmt.Errorf("invalid tagged VLAN mode %q for Port=%s", p.TaggedVLANMode, p.GetPath())
	}
	if p.TaggedVLANMode != TaggedVLANModeCustom && len(p.TaggedVLANIDs) > 0 {
		return fmt.Errorf("tagged VLAN IDs are only supported when tagged mode is Custom for Port=%s", p.GetPath())
	}
	if p.TaggedVLANMode == TaggedVLANModeCustom {
		for _, vlanID := range p.TaggedVLANIDs {
			if findVLANByID(vlanID) == nil {
				return fmt.Errorf("tagged VLAN ID %d not found for Port=%s", vlanID, p.GetPath())
			}
		}
	}

	if p.ConnectedTo != "" {
		if p.ConnectedTo == p.GetPath() {
			return fmt.Errorf("port cannot connect to itself for Port=%s", p.GetPath())
		}
		targetMenuItem := menuItems[p.ConnectedTo]
		if targetMenuItem == nil {
			return fmt.Errorf("connected port not found: %s", p.ConnectedTo)
		}
		targetPort, ok := targetMenuItem.(*Port)
		if !ok {
			return fmt.Errorf("connected item is not a port: %s", p.ConnectedTo)
		}
		if targetPort.ConnectedTo != p.GetPath() {
			return fmt.Errorf("connected port %s must point back to %s", targetPort.GetPath(), p.GetPath())
		}
	}

	if p.LAGGroup > 0 {
		members := []*Port{p}
		for _, sibling := range menuItems.GetChilds(parent) {
			other, ok := sibling.(*Port)
			if !ok || other == p {
				continue
			}
			if other.LAGGroup == p.LAGGroup {
				members = append(members, other)
			}
		}
		minNumber := p.Number()
		for _, member := range members {
			if member.Number() < minNumber {
				minNumber = member.Number()
			}
		}
		if minNumber != p.LAGGroup {
			return fmt.Errorf("LAG group must be lowest member port number (%d) for Port=%s", minNumber, p.GetPath())
		}
		for _, member := range members {
			if member == p {
				continue
			}
			if member.NativeVLANID != p.NativeVLANID ||
				member.TaggedVLANMode != p.TaggedVLANMode ||
				!slices.Equal(member.TaggedVLANIDs, p.TaggedVLANIDs) ||
				member.LAGMode != p.LAGMode {
				return fmt.Errorf("LAG members must share VLAN and LAG mode settings for Port=%s", p.GetPath())
			}
		}
	}

	return nil
}

func (p *Port) GetID() string {
	typeSpeed := strings.TrimSpace(strings.Join([]string{p.PortType, p.PoE, p.Speed}, " "))
	typeSpeed = strings.Join(strings.Fields(typeSpeed), " ")
	if p.Name != "" {
		return fmt.Sprintf("%s: %s (%s)", p.ID, p.Name, typeSpeed)
	}
	return fmt.Sprintf("%s (%s)", p.ID, typeSpeed)
}

func (p *Port) Compare(other MenuItem) int {
	if other == nil {
		return 1
	}
	otherPort, ok := other.(*Port)
	if !ok {
		return cmp.Compare(p.GetID(), other.GetID())
	}
	return cmp.Compare(p.Number(), otherPort.Number())
}

func (p *Port) OnChangedFunc() {
	details := new(strings.Builder)
	details.WriteString(fmt.Sprintf("Port Number          : %s\n", p.ID))
	if p.Name == "" {
		details.WriteString("Name                 : <none>\n")
	} else {
		details.WriteString(fmt.Sprintf("Name                 : %s\n", p.Name))
	}
	details.WriteString(fmt.Sprintf("Type                 : %s\n", p.PortType))
	details.WriteString(fmt.Sprintf("Speed                : %s\n", p.Speed))
	if p.PoE == "" {
		details.WriteString("PoE                  : <none>\n")
	} else {
		details.WriteString(fmt.Sprintf("PoE                  : %s\n", p.PoE))
	}
	if p.LAGGroup > 0 {
		details.WriteString(fmt.Sprintf("LAG Group            : %d\n", p.LAGGroup))
	} else {
		details.WriteString("LAG Group            : <none>\n")
	}
	if p.LAGMode != "" {
		details.WriteString(fmt.Sprintf("LAG Mode             : %s\n", p.LAGMode))
	} else {
		details.WriteString("LAG Mode             : <none>\n")
	}
	details.WriteString(fmt.Sprintf("Native VLAN          : %s\n", renderVLANID(p.NativeVLANID)))
	switch p.TaggedVLANMode {
	case TaggedVLANModeAllowAll:
		details.WriteString("Tagged VLANs         : Allow All\n")
	case TaggedVLANModeBlockAll:
		details.WriteString("Tagged VLANs         : Block All\n")
	case TaggedVLANModeCustom:
		custom := make([]string, 0, len(p.TaggedVLANIDs))
		for _, vlanID := range p.TaggedVLANIDs {
			custom = append(custom, renderVLANID(vlanID))
		}
		details.WriteString(fmt.Sprintf("Tagged VLANs         : Custom (%s)\n", strings.Join(custom, ", ")))
	default:
		details.WriteString("Tagged VLANs         : <none>\n")
	}
	if p.ConnectedTo != "" {
		details.WriteString(fmt.Sprintf("Connected To         : %s\n", renderPortLink(p.ConnectedTo)))
	} else {
		details.WriteString("Connected To         : <none>\n")
	}
	if p.Description != "" {
		details.WriteString(fmt.Sprintf("Description          : %s\n", p.Description))
	} else {
		details.WriteString("Description          : <none>\n")
	}

	detailsPanel.Clear()
	detailsPanel.SetText(details.String())
	currentFocusKeys = []string{
		"<u> Update Port",
		"<c> Connect Port",
		"<x> Disconnect Port",
		"<D> Delete Port",
	}
}

func (p *Port) OnSelectedFunc() {
	positionLine.Clear()
	positionLine.SetText(p.GetPath())
	currentMenuItemKeys = []string{}
}

func (p *Port) OnDoneFunc() {
	positionLine.Clear()
	positionLine.SetText(p.GetPath())
	currentMenuItemKeys = []string{}
}

func (p *Port) CurrentMenuInputCapture(event *tcell.EventKey) *tcell.EventKey {
	return event
}

func (p *Port) CurrentFocusInputCapture(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyRune:
		switch event.Rune() {
		case 'u':
			updatePortDialog.SetTitle(fmt.Sprintf("Update Port %s", p.ID))
			setTextFromInputField(updatePortDialog, "Port Number", p.ID)
			setTextFromInputField(updatePortDialog, "Name", p.Name)
			setTextFromInputField(updatePortDialog, "Port Type", p.PortType)
			setTextFromInputField(updatePortDialog, "Speed", p.Speed)
			setTextFromInputField(updatePortDialog, "PoE", p.PoE)
			if p.LAGGroup > 0 {
				setTextFromInputField(updatePortDialog, "LAG Group", strconv.Itoa(p.LAGGroup))
			} else {
				setTextFromInputField(updatePortDialog, "LAG Group", "")
			}
			setTextFromInputField(updatePortDialog, "LAG Mode", p.LAGMode)
			if p.NativeVLANID > 0 {
				setTextFromInputField(updatePortDialog, "Native VLAN ID", strconv.Itoa(p.NativeVLANID))
			} else {
				setTextFromInputField(updatePortDialog, "Native VLAN ID", "")
			}
			setTextFromInputField(updatePortDialog, "Tagged VLAN Mode", string(p.TaggedVLANMode))
			custom := make([]string, 0, len(p.TaggedVLANIDs))
			for _, vlanID := range p.TaggedVLANIDs {
				custom = append(custom, strconv.Itoa(vlanID))
			}
			setTextFromInputField(updatePortDialog, "Tagged VLAN IDs", strings.Join(custom, ","))
			setTextFromInputField(updatePortDialog, "Description", p.Description)
			updatePortDialog.SetFocus(0)
			pages.ShowPage(updatePortPage)
			app.SetFocus(updatePortDialog)
			return nil
		case 'c':
			options := []string{}
			paths := []string{}
			for _, menuItem := range menuItems {
				otherPort, ok := menuItem.(*Port)
				if !ok || otherPort == p {
					continue
				}
				if otherPort.ConnectedTo != "" {
					continue
				}
				if otherPort.GetParent() == p.GetParent() {
					continue
				}
				options = append(options, renderPortLink(otherPort.GetPath()))
				paths = append(paths, otherPort.GetPath())
			}
			if len(options) == 0 {
				statusLine.Clear()
				statusLine.SetText("No available ports to connect")
				return nil
			}
			portConnectTargets = paths
			_, item := getFormItemByLabel(connectPortDialog, "Target")
			dropdown, ok := item.(*tview.DropDown)
			if !ok {
				panic("failed to cast connect port target dropdown")
			}
			dropdown.SetOptions(options, nil)
			dropdown.SetCurrentOption(0)
			connectPortSelection = 0
			connectPortDialog.SetTitle(fmt.Sprintf("Connect %s", p.GetID()))
			connectPortDialog.SetFocus(0)
			pages.ShowPage(connectPortPage)
			app.SetFocus(connectPortDialog)
			return nil
		case 'x':
			if p.ConnectedTo == "" {
				statusLine.Clear()
				statusLine.SetText("Port is not connected")
				return nil
			}
			disconnectPortDialog.SetText(fmt.Sprintf("Disconnect %s from %s?", p.GetID(), renderPortLink(p.ConnectedTo)))
			disconnectPortDialog.SetFocus(1)
			pages.ShowPage(disconnectPortPage)
			app.SetFocus(disconnectPortDialog)
			return nil
		case 'D':
			deletePortDialog.SetText(fmt.Sprintf("Delete port %s?", p.GetID()))
			deletePortDialog.SetFocus(1)
			pages.ShowPage(deletePortPage)
			app.SetFocus(deletePortDialog)
			return nil
		}
	}
	return event
}

func parseTaggedMode(text string) (TaggedVLANMode, error) {
	trimmed := strings.TrimSpace(text)
	normalized := strings.ReplaceAll(trimmed, " ", "")
	switch trimmed {
	case "":
		return TaggedVLANModeNone, nil
	case "Allow All", "allow all":
		return TaggedVLANModeAllowAll, nil
	case "Block All", "block all":
		return TaggedVLANModeBlockAll, nil
	case "Custom", "custom":
		return TaggedVLANModeCustom, nil
	}
	switch normalized {
	case string(TaggedVLANModeAllowAll):
		return TaggedVLANModeAllowAll, nil
	case string(TaggedVLANModeBlockAll):
		return TaggedVLANModeBlockAll, nil
	case string(TaggedVLANModeCustom):
		return TaggedVLANModeCustom, nil
	default:
		return TaggedVLANModeNone, fmt.Errorf("must be one of: AllowAll, BlockAll, Custom")
	}
}

func parseOptionalIntField(value string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	return strconv.Atoi(trimmed)
}

func AddPort(portNumber, name, portType, speed, poe, lagGroup, lagMode, nativeVLAN, taggedMode, taggedVLANs, description string) {
	parent, ok := currentMenuItem.(*Equipment)
	if !ok {
		panic("AddPort called with non-equipment current menu item")
	}

	number, err := parsePositiveIntID(portNumber)
	if err != nil {
		statusLine.Clear()
		statusLine.SetText("Error adding Port: invalid port number")
		return
	}
	lagGroupValue, err := parseOptionalIntField(lagGroup)
	if err != nil || lagGroupValue < 0 {
		statusLine.Clear()
		statusLine.SetText("Error adding Port: invalid LAG group")
		return
	}
	nativeVLANID, err := parseOptionalIntField(nativeVLAN)
	if err != nil || nativeVLANID < 0 {
		statusLine.Clear()
		statusLine.SetText("Error adding Port: invalid native VLAN ID")
		return
	}
	mode, err := parseTaggedMode(taggedMode)
	if err != nil {
		statusLine.Clear()
		statusLine.SetText("Error adding Port: " + err.Error())
		return
	}
	tagged, err := parseVLANListCSV(taggedVLANs)
	if err != nil {
		statusLine.Clear()
		statusLine.SetText("Error adding Port: " + err.Error())
		return
	}

	port := &Port{
		MenuFolder: &MenuFolder{
			ID:         strconv.Itoa(number),
			ParentPath: parent.GetPath(),
		},
		Name:           strings.TrimSpace(name),
		PortType:       strings.TrimSpace(portType),
		Speed:          strings.TrimSpace(speed),
		PoE:            strings.TrimSpace(poe),
		LAGGroup:       lagGroupValue,
		LAGMode:        strings.TrimSpace(lagMode),
		NativeVLANID:   nativeVLANID,
		TaggedVLANMode: mode,
		TaggedVLANIDs:  tagged,
		Description:    strings.TrimSpace(description),
	}
	if err := port.Validate(); err != nil {
		statusLine.Clear()
		statusLine.SetText("Error adding Port: " + err.Error())
		return
	}

	menuItems.MustAdd(port)
	reloadMenu(port)
	statusLine.Clear()
	statusLine.SetText("Added Port: " + port.GetPath())
}

func UpdatePort(portNumber, name, portType, speed, poe, lagGroup, lagMode, nativeVLAN, taggedMode, taggedVLANs, description string) {
	focusedPort, ok := currentMenuFocus.(*Port)
	if !ok {
		panic("UpdatePort called on non-port")
	}

	number, err := parsePositiveIntID(portNumber)
	if err != nil {
		statusLine.Clear()
		statusLine.SetText("Error updating Port: invalid port number")
		return
	}
	if strconv.Itoa(number) != focusedPort.ID {
		statusLine.Clear()
		statusLine.SetText("Error updating Port: changing port number is not supported")
		return
	}

	lagGroupValue, err := parseOptionalIntField(lagGroup)
	if err != nil || lagGroupValue < 0 {
		statusLine.Clear()
		statusLine.SetText("Error updating Port: invalid LAG group")
		return
	}
	nativeVLANID, err := parseOptionalIntField(nativeVLAN)
	if err != nil || nativeVLANID < 0 {
		statusLine.Clear()
		statusLine.SetText("Error updating Port: invalid native VLAN ID")
		return
	}
	mode, err := parseTaggedMode(taggedMode)
	if err != nil {
		statusLine.Clear()
		statusLine.SetText("Error updating Port: " + err.Error())
		return
	}
	tagged, err := parseVLANListCSV(taggedVLANs)
	if err != nil {
		statusLine.Clear()
		statusLine.SetText("Error updating Port: " + err.Error())
		return
	}

	backup := *focusedPort
	focusedPort.Name = strings.TrimSpace(name)
	focusedPort.PortType = strings.TrimSpace(portType)
	focusedPort.Speed = strings.TrimSpace(speed)
	focusedPort.PoE = strings.TrimSpace(poe)
	focusedPort.LAGGroup = lagGroupValue
	focusedPort.LAGMode = strings.TrimSpace(lagMode)
	focusedPort.NativeVLANID = nativeVLANID
	focusedPort.TaggedVLANMode = mode
	focusedPort.TaggedVLANIDs = tagged
	focusedPort.Description = strings.TrimSpace(description)
	if err := focusedPort.Validate(); err != nil {
		*focusedPort = backup
		statusLine.Clear()
		statusLine.SetText("Error updating Port: " + err.Error())
		return
	}

	reloadMenu(focusedPort)
	statusLine.Clear()
	statusLine.SetText("Updated Port: " + focusedPort.GetPath())
}

func ConnectPort(targetPath string) {
	focusedPort, ok := currentMenuFocus.(*Port)
	if !ok {
		panic("ConnectPort called on non-port")
	}
	targetMenuItem := menuItems[targetPath]
	if targetMenuItem == nil {
		statusLine.Clear()
		statusLine.SetText("Error connecting Port: target not found")
		return
	}
	targetPort, ok := targetMenuItem.(*Port)
	if !ok {
		statusLine.Clear()
		statusLine.SetText("Error connecting Port: target is not a port")
		return
	}
	if targetPort.GetParent() == focusedPort.GetParent() {
		statusLine.Clear()
		statusLine.SetText("Error connecting Port: target must be on different equipment")
		return
	}
	if focusedPort.ConnectedTo != "" || targetPort.ConnectedTo != "" {
		statusLine.Clear()
		statusLine.SetText("Error connecting Port: one of ports is already connected")
		return
	}

	oldFocused := focusedPort.ConnectedTo
	oldTarget := targetPort.ConnectedTo
	focusedPort.ConnectedTo = targetPort.GetPath()
	targetPort.ConnectedTo = focusedPort.GetPath()
	if err := focusedPort.Validate(); err != nil {
		focusedPort.ConnectedTo = oldFocused
		targetPort.ConnectedTo = oldTarget
		statusLine.Clear()
		statusLine.SetText("Error connecting Port: " + err.Error())
		return
	}
	if err := targetPort.Validate(); err != nil {
		focusedPort.ConnectedTo = oldFocused
		targetPort.ConnectedTo = oldTarget
		statusLine.Clear()
		statusLine.SetText("Error connecting Port: " + err.Error())
		return
	}

	reloadMenu(focusedPort)
	statusLine.Clear()
	statusLine.SetText("Connected Port: " + focusedPort.GetPath())
}

func DisconnectPort() {
	focusedPort, ok := currentMenuFocus.(*Port)
	if !ok {
		panic("DisconnectPort called on non-port")
	}
	if focusedPort.ConnectedTo == "" {
		statusLine.Clear()
		statusLine.SetText("Port is not connected")
		return
	}
	targetPath := focusedPort.ConnectedTo
	targetMenuItem := menuItems[targetPath]
	focusedPort.ConnectedTo = ""
	if targetPort, ok := targetMenuItem.(*Port); ok {
		if targetPort.ConnectedTo == focusedPort.GetPath() {
			targetPort.ConnectedTo = ""
		}
	}

	reloadMenu(focusedPort)
	statusLine.Clear()
	statusLine.SetText("Disconnected Port: " + focusedPort.GetPath())
}

func DeletePort() {
	focusedPort, ok := currentMenuFocus.(*Port)
	if !ok {
		panic("DeletePort called on non-port")
	}

	if focusedPort.ConnectedTo != "" {
		targetMenuItem := menuItems[focusedPort.ConnectedTo]
		if targetPort, ok := targetMenuItem.(*Port); ok {
			if targetPort.ConnectedTo == focusedPort.GetPath() {
				targetPort.ConnectedTo = ""
			}
		}
	}

	menuItems.Delete(focusedPort)
	reloadMenu(focusedPort)
	statusLine.Clear()
	statusLine.SetText("Deleted Port: " + focusedPort.GetPath())
}
