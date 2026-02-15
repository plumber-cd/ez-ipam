package ui

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"github.com/plumber-cd/ez-ipam/internal/domain"
)

// ---------- Network operations ----------

func (a *App) AddNewNetwork(cidr string) {
	newNet := &domain.Network{
		Base: domain.Base{
			ID:         cidr,
			ParentPath: a.CurrentItem.GetPath(),
		},
	}

	if err := a.validateNewNetwork(newNet); err != nil {
		a.setStatus("Error adding new network: " + err.Error())
		return
	}

	if err := a.Catalog.Add(newNet); err != nil {
		a.setStatus("Error adding network: " + err.Error())
		return
	}
	a.ReloadMenu(newNet)

	a.setStatus("Added new network: " + cidr)
}

func (a *App) SplitNetwork(newPrefix int) {
	focusedNetwork, ok := a.CurrentFocus.(*domain.Network)
	if !ok {
		a.setStatus("Error: SplitNetwork requires a network to be focused")
		return
	}

	if focusedNetwork.AllocationMode != domain.AllocationModeUnallocated {
		a.setStatus("Error: cannot split an allocated network")
		return
	}

	newNetworks, err := domain.SplitNetwork(focusedNetwork.ID, newPrefix)
	if err != nil {
		a.setStatus("Error splitting network: " + err.Error())
		return
	}

	parent := a.Catalog.Get(focusedNetwork.GetParentPath())
	if parent == nil {
		a.setStatus("Error splitting network: parent not found")
		return
	}

	newMenuItems := []*domain.Network{}
	for _, newNet := range newNetworks {
		newMenuItem := &domain.Network{
			Base: domain.Base{
				ID:         newNet,
				ParentPath: parent.GetPath(),
			},
		}
		if err := a.validateNewNetworkIgnoring(newMenuItem, []*domain.Network{focusedNetwork}); err != nil {
			a.setStatus("Error adding newly split network: " + err.Error())
			return
		}
		newMenuItems = append(newMenuItems, newMenuItem)
	}

	a.Catalog.Delete(focusedNetwork)

	for _, newMenuItem := range newMenuItems {
		if err := a.Catalog.Add(newMenuItem); err != nil {
			a.setStatus("Error creating subnet: " + err.Error())
			return
		}
	}

	a.ReloadMenu(focusedNetwork)

	a.setStatus("Split network " + focusedNetwork.GetPath() + " into " + fmt.Sprintf("%d subnets", len(newNetworks)))
}

func (a *App) SummarizeNetworkSelection(candidates []*domain.Network, fromIndex, toIndex int) {
	if len(candidates) < 2 {
		a.setStatus("Error summarizing network: at least two unallocated sibling networks are required")
		return
	}

	if fromIndex > toIndex {
		fromIndex, toIndex = toIndex, fromIndex
	}
	if fromIndex < 0 || toIndex >= len(candidates) {
		a.setStatus("Error summarizing network: invalid selection range")
		return
	}
	if toIndex-fromIndex < 1 {
		a.setStatus("Error summarizing network: select at least two networks")
		return
	}

	selected := candidates[fromIndex : toIndex+1]
	parentPath := selected[0].GetParentPath()
	selectedCIDRs := make([]string, 0, len(selected))
	for _, n := range selected {
		if n == nil {
			a.setStatus("Error summarizing network: invalid selection")
			return
		}
		if n.GetParentPath() != parentPath {
			a.setStatus("Error summarizing network: selected networks must share the same parent")
			return
		}
		if n.AllocationMode != domain.AllocationModeUnallocated {
			a.setStatus("Error summarizing network: only unallocated networks can be summarized")
			return
		}
		selectedCIDRs = append(selectedCIDRs, n.ID)
	}

	newNetwork, err := domain.SummarizeCIDRs(selectedCIDRs)
	if err != nil {
		a.setStatus("Error summarizing network: " + err.Error())
		return
	}

	newMenuItem := &domain.Network{
		Base: domain.Base{
			ID:         newNetwork,
			ParentPath: parentPath,
		},
	}
	if err := a.validateNewNetworkIgnoring(newMenuItem, selected); err != nil {
		a.setStatus("Error validating summarized network: " + err.Error())
		return
	}

	for _, old := range selected {
		a.Catalog.Delete(old)
	}
	if err := a.Catalog.Add(newMenuItem); err != nil {
		a.setStatus("Error adding summarized network: " + err.Error())
		return
	}
	a.ReloadMenu(newMenuItem)

	a.setStatus("Summarized networks into " + newMenuItem.DisplayID())
}

func (a *App) AllocateNetworkInSubnetsMode(displayName, description string, subnetsPrefix int, vlanID int) {
	focusedNetwork, ok := a.CurrentFocus.(*domain.Network)
	if !ok {
		a.setStatus("Error: AllocateNetworkInSubnetsMode requires a network to be focused")
		return
	}
	if focusedNetwork.AllocationMode != domain.AllocationModeUnallocated {
		a.setStatus("Error: cannot allocate an already allocated network")
		return
	}

	// Pre-validate the modification.
	testCopy := *focusedNetwork
	testCopy.AllocationMode = domain.AllocationModeSubnets
	testCopy.DisplayName = displayName
	testCopy.Description = description
	testCopy.VLANID = vlanID
	if err := a.validateNetworkUpdate(&testCopy); err != nil {
		a.setStatus("Error allocating network: " + err.Error())
		return
	}

	newNetworks, err := domain.SplitNetwork(focusedNetwork.ID, subnetsPrefix)
	if err != nil {
		a.setStatus("Error splitting network: " + err.Error())
		return
	}

	// Apply the modification.
	focusedNetwork.AllocationMode = domain.AllocationModeSubnets
	focusedNetwork.DisplayName = displayName
	focusedNetwork.Description = description
	focusedNetwork.VLANID = vlanID

	for _, newNet := range newNetworks {
		newMenuItem := &domain.Network{
			Base: domain.Base{
				ID:         newNet,
				ParentPath: focusedNetwork.GetPath(),
			},
		}
		if err := a.Catalog.Add(newMenuItem); err != nil {
			a.setStatus("Error adding subnet: " + err.Error())
			return
		}
	}

	a.ReloadMenu(focusedNetwork)
	a.setStatus("Allocated network: " + focusedNetwork.GetPath())
}

