package domain

import (
	"fmt"
	"slices"
	"strconv"
)

// Catalog holds the in-memory state of all IPAM entities.
type Catalog struct {
	items map[string]Item
}

// NewCatalog creates an empty catalog.
func NewCatalog() *Catalog {
	return &Catalog{items: make(map[string]Item)}
}

// Put stores an item in the catalog without validation.
func (c *Catalog) Put(item Item) {
	c.items[item.GetPath()] = item
}

// Add validates and stores an item, returning an error on validation failure.
func (c *Catalog) Add(item Item) error {
	if err := item.Validate(c); err != nil {
		return err
	}
	c.items[item.GetPath()] = item
	return nil
}

// Delete removes an item and all its descendants from the catalog.
func (c *Catalog) Delete(item Item) {
	children := c.GetChildren(item)
	for _, child := range children {
		c.Delete(child)
	}
	delete(c.items, item.GetPath())
}

// Remove removes a single item (no cascade).
func (c *Catalog) Remove(path string) {
	delete(c.items, path)
}

// Get returns the item at the given path, or nil.
func (c *Catalog) Get(path string) Item {
	return c.items[path]
}

// GetChildren returns the sorted direct children of parent.
// If parent is nil, returns root-level items.
func (c *Catalog) GetChildren(parent Item) []Item {
	var children []Item
	var parentPath string
	if parent != nil {
		parentPath = parent.GetPath()
	}
	for _, item := range c.items {
		if parent == nil {
			if item.GetParentPath() == "" {
				children = append(children, item)
			}
		} else if item.GetParentPath() == parentPath {
			children = append(children, item)
		}
	}
	slices.SortStableFunc(children, func(a, b Item) int {
		return a.Compare(b)
	})
	return children
}

// GetByParentAndDisplayID finds a child of parent whose DisplayID matches.
func (c *Catalog) GetByParentAndDisplayID(parent Item, displayID string) Item {
	for _, item := range c.GetChildren(parent) {
		if item.DisplayID() == displayID {
			return item
		}
	}
	return nil
}

// FindVLANByID finds a VLAN by its numeric VLAN ID.
func (c *Catalog) FindVLANByID(vlanID int) *VLAN {
	vlansRoot := c.GetByParentAndDisplayID(nil, FolderVLANs)
	if vlansRoot == nil {
		return nil
	}
	for _, item := range c.GetChildren(vlansRoot) {
		vlan, ok := item.(*VLAN)
		if !ok {
			continue
		}
		if vlan.ID == fmt.Sprintf("%d", vlanID) {
			return vlan
		}
	}
	return nil
}

// All returns all items in the catalog (read-only view).
func (c *Catalog) All() map[string]Item {
	return c.items
}

// RenderVLANID returns a human-readable representation of a VLAN ID.
func (c *Catalog) RenderVLANID(vlanID int) string {
	if vlanID <= 0 {
		return "<none>"
	}
	vlan := c.FindVLANByID(vlanID)
	if vlan == nil {
		return strconv.Itoa(vlanID)
	}
	return fmt.Sprintf("%d (%s)", vlanID, vlan.DisplayName)
}
