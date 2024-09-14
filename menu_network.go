package main

import "fmt"

type MenuNetwork struct {
	*MenuFolder
	CIDR string
}

func (n *MenuNetwork) GetName() string {
	return fmt.Sprintf("%s (%s)", n.Name, n.CIDR)
}
