package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/plumber-cd/ez-ipam/internal/domain"
	"github.com/rivo/tview"
)

// onMenuKeyPress handles keys for the parent/menu context.
func (a *App) onMenuKeyPress(menuItem domain.Item, event *tcell.EventKey) *tcell.EventKey {
	switch v := menuItem.(type) {
	case *domain.StaticFolder:
		return a.staticFolderMenuKeyPress(v, event)
	case *domain.Network:
		return a.networkMenuKeyPress(v, event)
	case *domain.Equipment:
		return a.equipmentMenuKeyPress(v, event)
	}
	return event
}

// onFocusKeyPress handles keys for the currently focused item.
func (a *App) onFocusKeyPress(focusItem domain.Item, event *tcell.EventKey) *tcell.EventKey {
	switch v := focusItem.(type) {
	case *domain.Network:
		return a.networkFocusKeyPress(v, event)
	case *domain.IP:
		return a.ipFocusKeyPress(v, event)
	case *domain.VLAN:
		return a.vlanFocusKeyPress(v, event)
	case *domain.SSID:
		return a.ssidFocusKeyPress(v, event)
	case *domain.Zone:
		return a.zoneFocusKeyPress(v, event)
	case *domain.Equipment:
		return a.equipmentFocusKeyPress(v, event)
	case *domain.Port:
		return a.portFocusKeyPress(v, event)
	}
	return event
}

// ---------- Static folder menu keys ----------

func (a *App) staticFolderMenuKeyPress(sf *domain.StaticFolder, event *tcell.EventKey) *tcell.EventKey {
	if event.Key() != tcell.KeyRune {
		return event
	}
	switch sf.ID {
	case domain.FolderNetworks:
		if event.Rune() == 'n' {
			a.showDialogByName("*new_network*")
			return nil
		}
	case domain.FolderVLANs:
		if event.Rune() == 'v' {
			a.showVLANDialog("*add_vlan*", "Add VLAN", vlanDialogValues{}, true, func(vals vlanDialogValues) {
				a.AddVLAN(vals.VLANIDText, vals.Name, vals.Description, vals.SelectedZone)
			})
			return nil
		}
	case domain.FolderSSIDs:
		if event.Rune() == 'w' {
			a.showDialogByName("*add_ssid*")
			return nil
		}
	case domain.FolderZones:
		if event.Rune() == 'z' {
			a.showZoneDialog("*add_zone*", "Add Zone", zoneDialogValues{}, func(vals zoneDialogValues) {
				a.AddZone(vals.Name, vals.Description, buildVLANIDsCSV(vals.SelectedVLANs))
			})
			return nil
		}
	case domain.FolderEquipment:
		if event.Rune() == 'e' {
			a.showDialogByName("*add_equipment*")
			return nil
		}
	}
	return event
}

// ---------- Network menu/focus keys ----------

func (a *App) networkMenuKeyPress(n *domain.Network, event *tcell.EventKey) *tcell.EventKey {
	if event.Key() != tcell.KeyRune {
		return event
	}
	if event.Rune() == 'r' {
		if n.AllocationMode != domain.AllocationModeHosts {
			a.setStatus("Reserve IP is available only in Host Pool networks.")
			return nil
		}
		a.showDialogByNameWithTitle("*reserve_ip*", fmt.Sprintf("Reserve IP in %s", n.ID))
		return nil
	}
	return event
}

