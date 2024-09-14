package main

import (
	"cmp"
	"fmt"
	"net"
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

	ipLeft := ipNetLeft.IP.To4()
	ipRight := ipNetRight.IP.To4()

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
	detailsPanel.SetText("Details about " + n.GetName())
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
