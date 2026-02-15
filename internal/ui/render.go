package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/plumber-cd/ez-ipam/internal/domain"
)

// onItemChanged is called when nav panel focus moves to a new item.
func (a *App) onItemChanged(item domain.Item) {
	switch v := item.(type) {
	case *domain.StaticFolder:
		a.renderStaticFolder(v)
	case *domain.Network:
		a.renderNetwork(v)
	case *domain.IP:
		a.renderIP(v)
	case *domain.VLAN:
		a.renderVLAN(v)
	case *domain.SSID:
		a.renderSSID(v)
	case *domain.Zone:
		a.renderZone(v)
	case *domain.Equipment:
		a.renderEquipment(v)
	case *domain.Port:
		a.renderPort(v)
	default:
		a.DetailsPanel.Clear()
		a.CurrentFocusKeys = nil
	}
}

// onItemSelected is called when an item is entered (navigated into).
func (a *App) onItemSelected(item domain.Item) {
	a.PositionLine.Clear()
	a.PositionLine.SetText(item.GetPath())

	switch v := item.(type) {
	case *domain.StaticFolder:
		a.CurrentMenuItemKeys = a.staticFolderMenuKeys(v)
	case *domain.Network:
		if v.AllocationMode == domain.AllocationModeHosts {
			a.CurrentMenuItemKeys = []string{"<r> Reserve IP"}
		} else {
			a.CurrentMenuItemKeys = []string{}
		}
	case *domain.Equipment:
		a.CurrentMenuItemKeys = []string{"<p> New Port"}
	default:
		a.CurrentMenuItemKeys = []string{}
	}
}

// onItemDone is called when leaving an item (going back).
func (a *App) onItemDone(item domain.Item) {
	a.PositionLine.Clear()
	a.PositionLine.SetText(item.GetPath())
	a.CurrentMenuItemKeys = []string{}
}

// staticFolderMenuKeys returns keys for static folder context menus.
func (a *App) staticFolderMenuKeys(sf *domain.StaticFolder) []string {
	switch sf.ID {
	case domain.FolderNetworks:
		return []string{"<n> New Network"}
	case domain.FolderVLANs:
		return []string{"<v> New VLAN"}
	case domain.FolderSSIDs:
		return []string{"<w> New SSID"}
	case domain.FolderZones:
		return []string{"<z> New Zone"}
	case domain.FolderEquipment:
		return []string{"<e> New Equipment"}
	default:
		return nil
	}
}

// ---------- Detail renderers ----------

func (a *App) renderStaticFolder(sf *domain.StaticFolder) {
	a.DetailsPanel.Clear()
	a.DetailsPanel.SetText(sf.Description)
	a.CurrentFocusKeys = nil
}

func (a *App) renderNetwork(n *domain.Network) {
	a.DetailsPanel.Clear()
	a.DetailsPanel.SetText(n.RenderDetails(a.Catalog))

	parentIsNetwork := false
	parent := a.Catalog.Get(n.GetParentPath())
	if parent != nil {
		_, parentIsNetwork = parent.(*domain.Network)
	}

	if n.AllocationMode != domain.AllocationModeUnallocated {
		a.CurrentFocusKeys = []string{
			"<u> Update Metadata",
			"<d> Deallocate",
		}
	} else {
		a.CurrentFocusKeys = []string{
			"<a> Allocate Subnet Container",
			"<A> Allocate Host Pool",
			"<s> Split",
		}
		if a.hasAnySummarizableRange(n) {
			a.CurrentFocusKeys = append(a.CurrentFocusKeys, "<S> Summarize Range")
		}
	}
	if !parentIsNetwork {
		a.CurrentFocusKeys = append(a.CurrentFocusKeys, "<D> Delete")
	}
}

func (a *App) renderIP(ip *domain.IP) {
	a.DetailsPanel.Clear()
	description := ip.Description
	if description == "" {
		description = "<none>"
	}
	a.DetailsPanel.SetText(fmt.Sprintf(
		"IP Address           : %s\nDisplay Name         : %s\nDescription          : %s\nParent Network       : %s\n",
		ip.ID,
		ip.DisplayName,
		description,
		ip.GetParentPath(),
	))
	a.CurrentFocusKeys = []string{
		"<u> Update Reservation",
		"<R> Unreserve",
	}
}