func (a *App) AllocateNetworkInHostsMode(displayName, description string, vlanID int) {
	focusedNetwork, ok := a.CurrentFocus.(*domain.Network)
	if !ok {
		a.setStatus("Error: AllocateNetworkInHostsMode requires a network")
		return
	}
	if focusedNetwork.AllocationMode != domain.AllocationModeUnallocated {
		a.setStatus("Error: cannot allocate an already allocated network")
		return
	}

	testCopy := *focusedNetwork
	testCopy.AllocationMode = domain.AllocationModeHosts
	testCopy.DisplayName = displayName
	testCopy.Description = description
	testCopy.VLANID = vlanID
	if err := a.validateNetworkUpdate(&testCopy); err != nil {
		a.setStatus("Error allocating network: " + err.Error())
		return
	}

	focusedNetwork.AllocationMode = domain.AllocationModeHosts
	focusedNetwork.DisplayName = displayName
	focusedNetwork.Description = description
	focusedNetwork.VLANID = vlanID

	a.ReloadMenu(focusedNetwork)
	a.setStatus("Allocated network: " + focusedNetwork.GetPath())
}

func (a *App) UpdateNetworkAllocation(displayName, description string, vlanID int) {
	focusedNetwork, ok := a.CurrentFocus.(*domain.Network)
	if !ok {
		a.setStatus("Error: UpdateNetworkAllocation requires a network")
		return
	}
	if focusedNetwork.AllocationMode == domain.AllocationModeUnallocated {
		a.setStatus("Error: cannot update an unallocated network")
		return
	}

	testCopy := *focusedNetwork
	testCopy.DisplayName = displayName
	testCopy.Description = description
	testCopy.VLANID = vlanID
	if err := a.validateNetworkUpdate(&testCopy); err != nil {
		a.setStatus("Error updating network allocation: " + err.Error())
		return
	}

	focusedNetwork.DisplayName = displayName
	focusedNetwork.Description = description
	focusedNetwork.VLANID = vlanID

	a.ReloadMenu(focusedNetwork)
	a.setStatus("Allocated network updated: " + focusedNetwork.GetPath())
}

func (a *App) DeallocateNetwork() {
	focusedNetwork, ok := a.CurrentFocus.(*domain.Network)
	if !ok {
		a.setStatus("Error: DeallocateNetwork requires a network")
		return
	}
	if focusedNetwork.AllocationMode == domain.AllocationModeUnallocated {
		a.setStatus("Error: cannot deallocate an unallocated network")
		return
	}

	focusedNetwork.AllocationMode = domain.AllocationModeUnallocated
	focusedNetwork.DisplayName = ""
	focusedNetwork.Description = ""
	focusedNetwork.VLANID = 0

	children := a.Catalog.GetChildren(focusedNetwork)
	for _, child := range children {
		a.Catalog.Delete(child)
	}

	a.ReloadMenu(focusedNetwork)
	a.setStatus("Deallocated network: " + focusedNetwork.GetPath())
}

func (a *App) DeleteNetwork() {
	focusedNetwork, ok := a.CurrentFocus.(*domain.Network)
	if !ok {
		a.setStatus("Error: DeleteNetwork requires a network")
		return
	}

	parent := a.Catalog.Get(focusedNetwork.GetParentPath())
	if _, ok := parent.(*domain.Network); ok {
		a.setStatus("Error: cannot delete a child network; deallocate the parent instead")
		return
	}

	a.Catalog.Delete(focusedNetwork)
	a.ReloadMenu(focusedNetwork)

	a.setStatus("Deleted network: " + focusedNetwork.GetPath())
}

// ---------- IP operations ----------

func (a *App) ReserveIP(address, displayName, description string) {
	parent, ok := a.CurrentItem.(*domain.Network)
	if !ok {
		a.setStatus("Error: ReserveIP requires a network as current menu item")
		return
	}

	reserved := &domain.IP{
		Base: domain.Base{
			ID:         strings.TrimSpace(address),
			ParentPath: parent.GetPath(),
		},
		DisplayName: strings.TrimSpace(displayName),
		Description: strings.TrimSpace(description),
	}

	if err := a.validateNewIP(reserved, parent); err != nil {
		a.setStatus("Error reserving IP: " + err.Error())
		return
	}

	if err := a.Catalog.Add(reserved); err != nil {
		a.setStatus("Error reserving IP: " + err.Error())
		return
	}
	a.ReloadMenu(reserved)

	a.setStatus("Reserved IP: " + reserved.GetPath())
}

func (a *App) UpdateIPReservation(displayName, description string) {
	focusedIP, ok := a.CurrentFocus.(*domain.IP)
	if !ok {
		a.setStatus("Error: UpdateIPReservation requires an IP to be focused")
		return
	}

	oldDisplayName := focusedIP.DisplayName
	oldDescription := focusedIP.Description
	focusedIP.DisplayName = strings.TrimSpace(displayName)
	focusedIP.Description = strings.TrimSpace(description)
	if err := focusedIP.Validate(a.Catalog); err != nil {
		focusedIP.DisplayName = oldDisplayName
		focusedIP.Description = oldDescription
		a.setStatus("Error updating IP reservation: " + err.Error())
		return
	}

	a.ReloadMenu(focusedIP)
	a.setStatus("Updated IP reservation: " + focusedIP.GetPath())
}

func (a *App) UnreserveIP() {
	focusedIP, ok := a.CurrentFocus.(*domain.IP)
	if !ok {
		a.setStatus("Error: UnreserveIP requires an IP to be focused")
		return
	}

	a.Catalog.Delete(focusedIP)
	a.ReloadMenu(focusedIP)

	a.setStatus("Unreserved IP: " + focusedIP.GetPath())
}

// ---------- VLAN operations ----------