func (a *App) networkFocusKeyPress(n *domain.Network, event *tcell.EventKey) *tcell.EventKey {
	if event.Key() != tcell.KeyRune {
		return event
	}
	switch event.Rune() {
	case 'a':
		if n.AllocationMode != domain.AllocationModeUnallocated {
			a.setStatus("This network is already allocated. Deallocate it first to re-allocate as Subnet Container.")
			return nil
		}
		a.showNetworkAllocDialog("*allocate_network_subnets_mode*",
			fmt.Sprintf("Allocate as Subnet Container for %s", n.ID),
			networkAllocDialogValues{}, true,
			func(vals networkAllocDialogValues) {
				vlanID, err := domain.ParseOptionalVLANID(vals.VLANID)
				if err != nil {
					a.setStatus("Invalid VLAN ID: " + err.Error())
					return
				}
				subnetsPrefix := strings.TrimLeft(vals.ChildPrefixLen, "/")
				subnetsPrefixInt, err := strconv.Atoi(subnetsPrefix)
				if err != nil {
					a.setStatus("Invalid subnet prefix length: " + err.Error())
					return
				}
				a.AllocateNetworkInSubnetsMode(vals.Name, vals.Description, subnetsPrefixInt, vlanID)
			},
		)
		return nil
	case 'A':
		if n.AllocationMode != domain.AllocationModeUnallocated {
			a.setStatus("This network is already allocated. Deallocate it first to re-allocate as Host Pool.")
			return nil
		}
		a.showNetworkAllocDialog("*allocate_network_hosts_mode*",
			fmt.Sprintf("Allocate as Host Pool for %s", n.ID),
			networkAllocDialogValues{}, false,
			func(vals networkAllocDialogValues) {
				vlanID, err := domain.ParseOptionalVLANID(vals.VLANID)
				if err != nil {
					a.setStatus("Invalid VLAN ID: " + err.Error())
					return
				}
				a.AllocateNetworkInHostsMode(vals.Name, vals.Description, vlanID)
			},
		)
		return nil
	case 'u':
		if n.AllocationMode == domain.AllocationModeUnallocated {
			a.setStatus("No allocation metadata yet. Use Subnet Container or Host Pool first.")
			return nil
		}
		vlanIDStr := ""
		if n.VLANID > 0 {
			vlanIDStr = strconv.Itoa(n.VLANID)
		}
		a.showNetworkAllocDialog("*update_network_allocation*",
			fmt.Sprintf("Update Metadata for %s", n.ID),
			networkAllocDialogValues{Name: n.DisplayName, Description: n.Description, VLANID: vlanIDStr},
			false,
			func(vals networkAllocDialogValues) {
				vlanID, err := domain.ParseOptionalVLANID(vals.VLANID)
				if err != nil {
					a.setStatus("Invalid VLAN ID: " + err.Error())
					return
				}
				a.UpdateNetworkAllocation(vals.Name, vals.Description, vlanID)
			},
		)
		return nil
	case 's':
		if n.AllocationMode != domain.AllocationModeUnallocated {
			a.setStatus("Split is available only for unallocated networks.")
			return nil
		}
		a.showDialogByNameWithTitle("*split_network*", fmt.Sprintf("Split %s", n.ID))
		return nil
	case 'S':
		if n.AllocationMode != domain.AllocationModeUnallocated {
			a.setStatus("Summarize is available only for unallocated sibling networks.")
			return nil
		}

		candidates := a.getUnallocatedSiblingNetworks(n)
		if !a.hasAnySummarizableRange(n) {
			return event
		}

		focusedIndex := 0
		for i, candidate := range candidates {
			if candidate.GetPath() == n.GetPath() {
				focusedIndex = i
			}
		}

		fromIdx := focusedIndex
		toIdx := focusedIndex
		if focusedIndex < len(candidates)-1 {
			toIdx = focusedIndex + 1
		} else if focusedIndex > 0 {
			fromIdx = focusedIndex - 1
		}

		parent := a.Catalog.Get(n.GetParentPath())
		parentDisplayID := domain.FolderNetworks
		if parent != nil {
			parentDisplayID = parent.DisplayID()
		}
		a.showSummarizeDialog(candidates, fromIdx, toIdx, parentDisplayID)
		return nil
	case 'd':
		if n.AllocationMode == domain.AllocationModeUnallocated {
			return event
		}
		a.showModalByNameWithText("*deallocate_network*", fmt.Sprintf("Deallocate %s?\n\nAll child subnets will be removed.", n.DisplayID()))
		return nil
	case 'D':
		parentIsNetwork := false
		parent := a.Catalog.Get(n.GetParentPath())
		if parent != nil {
			_, parentIsNetwork = parent.(*domain.Network)
		}
		if parentIsNetwork {
			return event
		}
		a.showModalByNameWithText("*delete_network*", fmt.Sprintf("Delete %s?\n\nAll child subnets will be removed.", n.DisplayID()))
		return nil
	}
	return event
}

// ---------- IP focus keys ----------

func (a *App) ipFocusKeyPress(ip *domain.IP, event *tcell.EventKey) *tcell.EventKey {
	if event.Key() != tcell.KeyRune {
		return event
	}
	switch event.Rune() {
	case 'u':
		a.showDialogByNameWithTitle("*update_ip_reservation*", fmt.Sprintf("Update Reservation for %s", ip.ID))
		setTextFromInputField(a.getDialogForm("*update_ip_reservation*"), "Name", ip.DisplayName)
		setTextFromTextArea(a.getDialogForm("*update_ip_reservation*"), "Description", ip.Description)
		return nil
	case 'R':
		a.showModalByNameWithText("*unreserve_ip*", fmt.Sprintf("Unreserve %s?", ip.DisplayID()))
		return nil
	}
	return event
}

// ---------- VLAN focus keys ----------

