package main

import (
	"cmp"
	"fmt"

	"github.com/gdamore/tcell/v2"
)

type MenuStatic struct {
	*MenuFolder
	Index       int
	Description string
}

func (m *MenuStatic) Validate() error {
	if err := m.MenuFolder.Validate(); err != nil {
		return err
	}

	if m.Description == "" {
		return fmt.Errorf("Description must be set for MenuStatic=%s", m.GetPath())
	}
	if m.GetParentPath() != "" {
		return fmt.Errorf("ParentPath must be empty for MenuStatic=%s", m.GetPath())
	}

	return nil
}

func (m *MenuStatic) Compare(other MenuItem) int {
	if other == nil {
		return 1
	}

	otherMenu, ok := other.(*MenuStatic)
	if !ok {
		return cmp.Compare(m.GetID(), other.GetID())
	}

	return cmp.Compare(m.Index, otherMenu.Index)
}

func (m *MenuStatic) OnChangedFunc() {
	detailsPanel.Clear()
	detailsPanel.SetText(m.Description)
	currentFocusKeys = []string{}
}

func (m *MenuStatic) OnSelectedFunc() {
	positionLine.Clear()
	positionLine.SetText(m.GetPath())

	switch m.ID {
	case "Networks":
		currentMenuItemKeys = []string{
			"<n> New Network",
		}
	default:
		currentMenuItemKeys = []string{}
	}
}

func (m *MenuStatic) OnDoneFunc() {
	positionLine.Clear()
	positionLine.SetText(m.GetPath())
	currentMenuItemKeys = []string{}
}

func (m *MenuStatic) CurrentMenuInputCapture(event *tcell.EventKey) *tcell.EventKey {
	switch m.ID {
	case "Networks":
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'a':
				pages.ShowPage(newNetworkPage)
				app.SetFocus(newNetworkDialog)
				return nil
			}
		}
	}
	return event
}
