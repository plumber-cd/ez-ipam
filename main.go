package main

import (
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	mainPage                    = "*main*"
	newNetworkPage              = "*new_network*"
	splitNetworkPage            = "*split_network*"
	summarizeNetworkPage        = "*summarize_network*"
	allocateNetworkPage         = "*allocate_network*"
	updateNetworkAllocationPage = "*update_network_allocation*"
	deallocateNetworkPage       = "*deallocate_network*"
	deleteNetworkPage           = "*delete_network*"
	quitPage                    = "*quit*"
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

	newNetworkDialog              *tview.Form
	splitNetworkDialog            *tview.Form
	summarizeNetworkDialog        *tview.Modal
	allocateNetworkDialog         *tview.Form
	updateNetworkAllocationDialog *tview.Form
	deallocateNetworkDialog       *tview.Modal
	deleteNetworkDialog           *tview.Modal
	quitDialog                    *tview.Modal
)

func main() {
	{
		app = tview.NewApplication()
		pages = tview.NewPages()
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

		pages.AddPage(mainPage, rootFlex, true, true)
	}

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
			n, ok := selected.(*Network)
			if !ok {
				statusLine.Clear()
				statusLine.SetText("No child items for " + selected.GetPath())
				return
			}

			if n.Allocated {
				panic("How can allocated network not have any children?")
			}

			allocateNetworkDialog.SetFocus(0)
			pages.ShowPage(allocateNetworkPage)
			app.SetFocus(allocateNetworkDialog)
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
				pages.ShowPage(quitPage)
				app.SetFocus(quitDialog)
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

	{
		height := 7
		width := 51
		newNetworkDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("CIDR", "", 42, nil, nil).
			AddButton("Save", func() {
				AddNewNetwork(getAndClearTextFromInputField(newNetworkDialog, "CIDR"))
				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", func() {
				getAndClearTextFromInputField(newNetworkDialog, "CIDR")

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			})
		newNetworkDialog.SetBorder(true).SetTitle("New Network")
		newNetworkDialogFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(
				tview.NewFlex().SetDirection(tview.FlexRow).
					AddItem(nil, 0, 1, false).
					AddItem(newNetworkDialog, height, 1, false).
					AddItem(nil, 0, 1, false),
				width, 1, false).
			AddItem(nil, 0, 1, false)
		pages.AddPage(newNetworkPage, newNetworkDialogFlex, true, false)
	}

	{
		height := 7
		width := 66
		splitNetworkDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("New Networks Prefix", "", 42, nil, nil).
			AddButton("Save", func() {
				newPrefix := getAndClearTextFromInputField(splitNetworkDialog, "New Networks Prefix")
				newPrefix = strings.TrimLeft(newPrefix, "/")
				newPrefixInt, err := strconv.Atoi(newPrefix)
				if err != nil {
					statusLine.Clear()
					statusLine.SetText("Invalid new prefix, should be a number representing smaller networks than this parent " + err.Error())
				}
				SplitNetwork(newPrefixInt)
				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", func() {
				getAndClearTextFromInputField(splitNetworkDialog, "New Networks Prefix")

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			})
		splitNetworkDialog.SetBorder(true)
		splitNetworkDialogFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(
				tview.NewFlex().SetDirection(tview.FlexRow).
					AddItem(nil, 0, 1, false).
					AddItem(splitNetworkDialog, height, 1, false).
					AddItem(nil, 0, 1, false),
				width, 1, false).
			AddItem(nil, 0, 1, false)
		pages.AddPage(splitNetworkPage, splitNetworkDialogFlex, true, false)
	}

	{
		summarizeNetworkDialog = tview.NewModal().
			AddButtons([]string{"Yes", "No"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				switch buttonLabel {
				case "Yes":
					SummarizeNetwork()
					fallthrough
				case "No":
					fallthrough
				default:
					summarizeNetworkDialog.SetText("")
					pages.SwitchToPage(mainPage)
					app.SetFocus(navigationPanel)
				}
			})
		pages.AddPage(summarizeNetworkPage, summarizeNetworkDialog, true, false)
	}

	{
		height := 15
		width := 59
		allocateNetworkDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("Display Name", "", 40, nil, nil).
			AddTextArea("Description", "", 48, 5, 0, nil).
			AddInputField("Subnets Prefix", "", 40, nil, nil).
			AddButton("Save", func() {
				displayName := getAndClearTextFromInputField(allocateNetworkDialog, "Display Name")
				description := getAndClearTextFromTextArea(allocateNetworkDialog, "Description")
				subnetsPrefix := getAndClearTextFromInputField(allocateNetworkDialog, "Subnets Prefix")
				subnetsPrefix = strings.TrimLeft(subnetsPrefix, "/")
				subnetsPrefixInt, err := strconv.Atoi(subnetsPrefix)
				if err != nil {
					statusLine.Clear()
					statusLine.SetText("Invalid subnets prefix, should be a number representing smaller networks than this parent " + err.Error())
				}
				AllocateNetwork(displayName, description, subnetsPrefixInt)

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", func() {
				getAndClearTextFromInputField(allocateNetworkDialog, "Display Name")
				getAndClearTextFromTextArea(allocateNetworkDialog, "Description")
				getAndClearTextFromInputField(allocateNetworkDialog, "Subnets Prefix")

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			})
		allocateNetworkDialog.SetBorder(true)
		allocateNetworkDialogFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(
				tview.NewFlex().SetDirection(tview.FlexRow).
					AddItem(nil, 0, 1, false).
					AddItem(allocateNetworkDialog, height, 1, false).
					AddItem(nil, 0, 1, false),
				width, 1, false).
			AddItem(nil, 0, 1, false)
		pages.AddPage(allocateNetworkPage, allocateNetworkDialogFlex, true, false)
	}

	{
		height := 13
		width := 59
		updateNetworkAllocationDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("Display Name", "", 42, nil, nil).
			AddTextArea("Description", "", 48, 5, 0, nil).
			AddButton("Save", func() {
				displayName := getAndClearTextFromInputField(updateNetworkAllocationDialog, "Display Name")
				description := getAndClearTextFromTextArea(updateNetworkAllocationDialog, "Description")
				UpdateNetworkAllocation(displayName, description)

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", func() {
				getAndClearTextFromInputField(updateNetworkAllocationDialog, "Display Name")
				getAndClearTextFromTextArea(updateNetworkAllocationDialog, "Description")

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			})
		updateNetworkAllocationDialog.SetBorder(true)
		updateNetworkAllocationFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(
				tview.NewFlex().SetDirection(tview.FlexRow).
					AddItem(nil, 0, 1, false).
					AddItem(updateNetworkAllocationDialog, height, 1, false).
					AddItem(nil, 0, 1, false),
				width, 1, false).
			AddItem(nil, 0, 1, false)
		pages.AddPage(updateNetworkAllocationPage, updateNetworkAllocationFlex, true, false)
	}

	{
		deallocateNetworkDialog = tview.NewModal().
			AddButtons([]string{"Yes", "No"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				switch buttonLabel {
				case "Yes":
					DeallocateNetwork()
					fallthrough
				case "No":
					fallthrough
				default:
					deallocateNetworkDialog.SetText("")
					pages.SwitchToPage(mainPage)
					app.SetFocus(navigationPanel)
				}
			})
		pages.AddPage(deallocateNetworkPage, deallocateNetworkDialog, true, false)
	}

	{
		deleteNetworkDialog = tview.NewModal().
			AddButtons([]string{"Yes", "No"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				switch buttonLabel {
				case "Yes":
					DeleteNetwork()
					fallthrough
				case "No":
					fallthrough
				default:
					deleteNetworkDialog.SetText("")
					pages.SwitchToPage(mainPage)
					app.SetFocus(navigationPanel)
				}
			})
		pages.AddPage(deleteNetworkPage, deleteNetworkDialog, true, false)
	}

	{
		quitDialog = tview.NewModal().SetText("Do you want to quit? All unsaved changes will be lost.").
			AddButtons([]string{"Quit", "Cancel"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				switch buttonLabel {
				case "Quit":
					app.Stop()
				case "Cancel":
					fallthrough
				default:
					pages.SwitchToPage(mainPage)
					app.SetFocus(navigationPanel)
				}
			})
		pages.AddPage(quitPage, quitDialog, true, false)
	}

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
			// This is "hidden" quit without the confirmation dialog - use for local debugging, maybe should disable from the release version
			app.Stop()
			return nil
			// case tcell.KeyCtrlD:
			// 	statusLine.Clear()
			// 	currentFocus := app.GetFocus()
			// 	currentFocusStr := reflect.TypeOf(currentFocus).String()
			// 	statusLine.SetText(currentFocusStr)
			// 	return nil
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

func getFormItemByLabel(form *tview.Form, label string) (int, tview.FormItem) {
	formItemIndex := form.GetFormItemIndex(label)
	if formItemIndex < 0 {
		panic("Failed to find " + label + " form item index")
	}

	formItem := form.GetFormItem(formItemIndex)
	if formItem == nil {
		panic("Failed to find " + label + " form item")
	}

	return formItemIndex, formItem
}

func getAndClearTextFromInputField(form *tview.Form, label string) string {
	_, formItem := getFormItemByLabel(form, label)

	inputField, ok := formItem.(*tview.InputField)
	if !ok {
		panic("Failed to cast " + label + " input field")
	}

	text := inputField.GetText()
	inputField.SetText("")

	return text
}

func setTextFromInputField(form *tview.Form, label, value string) {
	_, formItem := getFormItemByLabel(form, label)

	inputField, ok := formItem.(*tview.InputField)
	if !ok {
		panic("Failed to cast " + label + " input field")
	}

	inputField.SetText(value)
}

func getAndClearTextFromTextArea(form *tview.Form, label string) string {
	_, formItem := getFormItemByLabel(form, label)

	textArea, ok := formItem.(*tview.TextArea)
	if !ok {
		panic("Failed to cast " + label + " text area")
	}

	text := textArea.GetText()
	textArea.SetText("", false)

	return text
}

func setTextFromTextArea(form *tview.Form, label, value string) {
	_, formItem := getFormItemByLabel(form, label)

	textArea, ok := formItem.(*tview.TextArea)
	if !ok {
		panic("Failed to cast " + label + " text area")
	}

	textArea.SetText(value, true)
}