func (a *App) vlanFocusKeyPress(v *domain.VLAN, event *tcell.EventKey) *tcell.EventKey {
	if event.Key() != tcell.KeyRune {
		return event
	}
	switch event.Rune() {
	case 'u':
		vlanID, _ := strconv.Atoi(v.ID)
		vals := vlanDialogValues{
			Name:         v.DisplayName,
			Description:  v.Description,
			SelectedZone: a.getZoneContainingVLAN(vlanID),
		}
		a.showVLANDialog("*update_vlan*", fmt.Sprintf("Update VLAN %s", v.ID), vals, false, func(result vlanDialogValues) {
			a.UpdateVLAN(result.Name, result.Description, result.SelectedZone)
		})
		return nil
	case 'D':
		a.showModalByNameWithText("*delete_vlan*", fmt.Sprintf("Delete VLAN %s (%s)?\n\nNetwork VLAN references will be cleared.", v.ID, v.DisplayName))
		return nil
	}
	return event
}

// ---------- SSID focus keys ----------

func (a *App) ssidFocusKeyPress(s *domain.SSID, event *tcell.EventKey) *tcell.EventKey {
	if event.Key() != tcell.KeyRune {
		return event
	}
	switch event.Rune() {
	case 'u':
		a.showDialogByNameWithTitle("*update_ssid*", fmt.Sprintf("Update WiFi SSID %s", s.ID))
		setTextFromTextArea(a.getDialogForm("*update_ssid*"), "Description", s.Description)
		return nil
	case 'D':
		a.showModalByNameWithText("*delete_ssid*", fmt.Sprintf("Delete WiFi SSID %s?", s.ID))
		return nil
	}
	return event
}

// ---------- Zone focus keys ----------

func (a *App) zoneFocusKeyPress(z *domain.Zone, event *tcell.EventKey) *tcell.EventKey {
	if event.Key() != tcell.KeyRune {
		return event
	}
	switch event.Rune() {
	case 'u':
		selectedVLANs := make(map[int]bool, len(z.VLANIDs))
		for _, id := range z.VLANIDs {
			selectedVLANs[id] = true
		}
		vals := zoneDialogValues{
			Name:          z.DisplayName,
			Description:   z.Description,
			SelectedVLANs: selectedVLANs,
		}
		a.showZoneDialog("*update_zone*", fmt.Sprintf("Update Zone %s", z.DisplayName), vals, func(result zoneDialogValues) {
			a.UpdateZone(result.Name, result.Description, buildVLANIDsCSV(result.SelectedVLANs))
		})
		return nil
	case 'D':
		a.showModalByNameWithText("*delete_zone*", fmt.Sprintf("Delete zone %s?", z.DisplayName))
		return nil
	}
	return event
}

// ---------- Equipment menu/focus keys ----------

func (a *App) equipmentMenuKeyPress(e *domain.Equipment, event *tcell.EventKey) *tcell.EventKey {
	if event.Key() != tcell.KeyRune {
		return event
	}
	if event.Rune() == 'p' {
		a.showPortDialog("*add_port*", fmt.Sprintf("Add Port in %s", e.DisplayName),
			portDialogValues{
				LAGMode:    LagModeDisabledOption,
				TaggedMode: TaggedModeNoneOption,
			},
			func(vals portDialogValues) {
				a.AddPort(vals.PortNumber, vals.Name, vals.PortType, vals.Speed, vals.PoE,
					vals.LAGGroup, vals.LAGMode, vals.NativeVLANID, vals.TaggedMode,
					vals.TaggedVLANIDs, vals.Description)
			},
		)
		return nil
	}
	return event
}

func (a *App) equipmentFocusKeyPress(e *domain.Equipment, event *tcell.EventKey) *tcell.EventKey {
	if event.Key() != tcell.KeyRune {
		return event
	}
	switch event.Rune() {
	case 'u':
		a.showDialogByNameWithTitle("*update_equipment*", fmt.Sprintf("Update Equipment %s", e.DisplayName))
		form := a.getDialogForm("*update_equipment*")
		setTextFromInputField(form, "Name", e.DisplayName)
		setTextFromInputField(form, "Model", e.Model)
		setTextFromTextArea(form, "Description", e.Description)
		return nil
	case 'D':
		a.showModalByNameWithText("*delete_equipment*", fmt.Sprintf("Delete equipment %s?\n\nAll child ports will be removed.", e.DisplayID()))
		return nil
	}
	return event
}

// ---------- Port focus keys ----------

