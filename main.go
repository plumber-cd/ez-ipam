package main

import (
	_ "embed"
	"fmt"
	"text/template"

	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"sigs.k8s.io/yaml"
)

const (
	mainPage                       = "*main*"
	newNetworkPage                 = "*new_network*"
	splitNetworkPage               = "*split_network*"
	summarizeNetworkPage           = "*summarize_network*"
	allocateNetworkSubnetsModePage = "*allocate_network_subnets_mode*"
	allocateNetworkHostsModePage   = "*allocate_network_hosts_mode*"
	updateNetworkAllocationPage    = "*update_network_allocation*"
	deallocateNetworkPage          = "*deallocate_network*"
	deleteNetworkPage              = "*delete_network*"
	reserveIPPage                  = "*reserve_ip*"
	updateIPReservationPage        = "*update_ip_reservation*"
	unreserveIPPage                = "*unreserve_ip*"
	quitPage                       = "*quit*"

	dataDirName      = ".ez-ipam"
	networksDirName  = "networks"
	ipsDirName       = "ips"
	markdownFileName = "EZ-IPAM.md"
)

var (
	//go:embed markdown.tmpl
	markdownTmpl string

	app   *tview.Application
	pages *tview.Pages

	positionLine    *tview.TextView
	navigationPanel *tview.List
	detailsPanel    *tview.TextView
	statusLine      *tview.TextView
	keysLine        *tview.TextView
	detailsFlex     *tview.Flex

	newNetworkDialog                 *tview.Form
	splitNetworkDialog               *tview.Form
	summarizeNetworkDialog           *tview.Form
	allocateNetworkSubnetsModeDialog *tview.Form
	allocateNetworkHostsModeDialog   *tview.Form
	updateNetworkAllocationDialog    *tview.Form
	deallocateNetworkDialog          *tview.Modal
	deleteNetworkDialog              *tview.Modal
	reserveIPDialog                  *tview.Form
	updateIPReservationDialog        *tview.Form
	unreserveIPDialog                *tview.Modal
	quitDialog                       *tview.Modal

	summarizeCandidates []*Network
	summarizeFromIndex  int
	summarizeToIndex    int
)

func resetState() {
	menuItems = MenuItems{}
	currentMenuItem = nil
	currentMenuFocus = nil
	currentMenuItemKeys = []string{}
	currentFocusKeys = []string{}
	summarizeCandidates = nil
	summarizeFromIndex = 0
	summarizeToIndex = 0
}

