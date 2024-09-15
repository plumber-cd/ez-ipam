package main

import (
	"cmp"
	"fmt"
	"math/big"
	"net"
	"net/netip"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func AddNewNetwork(cidr string) {
	newNet := &Network{
		MenuFolder: &MenuFolder{
			ID:         cidr,
			ParentPath: currentMenuItem.GetPath(),
		},
	}

	if err := newNet.Validate(); err != nil {
		statusLine.Clear()
		statusLine.SetText("Error adding new network: " + err.Error())
		return
	}

	if err := menuItems.Add(newNet); err != nil {
		statusLine.Clear()
		statusLine.SetText("Added new network: " + cidr)

		return
	}

	reloadMenu(newNet)

	statusLine.Clear()
	statusLine.SetText("Added new network: " + cidr)
}

type Network struct {
	*MenuFolder
	Allocated   bool   `json:"allocated"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

func (n *Network) Validate() error {
	if err := n.MenuFolder.Validate(); err != nil {
		return err
	}

	_, ipNet, err := net.ParseCIDR(n.ID)
	if err != nil {
		return fmt.Errorf("Invalid ID %s (must be a valid network CIDR): %s", n.ID, err)
	}

	parent := n.GetParent()
	if parent == nil {
		return fmt.Errorf("Parent not found for Network=%s", n.GetPath())
	}

	switch p := parent.(type) {
	case *MenuStatic:
		networksFolder := menuItems.GetByParentAndID(nil, "Networks")
		if networksFolder == nil {
			panic("Networks folder not found")
		}
		if p != networksFolder {
			return fmt.Errorf("Parent must be Networks for Network=%s", n.GetPath())
		}
	case *Network:
		if !p.Allocated {
			return fmt.Errorf("Parent must be allocated network for Network=%s", n.GetPath())
		}

		_, parentIPNet, err := net.ParseCIDR(p.ID)
		if err != nil {
			return fmt.Errorf("Error parsing parent CIDR %s: %s", p.ID, err)
		}

		if parentIPNet.String() == ipNet.String() {
			return fmt.Errorf("Network=%s cannot be the same as parent Network=%s", n.GetPath(), p.GetPath())
		}

		if !parentIPNet.Contains(ipNet.IP) {
			return fmt.Errorf("Network=%s is not within parent Network=%s", n.GetPath(), p.GetPath())
		}
	default:
		return fmt.Errorf("Parent must be Networks folder or another allocated network for Network=%s", n.GetPath())
	}

	// Check that other networks do not overlap
	otherNetworks := menuItems.GetChilds(parent)
	for _, other := range otherNetworks {
		otherNetwork, ok := other.(*Network)
		if !ok {
			panic("Non-network child found in Networks")
		}

		_, otherIPNet, err := net.ParseCIDR(otherNetwork.ID)
		if err != nil {
			return fmt.Errorf("Error parsing other CIDR %s: %s", other.GetID(), err)
		}

		if otherIPNet.String() == ipNet.String() {
			return fmt.Errorf("Network=%s cannot be the same as Network=%s", n.GetPath(), other.GetPath())
		}

		if otherIPNet.Contains(ipNet.IP) || ipNet.Contains(otherIPNet.IP) {
			return fmt.Errorf("Network=%s overlaps with Network=%s", n.GetPath(), other.GetPath())
		}
	}

	if n.Allocated {
		if n.DisplayName == "" {
			return fmt.Errorf("DisplayName must be set for allocated Network=%s", n.GetPath())
		}
		if n.Description == "" {
			return fmt.Errorf("Description must be set for allocated Network=%s", n.GetPath())
		}
	}

	return nil
}

func (n *Network) GetID() string {
	if n.Allocated {
		if n.DisplayName != "" {
			return fmt.Sprintf("%s (%s)", n.ID, n.DisplayName)
		}
		return n.ID
	} else {
		return fmt.Sprintf("%s (*)", n.ID)
	}
}

func (n *Network) Compare(other MenuItem) int {
	if other == nil {
		return 1
	}

	otherMenu, ok := other.(*Network)

	if !ok {
		return cmp.Compare(n.GetID(), other.GetID())
	}

	_, ipNetLeft, err := net.ParseCIDR(n.ID)
	if err != nil {
		panic(err)
	}

	_, ipNetRight, err := net.ParseCIDR(otherMenu.ID)
	if err != nil {
		panic(err)
	}

	ipLeft := ipNetLeft.IP.Mask(ipNetLeft.Mask)
	ipRight := ipNetRight.IP.Mask(ipNetRight.Mask)

	for i := 0; i < 4; i++ {
		if ipLeft[i] < ipRight[i] {
			return -1
		} else if ipLeft[i] > ipRight[i] {
			return 1
		}
	}

	maskSizeLeft, _ := ipNetLeft.Mask.Size()
	maskSizeRight, _ := ipNetRight.Mask.Size()

	return maskSizeRight - maskSizeLeft
}

func (n *Network) OnChangedFunc() {
	detailsPanel.Clear()
	detailsPanel.SetText(n.RenderDetails())
	if n.Allocated {
		currentFocusKeys = []string{
			"<d> Description",
		}
	} else {
		currentFocusKeys = []string{
			"<a> Allocate",
			"<s> Split",
		}
	}
}

func (n *Network) OnSelectedFunc() {
	positionLine.Clear()
	positionLine.SetText(n.GetPath())
	currentMenuItemKeys = []string{}
}

func (n *Network) OnDoneFunc() {
	positionLine.Clear()
	positionLine.SetText(n.GetPath())
	currentMenuItemKeys = []string{}
}

func (n *Network) RenderDetails() string {
	stringWriter := new(strings.Builder)
	p := message.NewPrinter(language.English) // sorry, rest of the world
	template := "%-20s: %s\n"
	stringWriter.WriteString(p.Sprintf(template, "CIDR", n.ID))

	_, ipnet, err := net.ParseCIDR(n.ID)
	if err != nil {
		return fmt.Sprintf("Error parsing CIDR %s: %s", n.ID, err)
	}

	firstIP := ipnet.IP
	stringWriter.WriteString(p.Sprintf(template, "Network Address", firstIP))

	maskBits, _ := ipnet.Mask.Size()
	stringWriter.WriteString(p.Sprintf(template, "Mask Bits", p.Sprintf("%d", maskBits)))

	subnetMask := make(net.IP, len(ipnet.Mask))
	copy(subnetMask, ipnet.Mask)
	subnetMaskStr := subnetMask.String()
	stringWriter.WriteString(p.Sprintf(template, "Subnet Mask", subnetMaskStr))

	lastIP := make(net.IP, len(firstIP))
	copy(lastIP, firstIP)
	for i := range lastIP {
		lastIP[i] = firstIP[i] | ^ipnet.Mask[i]
	}
	if ipnet.IP.To4() != nil {
		stringWriter.WriteString(p.Sprintf(template, "Broadcast Address", lastIP))
	}
	stringWriter.WriteString(p.Sprintf(template, "Range", p.Sprintf("%s - %s", firstIP, lastIP)))

	var totalHosts big.Int
	totalHosts.SetUint64(1)
	var usableHosts big.Int
	if ipnet.IP.To4() == nil { // IPv6
		totalHosts.Lsh(&totalHosts, uint(128-maskBits))
		usableHosts.Set(&totalHosts)
		usableHosts.Sub(&usableHosts, big.NewInt(1))

		if maskBits <= 64 {
			totalNetworks := 1 << (64 - maskBits)
			stringWriter.WriteString(p.Sprintf(template, "Total /64 Networks", p.Sprintf("%d", totalNetworks)))
		} else {
			stringWriter.WriteString(p.Sprintf(template, "Total Hosts", p.Sprintf("%d", totalHosts.Uint64())))
		}
	} else { // IPv4
		totalHosts.Lsh(&totalHosts, uint(32-maskBits))
		usableHosts.Set(&totalHosts)
		usableHosts.Sub(&usableHosts, big.NewInt(2))
		stringWriter.WriteString(p.Sprintf(template, "Total Hosts", p.Sprintf("%d", totalHosts.Uint64())))

		usableFirstIP := make(net.IP, len(firstIP))
		copy(usableFirstIP, firstIP)
		usableFirstIP[len(usableFirstIP)-1]++
		usableLastIP := make(net.IP, len(lastIP))
		copy(usableLastIP, lastIP)
		if ipnet.IP.To4() != nil {
			usableLastIP[len(usableLastIP)-1]--
		}
		stringWriter.WriteString(p.Sprintf(template, "Usable Range", p.Sprintf("%s - %s", usableFirstIP, usableLastIP)))
		stringWriter.WriteString(p.Sprintf(template, "Usable Hosts", p.Sprintf("%d", usableHosts.Uint64())))
	}

	stringWriter.WriteString("\n\n\n")
	if n.Allocated {
		stringWriter.WriteString(n.Description)
	} else {
		stringWriter.WriteString("Unallocated")
	}

	return stringWriter.String()
}

func broadcastAddress(ip netip.Addr, maskSize int) []byte {
	ipBytes := ip.AsSlice()
	for i := len(ipBytes) - 1; maskSize < (len(ipBytes) * 8); maskSize++ {
		byteIndex := i
		bitIndex := 7 - (maskSize % 8)
		ipBytes[byteIndex] |= 1 << uint(bitIndex)
		if bitIndex == 0 {
			i--
		}
	}
	return ipBytes
}

func nextIP(ip netip.Addr) netip.Addr {
	ipBytes := ip.AsSlice()
	for i := len(ipBytes) - 1; i >= 0; i-- {
		if ipBytes[i] < 255 {
			ipBytes[i]++
			newIP, ok := netip.AddrFromSlice(ipBytes)
			if !ok {
				return ip
			}
			return newIP
		}
		ipBytes[i] = 0
	}
	return ip
}

func prevIP(ip netip.Addr) netip.Addr {
	ipBytes := ip.AsSlice()
	for i := len(ipBytes) - 1; i >= 0; i-- {
		if ipBytes[i] > 0 {
			ipBytes[i]--
			newIP, ok := netip.AddrFromSlice(ipBytes)
			if !ok {
				return ip
			}
			return newIP
		}
		ipBytes[i] = 255
	}
	return ip
}

func subnetMaskFromBits(bits int) string {
	var mask uint32 = ^uint32(0) << (32 - bits)
	return fmt.Sprintf("%d.%d.%d.%d",
		byte(mask>>24),
		byte(mask>>16),
		byte(mask>>8),
		byte(mask))
}
