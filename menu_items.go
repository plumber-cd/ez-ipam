package main

import (
	"slices"

	"github.com/gdamore/tcell/v2"
)

type MenuItem interface {
	Validate() error
	GetID() string
	AsFolder() *MenuFolder
	GetParentPath() string
	GetParent() MenuItem
	GetPath() string
	Compare(MenuItem) int
	OnChangedFunc()
	OnSelectedFunc()
	OnDoneFunc()
	CurrentMenuInputCapture(event *tcell.EventKey) *tcell.EventKey
	CurrentFocusInputCapture(event *tcell.EventKey) *tcell.EventKey
}

type MenuItems map[string]MenuItem

func (m MenuItems) Add(menuItem MenuItem) error {
	if err := menuItem.Validate(); err != nil {
		return err
	}
	m[menuItem.GetPath()] = menuItem
	return nil
}

func (m MenuItems) Delete(menuItem MenuItem) {
	childs := m.GetChilds(menuItem)
	for _, child := range childs {
		menuItems.Delete(child)
	}
	delete(m, menuItem.GetPath())
}

func (m MenuItems) MustAdd(menuItem MenuItem) {
	if err := m.Add(menuItem); err != nil {
		panic(err)
	}
}

func (m MenuItems) GetChilds(parent MenuItem) []MenuItem {
	childs := []MenuItem{}

	for _, menuItem := range m {
		if parent == nil {
			if menuItem.GetParent() == nil {
				childs = append(childs, menuItem)
			} else {
				continue
			}
		} else if menuItem.GetParent() == parent {
			childs = append(childs, menuItem)
		}
	}

	slices.SortStableFunc(childs, func(left, right MenuItem) int {
		return left.Compare(right)
	})

	return childs
}

func (m MenuItems) GetByParentAndID(parent MenuItem, name string) MenuItem {
	for _, menuItem := range m.GetChilds(parent) {
		if menuItem.GetID() == name {
			return menuItem
		}
	}
	return nil
}

var (
	globalKeys          = []string{"<enter>/<double-click> Open", "<backspace> Back", "<q> Quit", "<ctrl+s> Save"}
	menuItems           = MenuItems{}
	currentMenuItem     MenuItem
	currentMenuItemKeys = []string{}
	currentMenuFocus    MenuItem
	currentFocusKeys    = []string{}
)
