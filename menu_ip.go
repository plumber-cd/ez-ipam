package main

import "fmt"

type MenuIP struct {
	*MenuFolder
	DisplayName string
}

func (i *MenuIP) GetID() string {
	return fmt.Sprintf("%s (%s)", i.ID, i.DisplayName)
}