func (a *App) AddVLAN(id, displayName, description, selectedZone string) {
	parent := a.CurrentItem
	sf, ok := parent.(*domain.StaticFolder)
	if !ok || sf.ID != domain.FolderVLANs {
		a.setStatus("Error: AddVLAN requires VLANs folder as current menu item")
		return
	}

	vlan := &domain.VLAN{
		Base: domain.Base{
			ID:         strings.TrimSpace(id),
			ParentPath: parent.GetPath(),
		},
		DisplayName: strings.TrimSpace(displayName),
		Description: strings.TrimSpace(description),
	}

	if err := vlan.Validate(a.Catalog); err != nil {
		a.setStatus("Error adding VLAN: " + err.Error())
		return
	}

	if a.Catalog.GetByParentAndDisplayID(parent, vlan.DisplayID()) != nil {
		a.setStatus("Error adding VLAN: duplicate VLAN")
		return
	}

	for _, sibling := range a.Catalog.GetChildren(parent) {
		other, ok := sibling.(*domain.VLAN)
		if !ok {
			continue
		}
		if other.ID == vlan.ID {
			a.setStatus("Error adding VLAN: VLAN ID already exists")
			return
		}
	}

	if err := a.Catalog.Add(vlan); err != nil {
		a.setStatus("Error adding VLAN: " + err.Error())
		return
	}

	// Update zone memberships.
	a.updateVLANZoneMemberships(vlan, selectedZone)

	a.ReloadMenu(vlan)

	a.setStatus("Added VLAN: " + vlan.GetPath())
}

func (a *App) UpdateVLAN(displayName, description, selectedZone string) {
	focusedVLAN, ok := a.CurrentFocus.(*domain.VLAN)
	if !ok {
		a.setStatus("Error: UpdateVLAN requires a VLAN to be focused")
		return
	}

	oldDisplayName := focusedVLAN.DisplayName
	oldDescription := focusedVLAN.Description
	focusedVLAN.DisplayName = strings.TrimSpace(displayName)
	focusedVLAN.Description = strings.TrimSpace(description)
	if err := focusedVLAN.Validate(a.Catalog); err != nil {
		focusedVLAN.DisplayName = oldDisplayName
		focusedVLAN.Description = oldDescription
		a.setStatus("Error updating VLAN: " + err.Error())
		return
	}

	// Update zone memberships.
	a.updateVLANZoneMemberships(focusedVLAN, selectedZone)

	a.ReloadMenu(focusedVLAN)
	a.setStatus("Updated VLAN: " + focusedVLAN.GetPath())
}

// updateVLANZoneMemberships ensures the VLAN belongs to at most one selected zone.
func (a *App) updateVLANZoneMemberships(vlan *domain.VLAN, selectedZone string) {
	vlanID, err := strconv.Atoi(vlan.ID)
	if err != nil {
		return
	}
	selectedZone = strings.TrimSpace(selectedZone)

	for _, item := range a.Catalog.All() {
		zone, ok := item.(*domain.Zone)
		if !ok {
			continue
		}
		filtered := make([]int, 0, len(zone.VLANIDs))
		for _, id := range zone.VLANIDs {
			if id != vlanID {
				filtered = append(filtered, id)
			}
		}
		zone.VLANIDs = filtered
		if selectedZone != "" && zone.DisplayName == selectedZone {
			zone.VLANIDs = append(zone.VLANIDs, vlanID)
			zone.Normalize()
		}
	}
}

func (a *App) DeleteVLAN() {
	focusedVLAN, ok := a.CurrentFocus.(*domain.VLAN)
	if !ok {
		a.setStatus("Error: DeleteVLAN requires a VLAN to be focused")
		return
	}

	vlanID, err := strconv.Atoi(focusedVLAN.ID)
	if err != nil {
		a.setStatus("Error: invalid VLAN ID: " + err.Error())
		return
	}

	for _, item := range a.Catalog.All() {
		network, ok := item.(*domain.Network)
		if ok {
			if network.VLANID == vlanID {
				network.VLANID = 0
			}
			continue
		}
		port, ok := item.(*domain.Port)
		if ok {
			if port.NativeVLANID == vlanID {
				port.NativeVLANID = 0
			}
			filtered := make([]int, 0, len(port.TaggedVLANIDs))
			for _, id := range port.TaggedVLANIDs {
				if id != vlanID {
					filtered = append(filtered, id)
				}
			}
			port.TaggedVLANIDs = filtered
			continue
		}
		zone, ok := item.(*domain.Zone)
		if ok {
			filtered := make([]int, 0, len(zone.VLANIDs))
			for _, id := range zone.VLANIDs {
				if id != vlanID {
					filtered = append(filtered, id)
				}
			}
			zone.VLANIDs = filtered
		}
	}

	a.Catalog.Delete(focusedVLAN)
	a.ReloadMenu(focusedVLAN)

	a.setStatus("Deleted VLAN: " + focusedVLAN.GetPath())
}

// ---------- SSID operations ----------

func (a *App) AddSSID(id, description string) {
	parent := a.CurrentItem
	sf, ok := parent.(*domain.StaticFolder)
	if !ok || sf.ID != domain.FolderSSIDs {
		a.setStatus("Error: AddSSID requires WiFi SSIDs folder as current menu item")
		return
	}

	ssid := &domain.SSID{
		Base: domain.Base{
			ID:         strings.TrimSpace(id),
			ParentPath: parent.GetPath(),
		},
		Description: strings.TrimSpace(description),
	}

	if err := ssid.Validate(a.Catalog); err != nil {
		a.setStatus("Error adding WiFi SSID: " + err.Error())
		return
	}

	if a.Catalog.GetByParentAndDisplayID(parent, ssid.DisplayID()) != nil {
		a.setStatus("Error adding WiFi SSID: duplicate WiFi SSID")
		return
	}

	for _, sibling := range a.Catalog.GetChildren(parent) {
		other, ok := sibling.(*domain.SSID)
		if !ok {
			continue
		}
		if other.ID == ssid.ID {
			a.setStatus("Error adding WiFi SSID: SSID already exists")
			return
		}
	}

	if err := a.Catalog.Add(ssid); err != nil {
		a.setStatus("Error adding WiFi SSID: " + err.Error())
		return
	}
	a.ReloadMenu(ssid)

	a.setStatus("Added WiFi SSID: " + ssid.GetPath())
}

