package main

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

type MenuFolder struct {
	ID         string `json:"id"`
	ParentPath string `json:"parent"`
}

func (f *MenuFolder) Validate() error {
	if f.ID == "" {
		return fmt.Errorf("ID must be set")
	}

	if f.ParentPath != "" {
		if _, ok := menuItems[f.GetParentPath()]; !ok {
			return fmt.Errorf("ParentPath does not exist for %s: %s", f.ID, f.ParentPath)
		}
	}

	return nil
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
	// ParentPath already stores the full parent chain. Avoid resolving through
	// current in-memory graph so load order or broken test data don't panic.
	return f.ParentPath + " -> " + f.ID
}

func (f *MenuFolder) CurrentMenuInputCapture(event *tcell.EventKey) *tcell.EventKey {
	return event
}

func (f *MenuFolder) CurrentFocusInputCapture(event *tcell.EventKey) *tcell.EventKey {
	return event
}
