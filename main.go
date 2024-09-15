package main

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	mainPage       = "*main*"
	newNetworkPage = "*new_network*"
)

var ()

var (
	app   *tview.Application
	pages *tview.Pages

	positionLine    *tview.TextView
	navigationPanel *tview.List
	detailsPanel    *tview.TextView
	statusLine      *tview.TextView
	keysLine        *tview.TextView

	newNetworkDialog *tview.Form
)

func main() {
	app = tview.NewApplication()
	rootFlex := tview.NewFlex().SetDirection(tview.FlexRow)

	positionLine = tview.NewTextView()
	positionLine.SetBorder(true)
	positionLine.SetTitle("Navigation")
	positionLine.SetText("Home")
	rootFlex.AddItem(positionLine, 3, 1, false)

	middleFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
	rootFlex.AddItem(middleFlex, 0, 2, false)

	navigationPanel = tview.NewList()
	navigationPanel.ShowSecondaryText(false)
	navigationPanel.SetBorder(true).SetTitle("Menu")
	middleFlex.AddItem(navigationPanel, 0, 1, false)

	detailsFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	middleFlex.AddItem(detailsFlex, 0, 2, false)

	detailsPanel = tview.NewTextView()
	detailsPanel.SetBorder(true).SetTitle("Details")
	detailsFlex.AddItem(detailsPanel, 0, 1, false)

	keysLine = tview.NewTextView()
	keysLine.SetBorder(false)
	updateKeysLine()
	rootFlex.AddItem(keysLine, 1, 1, false)

	statusLine = tview.NewTextView()
	statusLine.SetBorder(true)
	statusLine.SetTitle("Status")
	detailsFlex.AddItem(statusLine, 3, 1, false)

	positionLine.SetFocusFunc(func() {
		app.SetFocus(navigationPanel)
	})
	detailsPanel.SetFocusFunc(func() {
		app.SetFocus(navigationPanel)
	})
	statusLine.SetFocusFunc(func() {
		app.SetFocus(navigationPanel)
	})

	navigationPanel.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		selected := menuItems.GetByParentAndID(currentMenuItem, mainText)
		if selected == nil {
			panic("Failed to find currently changed menu item!")
		}

		currentMenuFocus = selected
		currentMenuFocus.OnChangedFunc()
		updateKeysLine()
	})

	navigationPanel.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		selected := menuItems.GetByParentAndID(currentMenuItem, mainText)
		if selected == nil {
			panic("Failed to find currently selected menu item!")
		}

		newChilds := menuItems.GetChilds(selected)
		if len(newChilds) > 0 {
			oldMenuItem := currentMenuItem
			currentMenuItem = selected

			reloadMenu(oldMenuItem)
			currentMenuItem.OnSelectedFunc()
		} else {
			statusLine.Clear()
			statusLine.SetText("No child items for " + selected.GetPath())
		}
		updateKeysLine()
	})

	navigationPanel.SetDoneFunc(func() {
		if currentMenuItem == nil {
			return
		}

		currentMenuItem.OnDoneFunc()

		oldMenuItem := currentMenuItem
		currentMenuItem = currentMenuItem.GetParent()

		reloadMenu(oldMenuItem)

		if currentMenuItem == nil {
			positionLine.Clear()
			positionLine.SetText("Home")
		} else {
			currentMenuItem.OnSelectedFunc()
		}

		updateKeysLine()
	})

	navigationPanel.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyBS, tcell.KeyBackspace2:
			return tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
		case tcell.KeyCtrlU:
			return tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone)
		case tcell.KeyCtrlD:
			return tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone)
		case tcell.KeyRune:
			switch event.Rune() {
			case 'h':
				return tcell.NewEventKey(tcell.KeyLeft, tcell.RuneLArrow, tcell.ModNone)
			case 'j':
				return tcell.NewEventKey(tcell.KeyDown, tcell.RuneDArrow, tcell.ModNone)
			case 'k':
				return tcell.NewEventKey(tcell.KeyUp, tcell.RuneUArrow, tcell.ModNone)
			case 'l':
				return tcell.NewEventKey(tcell.KeyRight, tcell.RuneRArrow, tcell.ModNone)
			case 'q':
				app.Stop()
				return nil
			}
		}

		if currentMenuItem != nil {
			if e := currentMenuItem.CurrentMenuInputCapture(event); e != event {
				return e
			}
		}
		if currentMenuFocus != nil {
			if e := currentMenuFocus.CurrentFocusInputCapture(event); e != event {
				return e
			}
		}

		return event
	})

	newNetworkDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
		AddInputField("CIDR", "", 42, nil, nil).
		AddButton("Save", func() {
			AddNewNetwork(getAndClearTextFromInputField(newNetworkDialog, "CIDR", true))
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}).
		AddButton("Cancel", func() {
			getAndClearTextFromInputField(newNetworkDialog, "CIDR", true)

			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		})
	newNetworkDialog.SetBorder(true).SetTitle("Add Network")
	newNetworkDialogFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(newNetworkDialog, 7, 1, false).
				AddItem(nil, 0, 1, false),
			51, 1, false).
		AddItem(nil, 0, 1, false)

	pages = tview.NewPages().
		AddPage(mainPage, rootFlex, true, true).
		AddPage(newNetworkPage, newNetworkDialogFlex, true, false)
	app.SetRoot(pages, true)
	app.EnableMouse(true)
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlC:
			return nil
		case tcell.KeyCtrlS:
			statusLine.Clear()
			statusLine.SetText("Saved")
			return nil
		case tcell.KeyCtrlQ:
			app.Stop()
			return nil
		}

		return event
	})
	pages.SwitchToPage(mainPage)
	app.SetFocus(navigationPanel)

	load()

	if err := app.Run(); err != nil {
		panic(err)
	}
}