func (a *App) renderVLAN(v *domain.VLAN) {
	details := new(strings.Builder)
	fmt.Fprintf(details, "VLAN ID              : %s\n", v.ID)
	fmt.Fprintf(details, "Display Name         : %s\n", v.DisplayName)
	if strings.TrimSpace(v.Description) == "" {
		details.WriteString("Description          : <none>\n")
	} else {
		fmt.Fprintf(details, "Description          : %s\n", v.Description)
	}

	associated := []string{}
	associatedPorts := []string{}
	associatedZones := []string{}
	for _, item := range a.Catalog.All() {
		network, ok := item.(*domain.Network)
		if ok {
			if network.VLANID > 0 && strconv.Itoa(network.VLANID) == v.ID {
				associated = append(associated, network.GetPath())
			}
			continue
		}
		port, ok := item.(*domain.Port)
		if ok {
			matched := false
			if port.NativeVLANID > 0 && strconv.Itoa(port.NativeVLANID) == v.ID {
				matched = true
			}
			if !matched {
				for _, tagged := range port.TaggedVLANIDs {
					if strconv.Itoa(tagged) == v.ID {
						matched = true
						break
					}
				}
			}
			if matched {
				associatedPorts = append(associatedPorts, port.GetPath())
			}
			continue
		}
		zone, ok := item.(*domain.Zone)
		if ok {
			for _, vlanID := range zone.VLANIDs {
				if strconv.Itoa(vlanID) == v.ID {
					associatedZones = append(associatedZones, zone.GetPath())
					break
				}
			}
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
	details.WriteString("\n\nAssociated Ports     :")
	if len(associatedPorts) == 0 {
		details.WriteString(" <none>")
	} else {
		for _, path := range associatedPorts {
			details.WriteString("\n- " + path)
		}
	}
	details.WriteString("\n\nAssociated Zones     :")
	if len(associatedZones) == 0 {
		details.WriteString(" <none>")
	} else {
		for _, path := range associatedZones {
			details.WriteString("\n- " + path)
		}
	}

	a.DetailsPanel.Clear()
	a.DetailsPanel.SetText(details.String())
	a.CurrentFocusKeys = []string{
		"<u> Update VLAN",
		"<D> Delete VLAN",
	}
}

func (a *App) renderSSID(s *domain.SSID) {
	details := new(strings.Builder)
	fmt.Fprintf(details, "SSID                 : %s\n", s.ID)
	if strings.TrimSpace(s.Description) == "" {
		details.WriteString("Description          : <none>\n")
	} else {
		fmt.Fprintf(details, "Description          : %s\n", s.Description)
	}

	a.DetailsPanel.Clear()
	a.DetailsPanel.SetText(details.String())
	a.CurrentFocusKeys = []string{
		"<u> Update WiFi SSID",
		"<D> Delete WiFi SSID",
	}
}

func (a *App) renderZone(z *domain.Zone) {
	details := new(strings.Builder)
	fmt.Fprintf(details, "Zone                 : %s\n", z.DisplayName)
	if strings.TrimSpace(z.Description) == "" {
		details.WriteString("Description          : <none>\n")
	} else {
		fmt.Fprintf(details, "Description          : %s\n", z.Description)
	}

	details.WriteString("Associated VLANs     :")
	if len(z.VLANIDs) == 0 {
		details.WriteString(" <none>\n")
	} else {
		for _, vlanID := range z.VLANIDs {
			details.WriteString("\n- " + a.Catalog.RenderVLANID(vlanID))
		}
		details.WriteString("\n")
	}

	a.DetailsPanel.Clear()
	a.DetailsPanel.SetText(details.String())
	a.CurrentFocusKeys = []string{
		"<u> Update Zone",
		"<D> Delete Zone",
	}
}

func (a *App) renderEquipment(e *domain.Equipment) {
	details := new(strings.Builder)
	fmt.Fprintf(details, "Name                 : %s\n", e.DisplayName)
	fmt.Fprintf(details, "Model                : %s\n", e.Model)
	if e.Description == "" {
		details.WriteString("Description          : <none>\n")
	} else {
		fmt.Fprintf(details, "Description          : %s\n", e.Description)
	}

	ports := 0
	for _, child := range a.Catalog.GetChildren(e) {
		if _, ok := child.(*domain.Port); ok {
			ports++
		}
	}
	fmt.Fprintf(details, "Ports                : %d\n", ports)

	a.DetailsPanel.Clear()
	a.DetailsPanel.SetText(details.String())
	a.CurrentFocusKeys = []string{
		"<u> Update Equipment",
		"<D> Delete Equipment",
	}
}

func (a *App) renderPort(p *domain.Port) {
	details := new(strings.Builder)
	fmt.Fprintf(details, "Port Number          : %s\n", p.ID)
	if p.Name == "" {
		details.WriteString("Name                 : <none>\n")
	} else {
		fmt.Fprintf(details, "Name                 : %s\n", p.Name)
	}
	fmt.Fprintf(details, "Type                 : %s\n", p.PortType)
	fmt.Fprintf(details, "Speed                : %s\n", p.Speed)
	if p.PoE == "" {
		details.WriteString("PoE                  : <none>\n")
	} else {
		fmt.Fprintf(details, "PoE                  : %s\n", p.PoE)
	}
	if p.LAGGroup > 0 {
		fmt.Fprintf(details, "LAG Group            : %d\n", p.LAGGroup)
	} else {
		details.WriteString("LAG Group            : <none>\n")
	}
	if p.LAGMode != "" {
		fmt.Fprintf(details, "LAG Mode             : %s\n", p.LAGMode)
	} else {
		details.WriteString("LAG Mode             : <none>\n")
	}
	fmt.Fprintf(details, "Native VLAN          : %s\n", a.Catalog.RenderVLANID(p.NativeVLANID))
	switch p.TaggedVLANMode {
	case domain.TaggedVLANModeAllowAll:
		details.WriteString("Tagged VLANs         : Allow All\n")
	case domain.TaggedVLANModeBlockAll:
		details.WriteString("Tagged VLANs         : Block All\n")
	case domain.TaggedVLANModeCustom:
		custom := make([]string, 0, len(p.TaggedVLANIDs))
		for _, vlanID := range p.TaggedVLANIDs {
			custom = append(custom, a.Catalog.RenderVLANID(vlanID))
		}
		fmt.Fprintf(details, "Tagged VLANs         : Custom (%s)\n", strings.Join(custom, ", "))
	default:
		details.WriteString("Tagged VLANs         : <none>\n")
	}
	if p.ConnectedTo != "" {
		fmt.Fprintf(details, "Connected To         : %s\n", domain.RenderPortLink(a.Catalog, p.ConnectedTo))
	} else {
		details.WriteString("Connected To         : <none>\n")
	}
	if p.Description != "" {
		fmt.Fprintf(details, "Description          : %s\n", p.Description)
	} else {
		details.WriteString("Description          : <none>\n")
	}

	a.DetailsPanel.Clear()
	a.DetailsPanel.SetText(details.String())
	a.CurrentFocusKeys = []string{
		"<u> Update Port",
		"<C> Copy Port",
		"<c> Connect Port",
		"<x> Disconnect Port",
		"<D> Delete Port",
	}
}

// hasAnySummarizableRange checks if any unallocated sibling networks can be summarized.
func (a *App) hasAnySummarizableRange(network *domain.Network) bool {
	candidates := a.getUnallocatedSiblingNetworks(network)
	if len(candidates) < 2 {
		return false
	}

	cidrs := make([]string, 0, len(candidates))
	for _, n := range candidates {
		cidrs = append(cidrs, n.ID)
	}
	for i := range cidrs {
		summarizeable, _, _ := domain.FindSummarizableRange(cidrs, i)
		if summarizeable {
			return true
		}
	}

	return false
}

// getUnallocatedSiblingNetworks returns all unallocated sibling networks.
func (a *App) getUnallocatedSiblingNetworks(network *domain.Network) []*domain.Network {
	parent := a.Catalog.Get(network.GetParentPath())
	unallocated := []*domain.Network{}
	for _, sibling := range a.Catalog.GetChildren(parent) {
		n, ok := sibling.(*domain.Network)
		if !ok {
			continue
		}
		if n.AllocationMode == domain.AllocationModeUnallocated {
			unallocated = append(unallocated, n)
		}
	}
	return unallocated
}
