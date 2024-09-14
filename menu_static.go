package main

import (
	"cmp"
)

type MenuStatic struct {
	*MenuFolder
	Index       int
	Description string
}

func (m *MenuStatic) Compare(other MenuItem) int {
	if other == nil {
		return 1
	}

	otherMenu, ok := other.(*MenuStatic)
	if !ok {
		return cmp.Compare(m.GetName(), other.GetName())
	}

	return cmp.Compare(m.Index, otherMenu.Index)
}

func (m *MenuStatic) OnChangedFunc() {
	detailsPanel.Clear()
	detailsPanel.SetText(m.Description)
}

func (m *MenuStatic) OnSelectedFunc() {
	positionLine.Clear()
	positionLine.SetText(m.GetPath())

	updateKeysLine([]string{})
}

func (m *MenuStatic) OnDoneFunc() {
	positionLine.Clear()
	positionLine.SetText(m.GetPath())
}