func load() {
	networks := &MenuStatic{
		MenuFolder: &MenuFolder{
			ID: "Networks",
		},
		Index: 0,
		Description: `
            In the network menu you can slice and dice the network
        `,
	}
	menuItems.MustAdd(networks)

	ips := &MenuStatic{
		MenuFolder: &MenuFolder{
			ID: "IPs",
		},
		Index: 1,
		Description: `
                In the IPs menu you can track IP reservations
            `,
	}
	menuItems.MustAdd(ips)

	cgNatNetwork := &Network{
		MenuFolder: &MenuFolder{
			ID:         "100.64.0.0/10",
			ParentPath: networks.GetPath(),
		},
		Allocated:   true,
		DisplayName: "CG-NAT",
		Description: "This is the CG-NAT network",
	}
	menuItems.MustAdd(cgNatNetwork)

	menuItems.MustAdd(
		&Network{
			MenuFolder: &MenuFolder{
				ID:         "100.64.0.0/11",
				ParentPath: cgNatNetwork.GetPath(),
			},
		},
	)
	menuItems.MustAdd(
		&Network{
			MenuFolder: &MenuFolder{
				ID:         "100.96.0.0/11",
				ParentPath: cgNatNetwork.GetPath(),
			},
		},
	)

	menuItems.MustAdd(
		&Network{
			MenuFolder: &MenuFolder{
				ID:         "10.0.0.0/8",
				ParentPath: networks.GetPath(),
			},
			DisplayName: "Home",
		},
	)

	menuItems.MustAdd(
		&Network{
			MenuFolder: &MenuFolder{
				ID:         "fdb1:77aa:038a::0/48",
				ParentPath: networks.GetPath(),
			},
			DisplayName: "Home IPv6",
		},
	)

	menuItems.MustAdd(
		&Network{
			MenuFolder: &MenuFolder{
				ID:         "fdb1:77aa:038b::0/64",
				ParentPath: networks.GetPath(),
			},
			DisplayName: "Home IPv6",
		},
	)

	menuItems.MustAdd(
		&Network{
			MenuFolder: &MenuFolder{
				ID:         "fdb1:77aa:038c::0/72",
				ParentPath: networks.GetPath(),
			},
			DisplayName: "Test IPv6",
		},
	)

	reloadMenu(nil)
}

func reloadMenu(focusedItem MenuItem) {
	navigationPanel.Clear()

	newMenuItems := menuItems.GetChilds(currentMenuItem)
	fromIndex := -1
	for i, menuItem := range newMenuItems {
		if focusedItem != nil && focusedItem.GetID() == menuItem.GetID() {
			fromIndex = i
		}
		navigationPanel.AddItem(menuItem.GetID(), menuItem.GetPath(), 0, nil)
	}

	if fromIndex >= 0 {
		navigationPanel.SetCurrentItem(fromIndex)
	}
}

func updateKeysLine() {
	keysLine.Clear()
	keysLine.SetText(" " + strings.Join(append(append(globalKeys, currentMenuItemKeys...), currentFocusKeys...), " | "))
}

func getAndClearTextFromInputField(form *tview.Form, label string, leaveInFocus bool) string {
	formItemIndex := form.GetFormItemIndex(label)
	if formItemIndex < 0 {
		panic("Failed to find " + label + " input field index")
	}

	formItem := form.GetFormItem(formItemIndex)
	if formItem == nil {
		panic("Failed to find " + label + " input field")
	}

	inputField, ok := formItem.(*tview.InputField)
	if !ok {
		panic("Failed to cast " + label + " input field")
	}

	text := inputField.GetText()
	inputField.SetText("")

	if leaveInFocus {
		form.SetFocus(formItemIndex)
	}

	return text
}
