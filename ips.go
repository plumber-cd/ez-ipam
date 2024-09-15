package main

import "fmt"

type IP struct {
	*MenuFolder
	DisplayName string
}

func (i *IP) GetID() string {
	return fmt.Sprintf("%s (%s)", i.ID, i.DisplayName)
}
