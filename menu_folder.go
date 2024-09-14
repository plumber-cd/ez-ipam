package main

import "github.com/gdamore/tcell/v2"

type MenuFolder struct {
	ID         string `json:"id"`
	ParentPath string `json:"parent"`
}

func (f *MenuFolder) GetID() string {
	return f.ID
}

func (f *MenuFolder) AsFolder() *MenuFolder {
	return f
}

func (f *MenuFolder) GetParentPath() string {
	return f.ParentPath
}

func (f *MenuFolder) GetParent() MenuItem {
	return menuItems[f.ParentPath]
}

func (f *MenuFolder) GetPath() string {
	if f.ParentPath == "" {
		return f.ID
	}
	return f.GetParent().GetPath() + " -> " + f.ID
}

func (f *MenuFolder) CurrentMenuInputCapture(event *tcell.EventKey) *tcell.EventKey {
	return event
}

func (f *MenuFolder) CurrentFocusInputCapture(event *tcell.EventKey) *tcell.EventKey {
	return event
}
