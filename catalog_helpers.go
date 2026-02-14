package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func parsePositiveIntID(id string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(id))
	if err != nil {
		return 0, err
	}
	if value < 1 {
		return 0, fmt.Errorf("must be >= 1")
	}
	return value, nil
}

func findVLANByID(vlanID int) *VLAN {
	vlansRoot := menuItems.GetByParentAndID(nil, "VLANs")
	if vlansRoot == nil {
		return nil
	}

	target := strconv.Itoa(vlanID)
	for _, menuItem := range menuItems.GetChilds(vlansRoot) {
		vlan, ok := menuItem.(*VLAN)
		if !ok {
			continue
		}
		if vlan.ID == target {
			return vlan
		}
	}

	return nil
}

func renderVLANID(vlanID int) string {
	if vlanID <= 0 {
		return "Default"
	}
	vlan := findVLANByID(vlanID)
	if vlan == nil {
		return strconv.Itoa(vlanID)
	}
	return fmt.Sprintf("%d (%s)", vlanID, vlan.DisplayName)
}

func parseVLANListCSV(input string) ([]int, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, nil
	}

	parts := strings.Split(trimmed, ",")
	ids := make([]int, 0, len(parts))
	seen := map[int]struct{}{}
	for _, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("invalid VLAN ID %q", strings.TrimSpace(part))
		}
		if value < 1 || value > 4094 {
			return nil, fmt.Errorf("invalid VLAN ID %d: must be in range 1-4094", value)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		ids = append(ids, value)
	}
	sort.Ints(ids)
	return ids, nil
}
