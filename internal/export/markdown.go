package export

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/plumber-cd/ez-ipam/internal/domain"
)

//go:embed markdown.tmpl
var markdownTmpl string

// RenderMarkdown generates the full markdown report from a catalog.
func RenderMarkdown(catalog *domain.Catalog) (string, error) {
	networksOrder := []string{}
	networksTitle := map[string]string{}
	networksMode := map[string]string{}
	networksDescription := map[string]string{}
	networksHeading := map[string]string{}
	networksAnchor := map[string]string{}
	networksDataIndex := map[string][]string{}
	networksData := map[string]map[string]string{}
	networksHasReservedIPs := map[string]bool{}
	networksReservedIPs := map[string][]map[string]string{}
	summaryRows := []map[string]string{}
	dnsRows := []map[string]string{}

	buildTreePrefix := func(ancestorHasNext []bool, isLast bool) string {
		sb := new(strings.Builder)
		for _, hasNext := range ancestorHasNext {
			if hasNext {
				sb.WriteString("│ ")
			} else {
				sb.WriteString("  ")
			}
		}
		if isLast {
			sb.WriteString("└── ")
		} else {
			sb.WriteString("├── ")
		}
		return sb.String()
	}

	vlanLabel := func(vlanID int) string {
		if vlanID <= 0 {
			return "-"
		}
		value := strconv.Itoa(vlanID)
		vlan := catalog.FindVLANByID(vlanID)
		if vlan != nil {
			value = fmt.Sprintf("%s (%s)", vlan.ID, vlan.DisplayName)
		}
		return value
	}

	var walkErr error
	var walkNetwork func(n *domain.Network, depth int, ancestorHasNext []bool, isLast bool)
	walkNetwork = func(n *domain.Network, depth int, ancestorHasNext []bool, isLast bool) {
		if walkErr != nil {
			return
		}
		index, data, err := n.RenderDetailsMap(catalog)
		if err != nil {
			walkErr = fmt.Errorf("render details for %s: %w", n.GetPath(), err)
			return
		}

		path := n.GetPath()
		cidrIdentifier, err := domain.CIDRToIdentifier(n.ID)
		if err != nil {
			walkErr = fmt.Errorf("build anchor for %s: %w", n.ID, err)
			return
		}
		networksOrder = append(networksOrder, path)
		networksDescription[path] = markdownBlockquote(n.Description)
		networksAnchor[path] = "network-" + cidrIdentifier
		networksDataIndex[path] = index
		networksData[path] = data
		headingLevel := 3 + depth
		if headingLevel > 6 {
			headingLevel = 6
		}
		networksHeading[path] = strings.Repeat("#", headingLevel)
		networkTreePrefix := ""
		if depth > 0 {
			networkTreePrefix = buildTreePrefix(ancestorHasNext, isLast)
		}

		switch n.AllocationMode {
		case domain.AllocationModeUnallocated:
			networksMode[path] = "Unallocated"
			networksTitle[path] = fmt.Sprintf("`%s` _(Unallocated)_", n.ID)
		case domain.AllocationModeSubnets:
			networksMode[path] = "Subnet Container"
			if n.DisplayName != "" {
				networksTitle[path] = fmt.Sprintf("`%s` -- %s", n.ID, n.DisplayName)
			} else {
				networksTitle[path] = fmt.Sprintf("`%s`", n.ID)
			}
		case domain.AllocationModeHosts:
			networksMode[path] = "Host Pool"
			if n.DisplayName != "" {
				networksTitle[path] = fmt.Sprintf("`%s` -- %s", n.ID, n.DisplayName)
			} else {
				networksTitle[path] = fmt.Sprintf("`%s`", n.ID)
			}
		}

		children := catalog.GetChildren(n)

		reserved := []map[string]string{}
		networkChildren := make([]*domain.Network, 0, len(children))
		for _, child := range children {
			if ip, ok := child.(*domain.IP); ok {
				reserved = append(reserved, map[string]string{
					"Address":     ip.ID,
					"DisplayName": ip.DisplayName,
					"MACAddress":  ip.MACAddress,
					"Description": ip.Description,
				})
			}
			if nn, ok := child.(*domain.Network); ok {
				networkChildren = append(networkChildren, nn)
			}
		}
		networksHasReservedIPs[path] = len(reserved) > 0
		networksReservedIPs[path] = reserved

		networkCell := fmt.Sprintf("%s [link](#%s)", markdownCode(networkTreePrefix+n.ID), networksAnchor[path])
		summaryRows = append(summaryRows, map[string]string{
			"Network":     networkCell,
			"Name":        markdownInline(defaultIfEmpty(n.DisplayName, "-")),
			"Allocation":  markdownInline(defaultIfEmpty(networksMode[path], "-")),
			"VLAN":        markdownInline(vlanLabel(n.VLANID)),
			"Description": markdownInline(clampOverviewDescription(defaultIfEmpty(n.Description, "-"), 60)),
		})

		if len(reserved) > 0 {
			ipAncestorHasNext := append(append([]bool{}, ancestorHasNext...), !isLast)
			for i, ip := range reserved {
				ipTreePrefix := buildTreePrefix(ipAncestorHasNext, i == len(reserved)-1)
				summaryRows = append(summaryRows, map[string]string{
					"Network":     markdownCode(ipTreePrefix + ip["Address"]),
					"Name":        markdownInline(defaultIfEmpty(ip["DisplayName"], "-")),
					"Allocation":  "Reserved IP",
					"VLAN":        markdownInline(vlanLabel(n.VLANID)),
					"Description": markdownInline(clampOverviewDescription(defaultIfEmpty(ip["Description"], "-"), 60)),
				})
			}
		}
		for i, childNetwork := range networkChildren {
			childAncestors := append(append([]bool{}, ancestorHasNext...), !isLast)
			walkNetwork(childNetwork, depth+1, childAncestors, i == len(networkChildren)-1)
		}
	}

	networksMenuItem := catalog.GetByParentAndDisplayID(nil, domain.FolderNetworks)
	topLevelNetworks := make([]*domain.Network, 0)
	for _, item := range catalog.GetChildren(networksMenuItem) {
		n, ok := item.(*domain.Network)
		if !ok {
			return "", fmt.Errorf("expected network under Networks folder, got %T at %s", item, item.GetPath())
		}
		topLevelNetworks = append(topLevelNetworks, n)
	}
	for i, topLevelNetwork := range topLevelNetworks {
		walkNetwork(topLevelNetwork, 0, []bool{}, i == len(topLevelNetworks)-1)
	}
	if walkErr != nil {
		return "", walkErr
	}

	vlanRows := []map[string]string{}
	vlansMenuItem := catalog.GetByParentAndDisplayID(nil, domain.FolderVLANs)
	for _, item := range catalog.GetChildren(vlansMenuItem) {
		vlan, ok := item.(*domain.VLAN)
		if !ok {
			continue
		}
		vlanRows = append(vlanRows, map[string]string{
			"ID":          markdownCode(vlan.ID),
			"Name":        markdownCode(vlan.DisplayName),
			"Description": markdownTableCell(defaultIfEmpty(vlan.Description, "-")),
		})
	}

	ssidRows := []map[string]string{}
	dnsMenuItem := catalog.GetByParentAndDisplayID(nil, domain.FolderDNS)
	for _, item := range catalog.GetChildren(dnsMenuItem) {
		record, ok := item.(*domain.DNSRecord)
		if !ok {
			continue
		}
		recordType := record.RecordType
		recordValue := record.RecordValue
		valueCell := "-"
		if strings.TrimSpace(record.ReservedIPPath) != "" {
			recordType = "Alias"
			recordValue = "<missing>"
			if ip, ok := catalog.Get(record.ReservedIPPath).(*domain.IP); ok {
				recordValue = formatDNSAliasValue(ip)
			}
			valueCell = markdownTableCell(defaultIfEmpty(recordValue, "-"))
		} else {
			if strings.TrimSpace(recordValue) != "" {
				// Keep normal DNS record values copy-friendly in the report.
				valueCell = markdownCode(recordValue)
			}
		}
		dnsRows = append(dnsRows, map[string]string{
			"FQDN":        markdownCode(record.ID),
			"Type":        markdownInline(defaultIfEmpty(recordType, "-")),
			"Value":       valueCell,
			"Description": markdownTableCell(defaultIfEmpty(record.Description, "-")),
		})
	}

	ssidsMenuItem := catalog.GetByParentAndDisplayID(nil, domain.FolderSSIDs)
	for _, item := range catalog.GetChildren(ssidsMenuItem) {
		ssid, ok := item.(*domain.SSID)
		if !ok {
			continue
		}
		ssidRows = append(ssidRows, map[string]string{
			"ID":          markdownCode(ssid.ID),
			"Description": markdownTableCell(defaultIfEmpty(ssid.Description, "-")),
		})
	}

	zoneRows := []map[string]string{}
	zonesMenuItem := catalog.GetByParentAndDisplayID(nil, domain.FolderZones)
	for _, item := range catalog.GetChildren(zonesMenuItem) {
		zone, ok := item.(*domain.Zone)
		if !ok {
			continue
		}
		vlanLabels := make([]string, 0, len(zone.VLANIDs))
		for _, vlanID := range zone.VLANIDs {
			vlanLabels = append(vlanLabels, catalog.RenderVLANID(vlanID))
		}
		zoneRows = append(zoneRows, map[string]string{
			"Name":        markdownCode(zone.DisplayName),
			"VLANs":       markdownTableCell(defaultIfEmpty(strings.Join(vlanLabels, "\n"), "-")),
			"Description": markdownTableCell(defaultIfEmpty(zone.Description, "-")),
		})
	}

	equipmentRows := []map[string]string{}
	equipmentPorts := map[string][]map[string]string{}
	equipmentMenuItem := catalog.GetByParentAndDisplayID(nil, domain.FolderEquipment)
	for _, item := range catalog.GetChildren(equipmentMenuItem) {
		equipment, ok := item.(*domain.Equipment)
		if !ok {
			continue
		}
		rows := []map[string]string{}
		for _, child := range catalog.GetChildren(equipment) {
			port, ok := child.(*domain.Port)
			if !ok {
				continue
			}
			portName := "-"
			if strings.TrimSpace(port.Name) != "" {
				portName = port.Name
			}
			if port.Disabled {
				portName = "[disabled]"
			}
			var portTypeParts []string
			for _, part := range []string{port.PortType, port.Speed, port.PoE} {
				if strings.TrimSpace(part) != "" {
					portTypeParts = append(portTypeParts, part)
				}
			}
			portType := strings.Join(portTypeParts, "\n")
			networks := "-"
			destination := "-"
			if !port.Disabled {
				nativeVLANID, taggedMode, taggedVLANIDs := catalog.GetEffectivePortVLANSettings(port)
				tagged := "-"
				switch taggedMode {
				case domain.TaggedVLANModeAllowAll:
					tagged = "Allow All"
				case domain.TaggedVLANModeBlockAll:
					tagged = "Block All"
				case domain.TaggedVLANModeCustom:
					values := make([]string, 0, len(taggedVLANIDs))
					for _, vlanID := range taggedVLANIDs {
						values = append(values, catalog.RenderVLANID(vlanID))
					}
					tagged = strings.Join(values, "\n")
				}
				destinationParts := []string{}
				if port.ConnectedTo != "" {
					destinationParts = append(destinationParts, domain.RenderPortLink(catalog, port.ConnectedTo))
				}
				if port.DestinationNotes != "" {
					destinationParts = append(destinationParts, port.DestinationNotes)
				}
				if len(destinationParts) > 0 {
					destination = strings.Join(destinationParts, "\n")
				}
				networkLines := []string{
					fmt.Sprintf("Native: %s", catalog.RenderVLANID(nativeVLANID)),
					fmt.Sprintf("Tagged: %s", tagged),
				}
				networks = strings.Join(networkLines, "\n")
			}
			rows = append(rows, map[string]string{
				"Number":      markdownCode(port.ID),
				"Name":        markdownTableCell(portName),
				"Type":        markdownTableCell(portType),
				"Networks":    markdownTableCell(networks),
				"Destination": markdownTableCell(destination),
			})
		}
		equipmentPorts[equipment.GetPath()] = rows
		equipmentRows = append(equipmentRows, map[string]string{
			"DisplayName":   markdownInline(equipment.DisplayName),
			"Model":         markdownInline(equipment.Model),
			"Description":   markdownBlockquote(strings.TrimSpace(equipment.Description)),
			"EquipmentPath": equipment.GetPath(),
		})
	}

	tmpl := template.Must(template.New("markdown").Parse(markdownTmpl))
	input := map[string]interface{}{
		"NetworksOrder":          networksOrder,
		"NetworksTitle":          networksTitle,
		"NetworksDescription":    networksDescription,
		"NetworksHeading":        networksHeading,
		"NetworksAnchor":         networksAnchor,
		"NetworksDataIndex":      networksDataIndex,
		"NetworksData":           networksData,
		"NetworksHasReservedIPs": networksHasReservedIPs,
		"NetworksReservedIPs":    networksReservedIPs,
		"VLANRows":               vlanRows,
		"DNSRows":                dnsRows,
		"SSIDRows":               ssidRows,
		"ZoneRows":               zoneRows,
		"EquipmentRows":          equipmentRows,
		"EquipmentPorts":         equipmentPorts,
		"SummaryRows":            summaryRows,
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, input); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}
	return sb.String(), nil
}

func markdownInline(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return value
}

func markdownCode(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "`", "'")
	return "`" + value + "`"
}

func markdownBlockquote(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.ReplaceAll(value, "\n", "\n>\n> ")
	return value
}

func markdownTableCell(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", "<br>")
	return value
}

func clampOverviewDescription(value string, maxLen int) string {
	if maxLen < 1 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxLen {
		return value
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

func defaultIfEmpty(value, fallback string) string { //nolint:unparam // fallback is always "-" but kept for clarity
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func formatDNSAliasValue(ip *domain.IP) string {
	if ip == nil {
		return "<missing>"
	}
	if strings.TrimSpace(ip.MACAddress) != "" {
		return fmt.Sprintf("`%s` (%s `%s`)", ip.ID, ip.DisplayName, ip.MACAddress)
	}
	return fmt.Sprintf("`%s` (%s)", ip.ID, ip.DisplayName)
}
