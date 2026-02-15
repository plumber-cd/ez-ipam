package domain

import (
	"fmt"
	"math/big"
	"net"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// RenderDetailsMap returns an ordered list of keys and a map of detail values
// for a network. This is pure computation without UI side effects.
func (n *Network) RenderDetailsMap(c *Catalog) ([]string, map[string]string, error) {
	index := []string{}
	result := map[string]string{}

	p := message.NewPrinter(language.English)

	index = append(index, "CIDR")
	result["CIDR"] = n.ID

	_, ipnet, err := net.ParseCIDR(n.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("error parsing CIDR %s: %w", n.ID, err)
	}

	firstIP := ipnet.IP
	index = append(index, "Network Address")
	result["Network Address"] = firstIP.String()

	maskBits, _ := ipnet.Mask.Size()
	index = append(index, "Mask Bits")
	result["Mask Bits"] = p.Sprintf("%d", maskBits)

	subnetMask := make(net.IP, len(ipnet.Mask))
	copy(subnetMask, ipnet.Mask)
	subnetMaskStr := subnetMask.String()
	index = append(index, "Subnet Mask")
	result["Subnet Mask"] = subnetMaskStr

	lastIP := make(net.IP, len(firstIP))
	copy(lastIP, firstIP)
	for i := range lastIP {
		lastIP[i] = firstIP[i] | ^ipnet.Mask[i]
	}
	if ipnet.IP.To4() != nil {
		index = append(index, "Broadcast Address")
		result["Broadcast Address"] = lastIP.String()
	}
	index = append(index, "Range")
	result["Range"] = p.Sprintf("%s - %s", firstIP, lastIP)

	var totalHosts big.Int
	totalHosts.SetUint64(1)
	var usableHosts big.Int
	if ipnet.IP.To4() == nil { // IPv6
		totalHosts.Lsh(&totalHosts, uint(128-maskBits))
		usableHosts.Set(&totalHosts)
		usableHosts.Sub(&usableHosts, big.NewInt(1))

		if maskBits <= 64 {
			totalNetworks := 1 << (64 - maskBits)
			index = append(index, "Total /64 Networks")
			result["Total /64 Networks"] = p.Sprintf("%d", totalNetworks)
		} else {
			index = append(index, "Total Hosts")
			result["Total Hosts"] = p.Sprintf("%d", totalHosts.Uint64())
		}
	} else { // IPv4
		totalHosts.Lsh(&totalHosts, uint(32-maskBits))
		usableHosts.Set(&totalHosts)
		usableHosts.Sub(&usableHosts, big.NewInt(2))
		index = append(index, "Total Hosts")
		result["Total Hosts"] = p.Sprintf("%d", totalHosts.Uint64())

		usableFirstIP := make(net.IP, len(firstIP))
		copy(usableFirstIP, firstIP)
		usableFirstIP[len(usableFirstIP)-1]++
		usableLastIP := make(net.IP, len(lastIP))
		copy(usableLastIP, lastIP)
		if ipnet.IP.To4() != nil {
			usableLastIP[len(usableLastIP)-1]--
		}
		index = append(index, "Usable Range")
		result["Usable Range"] = p.Sprintf("%s - %s", usableFirstIP, usableLastIP)
		index = append(index, "Usable Hosts")
		result["Usable Hosts"] = p.Sprintf("%d", usableHosts.Uint64())
	}

	index = append(index, "Allocation Mode")
	switch n.AllocationMode {
	case AllocationModeUnallocated:
		result["Allocation Mode"] = "Unallocated"
		index = append(index, "Mode Meaning")
		result["Mode Meaning"] = "Can be split or assigned as Subnet Container / Host Pool."
	case AllocationModeSubnets:
		result["Allocation Mode"] = "Subnet Container"
		index = append(index, "Mode Meaning")
		result["Mode Meaning"] = "Branch node: contains child networks."
	case AllocationModeHosts:
		result["Allocation Mode"] = "Host Pool"
		index = append(index, "Mode Meaning")
		result["Mode Meaning"] = "Leaf node: reserve concrete IP addresses here."
	default:
		panic("Unknown AllocationMode")
	}
	if n.AllocationMode != AllocationModeUnallocated {
		result["Description"] = n.Description
	}
	if n.VLANID > 0 {
		vlanName := "<unknown>"
		if c != nil {
			vlan := c.FindVLANByID(n.VLANID)
			if vlan != nil {
				vlanName = vlan.DisplayName
			}
		}
		index = append(index, "VLAN")
		result["VLAN"] = fmt.Sprintf("%d (%s)", n.VLANID, vlanName)
	}

	return index, result, nil
}

// RenderDetails returns a human-readable string of network details.
func (n *Network) RenderDetails(c *Catalog) string {
	stringWriter := new(strings.Builder)
	template := "%-20s: %s\n"

	index, data, err := n.RenderDetailsMap(c)
	if err != nil {
		return fmt.Sprintf("Error rendering details: %v", err)
	}

	for _, key := range index {
		if key == "Description" {
			continue
		}
		fmt.Fprintf(stringWriter, template, key, data[key])
	}

	if n.AllocationMode != AllocationModeUnallocated {
		stringWriter.WriteString("\n\n\n")
		stringWriter.WriteString(n.Description)
	}

	return stringWriter.String()
}

// RenderPortLink returns a display-friendly connection string for a port.
func RenderPortLink(c *Catalog, connectedTo string) string {
	if connectedTo == "" {
		return ""
	}
	target := c.Get(connectedTo)
	if target == nil {
		return connectedTo
	}
	port, ok := target.(*Port)
	if !ok {
		return connectedTo
	}
	parent := c.Get(port.GetParentPath())
	if parent == nil {
		return connectedTo
	}
	equipment, ok := parent.(*Equipment)
	if !ok {
		return connectedTo
	}
	number := port.Number()
	if strings.TrimSpace(port.Name) != "" {
		defaultName := fmt.Sprintf("Port %d", number)
		if port.Name != defaultName {
			return fmt.Sprintf("%s Port %d (%s)", equipment.DisplayName, number, port.Name)
		}
	}
	return fmt.Sprintf("%s Port %d", equipment.DisplayName, number)
}