func (a *App) UpdateSSID(description string) {
	focusedSSID, ok := a.CurrentFocus.(*domain.SSID)
	if !ok {
		a.setStatus("Error: UpdateSSID requires an SSID to be focused")
		return
	}

	oldDescription := focusedSSID.Description
	focusedSSID.Description = strings.TrimSpace(description)
	if err := focusedSSID.Validate(a.Catalog); err != nil {
		focusedSSID.Description = oldDescription
		a.setStatus("Error updating WiFi SSID: " + err.Error())
		return
	}

	a.ReloadMenu(focusedSSID)
	a.setStatus("Updated WiFi SSID: " + focusedSSID.GetPath())
}

func (a *App) DeleteSSID() {
	focusedSSID, ok := a.CurrentFocus.(*domain.SSID)
	if !ok {
		a.setStatus("Error: DeleteSSID requires an SSID to be focused")
		return
	}

	a.Catalog.Delete(focusedSSID)
	a.ReloadMenu(focusedSSID)

	a.setStatus("Deleted WiFi SSID: " + focusedSSID.GetPath())
}

// ---------- Zone operations ----------

func (a *App) AddZone(displayName, description, vlanIDsText string) {
	parent := a.CurrentItem
	sf, ok := parent.(*domain.StaticFolder)
	if !ok || sf.ID != domain.FolderZones {
		a.setStatus("Error: AddZone requires Zones folder as current menu item")
		return
	}

	vlanIDs, err := domain.ParseVLANListCSV(vlanIDsText)
	if err != nil {
		a.setStatus("Error adding Zone: " + err.Error())
		return
	}

	name := strings.TrimSpace(displayName)
	zone := &domain.Zone{
		Base: domain.Base{
			ID:         name,
			ParentPath: parent.GetPath(),
		},
		DisplayName: name,
		Description: strings.TrimSpace(description),
		VLANIDs:     vlanIDs,
	}
	zone.Normalize()
	if err := zone.Validate(a.Catalog); err != nil {
		a.setStatus("Error adding Zone: " + err.Error())
		return
	}

	for _, sibling := range a.Catalog.GetChildren(parent) {
		other, ok := sibling.(*domain.Zone)
		if !ok {
			continue
		}
		if strings.EqualFold(other.DisplayName, zone.DisplayName) {
			a.setStatus("Error adding Zone: zone already exists")
			return
		}
	}

	if err := a.Catalog.Add(zone); err != nil {
		a.setStatus("Error adding Zone: " + err.Error())
		return
	}
	a.ReloadMenu(zone)
	a.setStatus("Added Zone: " + zone.GetPath())
}

func (a *App) UpdateZone(displayName, description, vlanIDsText string) {
	focusedZone, ok := a.CurrentFocus.(*domain.Zone)
	if !ok {
		a.setStatus("Error: UpdateZone requires a zone to be focused")
		return
	}

	vlanIDs, err := domain.ParseVLANListCSV(vlanIDsText)
	if err != nil {
		a.setStatus("Error updating Zone: " + err.Error())
		return
	}

	name := strings.TrimSpace(displayName)
	updated := &domain.Zone{
		Base: domain.Base{
			ID:         name,
			ParentPath: focusedZone.ParentPath,
		},
		DisplayName: name,
		Description: strings.TrimSpace(description),
		VLANIDs:     vlanIDs,
	}
	updated.Normalize()
	if err := updated.Validate(a.Catalog); err != nil {
		a.setStatus("Error updating Zone: " + err.Error())
		return
	}

	for _, sibling := range a.Catalog.GetChildren(a.Catalog.Get(focusedZone.GetParentPath())) {
		other, ok := sibling.(*domain.Zone)
		if !ok || other.GetPath() == focusedZone.GetPath() {
			continue
		}
		if strings.EqualFold(other.DisplayName, updated.DisplayName) {
			a.setStatus("Error updating Zone: zone already exists")
			return
		}
	}

	// Use Put (bypasses validation, which we already did above) so the
	// operation is atomic: if the old path differs from the new one we
	// remove the old entry only after the new one is safely stored.
	a.Catalog.Put(updated)
	if updated.GetPath() != focusedZone.GetPath() {
		a.Catalog.Remove(focusedZone.GetPath())
	}
	a.CurrentFocus = updated
	a.ReloadMenu(updated)
	a.setStatus("Updated Zone: " + updated.GetPath())
}

func (a *App) DeleteZone() {
	focusedZone, ok := a.CurrentFocus.(*domain.Zone)
	if !ok {
		a.setStatus("Error: DeleteZone requires a zone to be focused")
		return
	}

	a.Catalog.Delete(focusedZone)
	a.ReloadMenu(focusedZone)
	a.setStatus("Deleted Zone: " + focusedZone.GetPath())
}

// ---------- Equipment operations ----------

func (a *App) AddEquipment(displayName, model, description string) {
	parent := a.CurrentItem
	sf, ok := parent.(*domain.StaticFolder)
	if !ok || sf.ID != domain.FolderEquipment {
		a.setStatus("Error: AddEquipment requires Equipment folder as current menu item")
		return
	}

	equipment := &domain.Equipment{
		Base: domain.Base{
			ID:         strings.TrimSpace(displayName),
			ParentPath: parent.GetPath(),
		},
		DisplayName: strings.TrimSpace(displayName),
		Model:       strings.TrimSpace(model),
		Description: strings.TrimSpace(description),
	}
	if err := equipment.Validate(a.Catalog); err != nil {
		a.setStatus("Error adding Equipment: " + err.Error())
		return
	}

	for _, sibling := range a.Catalog.GetChildren(parent) {
		other, ok := sibling.(*domain.Equipment)
		if !ok {
			continue
		}
		if strings.EqualFold(other.DisplayName, equipment.DisplayName) {
			a.setStatus("Error adding Equipment: duplicate equipment")
			return
		}
	}

	if err := a.Catalog.Add(equipment); err != nil {
		a.setStatus("Error adding Equipment: " + err.Error())
		return
	}
	a.ReloadMenu(equipment)
	a.setStatus("Added Equipment: " + equipment.GetPath())
}

