package main

import (
	"cmp"
	"fmt"
	"math/big"
	"net"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type MenuNetwork struct {
	*MenuFolder
	CIDR string
}

func (n *MenuNetwork) GetName() string {
	return fmt.Sprintf("%s (%s)", n.Name, n.CIDR)
}

func (n *MenuNetwork) Compare(other MenuItem) int {
	if other == nil {
		return 1
	}

	otherMenu, ok := other.(*MenuNetwork)

	if !ok {
		return cmp.Compare(n.GetName(), other.GetName())
	}

	_, ipNetLeft, err := net.ParseCIDR(n.CIDR)
	if err != nil {
		panic(err)
	}

	_, ipNetRight, err := net.ParseCIDR(otherMenu.CIDR)
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

func (n *MenuNetwork) OnChangedFunc() {
	detailsPanel.Clear()
	detailsPanel.SetText(n.RenderDetails())
}

func (n *MenuNetwork) OnSelectedFunc() {
	positionLine.Clear()
	positionLine.SetText(n.GetPath())

	updateKeysLine([]string{})
}

func (n *MenuNetwork) OnDoneFunc() {
	positionLine.Clear()
	positionLine.SetText(n.GetPath())
}

func (n *MenuNetwork) RenderDetails() string {
	ip, ipNet, err := net.ParseCIDR(n.CIDR)
	if err != nil {
		return fmt.Sprintf("invalid CIDR %s: %v", n.CIDR, err)
	}

	ip = ip.To4()
	if ip == nil {
		return fmt.Sprintf("IPv6 addresses are not supported")
	}

	networkIP := ip.Mask(ipNet.Mask)

	broadcastIP := make(net.IP, len(networkIP))
	for i := 0; i < len(networkIP); i++ {
		broadcastIP[i] = networkIP[i] | ^ipNet.Mask[i]
	}

	ones, bits := ipNet.Mask.Size()
	totalHosts := big.NewInt(0).Exp(big.NewInt(2), big.NewInt(int64(bits-ones)), nil)

	var firstUsableIP, lastUsableIP net.IP
	usableHosts := new(big.Int)
	if totalHosts.Cmp(big.NewInt(2)) > 0 {
		firstUsableIP = make(net.IP, len(networkIP))
		copy(firstUsableIP, networkIP)
		firstUsableIP[3]++

		lastUsableIP = make(net.IP, len(broadcastIP))
		copy(lastUsableIP, broadcastIP)
		lastUsableIP[3]--

		usableHosts = new(big.Int).Sub(totalHosts, big.NewInt(2))
	} else {
		firstUsableIP = networkIP
		lastUsableIP = broadcastIP
		usableHosts.SetInt64(0)
	}

	stringWriter := new(strings.Builder)
	p := message.NewPrinter(language.English) // sorry, rest of the world

	stringWriter.WriteString(p.Sprintf("CIDR:                %s\n", n.CIDR))
	stringWriter.WriteString(p.Sprintf("Network Address:     %s\n", networkIP))
	stringWriter.WriteString(p.Sprintf("Mask Bits:           /%d\n", ones))
	stringWriter.WriteString(p.Sprintf("Range:               %s - %s\n", networkIP, broadcastIP))
	stringWriter.WriteString(p.Sprintf("Total Hosts:         %d\n", totalHosts.Int64()))
	stringWriter.WriteString(p.Sprintf("Usable Range:        %s - %s\n", firstUsableIP, lastUsableIP))
	stringWriter.WriteString(p.Sprintf("Usable Hosts:        %d\n", usableHosts.Int64()))
	stringWriter.WriteString(p.Sprintf("Broadcast Address:   %s\n", broadcastIP))
	stringWriter.WriteString(p.Sprintf("Netmask:             %s\n", net.IP(ipNet.Mask)))

	stringWriter.WriteString("\n\n\n")
	stringWriter.WriteString(n.Description)

	return stringWriter.String()
}
