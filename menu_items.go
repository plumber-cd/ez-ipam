package main

import "slices"

type MenuItem interface {
	GetName() string
	AsFolder() *MenuFolder
	GetParentPath() string
	GetParent() MenuItem
	GetPath() string
	Compare(MenuItem) int
	OnChangedFunc()
	OnSelectedFunc()
	OnDoneFunc()
}

type MenuItems map[string]MenuItem

func (m MenuItems) Add(menuItem MenuItem) MenuItems {
	m[menuItem.GetPath()] = menuItem
	return m
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

func (m MenuItems) GetByParentAndName(parent MenuItem, name string) MenuItem {
	for _, menuItem := range m.GetChilds(parent) {
		if menuItem.GetName() == name {
			return menuItem
		}
	}
	return nil
}

var (
	menuItems       = MenuItems{}
	currentMenuItem MenuItem
)