func (a *App) UpdateEquipment(displayName, model, description string) {
	focusedEquipment, ok := a.CurrentFocus.(*domain.Equipment)
	if !ok {
		a.setStatus("Error: UpdateEquipment requires equipment to be focused")
		return
	}

	newName := strings.TrimSpace(displayName)
	newModel := strings.TrimSpace(model)
	newDescription := strings.TrimSpace(description)
	newPath := focusedEquipment.ParentPath + " -> " + newName
	oldPath := focusedEquipment.GetPath()

	if strings.EqualFold(newName, focusedEquipment.DisplayName) {
		newName = focusedEquipment.DisplayName
		newPath = oldPath
	}

	candidate := &domain.Equipment{
		Base: domain.Base{
			ID:         newName,
			ParentPath: focusedEquipment.ParentPath,
		},
		DisplayName: newName,
		Model:       newModel,
		Description: newDescription,
	}
	if err := candidate.Validate(a.Catalog); err != nil {
		a.setStatus("Error updating Equipment: " + err.Error())
		return
	}

	for _, sibling := range a.Catalog.GetChildren(a.Catalog.Get(focusedEquipment.GetParentPath())) {
		other, ok := sibling.(*domain.Equipment)
		if !ok || other.GetPath() == focusedEquipment.GetPath() {
			continue
		}
		if strings.EqualFold(other.DisplayName, candidate.DisplayName) {
			a.setStatus("Error updating Equipment: duplicate equipment")
			return
		}
	}

	children := a.Catalog.GetChildren(focusedEquipment)
	ports := []*domain.Port{}
	for _, child := range children {
		port, ok := child.(*domain.Port)
		if !ok {
			continue
		}
		ports = append(ports, port)
	}

	// Store the new equipment first, then clean up the old entry so the
	// operation is atomic -- we never lose data if something fails.
	a.Catalog.Put(candidate)
	if oldPath != newPath {
		a.Catalog.Remove(oldPath)
	}

	for _, port := range ports {
		oldPortPath := port.GetPath()
		port.ParentPath = newPath
		a.Catalog.Put(port)
		if oldPortPath != port.GetPath() {
			a.Catalog.Remove(oldPortPath)
		}
	}

	oldPrefix := oldPath + " -> "
	newPrefix := newPath + " -> "
	for _, item := range a.Catalog.All() {
		port, ok := item.(*domain.Port)
		if !ok || port.ConnectedTo == "" {
			continue
		}
		if strings.HasPrefix(port.ConnectedTo, oldPrefix) {
			port.ConnectedTo = newPrefix + strings.TrimPrefix(port.ConnectedTo, oldPrefix)
		}
	}

	a.CurrentFocus = candidate
	a.ReloadMenu(candidate)
	a.setStatus("Updated Equipment: " + candidate.GetPath())
}

func (a *App) DeleteEquipment() {
	focusedEquipment, ok := a.CurrentFocus.(*domain.Equipment)
	if !ok {
		a.setStatus("Error: DeleteEquipment requires equipment to be focused")
		return
	}

	children := a.Catalog.GetChildren(focusedEquipment)
	portPaths := map[string]struct{}{}
	for _, child := range children {
		port, ok := child.(*domain.Port)
		if !ok {
			continue
		}
		portPaths[port.GetPath()] = struct{}{}
	}
	for _, item := range a.Catalog.All() {
		port, ok := item.(*domain.Port)
		if !ok || port.ConnectedTo == "" {
			continue
		}
		if _, removing := portPaths[port.ConnectedTo]; removing {
			port.ConnectedTo = ""
		}
	}

	a.Catalog.Delete(focusedEquipment)
	a.ReloadMenu(focusedEquipment)
	a.setStatus("Deleted Equipment: " + focusedEquipment.GetPath())
}

// ---------- Port operations ----------

// nextAvailablePortNumber returns the next port number (max + 1) for the given equipment.
func (a *App) nextAvailablePortNumber(equipment *domain.Equipment) string {
	maxNum := 0
	for _, child := range a.Catalog.GetChildren(equipment) {
		port, ok := child.(*domain.Port)
		if !ok {
			continue
		}
		if n := port.Number(); n > maxNum {
			maxNum = n
		}
	}
	return strconv.Itoa(maxNum + 1)
}

func (a *App) AddPort(portNumber string, enabled bool, name, portType, speed, poe, lagGroup, lagMode, nativeVLAN, taggedMode, taggedVLANs, destinationNotes string) {
	parent, ok := a.CurrentItem.(*domain.Equipment)
	if !ok {
		a.setStatus("Error: AddPort requires equipment as current menu item")
		return
	}

	lagMode = strings.TrimSpace(lagMode)
	if lagMode == LagModeDisabledOption {
		lagMode = ""
	}
	taggedMode = strings.TrimSpace(taggedMode)
	if taggedMode == TaggedModeNoneOption {
		taggedMode = ""
	}

	number, err := domain.ParsePositiveIntID(portNumber)
	if err != nil {
		a.setStatus("Error adding Port: invalid port number")
		return
	}
	lagGroup = strings.TrimSpace(lagGroup)
	if strings.EqualFold(lagGroup, "self") {
		lagGroup = strconv.Itoa(number)
	}
	lagGroupValue, err := domain.ParseOptionalIntField(lagGroup)
	if err != nil || lagGroupValue < 0 {
		a.setStatus("Error adding Port: invalid LAG group")
		return
	}
	nativeVLANID, err := domain.ParseOptionalIntField(nativeVLAN)
	if err != nil || nativeVLANID < 0 {
		a.setStatus("Error adding Port: invalid native VLAN ID")
		return
	}
	mode := domain.ParseTaggedMode(taggedMode)
	tagged, err := domain.ParseVLANListCSV(taggedVLANs)
	if err != nil {
		a.setStatus("Error adding Port: " + err.Error())
		return
	}
	if lagGroupValue > 0 && lagGroupValue != number {
		// LAG member ports inherit VLAN display from master and do not store VLAN state.
		nativeVLANID = 0
		mode = domain.TaggedVLANModeNone
		tagged = nil
	}
	if !enabled {
		name = ""
		lagGroupValue = 0
		lagMode = ""
		nativeVLANID = 0
		mode = domain.TaggedVLANModeNone
		tagged = nil
	}

	port := &domain.Port{
		Base: domain.Base{
			ID:         strconv.Itoa(number),
			ParentPath: parent.GetPath(),
		},
		Disabled:         !enabled,
		Name:             strings.TrimSpace(name),
		PortType:         strings.TrimSpace(portType),
		Speed:            strings.TrimSpace(speed),
		PoE:              strings.TrimSpace(poe),
		LAGGroup:         lagGroupValue,
		LAGMode:          strings.TrimSpace(lagMode),
		NativeVLANID:     nativeVLANID,
		TaggedVLANMode:   mode,
		TaggedVLANIDs:    tagged,
		DestinationNotes: strings.TrimSpace(destinationNotes),
	}

	if err := a.validateNewPort(port, parent); err != nil {
		a.setStatus("Error adding Port: " + err.Error())
		return
	}

	if err := a.Catalog.Add(port); err != nil {
		a.setStatus("Error adding Port: " + err.Error())
		return
	}
	a.ReloadMenu(port)
	a.setStatus("Added Port: " + port.GetPath())
}