func (a *App) portFocusKeyPress(p *domain.Port, event *tcell.EventKey) *tcell.EventKey {
	if event.Key() != tcell.KeyRune {
		return event
	}
	switch event.Rune() {
	case 'u':
		vals := portDialogValues{
			PortNumber:  p.ID,
			Name:        p.Name,
			PortType:    p.PortType,
			Speed:       p.Speed,
			PoE:         p.PoE,
			LAGMode:     normalizeLagModeOption(p.LAGMode),
			TaggedMode:  normalizeTaggedModeOption(string(p.TaggedVLANMode)),
			Description: p.Description,
		}
		if p.LAGGroup > 0 {
			vals.LAGGroup = strconv.Itoa(p.LAGGroup)
		}
		if p.NativeVLANID > 0 {
			vals.NativeVLANID = strconv.Itoa(p.NativeVLANID)
		}
		custom := make([]string, 0, len(p.TaggedVLANIDs))
		for _, vlanID := range p.TaggedVLANIDs {
			custom = append(custom, strconv.Itoa(vlanID))
		}
		vals.TaggedVLANIDs = strings.Join(custom, ",")

		a.showPortDialog("*update_port*", fmt.Sprintf("Update Port %s", p.ID), vals,
			func(result portDialogValues) {
				a.UpdatePort(result.PortNumber, result.Name, result.PortType, result.Speed, result.PoE,
					result.LAGGroup, result.LAGMode, result.NativeVLANID, result.TaggedMode,
					result.TaggedVLANIDs, result.Description)
			},
		)
		return nil
	case 'c':
		options := []string{}
		paths := []string{}
		for _, item := range a.Catalog.All() {
			otherPort, ok := item.(*domain.Port)
			if !ok || otherPort.GetPath() == p.GetPath() {
				continue
			}
			if otherPort.ConnectedTo != "" {
				continue
			}
			if otherPort.GetParentPath() == p.GetParentPath() {
				continue
			}
			options = append(options, domain.RenderPortLink(a.Catalog, otherPort.GetPath()))
			paths = append(paths, otherPort.GetPath())
		}
		if len(options) == 0 {
			a.setStatus("No available ports to connect")
			return nil
		}
		a.showConnectPortDialog(p.DisplayID(), options, paths)
		return nil
	case 'x':
		if p.ConnectedTo == "" {
			a.setStatus("Port is not connected")
			return nil
		}
		a.showModalByNameWithText("*disconnect_port*", fmt.Sprintf("Disconnect %s from %s?", p.DisplayID(), domain.RenderPortLink(a.Catalog, p.ConnectedTo)))
		return nil
	case 'D':
		a.showModalByNameWithText("*delete_port*", fmt.Sprintf("Delete port %s?", p.DisplayID()))
		return nil
	}
	return event
}

// ---------- Dialog/modal helpers ----------

func (a *App) showDialogByName(pageName string) {
	a.Pages.ShowPage(pageName)
	// For forms, set focus to the first field.
	form := a.getDialogForm(pageName)
	if form != nil {
		form.SetFocus(0)
		a.TviewApp.SetFocus(form)
	}
}

func (a *App) showDialogByNameWithTitle(pageName, title string) {
	// Show the page first so GetFrontPage returns the correct page for getDialogForm.
	a.Pages.ShowPage(pageName)
	form := a.getDialogForm(pageName)
	if form != nil {
		form.SetTitle(title)
		form.SetFocus(0)
		a.TviewApp.SetFocus(form)
	}
}

func (a *App) showModalByNameWithText(pageName, text string) {
	a.Pages.ShowPage(pageName)
	handler := a.getModalFromPage()
	if handler != nil {
		handler.SetText(text)
		handler.SetFocus(1)
		a.TviewApp.SetFocus(handler)
	}
}

// getDialogForm finds the form associated with a dialog page.
func (a *App) getDialogForm(pageName string) *tview.Form {
	// Look up the form from the registry (static dialogs created at init).
	if form, ok := a.dialogForms[pageName]; ok {
		return form
	}
	// Fallback: walk the current front page.
	_, p := a.Pages.GetFrontPage()
	if p == nil {
		return nil
	}
	return findFormInPrimitive(p)
}

// getModalFromPage tries to find a modal within a pages layer.
func (a *App) getModalFromPage() *tview.Modal {
	// All our modal pages contain a *tview.Modal directly as the page content.
	// GetPage is not available, but we can use the front page after showing.
	_, p := a.Pages.GetFrontPage()
	if p == nil {
		return nil
	}
	if modal, ok := p.(*tview.Modal); ok {
		return modal
	}
	return nil
}

func findFormInPrimitive(p tview.Primitive) *tview.Form {
	if form, ok := p.(*tview.Form); ok {
		return form
	}
	if flex, ok := p.(*tview.Flex); ok {
		// Look through flex items.
		for i := range flex.GetItemCount() {
			item := flex.GetItem(i)
			if result := findFormInPrimitive(item); result != nil {
				return result
			}
		}
	}
	return nil
}
