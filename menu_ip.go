package main

import "fmt"

type MenuIP struct {
	*MenuFolder
	IP string
}

func (i *MenuIP) GetName() string {
	return fmt.Sprintf("%s (%s)", i.Name, i.IP)
}