func (a *App) UpdatePort(portNumber string, enabled bool, name, portType, speed, poe, lagGroup, lagMode, nativeVLAN, taggedMode, taggedVLANs, destinationNotes string) {
	focusedPort, ok := a.CurrentFocus.(*domain.Port)
	if !ok {
		a.setStatus("Error: UpdatePort requires a port to be focused")
		return
	}

	lagMode = strings.TrimSpace(lagMode)
	if lagMode == LagModeDisabledOption {
		lagMode = ""
	}
	taggedMode = strings.TrimSpace(taggedMode)
	if taggedMode == TaggedModeNoneOption {
		taggedMode = ""
	}

	number, err := domain.ParsePositiveIntID(portNumber)
	if err != nil {
		a.setStatus("Error updating Port: invalid port number")
		return
	}
	if strconv.Itoa(number) != focusedPort.ID {
		a.setStatus("Error updating Port: changing port number is not supported")
		return
	}

	lagGroup = strings.TrimSpace(lagGroup)
	if strings.EqualFold(lagGroup, "self") {
		lagGroup = strconv.Itoa(number)
	}
	lagGroupValue, err := domain.ParseOptionalIntField(lagGroup)
	if err != nil || lagGroupValue < 0 {
		a.setStatus("Error updating Port: invalid LAG group")
		return
	}
	nativeVLANID, err := domain.ParseOptionalIntField(nativeVLAN)
	if err != nil || nativeVLANID < 0 {
		a.setStatus("Error updating Port: invalid native VLAN ID")
		return
	}
	mode := domain.ParseTaggedMode(taggedMode)
	tagged, err := domain.ParseVLANListCSV(taggedVLANs)
	if err != nil {
		a.setStatus("Error updating Port: " + err.Error())
		return
	}
	if lagGroupValue > 0 && lagGroupValue != number {
		// LAG member ports inherit VLAN display from master and do not store VLAN state.
		nativeVLANID = 0
		mode = domain.TaggedVLANModeNone
		tagged = nil
	}
	if !enabled {
		name = ""
		lagGroupValue = 0
		lagMode = ""
		nativeVLANID = 0
		mode = domain.TaggedVLANModeNone
		tagged = nil
	}

	backup := *focusedPort
	focusedPort.Disabled = !enabled
	focusedPort.Name = strings.TrimSpace(name)
	focusedPort.PortType = strings.TrimSpace(portType)
	focusedPort.Speed = strings.TrimSpace(speed)
	focusedPort.PoE = strings.TrimSpace(poe)
	focusedPort.LAGGroup = lagGroupValue
	focusedPort.LAGMode = strings.TrimSpace(lagMode)
	focusedPort.NativeVLANID = nativeVLANID
	focusedPort.TaggedVLANMode = mode
	focusedPort.TaggedVLANIDs = tagged
	focusedPort.DestinationNotes = strings.TrimSpace(destinationNotes)
	if err := a.validateExistingPort(focusedPort); err != nil {
		*focusedPort = backup
		a.setStatus("Error updating Port: " + err.Error())
		return
	}

	a.ReloadMenu(focusedPort)
	a.setStatus("Updated Port: " + focusedPort.GetPath())
}

func (a *App) ConnectPort(targetPath string) {
	focusedPort, ok := a.CurrentFocus.(*domain.Port)
	if !ok {
		a.setStatus("Error: ConnectPort requires a port to be focused")
		return
	}
	targetItem := a.Catalog.Get(targetPath)
	if targetItem == nil {
		a.setStatus("Error connecting Port: target not found")
		return
	}
	targetPort, ok := targetItem.(*domain.Port)
	if !ok {
		a.setStatus("Error connecting Port: target is not a port")
		return
	}
	if targetPort.GetParentPath() == focusedPort.GetParentPath() {
		a.setStatus("Error connecting Port: target must be on different equipment")
		return
	}
	if focusedPort.ConnectedTo != "" || targetPort.ConnectedTo != "" {
		a.setStatus("Error connecting Port: one of ports is already connected")
		return
	}

	oldFocused := focusedPort.ConnectedTo
	oldTarget := targetPort.ConnectedTo
	focusedPort.ConnectedTo = targetPort.GetPath()
	targetPort.ConnectedTo = focusedPort.GetPath()
	if err := focusedPort.Validate(a.Catalog); err != nil {
		focusedPort.ConnectedTo = oldFocused
		targetPort.ConnectedTo = oldTarget
		a.setStatus("Error connecting Port: " + err.Error())
		return
	}
	if err := targetPort.Validate(a.Catalog); err != nil {
		focusedPort.ConnectedTo = oldFocused
		targetPort.ConnectedTo = oldTarget
		a.setStatus("Error connecting Port: " + err.Error())
		return
	}

	a.ReloadMenu(focusedPort)
	a.setStatus("Connected Port: " + focusedPort.GetPath())
}

