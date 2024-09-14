package main

type MenuFolder struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parent      string `json:"parent"`
}

func (f *MenuFolder) GetName() string {
	return f.Name
}

func (f *MenuFolder) AsFolder() *MenuFolder {
	return f
}

func (f *MenuFolder) GetParentPath() string {
	return f.Parent
}

func (f *MenuFolder) GetParent() MenuItem {
	return menuItems[f.Parent]
}

func (f *MenuFolder) GetPath() string {
	if f.Parent == "" {
		return f.Name
	}
	return f.GetParent().GetPath() + " -> " + f.Name
}