func setupApp() {
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

		detailsFlex = tview.NewFlex().SetDirection(tview.FlexRow)
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
		statusLine.SetWrap(true)
		statusLine.SetWordWrap(true)
		statusLine.SetChangedFunc(func() {
			resizeStatusLine()
		})
		detailsFlex.AddItem(statusLine, 3, 0, false)

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
	keysLine.SetFocusFunc(func() {
		app.SetFocus(navigationPanel)
	})

	mouseSelectArmed := true
	navigationPanel.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		switch action {
		case tview.MouseLeftClick:
			// Single click should only highlight an item.
			mouseSelectArmed = false
		case tview.MouseLeftDoubleClick:
			// Double click should navigate like Enter.
			mouseSelectArmed = true
			return tview.MouseLeftClick, event
		}

		return action, event
	})

	navigationPanel.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		selected := menuItems.GetByParentAndID(currentMenuItem, mainText)
		if selected == nil {
			// Ignore stale changed events produced by list mouse callbacks while menu changes.
			return
		}

		currentMenuFocus = selected
		currentMenuFocus.OnChangedFunc()
		updateKeysLine()
	})

	navigationPanel.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		if !mouseSelectArmed {
			mouseSelectArmed = true
			return
		}

		selected := menuItems.GetByParentAndID(currentMenuItem, mainText)
		if selected == nil {
			return
		}

		oldMenuItem := currentMenuItem
		currentMenuItem = selected

		reloadMenu(oldMenuItem)
		currentMenuItem.OnSelectedFunc()
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
				quitDialog.SetFocus(1)
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

	mouseBlocker := func() *tview.Box {
		box := tview.NewBox()
		box.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
			return action, nil
		})
		return box
	}
	createDialogPage := func(content tview.Primitive, width, height int) tview.Primitive {
		return tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(mouseBlocker(), 0, 1, false).
			AddItem(
				tview.NewFlex().SetDirection(tview.FlexRow).
					AddItem(mouseBlocker(), 0, 1, false).
					AddItem(content, height, 1, false).
					AddItem(mouseBlocker(), 0, 1, false),
				width, 1, false).
			AddItem(mouseBlocker(), 0, 1, false)
	}
	submitPrimaryFormButton := func(form *tview.Form) {
		if form.GetButtonCount() == 0 {
			return
		}
		handler := form.GetButton(0).InputHandler()
		if handler == nil {
			return
		}
		handler(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(p tview.Primitive) {
			app.SetFocus(p)
		})
	}
	wireDialogFormKeys := func(form *tview.Form, onCancel func()) {
		form.SetCancelFunc(onCancel)
		form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			formItemIndex, _ := form.GetFocusedItemIndex()
			if formItemIndex >= 0 {
				switch form.GetFormItem(formItemIndex).(type) {
				case *tview.DropDown:
					// Let dropdown handle Enter/Escape itself:
					// - Enter selects/toggles options
					// - Escape closes the options list
					return event
				}
			}

			switch event.Key() {
			case tcell.KeyEscape:
				onCancel()
				return nil
			case tcell.KeyEnter:
				if formItemIndex >= 0 {
					if _, ok := form.GetFormItem(formItemIndex).(*tview.TextArea); ok {
						return event
					}
				}
				submitPrimaryFormButton(form)
				return nil
			}
			return event
		})
	}
	{
		height := 7
		width := 51
		cancelDialog := func() {
			getAndClearTextFromInputField(newNetworkDialog, "CIDR")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		newNetworkDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("CIDR", "", 42, nil, nil).
			AddButton("Save", func() {
				AddNewNetwork(getAndClearTextFromInputField(newNetworkDialog, "CIDR"))
				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		newNetworkDialog.SetBorder(true).SetTitle("Add Network")
		wireDialogFormKeys(newNetworkDialog, cancelDialog)
		newNetworkDialogFlex := createDialogPage(newNetworkDialog, width, height)
		pages.AddPage(newNetworkPage, newNetworkDialogFlex, true, false)
	}

	{
		height := 7
		width := 66
		cancelDialog := func() {
			getAndClearTextFromInputField(splitNetworkDialog, "New Prefix Length")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		splitNetworkDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("New Prefix Length", "", 42, nil, nil).
			AddButton("Save", func() {
				newPrefix := getAndClearTextFromInputField(splitNetworkDialog, "New Prefix Length")
				newPrefix = strings.TrimLeft(newPrefix, "/")
				newPrefixInt, err := strconv.Atoi(newPrefix)
				if err != nil {
					statusLine.Clear()
					statusLine.SetText("Invalid prefix length. Enter a number larger than the current prefix: " + err.Error())
					return
				}
				SplitNetwork(newPrefixInt)
				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		splitNetworkDialog.SetBorder(true).SetTitle("Split Network")
		wireDialogFormKeys(splitNetworkDialog, cancelDialog)
		splitNetworkDialogFlex := createDialogPage(splitNetworkDialog, width, height)
		pages.AddPage(splitNetworkPage, splitNetworkDialogFlex, true, false)
	}

	{
		height := 9
		width := 72
		cancelDialog := func() {
			summarizeCandidates = nil
			summarizeFromIndex = 0
			summarizeToIndex = 0
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		summarizeNetworkDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddDropDown("From", nil, 0, func(option string, optionIndex int) {
				if optionIndex >= 0 {
					summarizeFromIndex = optionIndex
				}
			}).
			AddDropDown("To", nil, 0, func(option string, optionIndex int) {
				if optionIndex >= 0 {
					summarizeToIndex = optionIndex
				}
			}).
			AddButton("Summarize", func() {
				SummarizeNetworkSelection(summarizeCandidates, summarizeFromIndex, summarizeToIndex)
				summarizeCandidates = nil
				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		summarizeNetworkDialog.SetBorder(true).SetTitle("Summarize Networks")
		_, fromItem := getFormItemByLabel(summarizeNetworkDialog, "From")
		fromDropdown, ok := fromItem.(*tview.DropDown)
		if !ok {
			panic("failed to cast summarize From dropdown")
		}
		fromDropdown.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyRune {
				return nil
			}
			return event
		})

		_, toItem := getFormItemByLabel(summarizeNetworkDialog, "To")
		toDropdown, ok := toItem.(*tview.DropDown)
		if !ok {
			panic("failed to cast summarize To dropdown")
		}
		toDropdown.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyRune {
				return nil
			}
			return event
		})
		wireDialogFormKeys(summarizeNetworkDialog, cancelDialog)
		pages.AddPage(summarizeNetworkPage, createDialogPage(summarizeNetworkDialog, width, height), true, false)
	}

	{
		height := 13
		width := 64
		cancelDialog := func() {
			getAndClearTextFromInputField(allocateNetworkSubnetsModeDialog, "Name")
			getAndClearTextFromTextArea(allocateNetworkSubnetsModeDialog, "Description")
			getAndClearTextFromInputField(allocateNetworkSubnetsModeDialog, "Prefix Len")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		allocateNetworkSubnetsModeDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("Name", "", 42, nil, nil).
			AddTextArea("Description", "", 50, 3, 0, nil).
			AddInputField("Prefix Len", "", 42, nil, nil).
			AddButton("Save", func() {
				displayName := getAndClearTextFromInputField(allocateNetworkSubnetsModeDialog, "Name")
				description := getAndClearTextFromTextArea(allocateNetworkSubnetsModeDialog, "Description")
				subnetsPrefix := getAndClearTextFromInputField(allocateNetworkSubnetsModeDialog, "Prefix Len")
				subnetsPrefix = strings.TrimLeft(subnetsPrefix, "/")
				subnetsPrefixInt, err := strconv.Atoi(subnetsPrefix)
				if err != nil {
					statusLine.Clear()
					statusLine.SetText("Invalid subnet prefix length: " + err.Error())
					return
				}
				AllocateNetworkInSubnetsMode(displayName, description, subnetsPrefixInt)

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		allocateNetworkSubnetsModeDialog.SetBorder(true).SetTitle("Allocate Subnets")
		wireDialogFormKeys(allocateNetworkSubnetsModeDialog, cancelDialog)
		allocateNetworkSubnetsModeFlex := createDialogPage(allocateNetworkSubnetsModeDialog, width, height)
		pages.AddPage(allocateNetworkSubnetsModePage, allocateNetworkSubnetsModeFlex, true, false)
	}

	{
		height := 11
		width := 62
		cancelDialog := func() {
			getAndClearTextFromInputField(allocateNetworkHostsModeDialog, "Name")
			getAndClearTextFromTextArea(allocateNetworkHostsModeDialog, "Description")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		allocateNetworkHostsModeDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("Name", "", 42, nil, nil).
			AddTextArea("Description", "", 50, 3, 0, nil).
			AddButton("Save", func() {
				displayName := getAndClearTextFromInputField(allocateNetworkHostsModeDialog, "Name")
				description := getAndClearTextFromTextArea(allocateNetworkHostsModeDialog, "Description")

				AllocateNetworkInHostsMode(displayName, description)

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		allocateNetworkHostsModeDialog.SetBorder(true).SetTitle("Allocate Hosts")
		wireDialogFormKeys(allocateNetworkHostsModeDialog, cancelDialog)
		allocateNetworkHostsModeFlex := createDialogPage(allocateNetworkHostsModeDialog, width, height)
		pages.AddPage(allocateNetworkHostsModePage, allocateNetworkHostsModeFlex, true, false)
	}

	{
		height := 11
		width := 62
		cancelDialog := func() {
			getAndClearTextFromInputField(updateNetworkAllocationDialog, "Name")
			getAndClearTextFromTextArea(updateNetworkAllocationDialog, "Description")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		updateNetworkAllocationDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("Name", "", 42, nil, nil).
			AddTextArea("Description", "", 50, 3, 0, nil).
			AddButton("Save", func() {
				displayName := getAndClearTextFromInputField(updateNetworkAllocationDialog, "Name")
				description := getAndClearTextFromTextArea(updateNetworkAllocationDialog, "Description")
				UpdateNetworkAllocation(displayName, description)

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		updateNetworkAllocationDialog.SetBorder(true).SetTitle("Update Allocation")
		wireDialogFormKeys(updateNetworkAllocationDialog, cancelDialog)
		updateNetworkAllocationFlex := createDialogPage(updateNetworkAllocationDialog, width, height)
		pages.AddPage(updateNetworkAllocationPage, updateNetworkAllocationFlex, true, false)
	}

	{
		height := 11
		width := 64
		cancelDialog := func() {
			getAndClearTextFromInputField(reserveIPDialog, "IP Address")
			getAndClearTextFromInputField(reserveIPDialog, "Name")
			getAndClearTextFromInputField(reserveIPDialog, "Description")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		reserveIPDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("IP Address", "", 42, nil, nil).
			AddInputField("Name", "", 42, nil, nil).
			AddInputField("Description", "", 42, nil, nil).
			AddButton("Save", func() {
				address := getAndClearTextFromInputField(reserveIPDialog, "IP Address")
				displayName := getAndClearTextFromInputField(reserveIPDialog, "Name")
				description := getAndClearTextFromInputField(reserveIPDialog, "Description")
				ReserveIP(address, displayName, description)

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		reserveIPDialog.SetBorder(true).SetTitle("Reserve IP")
		wireDialogFormKeys(reserveIPDialog, cancelDialog)
		reserveIPFlex := createDialogPage(reserveIPDialog, width, height)
		pages.AddPage(reserveIPPage, reserveIPFlex, true, false)
	}

	{
		height := 9
		width := 62
		cancelDialog := func() {
			getAndClearTextFromInputField(updateIPReservationDialog, "Name")
			getAndClearTextFromInputField(updateIPReservationDialog, "Description")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		updateIPReservationDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("Name", "", 42, nil, nil).
			AddInputField("Description", "", 42, nil, nil).
			AddButton("Save", func() {
				displayName := getAndClearTextFromInputField(updateIPReservationDialog, "Name")
				description := getAndClearTextFromInputField(updateIPReservationDialog, "Description")
				UpdateIPReservation(displayName, description)

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		updateIPReservationDialog.SetBorder(true).SetTitle("Update IP Reservation")
		wireDialogFormKeys(updateIPReservationDialog, cancelDialog)
		updateIPReservationFlex := createDialogPage(updateIPReservationDialog, width, height)
		pages.AddPage(updateIPReservationPage, updateIPReservationFlex, true, false)
	}

	{
		unreserveIPDialog = tview.NewModal().
			SetText("Unreserve this IP address?").
			AddButtons([]string{"Yes", "No"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				switch buttonLabel {
				case "Yes":
					UnreserveIP()
					fallthrough
				case "No":
					fallthrough
				default:
					unreserveIPDialog.SetText("")
					pages.SwitchToPage(mainPage)
					app.SetFocus(navigationPanel)
				}
			})
		pages.AddPage(unreserveIPPage, unreserveIPDialog, true, false)
	}

	{
		deallocateNetworkDialog = tview.NewModal().
			SetText("Deallocate this network and remove its subnets?").
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
			SetText("Delete this top-level network and all of its subnets?").
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
	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		// Keep status panel height in sync with terminal resizes.
		resizeStatusLine()
		return false
	})
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlC:
			pages.ShowPage(quitPage)
			quitDialog.SetFocus(1)
			app.SetFocus(quitDialog)
			return nil
		case tcell.KeyCtrlS:
			save()
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

}

func main() {
	setupApp()
	if err := app.Run(); err != nil {
		panic(err)
	}
}

func load() {
	networks := &MenuStatic{
		MenuFolder: &MenuFolder{
			ID: "Networks",
		},
		Index:       0,
		Description: "Manage your address space here.\n\nUse Enter or double-click to open items.\nUse Backspace to go up one level.",
	}
	menuItems.MustAdd(networks)

	currentDir, err := os.Getwd()
	if err != nil {
		panic("Failed to get current directory: " + err.Error())
	}

	dataDir := filepath.Join(currentDir, dataDirName)
	networkDir := filepath.Join(dataDir, networksDirName)
	ipsDir := filepath.Join(dataDir, ipsDirName)

	networkFiles, err := os.ReadDir(networkDir)
	if err != nil {
		if !os.IsNotExist(err) {
			panic("Failed to read " + networkDir + " directory: " + err.Error())
		}
		if err := os.MkdirAll(networkDir, 0755); err != nil {
			panic("Failed to create " + networkDir + " directory: " + err.Error())
		}
	}
	for _, networkFile := range networkFiles {
		if networkFile.IsDir() {
			continue
		}

		bytes, err := os.ReadFile(filepath.Join(networkDir, networkFile.Name()))
		if err != nil {
			panic("Failed to read " + networkFile.Name() + " file: " + err.Error())
		}

		network := &Network{}
		if err := yaml.Unmarshal(bytes, network); err != nil {
			panic("Failed to unmarshal " + networkFile.Name() + " file: " + err.Error())
		}

		menuItems[network.GetPath()] = network
	}

	ipFiles, err := os.ReadDir(ipsDir)
	if err != nil {
		if !os.IsNotExist(err) {
			panic("Failed to read " + ipsDir + " directory: " + err.Error())
		}
		if err := os.MkdirAll(ipsDir, 0755); err != nil {
			panic("Failed to create " + ipsDir + " directory: " + err.Error())
		}
	}
	for _, ipFile := range ipFiles {
		if ipFile.IsDir() {
			continue
		}

		bytes, err := os.ReadFile(filepath.Join(ipsDir, ipFile.Name()))
		if err != nil {
			panic("Failed to read " + ipFile.Name() + " file: " + err.Error())
		}

		ip := &IP{}
		if err := yaml.Unmarshal(bytes, ip); err != nil {
			panic("Failed to unmarshal " + ipFile.Name() + " file: " + err.Error())
		}

		menuItems[ip.GetPath()] = ip
	}

	for _, menuItem := range menuItems {
		if err := menuItem.Validate(); err != nil {
			panic("Failed to load " + menuItem.GetPath() + ": " + err.Error())
		}
	}

	reloadMenu(nil)
}

func save() {
	currentDir, err := os.Getwd()
	if err != nil {
		panic("Failed to get current directory: " + err.Error())
	}

	dataDir := filepath.Join(currentDir, dataDirName)
	dataTmpDir := dataDir + ".tmp"
	dataOldDir := dataDir + ".old"

	networksTmpDir := filepath.Join(dataTmpDir, networksDirName)
	ipsTmpDir := filepath.Join(dataTmpDir, ipsDirName)
	if err := os.RemoveAll(dataTmpDir); err != nil {
		panic("Failed to remove " + dataTmpDir + " directory: " + err.Error())
	}
	if err := os.MkdirAll(networksTmpDir, 0755); err != nil {
		panic("Failed to create " + networksTmpDir + " directory: " + err.Error())
	}
	if err := os.MkdirAll(ipsTmpDir, 0755); err != nil {
		panic("Failed to create " + ipsTmpDir + " directory: " + err.Error())
	}

	for _, menuItem := range menuItems {
		switch m := menuItem.(type) {
		case *MenuStatic:
			// This is not serializable
		case *Network:
			id, err := CIDRToIdentifier(m.ID)
			if err != nil {
				panic("Failed to convert " + m.ID + " to identifier: " + err.Error())
			}

			fileName := filepath.Join(networksTmpDir, id+".yaml")
			bytes, err := yaml.Marshal(menuItem)
			if err != nil {
				panic("Failed to marshal " + menuItem.GetPath() + " to yaml: " + err.Error())
			}
			if err := os.WriteFile(fileName, bytes, 0644); err != nil {
				panic("Failed to write " + fileName + " file: " + err.Error())
			}
		case *IP:
			id, err := IPToIdentifier(m.ID)
			if err != nil {
				panic("Failed to convert " + m.ID + " to identifier: " + err.Error())
			}

			fileName := filepath.Join(ipsTmpDir, id+".yaml")
			bytes, err := yaml.Marshal(menuItem)
			if err != nil {
				panic("Failed to marshal " + menuItem.GetPath() + " to yaml: " + err.Error())
			}
			if err := os.WriteFile(fileName, bytes, 0644); err != nil {
				panic("Failed to write " + fileName + " file: " + err.Error())
			}
		default:
		}
	}

	if err := os.RemoveAll(dataOldDir); err != nil {
		panic("Failed to remove " + dataOldDir + " directory: " + err.Error())
	}
	if _, err := os.Stat(dataDir); err == nil {
		if err := os.Rename(dataDir, dataOldDir); err != nil {
			panic("Failed to rename " + dataDir + " to " + dataOldDir + " directory: " + err.Error())
		}
	} else if !os.IsNotExist(err) {
		panic("Failed to stat " + dataDir + " directory: " + err.Error())
	}
	if err := os.Rename(dataTmpDir, dataDir); err != nil {
		// best-effort rollback
		if _, rollbackErr := os.Stat(dataOldDir); rollbackErr == nil {
			_ = os.Rename(dataOldDir, dataDir)
		}
		panic("Failed to rename " + dataTmpDir + " to " + dataDir + " directory: " + err.Error())
	}
	if err := os.RemoveAll(dataOldDir); err != nil {
		panic("Failed to remove " + dataOldDir + " directory: " + err.Error())
	}

	networksOrder := []string{}
	networksCIDR := map[string]string{}
	networksDisplayName := map[string]string{}
	networksTitle := map[string]string{}
	networksMode := map[string]string{}
	networksDescription := map[string]string{}
	networksDepth := map[string]int{}
	networksHeading := map[string]string{}
	networksAnchor := map[string]string{}
	networksDataIndex := map[string][]string{}
	networksData := map[string]map[string]string{}
	networksHasReservedIPs := map[string]bool{}
	networksReservedIPs := map[string][]map[string]string{}
	summaryRows := []map[string]string{}
	buildTreePrefix := func(ancestorHasNext []bool, isLast bool) string {
		stringWriter := new(strings.Builder)
		for _, hasNext := range ancestorHasNext {
			if hasNext {
				stringWriter.WriteString("│   ")
			} else {
				stringWriter.WriteString("    ")
			}
		}
		if isLast {
			stringWriter.WriteString("└── ")
		} else {
			stringWriter.WriteString("├── ")
		}
		return stringWriter.String()
	}

	var recursivelyPopulateNetworksData func(n *Network, depth int, ancestorHasNext []bool, isLast bool)
	recursivelyPopulateNetworksData = func(n *Network, depth int, ancestorHasNext []bool, isLast bool) {
		index, data, err := n.RenderDetailsMap()
		if err != nil {
			panic("Failed to render details map for " + n.GetPath() + ": " + err.Error())
		}

		path := n.GetPath()
		cidrIdentifier, err := CIDRToIdentifier(n.ID)
		if err != nil {
			panic("Failed to build anchor for " + n.ID + ": " + err.Error())
		}
		networksOrder = append(networksOrder, path)
		networksCIDR[path] = n.ID
		networksDisplayName[path] = n.DisplayName
		networksDescription[path] = n.Description
		networksAnchor[path] = "network-" + cidrIdentifier
		networksDataIndex[path] = index
		networksData[path] = data
		networksDepth[path] = depth
		headingLevel := 3 + depth
		if headingLevel > 6 {
			headingLevel = 6
		}
		networksHeading[path] = strings.Repeat("#", headingLevel)
		networkTreePrefix := ""
		if depth > 0 {
			networkTreePrefix = buildTreePrefix(ancestorHasNext, isLast)
		}

		switch n.AllocationMode {
		case AllocationModeUnallocated:
			networksMode[path] = "Unallocated"
			networksTitle[path] = fmt.Sprintf("`%s` _(Unallocated)_", n.ID)
		case AllocationModeSubnets:
			networksMode[path] = "Subnets"
			if n.DisplayName != "" {
				networksTitle[path] = fmt.Sprintf("`%s` -- %s", n.ID, n.DisplayName)
			} else {
				networksTitle[path] = fmt.Sprintf("`%s`", n.ID)
			}
		case AllocationModeHosts:
			networksMode[path] = "Hosts"
			if n.DisplayName != "" {
				networksTitle[path] = fmt.Sprintf("`%s` -- %s", n.ID, n.DisplayName)
			} else {
				networksTitle[path] = fmt.Sprintf("`%s`", n.ID)
			}
		}

		reserved := []map[string]string{}
		for _, child := range menuItems.GetChilds(n) {
			ip, ok := child.(*IP)
			if !ok {
				continue
			}
			reserved = append(reserved, map[string]string{
				"Address":     ip.ID,
				"DisplayName": ip.DisplayName,
				"Description": ip.Description,
			})
		}
		networksHasReservedIPs[path] = len(reserved) > 0
		networksReservedIPs[path] = reserved
		networkCell := fmt.Sprintf("[`%s`](#%s)", n.ID, networksAnchor[path])
		if networkTreePrefix != "" {
			networkCell = fmt.Sprintf("`%s` %s", networkTreePrefix, networkCell)
		}
		summaryRows = append(summaryRows, map[string]string{
			"Network":     networkCell,
			"Name":        markdownInline(defaultIfEmpty(n.DisplayName, "-")),
			"Allocation":  markdownInline(defaultIfEmpty(networksMode[path], "-")),
			"Description": markdownInline(defaultIfEmpty(n.Description, "-")),
		})

		if len(reserved) > 0 {
			ipAncestorHasNext := append(append([]bool{}, ancestorHasNext...), !isLast)
			for i, ip := range reserved {
				ipTreePrefix := buildTreePrefix(ipAncestorHasNext, i == len(reserved)-1)
				summaryRows = append(summaryRows, map[string]string{
					"Network":     fmt.Sprintf("`%s%s`", ipTreePrefix, ip["Address"]),
					"Name":        markdownInline(defaultIfEmpty(ip["DisplayName"], "-")),
					"Allocation":  "Reserved IP",
					"Description": markdownInline(defaultIfEmpty(ip["Description"], "-")),
				})
			}
		}

		childs := menuItems.GetChilds(n)
		networkChildren := make([]*Network, 0, len(childs))
		for _, child := range childs {
			nn, ok := child.(*Network)
			if !ok {
				continue
			}
			networkChildren = append(networkChildren, nn)
		}
		for i, childNetwork := range networkChildren {
			childAncestors := append(append([]bool{}, ancestorHasNext...), !isLast)
			recursivelyPopulateNetworksData(childNetwork, depth+1, childAncestors, i == len(networkChildren)-1)
		}
	}

	networksMenuItem := menuItems.GetByParentAndID(nil, "Networks")
	topLevelNetworks := make([]*Network, 0, len(menuItems.GetChilds(networksMenuItem)))
	for _, menuItem := range menuItems.GetChilds(networksMenuItem) {
		n, ok := menuItem.(*Network)
		if !ok {
			panic("Failed to cast " + menuItem.GetPath() + " to network")
		}
		topLevelNetworks = append(topLevelNetworks, n)
	}
	for i, topLevelNetwork := range topLevelNetworks {
		recursivelyPopulateNetworksData(topLevelNetwork, 0, []bool{}, i == len(topLevelNetworks)-1)
	}

	template := template.Must(template.New(markdownFileName).Parse(markdownTmpl))
	input := map[string]interface{}{
		"NetworksOrder":          networksOrder,
		"NetworksCIDR":           networksCIDR,
		"NetworksDisplayName":    networksDisplayName,
		"NetworksTitle":          networksTitle,
		"NetworksMode":           networksMode,
		"NetworksDescription":    networksDescription,
		"NetworksDepth":          networksDepth,
		"NetworksHeading":        networksHeading,
		"NetworksAnchor":         networksAnchor,
		"NetworksDataIndex":      networksDataIndex,
		"NetworksData":           networksData,
		"NetworksHasReservedIPs": networksHasReservedIPs,
		"NetworksReservedIPs":    networksReservedIPs,
		"SummaryRows":            summaryRows,
	}

	mdFile, err := os.Create(filepath.Join(currentDir, markdownFileName))
	if err != nil {
		panic("Failed to create " + markdownFileName + " file: " + err.Error())
	}
	defer mdFile.Close()

	if err := template.Execute(mdFile, input); err != nil {
		panic("Failed to execute template for " + markdownFileName + " file: " + err.Error())
	}

	statusLine.Clear()
	statusLine.SetText("Saved to .ez-ipam/ and EZ-IPAM.md")
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

func resizeStatusLine() {
	if statusLine == nil || detailsFlex == nil {
		return
	}

	_, _, innerWidth, _ := statusLine.GetInnerRect()
	if innerWidth <= 0 {
		detailsFlex.ResizeItem(statusLine, 3, 0)
		return
	}

	text := statusLine.GetText(false)
	requiredLines := wrappedLineCount(text, innerWidth)
	height := requiredLines + 2 // account for top and bottom border
	if height < 3 {
		height = 3
	}

	detailsFlex.ResizeItem(statusLine, height, 0)
}

func wrappedLineCount(text string, width int) int {
	if width <= 0 {
		return 1
	}

	totalLines := 0
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			totalLines++
			continue
		}
		wrapped := tview.WordWrap(line, width)
		if len(wrapped) == 0 {
			totalLines++
			continue
		}
		totalLines += len(wrapped)
	}

	if totalLines < 1 {
		return 1
	}

	return totalLines
}

func markdownInline(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return value
}

func defaultIfEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
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