func (a *App) DisconnectPort() {
	focusedPort, ok := a.CurrentFocus.(*domain.Port)
	if !ok {
		a.setStatus("Error: DisconnectPort requires a port to be focused")
		return
	}
	if focusedPort.ConnectedTo == "" {
		a.setStatus("Port is not connected")
		return
	}
	targetItem := a.Catalog.Get(focusedPort.ConnectedTo)
	focusedPort.ConnectedTo = ""
	if targetPort, ok := targetItem.(*domain.Port); ok {
		if targetPort.ConnectedTo == focusedPort.GetPath() {
			targetPort.ConnectedTo = ""
		}
	}

	a.ReloadMenu(focusedPort)
	a.setStatus("Disconnected Port: " + focusedPort.GetPath())
}

func (a *App) DeletePort() {
	focusedPort, ok := a.CurrentFocus.(*domain.Port)
	if !ok {
		a.setStatus("Error: DeletePort requires a port to be focused")
		return
	}

	if focusedPort.ConnectedTo != "" {
		targetItem := a.Catalog.Get(focusedPort.ConnectedTo)
		if targetPort, ok := targetItem.(*domain.Port); ok {
			if targetPort.ConnectedTo == focusedPort.GetPath() {
				targetPort.ConnectedTo = ""
			}
		}
	}
	if focusedPort.LAGGroup > 0 && focusedPort.LAGGroup == focusedPort.Number() {
		parentItem := a.Catalog.Get(focusedPort.GetParentPath())
		parent, ok := parentItem.(*domain.Equipment)
		if !ok {
			a.setStatus("Error deleting Port: parent equipment not found")
			return
		}
		for _, sibling := range a.Catalog.GetChildren(parent) {
			member, ok := sibling.(*domain.Port)
			if !ok || member.GetPath() == focusedPort.GetPath() {
				continue
			}
			if member.LAGGroup == focusedPort.Number() {
				a.setStatus(fmt.Sprintf("Error deleting Port: cannot delete LAG master while member port %s is attached", member.ID))
				return
			}
		}
	}

	a.Catalog.Delete(focusedPort)
	a.ReloadMenu(focusedPort)
	a.setStatus("Deleted Port: " + focusedPort.GetPath())
}

// ---------- Validation helpers ----------

func (a *App) validateNewNetwork(n *domain.Network) error {
	// Extra checks for user-supplied CIDRs (not needed for computed summarizations).
	_, ipNet, err := net.ParseCIDR(n.ID)
	if err == nil {
		maskBits, _ := ipNet.Mask.Size()
		if ipNet.IP.To4() != nil && maskBits == 32 {
			return fmt.Errorf("provided CIDR is a single IPv4 address, not a network: %s", n.ID)
		} else if ipNet.IP.To4() == nil && maskBits == 128 {
			return fmt.Errorf("provided CIDR is a single IPv6 address, not a network: %s", n.ID)
		}
	}

	parent := a.Catalog.Get(n.GetParentPath())
	if parent != nil {
		if sf, ok := parent.(*domain.StaticFolder); ok {
			networksFolder := a.Catalog.GetByParentAndDisplayID(nil, domain.FolderNetworks)
			if networksFolder == nil {
				return fmt.Errorf("networks folder not found in catalog")
			}
			if sf.GetPath() != networksFolder.GetPath() {
				return fmt.Errorf("parent must be Networks for Network=%s", n.GetPath())
			}
		}
		if p, ok := parent.(*domain.Network); ok {
			if ipNet != nil && ipNet.String() == p.ID {
				return fmt.Errorf("network=%s cannot be the same as parent network=%s", n.GetPath(), p.GetPath())
			}
		}
	}

	return a.validateNewNetworkIgnoring(n, nil)
}

func (a *App) validateNewNetworkIgnoring(n *domain.Network, ignore []*domain.Network) error {
	if err := n.Validate(a.Catalog); err != nil {
		return err
	}

	ip, ipNet, err := net.ParseCIDR(n.ID)
	if err != nil {
		return fmt.Errorf("invalid ID %s (must be a valid network CIDR): %w", n.ID, err)
	}
	if !ip.Equal(ipNet.IP) {
		return fmt.Errorf("provided CIDR specifies a host, not a network: %s (should be %s)", n.ID, ipNet.String())
	}

	parent := a.Catalog.Get(n.GetParentPath())
	if parent == nil {
		return fmt.Errorf("parent not found for Network=%s", n.GetPath())
	}

	switch p := parent.(type) {
	case *domain.StaticFolder:
		// OK
	case *domain.Network:
		if p.AllocationMode != domain.AllocationModeSubnets {
			return fmt.Errorf("parent network must be allocated in subnets mode for Network=%s", n.GetPath())
		}
		_, parentIPNet, err := net.ParseCIDR(p.ID)
		if err != nil {
			return fmt.Errorf("error parsing parent CIDR %s: %w", p.ID, err)
		}
		if !parentIPNet.Contains(ipNet.IP) {
			return fmt.Errorf("network=%s is not within parent network=%s", n.GetPath(), p.GetPath())
		}
	default:
		return fmt.Errorf("parent must be Networks folder or another allocated network for Network=%s", n.GetPath())
	}

	return a.checkNetworkOverlap(n, ignore)
}

func (a *App) validateNetworkUpdate(updated *domain.Network) error {
	if err := updated.Validate(a.Catalog); err != nil {
		return err
	}
	if updated.VLANID > 0 && a.Catalog.FindVLANByID(updated.VLANID) == nil {
		return fmt.Errorf("VLAN ID %d not found for Network=%s", updated.VLANID, updated.GetPath())
	}
	return nil
}

func (a *App) checkNetworkOverlap(n *domain.Network, ignore []*domain.Network) error {
	parent := a.Catalog.Get(n.GetParentPath())
	if parent == nil {
		return nil
	}

	_, ipNet, err := net.ParseCIDR(n.ID)
	if err != nil {
		return err
	}

	otherNetworks := a.Catalog.GetChildren(parent)
	for _, other := range otherNetworks {
		otherNetwork, ok := other.(*domain.Network)
		if !ok {
			continue
		}

		mustSkip := false
		for _, ig := range ignore {
			if ig.GetPath() == other.GetPath() {
				mustSkip = true
			}
		}
		if mustSkip {
			continue
		}

		_, otherIPNet, err := net.ParseCIDR(otherNetwork.ID)
		if err != nil {
			return fmt.Errorf("error parsing other CIDR %s: %w", other.DisplayID(), err)
		}

		if otherIPNet.String() == ipNet.String() {
			return fmt.Errorf("network=%s cannot be the same as network=%s", n.GetPath(), other.GetPath())
		}
		if otherIPNet.Contains(ipNet.IP) || ipNet.Contains(otherIPNet.IP) {
			return fmt.Errorf("network=%s overlaps with network=%s", n.GetPath(), other.GetPath())
		}
	}

	return nil
}

func (a *App) validateNewIP(ip *domain.IP, parent *domain.Network) error {
	if err := ip.Validate(a.Catalog); err != nil {
		return err
	}

	addr, err := netip.ParseAddr(ip.ID)
	if err != nil {
		return fmt.Errorf("invalid IP ID %q: %w", ip.ID, err)
	}

	if parent.AllocationMode != domain.AllocationModeHosts {
		return fmt.Errorf("parent network must be allocated in hosts mode for IP=%s", ip.GetPath())
	}

	prefix, err := netip.ParsePrefix(parent.ID)
	if err != nil {
		return fmt.Errorf("failed to parse parent network CIDR %q: %w", parent.ID, err)
	}
	if prefix.Addr().Is4() != addr.Is4() {
		return fmt.Errorf("IP family mismatch between IP=%s and parent=%s", ip.GetPath(), parent.GetPath())
	}
	if !prefix.Contains(addr) {
		return fmt.Errorf("IP=%s is not within parent network=%s", ip.GetPath(), parent.GetPath())
	}

	for _, sibling := range a.Catalog.GetChildren(parent) {
		other, ok := sibling.(*domain.IP)
		if !ok {
			continue
		}
		if other.ID == ip.ID {
			return fmt.Errorf("IP=%s already reserved in %s", ip.ID, parent.GetPath())
		}
	}

	return nil
}

func (a *App) validateNewPort(port *domain.Port, parent *domain.Equipment) error {
	return a.validatePort(port, parent, false)
}

func (a *App) validateExistingPort(port *domain.Port) error {
	parent := a.Catalog.Get(port.GetParentPath())
	if parent == nil {
		return fmt.Errorf("parent not found for Port=%s", port.GetPath())
	}
	equipment, ok := parent.(*domain.Equipment)
	if !ok {
		return fmt.Errorf("parent must be equipment for Port=%s", port.GetPath())
	}
	return a.validatePort(port, equipment, true)
}

// validatePort performs all port validation. When checkConnectedTo is true,
// it additionally validates the ConnectedTo back-link (used for existing ports).
func (a *App) validatePort(port *domain.Port, parent *domain.Equipment, checkConnectedTo bool) error {
	if err := port.Validate(a.Catalog); err != nil {
		return err
	}

	for _, sibling := range a.Catalog.GetChildren(parent) {
		other, ok := sibling.(*domain.Port)
		if !ok || other.GetPath() == port.GetPath() {
			continue
		}
		if other.ID == port.ID {
			return fmt.Errorf("port number %s already exists in %s", port.ID, parent.GetPath())
		}
		if port.Name != "" && strings.EqualFold(other.Name, port.Name) {
			return fmt.Errorf("port name %q already exists in %s", port.Name, parent.GetPath())
		}
	}

	if port.NativeVLANID > 0 && a.Catalog.FindVLANByID(port.NativeVLANID) == nil {
		return fmt.Errorf("native VLAN ID %d not found for Port=%s", port.NativeVLANID, port.GetPath())
	}

	if port.TaggedVLANMode == domain.TaggedVLANModeCustom {
		for _, vlanID := range port.TaggedVLANIDs {
			if a.Catalog.FindVLANByID(vlanID) == nil {
				return fmt.Errorf("tagged VLAN ID %d not found for Port=%s", vlanID, port.GetPath())
			}
		}
	}

	if checkConnectedTo && port.ConnectedTo != "" {
		if port.ConnectedTo == port.GetPath() {
			return fmt.Errorf("port cannot connect to itself for Port=%s", port.GetPath())
		}
		target := a.Catalog.Get(port.ConnectedTo)
		if target == nil {
			return fmt.Errorf("connected port not found: %s", port.ConnectedTo)
		}
		targetPort, ok := target.(*domain.Port)
		if !ok {
			return fmt.Errorf("connected item is not a port: %s", port.ConnectedTo)
		}
		if targetPort.ConnectedTo != port.GetPath() {
			return fmt.Errorf("connected port %s must point back to %s", targetPort.GetPath(), port.GetPath())
		}
	}

	if port.LAGGroup > 0 {
		if port.LAGGroup != port.Number() {
			if port.NativeVLANID != 0 || port.TaggedVLANMode != domain.TaggedVLANModeNone || len(port.TaggedVLANIDs) > 0 {
				return fmt.Errorf("LAG member ports cannot store VLAN settings for Port=%s", port.GetPath())
			}
			masterPath := parent.GetPath() + " -> " + strconv.Itoa(port.LAGGroup)
			masterItem := a.Catalog.Get(masterPath)
			masterPort, ok := masterItem.(*domain.Port)
			if !ok {
				return fmt.Errorf("LAG master port %d not found for Port=%s", port.LAGGroup, port.GetPath())
			}
			if masterPort.LAGGroup != masterPort.Number() {
				return fmt.Errorf("LAG master port %s must reference itself as LAG group", masterPort.ID)
			}
			if strings.TrimSpace(masterPort.LAGMode) == "" {
				return fmt.Errorf("LAG master port %s must have LAG mode enabled", masterPort.ID)
			}
			if masterPort.LAGMode != port.LAGMode {
				return fmt.Errorf("LAG member and master must share LAG mode for Port=%s", port.GetPath())
			}
		}
	}
	if port.LAGGroup != port.Number() || strings.TrimSpace(port.LAGMode) == "" {
		for _, sibling := range a.Catalog.GetChildren(parent) {
			member, ok := sibling.(*domain.Port)
			if !ok || member.GetPath() == port.GetPath() {
				continue
			}
			if member.LAGGroup == port.Number() {
				return fmt.Errorf("cannot disable LAG: port %d is still referenced as LAG master by other ports", port.Number())
			}
		}
	}

	return nil
}
