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
	addVLANPage                    = "*add_vlan*"
	updateVLANPage                 = "*update_vlan*"
	deleteVLANPage                 = "*delete_vlan*"
	addSSIDPage                    = "*add_ssid*"
	updateSSIDPage                 = "*update_ssid*"
	deleteSSIDPage                 = "*delete_ssid*"
	addZonePage                    = "*add_zone*"
	updateZonePage                 = "*update_zone*"
	deleteZonePage                 = "*delete_zone*"
	addEquipmentPage               = "*add_equipment*"
	updateEquipmentPage            = "*update_equipment*"
	deleteEquipmentPage            = "*delete_equipment*"
	addPortPage                    = "*add_port*"
	updatePortPage                 = "*update_port*"
	connectPortPage                = "*connect_port*"
	disconnectPortPage             = "*disconnect_port*"
	deletePortPage                 = "*delete_port*"
	quitPage                       = "*quit*"

	dataDirName      = ".ez-ipam"
	networksDirName  = "networks"
	ipsDirName       = "ips"
	vlansDirName     = "vlans"
	ssidsDirName     = "ssids"
	zonesDirName     = "zones"
	equipmentDirName = "equipment"
	portsDirName     = "ports"
	markdownFileName = "EZ-IPAM.md"
	formFieldWidth   = 42
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
	addVLANDialog                    *tview.Form
	updateVLANDialog                 *tview.Form
	deleteVLANDialog                 *tview.Modal
	addSSIDDialog                    *tview.Form
	updateSSIDDialog                 *tview.Form
	deleteSSIDDialog                 *tview.Modal
	addZoneDialog                    *tview.Form
	updateZoneDialog                 *tview.Form
	deleteZoneDialog                 *tview.Modal
	addEquipmentDialog               *tview.Form
	updateEquipmentDialog            *tview.Form
	deleteEquipmentDialog            *tview.Modal
	addPortDialog                    *tview.Form
	updatePortDialog                 *tview.Form
	connectPortDialog                *tview.Form
	disconnectPortDialog             *tview.Modal
	deletePortDialog                 *tview.Modal
	quitDialog                       *tview.Modal

	summarizeCandidates  []*Network
	summarizeFromIndex   int
	summarizeToIndex     int
	connectPortSelection int
	portConnectTargets   []string
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
	connectPortSelection = 0
	portConnectTargets = nil
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
			AddInputField("CIDR", "", formFieldWidth, nil, nil).
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
			AddInputField("New Prefix Length", "", formFieldWidth, nil, nil).
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
		height := 17
		width := 64
		cancelDialog := func() {
			getAndClearTextFromInputField(allocateNetworkSubnetsModeDialog, "Name")
			getAndClearTextFromTextArea(allocateNetworkSubnetsModeDialog, "Description")
			getAndClearTextFromInputField(allocateNetworkSubnetsModeDialog, "VLAN ID")
			getAndClearTextFromInputField(allocateNetworkSubnetsModeDialog, "Child Prefix Len")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		allocateNetworkSubnetsModeDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("Name", "", formFieldWidth, nil, nil).
			AddTextArea("Description", "", formFieldWidth, 3, 0, nil).
			AddInputField("VLAN ID", "", formFieldWidth, nil, nil).
			AddInputField("Child Prefix Len", "", formFieldWidth, nil, nil).
			AddButton("Save", func() {
				displayName := getAndClearTextFromInputField(allocateNetworkSubnetsModeDialog, "Name")
				description := getAndClearTextFromTextArea(allocateNetworkSubnetsModeDialog, "Description")
				vlanText := getAndClearTextFromInputField(allocateNetworkSubnetsModeDialog, "VLAN ID")
				vlanID, err := parseOptionalVLANID(vlanText)
				if err != nil {
					statusLine.Clear()
					statusLine.SetText("Invalid VLAN ID: " + err.Error())
					return
				}
				subnetsPrefix := getAndClearTextFromInputField(allocateNetworkSubnetsModeDialog, "Child Prefix Len")
				subnetsPrefix = strings.TrimLeft(subnetsPrefix, "/")
				subnetsPrefixInt, err := strconv.Atoi(subnetsPrefix)
				if err != nil {
					statusLine.Clear()
					statusLine.SetText("Invalid subnet prefix length: " + err.Error())
					return
				}
				AllocateNetworkInSubnetsMode(displayName, description, subnetsPrefixInt, vlanID)

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		allocateNetworkSubnetsModeDialog.SetBorder(true).SetTitle("Allocate as Subnet Container")
		wireDialogFormKeys(allocateNetworkSubnetsModeDialog, cancelDialog)
		allocateNetworkSubnetsModeFlex := createDialogPage(allocateNetworkSubnetsModeDialog, width, height)
		pages.AddPage(allocateNetworkSubnetsModePage, allocateNetworkSubnetsModeFlex, true, false)
	}

	{
		height := 13
		width := 62
		cancelDialog := func() {
			getAndClearTextFromInputField(allocateNetworkHostsModeDialog, "Name")
			getAndClearTextFromTextArea(allocateNetworkHostsModeDialog, "Description")
			getAndClearTextFromInputField(allocateNetworkHostsModeDialog, "VLAN ID")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		allocateNetworkHostsModeDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("Name", "", formFieldWidth, nil, nil).
			AddTextArea("Description", "", formFieldWidth, 3, 0, nil).
			AddInputField("VLAN ID", "", formFieldWidth, nil, nil).
			AddButton("Save", func() {
				displayName := getAndClearTextFromInputField(allocateNetworkHostsModeDialog, "Name")
				description := getAndClearTextFromTextArea(allocateNetworkHostsModeDialog, "Description")
				vlanText := getAndClearTextFromInputField(allocateNetworkHostsModeDialog, "VLAN ID")
				vlanID, err := parseOptionalVLANID(vlanText)
				if err != nil {
					statusLine.Clear()
					statusLine.SetText("Invalid VLAN ID: " + err.Error())
					return
				}

				AllocateNetworkInHostsMode(displayName, description, vlanID)

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		allocateNetworkHostsModeDialog.SetBorder(true).SetTitle("Allocate as Host Pool")
		wireDialogFormKeys(allocateNetworkHostsModeDialog, cancelDialog)
		allocateNetworkHostsModeFlex := createDialogPage(allocateNetworkHostsModeDialog, width, height)
		pages.AddPage(allocateNetworkHostsModePage, allocateNetworkHostsModeFlex, true, false)
	}

	{
		height := 13
		width := 62
		cancelDialog := func() {
			getAndClearTextFromInputField(updateNetworkAllocationDialog, "Name")
			getAndClearTextFromTextArea(updateNetworkAllocationDialog, "Description")
			getAndClearTextFromInputField(updateNetworkAllocationDialog, "VLAN ID")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		updateNetworkAllocationDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("Name", "", formFieldWidth, nil, nil).
			AddTextArea("Description", "", formFieldWidth, 3, 0, nil).
			AddInputField("VLAN ID", "", formFieldWidth, nil, nil).
			AddButton("Save", func() {
				displayName := getAndClearTextFromInputField(updateNetworkAllocationDialog, "Name")
				description := getAndClearTextFromTextArea(updateNetworkAllocationDialog, "Description")
				vlanText := getAndClearTextFromInputField(updateNetworkAllocationDialog, "VLAN ID")
				vlanID, err := parseOptionalVLANID(vlanText)
				if err != nil {
					statusLine.Clear()
					statusLine.SetText("Invalid VLAN ID: " + err.Error())
					return
				}
				UpdateNetworkAllocation(displayName, description, vlanID)

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		updateNetworkAllocationDialog.SetBorder(true).SetTitle("Update Metadata")
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
			AddInputField("IP Address", "", formFieldWidth, nil, nil).
			AddInputField("Name", "", formFieldWidth, nil, nil).
			AddInputField("Description", "", formFieldWidth, nil, nil).
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
			AddInputField("Name", "", formFieldWidth, nil, nil).
			AddInputField("Description", "", formFieldWidth, nil, nil).
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
		height := 13
		width := 62
		cancelDialog := func() {
			getAndClearTextFromInputField(addVLANDialog, "VLAN ID")
			getAndClearTextFromInputField(addVLANDialog, "Name")
			getAndClearTextFromTextArea(addVLANDialog, "Description")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		addVLANDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("VLAN ID", "", formFieldWidth, nil, nil).
			AddInputField("Name", "", formFieldWidth, nil, nil).
			AddTextArea("Description", "", formFieldWidth, 4, 0, nil).
			AddButton("Save", func() {
				vlanID := getAndClearTextFromInputField(addVLANDialog, "VLAN ID")
				name := getAndClearTextFromInputField(addVLANDialog, "Name")
				description := getAndClearTextFromTextArea(addVLANDialog, "Description")
				AddVLAN(vlanID, name, description)

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		addVLANDialog.SetBorder(true).SetTitle("Add VLAN")
		wireDialogFormKeys(addVLANDialog, cancelDialog)
		addVLANFlex := createDialogPage(addVLANDialog, width, height)
		pages.AddPage(addVLANPage, addVLANFlex, true, false)
	}

	{
		height := 13
		width := 62
		cancelDialog := func() {
			getAndClearTextFromInputField(updateVLANDialog, "Name")
			getAndClearTextFromTextArea(updateVLANDialog, "Description")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		updateVLANDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("Name", "", formFieldWidth, nil, nil).
			AddTextArea("Description", "", formFieldWidth, 4, 0, nil).
			AddButton("Save", func() {
				name := getAndClearTextFromInputField(updateVLANDialog, "Name")
				description := getAndClearTextFromTextArea(updateVLANDialog, "Description")
				UpdateVLAN(name, description)

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		updateVLANDialog.SetBorder(true).SetTitle("Update VLAN")
		wireDialogFormKeys(updateVLANDialog, cancelDialog)
		updateVLANFlex := createDialogPage(updateVLANDialog, width, height)
		pages.AddPage(updateVLANPage, updateVLANFlex, true, false)
	}

	{
		height := 13
		width := 62
		cancelDialog := func() {
			getAndClearTextFromInputField(addSSIDDialog, "SSID")
			getAndClearTextFromTextArea(addSSIDDialog, "Description")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		addSSIDDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("SSID", "", formFieldWidth, nil, nil).
			AddTextArea("Description", "", formFieldWidth, 4, 0, nil).
			AddButton("Save", func() {
				ssidID := getAndClearTextFromInputField(addSSIDDialog, "SSID")
				description := getAndClearTextFromTextArea(addSSIDDialog, "Description")
				AddSSID(ssidID, description)

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		addSSIDDialog.SetBorder(true).SetTitle("Add WiFi SSID")
		wireDialogFormKeys(addSSIDDialog, cancelDialog)
		addSSIDFlex := createDialogPage(addSSIDDialog, width, height)
		pages.AddPage(addSSIDPage, addSSIDFlex, true, false)
	}

	{
		height := 11
		width := 62
		cancelDialog := func() {
			getAndClearTextFromTextArea(updateSSIDDialog, "Description")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		updateSSIDDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddTextArea("Description", "", formFieldWidth, 4, 0, nil).
			AddButton("Save", func() {
				description := getAndClearTextFromTextArea(updateSSIDDialog, "Description")
				UpdateSSID(description)

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		updateSSIDDialog.SetBorder(true).SetTitle("Update WiFi SSID")
		wireDialogFormKeys(updateSSIDDialog, cancelDialog)
		updateSSIDFlex := createDialogPage(updateSSIDDialog, width, height)
		pages.AddPage(updateSSIDPage, updateSSIDFlex, true, false)
	}

	{
		height := 13
		width := 62
		cancelDialog := func() {
			getAndClearTextFromInputField(addZoneDialog, "Name")
			getAndClearTextFromInputField(addZoneDialog, "Description")
			getAndClearTextFromInputField(addZoneDialog, "VLAN IDs")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		addZoneDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("Name", "", formFieldWidth, nil, nil).
			AddInputField("Description", "", formFieldWidth, nil, nil).
			AddInputField("VLAN IDs", "", formFieldWidth, nil, nil).
			AddButton("Save", func() {
				name := getAndClearTextFromInputField(addZoneDialog, "Name")
				description := getAndClearTextFromInputField(addZoneDialog, "Description")
				vlanIDs := getAndClearTextFromInputField(addZoneDialog, "VLAN IDs")
				AddZone(name, description, vlanIDs)
				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		addZoneDialog.SetBorder(true).SetTitle("Add Zone")
		wireDialogFormKeys(addZoneDialog, cancelDialog)
		addZoneFlex := createDialogPage(addZoneDialog, width, height)
		pages.AddPage(addZonePage, addZoneFlex, true, false)
	}

	{
		height := 13
		width := 62
		cancelDialog := func() {
			getAndClearTextFromInputField(updateZoneDialog, "Name")
			getAndClearTextFromInputField(updateZoneDialog, "Description")
			getAndClearTextFromInputField(updateZoneDialog, "VLAN IDs")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		updateZoneDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("Name", "", formFieldWidth, nil, nil).
			AddInputField("Description", "", formFieldWidth, nil, nil).
			AddInputField("VLAN IDs", "", formFieldWidth, nil, nil).
			AddButton("Save", func() {
				name := getAndClearTextFromInputField(updateZoneDialog, "Name")
				description := getAndClearTextFromInputField(updateZoneDialog, "Description")
				vlanIDs := getAndClearTextFromInputField(updateZoneDialog, "VLAN IDs")
				UpdateZone(name, description, vlanIDs)
				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		updateZoneDialog.SetBorder(true).SetTitle("Update Zone")
		wireDialogFormKeys(updateZoneDialog, cancelDialog)
		updateZoneFlex := createDialogPage(updateZoneDialog, width, height)
		pages.AddPage(updateZonePage, updateZoneFlex, true, false)
	}

	{
		height := 13
		width := 62
		cancelDialog := func() {
			getAndClearTextFromInputField(addEquipmentDialog, "Name")
			getAndClearTextFromInputField(addEquipmentDialog, "Model")
			getAndClearTextFromInputField(addEquipmentDialog, "Description")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		addEquipmentDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("Name", "", formFieldWidth, nil, nil).
			AddInputField("Model", "", formFieldWidth, nil, nil).
			AddInputField("Description", "", formFieldWidth, nil, nil).
			AddButton("Save", func() {
				name := getAndClearTextFromInputField(addEquipmentDialog, "Name")
				model := getAndClearTextFromInputField(addEquipmentDialog, "Model")
				description := getAndClearTextFromInputField(addEquipmentDialog, "Description")
				AddEquipment(name, model, description)
				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		addEquipmentDialog.SetBorder(true).SetTitle("Add Equipment")
		wireDialogFormKeys(addEquipmentDialog, cancelDialog)
		addEquipmentFlex := createDialogPage(addEquipmentDialog, width, height)
		pages.AddPage(addEquipmentPage, addEquipmentFlex, true, false)
	}

	{
		height := 13
		width := 62
		cancelDialog := func() {
			getAndClearTextFromInputField(updateEquipmentDialog, "Name")
			getAndClearTextFromInputField(updateEquipmentDialog, "Model")
			getAndClearTextFromInputField(updateEquipmentDialog, "Description")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		updateEquipmentDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("Name", "", formFieldWidth, nil, nil).
			AddInputField("Model", "", formFieldWidth, nil, nil).
			AddInputField("Description", "", formFieldWidth, nil, nil).
			AddButton("Save", func() {
				name := getAndClearTextFromInputField(updateEquipmentDialog, "Name")
				model := getAndClearTextFromInputField(updateEquipmentDialog, "Model")
				description := getAndClearTextFromInputField(updateEquipmentDialog, "Description")
				UpdateEquipment(name, model, description)
				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		updateEquipmentDialog.SetBorder(true).SetTitle("Update Equipment")
		wireDialogFormKeys(updateEquipmentDialog, cancelDialog)
		updateEquipmentFlex := createDialogPage(updateEquipmentDialog, width, height)
		pages.AddPage(updateEquipmentPage, updateEquipmentFlex, true, false)
	}

	{
		height := 23
		width := 66
		cancelDialog := func() {
			getAndClearTextFromInputField(addPortDialog, "Port Number")
			getAndClearTextFromInputField(addPortDialog, "Name")
			getAndClearTextFromInputField(addPortDialog, "Port Type")
			getAndClearTextFromInputField(addPortDialog, "Speed")
			getAndClearTextFromInputField(addPortDialog, "PoE")
			getAndClearTextFromInputField(addPortDialog, "LAG Group")
			getAndClearTextFromInputField(addPortDialog, "LAG Mode")
			getAndClearTextFromInputField(addPortDialog, "Native VLAN ID")
			getAndClearTextFromInputField(addPortDialog, "Tagged VLAN Mode")
			getAndClearTextFromInputField(addPortDialog, "Tagged VLAN IDs")
			getAndClearTextFromInputField(addPortDialog, "Description")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		addPortDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("Port Number", "", formFieldWidth, nil, nil).
			AddInputField("Name", "", formFieldWidth, nil, nil).
			AddInputField("Port Type", "", formFieldWidth, nil, nil).
			AddInputField("Speed", "", formFieldWidth, nil, nil).
			AddInputField("PoE", "", formFieldWidth, nil, nil).
			AddInputField("LAG Group", "", formFieldWidth, nil, nil).
			AddInputField("LAG Mode", "", formFieldWidth, nil, nil).
			AddInputField("Native VLAN ID", "", formFieldWidth, nil, nil).
			AddInputField("Tagged VLAN Mode", "", formFieldWidth, nil, nil).
			AddInputField("Tagged VLAN IDs", "", formFieldWidth, nil, nil).
			AddInputField("Description", "", formFieldWidth, nil, nil).
			AddButton("Save", func() {
				AddPort(
					getAndClearTextFromInputField(addPortDialog, "Port Number"),
					getAndClearTextFromInputField(addPortDialog, "Name"),
					getAndClearTextFromInputField(addPortDialog, "Port Type"),
					getAndClearTextFromInputField(addPortDialog, "Speed"),
					getAndClearTextFromInputField(addPortDialog, "PoE"),
					getAndClearTextFromInputField(addPortDialog, "LAG Group"),
					getAndClearTextFromInputField(addPortDialog, "LAG Mode"),
					getAndClearTextFromInputField(addPortDialog, "Native VLAN ID"),
					getAndClearTextFromInputField(addPortDialog, "Tagged VLAN Mode"),
					getAndClearTextFromInputField(addPortDialog, "Tagged VLAN IDs"),
					getAndClearTextFromInputField(addPortDialog, "Description"),
				)
				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		addPortDialog.SetBorder(true).SetTitle("Add Port")
		wireDialogFormKeys(addPortDialog, cancelDialog)
		addPortFlex := createDialogPage(addPortDialog, width, height)
		pages.AddPage(addPortPage, addPortFlex, true, false)
	}

	{
		height := 23
		width := 66
		cancelDialog := func() {
			getAndClearTextFromInputField(updatePortDialog, "Port Number")
			getAndClearTextFromInputField(updatePortDialog, "Name")
			getAndClearTextFromInputField(updatePortDialog, "Port Type")
			getAndClearTextFromInputField(updatePortDialog, "Speed")
			getAndClearTextFromInputField(updatePortDialog, "PoE")
			getAndClearTextFromInputField(updatePortDialog, "LAG Group")
			getAndClearTextFromInputField(updatePortDialog, "LAG Mode")
			getAndClearTextFromInputField(updatePortDialog, "Native VLAN ID")
			getAndClearTextFromInputField(updatePortDialog, "Tagged VLAN Mode")
			getAndClearTextFromInputField(updatePortDialog, "Tagged VLAN IDs")
			getAndClearTextFromInputField(updatePortDialog, "Description")
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		updatePortDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("Port Number", "", formFieldWidth, nil, nil).
			AddInputField("Name", "", formFieldWidth, nil, nil).
			AddInputField("Port Type", "", formFieldWidth, nil, nil).
			AddInputField("Speed", "", formFieldWidth, nil, nil).
			AddInputField("PoE", "", formFieldWidth, nil, nil).
			AddInputField("LAG Group", "", formFieldWidth, nil, nil).
			AddInputField("LAG Mode", "", formFieldWidth, nil, nil).
			AddInputField("Native VLAN ID", "", formFieldWidth, nil, nil).
			AddInputField("Tagged VLAN Mode", "", formFieldWidth, nil, nil).
			AddInputField("Tagged VLAN IDs", "", formFieldWidth, nil, nil).
			AddInputField("Description", "", formFieldWidth, nil, nil).
			AddButton("Save", func() {
				UpdatePort(
					getAndClearTextFromInputField(updatePortDialog, "Port Number"),
					getAndClearTextFromInputField(updatePortDialog, "Name"),
					getAndClearTextFromInputField(updatePortDialog, "Port Type"),
					getAndClearTextFromInputField(updatePortDialog, "Speed"),
					getAndClearTextFromInputField(updatePortDialog, "PoE"),
					getAndClearTextFromInputField(updatePortDialog, "LAG Group"),
					getAndClearTextFromInputField(updatePortDialog, "LAG Mode"),
					getAndClearTextFromInputField(updatePortDialog, "Native VLAN ID"),
					getAndClearTextFromInputField(updatePortDialog, "Tagged VLAN Mode"),
					getAndClearTextFromInputField(updatePortDialog, "Tagged VLAN IDs"),
					getAndClearTextFromInputField(updatePortDialog, "Description"),
				)
				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		updatePortDialog.SetBorder(true).SetTitle("Update Port")
		wireDialogFormKeys(updatePortDialog, cancelDialog)
		updatePortFlex := createDialogPage(updatePortDialog, width, height)
		pages.AddPage(updatePortPage, updatePortFlex, true, false)
	}

	{
		height := 9
		width := 66
		cancelDialog := func() {
			portConnectTargets = nil
			connectPortSelection = 0
			pages.SwitchToPage(mainPage)
			app.SetFocus(navigationPanel)
		}
		connectPortDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddDropDown("Target", nil, 0, func(option string, optionIndex int) {
				if optionIndex >= 0 {
					connectPortSelection = optionIndex
				}
			}).
			AddButton("Connect", func() {
				if connectPortSelection >= 0 && connectPortSelection < len(portConnectTargets) {
					ConnectPort(portConnectTargets[connectPortSelection])
				}
				portConnectTargets = nil
				connectPortSelection = 0
				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", cancelDialog)
		connectPortDialog.SetBorder(true).SetTitle("Connect Port")
		wireDialogFormKeys(connectPortDialog, cancelDialog)
		connectPortFlex := createDialogPage(connectPortDialog, width, height)
		pages.AddPage(connectPortPage, connectPortFlex, true, false)
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
		deleteVLANDialog = tview.NewModal().
			SetText("Delete this VLAN?").
			AddButtons([]string{"Yes", "No"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				switch buttonLabel {
				case "Yes":
					DeleteVLAN()
					fallthrough
				case "No":
					fallthrough
				default:
					deleteVLANDialog.SetText("")
					pages.SwitchToPage(mainPage)
					app.SetFocus(navigationPanel)
				}
			})
		pages.AddPage(deleteVLANPage, deleteVLANDialog, true, false)
	}

	{
		deleteSSIDDialog = tview.NewModal().
			SetText("Delete this WiFi SSID?").
			AddButtons([]string{"Yes", "No"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				switch buttonLabel {
				case "Yes":
					DeleteSSID()
					fallthrough
				case "No":
					fallthrough
				default:
					deleteSSIDDialog.SetText("")
					pages.SwitchToPage(mainPage)
					app.SetFocus(navigationPanel)
				}
			})
		pages.AddPage(deleteSSIDPage, deleteSSIDDialog, true, false)
	}

	{
		deleteZoneDialog = tview.NewModal().
			SetText("Delete this zone?").
			AddButtons([]string{"Yes", "No"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				switch buttonLabel {
				case "Yes":
					DeleteZone()
					fallthrough
				case "No":
					fallthrough
				default:
					deleteZoneDialog.SetText("")
					pages.SwitchToPage(mainPage)
					app.SetFocus(navigationPanel)
				}
			})
		pages.AddPage(deleteZonePage, deleteZoneDialog, true, false)
	}

	{
		deleteEquipmentDialog = tview.NewModal().
			SetText("Delete this equipment and all ports?").
			AddButtons([]string{"Yes", "No"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				switch buttonLabel {
				case "Yes":
					DeleteEquipment()
					fallthrough
				case "No":
					fallthrough
				default:
					deleteEquipmentDialog.SetText("")
					pages.SwitchToPage(mainPage)
					app.SetFocus(navigationPanel)
				}
			})
		pages.AddPage(deleteEquipmentPage, deleteEquipmentDialog, true, false)
	}

	{
		disconnectPortDialog = tview.NewModal().
			SetText("Disconnect this port?").
			AddButtons([]string{"Yes", "No"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				switch buttonLabel {
				case "Yes":
					DisconnectPort()
					fallthrough
				case "No":
					fallthrough
				default:
					disconnectPortDialog.SetText("")
					pages.SwitchToPage(mainPage)
					app.SetFocus(navigationPanel)
				}
			})
		pages.AddPage(disconnectPortPage, disconnectPortDialog, true, false)
	}

	{
		deletePortDialog = tview.NewModal().
			SetText("Delete this port?").
			AddButtons([]string{"Yes", "No"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				switch buttonLabel {
				case "Yes":
					DeletePort()
					fallthrough
				case "No":
					fallthrough
				default:
					deletePortDialog.SetText("")
					pages.SwitchToPage(mainPage)
					app.SetFocus(navigationPanel)
				}
			})
		pages.AddPage(deletePortPage, deletePortDialog, true, false)
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
	zones := &MenuStatic{
		MenuFolder: &MenuFolder{
			ID: "Zones",
		},
		Index:       1,
		Description: "Document network security zones and associated VLANs.",
	}
	menuItems.MustAdd(zones)
	vlans := &MenuStatic{
		MenuFolder: &MenuFolder{
			ID: "VLANs",
		},
		Index:       2,
		Description: "Manage VLAN IDs and their metadata here.",
	}
	menuItems.MustAdd(vlans)
	ssids := &MenuStatic{
		MenuFolder: &MenuFolder{
			ID: "WiFi SSIDs",
		},
		Index:       3,
		Description: "Manage WiFi SSIDs and their metadata here.",
	}
	menuItems.MustAdd(ssids)
	equipment := &MenuStatic{
		MenuFolder: &MenuFolder{
			ID: "Equipment",
		},
		Index:       4,
		Description: "Track network equipment, ports, VLAN profiles, and links.",
	}
	menuItems.MustAdd(equipment)

	currentDir, err := os.Getwd()
	if err != nil {
		panic("Failed to get current directory: " + err.Error())
	}

	dataDir := filepath.Join(currentDir, dataDirName)
	networkDir := filepath.Join(dataDir, networksDirName)
	ipsDir := filepath.Join(dataDir, ipsDirName)
	vlansDir := filepath.Join(dataDir, vlansDirName)
	ssidsDir := filepath.Join(dataDir, ssidsDirName)
	zonesDir := filepath.Join(dataDir, zonesDirName)
	equipmentDir := filepath.Join(dataDir, equipmentDirName)
	portsDir := filepath.Join(dataDir, portsDirName)

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
	vlanFiles, err := os.ReadDir(vlansDir)
	if err != nil {
		if !os.IsNotExist(err) {
			panic("Failed to read " + vlansDir + " directory: " + err.Error())
		}
		if err := os.MkdirAll(vlansDir, 0755); err != nil {
			panic("Failed to create " + vlansDir + " directory: " + err.Error())
		}
	}
	for _, vlanFile := range vlanFiles {
		if vlanFile.IsDir() {
			continue
		}

		bytes, err := os.ReadFile(filepath.Join(vlansDir, vlanFile.Name()))
		if err != nil {
			panic("Failed to read " + vlanFile.Name() + " file: " + err.Error())
		}

		vlan := &VLAN{}
		if err := yaml.Unmarshal(bytes, vlan); err != nil {
			panic("Failed to unmarshal " + vlanFile.Name() + " file: " + err.Error())
		}

		menuItems[vlan.GetPath()] = vlan
	}
	ssidFiles, err := os.ReadDir(ssidsDir)
	if err != nil {
		if !os.IsNotExist(err) {
			panic("Failed to read " + ssidsDir + " directory: " + err.Error())
		}
		if err := os.MkdirAll(ssidsDir, 0755); err != nil {
			panic("Failed to create " + ssidsDir + " directory: " + err.Error())
		}
	}
	for _, ssidFile := range ssidFiles {
		if ssidFile.IsDir() {
			continue
		}

		bytes, err := os.ReadFile(filepath.Join(ssidsDir, ssidFile.Name()))
		if err != nil {
			panic("Failed to read " + ssidFile.Name() + " file: " + err.Error())
		}

		ssid := &SSID{}
		if err := yaml.Unmarshal(bytes, ssid); err != nil {
			panic("Failed to unmarshal " + ssidFile.Name() + " file: " + err.Error())
		}

		menuItems[ssid.GetPath()] = ssid
	}
	zoneFiles, err := os.ReadDir(zonesDir)
	if err != nil {
		if !os.IsNotExist(err) {
			panic("Failed to read " + zonesDir + " directory: " + err.Error())
		}
		if err := os.MkdirAll(zonesDir, 0755); err != nil {
			panic("Failed to create " + zonesDir + " directory: " + err.Error())
		}
	}
	for _, zoneFile := range zoneFiles {
		if zoneFile.IsDir() {
			continue
		}

		bytes, err := os.ReadFile(filepath.Join(zonesDir, zoneFile.Name()))
		if err != nil {
			panic("Failed to read " + zoneFile.Name() + " file: " + err.Error())
		}

		zone := &Zone{}
		if err := yaml.Unmarshal(bytes, zone); err != nil {
			panic("Failed to unmarshal " + zoneFile.Name() + " file: " + err.Error())
		}

		menuItems[zone.GetPath()] = zone
	}
	equipmentFiles, err := os.ReadDir(equipmentDir)
	if err != nil {
		if !os.IsNotExist(err) {
			panic("Failed to read " + equipmentDir + " directory: " + err.Error())
		}
		if err := os.MkdirAll(equipmentDir, 0755); err != nil {
			panic("Failed to create " + equipmentDir + " directory: " + err.Error())
		}
	}
	for _, equipmentFile := range equipmentFiles {
		if equipmentFile.IsDir() {
			continue
		}

		bytes, err := os.ReadFile(filepath.Join(equipmentDir, equipmentFile.Name()))
		if err != nil {
			panic("Failed to read " + equipmentFile.Name() + " file: " + err.Error())
		}

		equipment := &Equipment{}
		if err := yaml.Unmarshal(bytes, equipment); err != nil {
			panic("Failed to unmarshal " + equipmentFile.Name() + " file: " + err.Error())
		}

		menuItems[equipment.GetPath()] = equipment
	}
	portFiles, err := os.ReadDir(portsDir)
	if err != nil {
		if !os.IsNotExist(err) {
			panic("Failed to read " + portsDir + " directory: " + err.Error())
		}
		if err := os.MkdirAll(portsDir, 0755); err != nil {
			panic("Failed to create " + portsDir + " directory: " + err.Error())
		}
	}
	for _, portFile := range portFiles {
		if portFile.IsDir() {
			continue
		}

		bytes, err := os.ReadFile(filepath.Join(portsDir, portFile.Name()))
		if err != nil {
			panic("Failed to read " + portFile.Name() + " file: " + err.Error())
		}

		port := &Port{}
		if err := yaml.Unmarshal(bytes, port); err != nil {
			panic("Failed to unmarshal " + portFile.Name() + " file: " + err.Error())
		}

		menuItems[port.GetPath()] = port
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
	vlansTmpDir := filepath.Join(dataTmpDir, vlansDirName)
	ssidsTmpDir := filepath.Join(dataTmpDir, ssidsDirName)
	zonesTmpDir := filepath.Join(dataTmpDir, zonesDirName)
	equipmentTmpDir := filepath.Join(dataTmpDir, equipmentDirName)
	portsTmpDir := filepath.Join(dataTmpDir, portsDirName)
	if err := os.RemoveAll(dataTmpDir); err != nil {
		panic("Failed to remove " + dataTmpDir + " directory: " + err.Error())
	}
	if err := os.MkdirAll(networksTmpDir, 0755); err != nil {
		panic("Failed to create " + networksTmpDir + " directory: " + err.Error())
	}
	if err := os.MkdirAll(ipsTmpDir, 0755); err != nil {
		panic("Failed to create " + ipsTmpDir + " directory: " + err.Error())
	}
	if err := os.MkdirAll(vlansTmpDir, 0755); err != nil {
		panic("Failed to create " + vlansTmpDir + " directory: " + err.Error())
	}
	if err := os.MkdirAll(ssidsTmpDir, 0755); err != nil {
		panic("Failed to create " + ssidsTmpDir + " directory: " + err.Error())
	}
	if err := os.MkdirAll(zonesTmpDir, 0755); err != nil {
		panic("Failed to create " + zonesTmpDir + " directory: " + err.Error())
	}
	if err := os.MkdirAll(equipmentTmpDir, 0755); err != nil {
		panic("Failed to create " + equipmentTmpDir + " directory: " + err.Error())
	}
	if err := os.MkdirAll(portsTmpDir, 0755); err != nil {
		panic("Failed to create " + portsTmpDir + " directory: " + err.Error())
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
		case *VLAN:
			fileName := filepath.Join(vlansTmpDir, m.ID+".yaml")
			bytes, err := yaml.Marshal(menuItem)
			if err != nil {
				panic("Failed to marshal " + menuItem.GetPath() + " to yaml: " + err.Error())
			}
			if err := os.WriteFile(fileName, bytes, 0644); err != nil {
				panic("Failed to write " + fileName + " file: " + err.Error())
			}
		case *SSID:
			fileName := filepath.Join(ssidsTmpDir, m.ID+".yaml")
			bytes, err := yaml.Marshal(menuItem)
			if err != nil {
				panic("Failed to marshal " + menuItem.GetPath() + " to yaml: " + err.Error())
			}
			if err := os.WriteFile(fileName, bytes, 0644); err != nil {
				panic("Failed to write " + fileName + " file: " + err.Error())
			}
		case *Zone:
			fileName := filepath.Join(zonesTmpDir, safeFileNameSegment(m.ID)+".yaml")
			bytes, err := yaml.Marshal(menuItem)
			if err != nil {
				panic("Failed to marshal " + menuItem.GetPath() + " to yaml: " + err.Error())
			}
			if err := os.WriteFile(fileName, bytes, 0644); err != nil {
				panic("Failed to write " + fileName + " file: " + err.Error())
			}
		case *Equipment:
			fileName := filepath.Join(equipmentTmpDir, safeFileNameSegment(m.ID)+".yaml")
			bytes, err := yaml.Marshal(menuItem)
			if err != nil {
				panic("Failed to marshal " + menuItem.GetPath() + " to yaml: " + err.Error())
			}
			if err := os.WriteFile(fileName, bytes, 0644); err != nil {
				panic("Failed to write " + fileName + " file: " + err.Error())
			}
		case *Port:
			parent, ok := m.GetParent().(*Equipment)
			if !ok {
				panic("Port parent is not equipment for " + m.GetPath())
			}
			fileName := filepath.Join(portsTmpDir, safeFileNameSegment(parent.ID)+"_"+m.ID+".yaml")
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
	vlanRows := []map[string]string{}
	ssidRows := []map[string]string{}
	zoneRows := []map[string]string{}
	equipmentRows := []map[string]string{}
	summaryRows := []map[string]string{}
	buildTreePrefix := func(ancestorHasNext []bool, isLast bool) string {
		stringWriter := new(strings.Builder)
		for _, hasNext := range ancestorHasNext {
			if hasNext {
				stringWriter.WriteString("│ ")
			} else {
				stringWriter.WriteString("  ")
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
			networksMode[path] = "Subnet Container"
			if n.DisplayName != "" {
				networksTitle[path] = fmt.Sprintf("`%s` -- %s", n.ID, n.DisplayName)
			} else {
				networksTitle[path] = fmt.Sprintf("`%s`", n.ID)
			}
		case AllocationModeHosts:
			networksMode[path] = "Host Pool"
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
		networkCell := fmt.Sprintf("%s [link](#%s)", markdownCode(networkTreePrefix+n.ID), networksAnchor[path])
		vlanValue := "-"
		if n.VLANID > 0 {
			vlanValue = strconv.Itoa(n.VLANID)
			vlansMenu := menuItems.GetByParentAndID(nil, "VLANs")
			if vlansMenu != nil {
				for _, menuItem := range menuItems.GetChilds(vlansMenu) {
					vlan, ok := menuItem.(*VLAN)
					if !ok {
						continue
					}
					if vlan.ID == strconv.Itoa(n.VLANID) {
						vlanValue = fmt.Sprintf("%s (%s)", vlan.ID, vlan.DisplayName)
						break
					}
				}
			}
		}

		summaryRows = append(summaryRows, map[string]string{
			"Network":     networkCell,
			"Name":        markdownInline(defaultIfEmpty(n.DisplayName, "-")),
			"Allocation":  markdownInline(defaultIfEmpty(networksMode[path], "-")),
			"VLAN":        markdownInline(vlanValue),
			"Description": markdownInline(clampOverviewDescription(defaultIfEmpty(n.Description, "-"), 60)),
		})

		if len(reserved) > 0 {
			ipAncestorHasNext := append(append([]bool{}, ancestorHasNext...), !isLast)
			for i, ip := range reserved {
				ipTreePrefix := buildTreePrefix(ipAncestorHasNext, i == len(reserved)-1)
				ipVLAN := "-"
				if n.VLANID > 0 {
					ipVLAN = strconv.Itoa(n.VLANID)
					vlansMenu := menuItems.GetByParentAndID(nil, "VLANs")
					if vlansMenu != nil {
						for _, menuItem := range menuItems.GetChilds(vlansMenu) {
							vlan, ok := menuItem.(*VLAN)
							if !ok {
								continue
							}
							if vlan.ID == strconv.Itoa(n.VLANID) {
								ipVLAN = fmt.Sprintf("%s (%s)", vlan.ID, vlan.DisplayName)
								break
							}
						}
					}
				}
				summaryRows = append(summaryRows, map[string]string{
					"Network":     markdownCode(ipTreePrefix + ip["Address"]),
					"Name":        markdownInline(defaultIfEmpty(ip["DisplayName"], "-")),
					"Allocation":  "Reserved IP",
					"VLAN":        markdownInline(ipVLAN),
					"Description": markdownInline(clampOverviewDescription(defaultIfEmpty(ip["Description"], "-"), 60)),
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
	vlansMenuItem := menuItems.GetByParentAndID(nil, "VLANs")
	for _, menuItem := range menuItems.GetChilds(vlansMenuItem) {
		vlan, ok := menuItem.(*VLAN)
		if !ok {
			continue
		}
		vlanRows = append(vlanRows, map[string]string{
			"ID":          markdownCode(vlan.ID),
			"Name":        markdownCode(vlan.DisplayName),
			"Description": markdownTableCell(defaultIfEmpty(vlan.Description, "-")),
		})
	}
	ssidsMenuItem := menuItems.GetByParentAndID(nil, "WiFi SSIDs")
	for _, menuItem := range menuItems.GetChilds(ssidsMenuItem) {
		ssid, ok := menuItem.(*SSID)
		if !ok {
			continue
		}
		ssidRows = append(ssidRows, map[string]string{
			"ID":          markdownCode(ssid.ID),
			"Description": markdownTableCell(defaultIfEmpty(ssid.Description, "-")),
		})
	}
	zonesMenuItem := menuItems.GetByParentAndID(nil, "Zones")
	for _, menuItem := range menuItems.GetChilds(zonesMenuItem) {
		zone, ok := menuItem.(*Zone)
		if !ok {
			continue
		}
		vlanLabels := make([]string, 0, len(zone.VLANIDs))
		for _, vlanID := range zone.VLANIDs {
			vlanLabels = append(vlanLabels, renderVLANID(vlanID))
		}
		zoneRows = append(zoneRows, map[string]string{
			"Name":        markdownCode(zone.DisplayName),
			"VLANs":       markdownTableCell(defaultIfEmpty(strings.Join(vlanLabels, ", "), "-")),
			"Description": markdownTableCell(defaultIfEmpty(zone.Description, "-")),
		})
	}
	equipmentMenuItem := menuItems.GetByParentAndID(nil, "Equipment")
	for _, menuItem := range menuItems.GetChilds(equipmentMenuItem) {
		equipment, ok := menuItem.(*Equipment)
		if !ok {
			continue
		}
		portRows := []map[string]string{}
		for _, child := range menuItems.GetChilds(equipment) {
			port, ok := child.(*Port)
			if !ok {
				continue
			}
			portName := "-"
			if strings.TrimSpace(port.Name) != "" {
				portName = port.Name
			}
			portType := strings.Join(strings.Fields(strings.Join([]string{port.PortType, port.PoE, port.Speed}, " ")), " ")
			tagged := "-"
			switch port.TaggedVLANMode {
			case TaggedVLANModeAllowAll:
				tagged = "Allow All"
			case TaggedVLANModeBlockAll:
				tagged = "Block All"
			case TaggedVLANModeCustom:
				values := make([]string, 0, len(port.TaggedVLANIDs))
				for _, vlanID := range port.TaggedVLANIDs {
					values = append(values, renderVLANID(vlanID))
				}
				tagged = strings.Join(values, ", ")
			}
			destinationParts := []string{}
			if port.ConnectedTo != "" {
				destinationParts = append(destinationParts, renderPortLink(port.ConnectedTo))
			}
			if port.Description != "" {
				destinationParts = append(destinationParts, port.Description)
			}
			destination := "-"
			if len(destinationParts) > 0 {
				destination = strings.Join(destinationParts, " | ")
			}
			portRows = append(portRows, map[string]string{
				"Number":      markdownCode(port.ID),
				"Name":        markdownTableCell(portName),
				"Type":        markdownTableCell(portType),
				"Networks":    markdownTableCell(fmt.Sprintf("Native: %s Tagged: %s", renderVLANID(port.NativeVLANID), tagged)),
				"Destination": markdownTableCell(destination),
			})
		}
		equipmentRows = append(equipmentRows, map[string]string{
			"DisplayName":   markdownInline(equipment.DisplayName),
			"Model":         markdownInline(equipment.Model),
			"Description":   markdownInline(defaultIfEmpty(equipment.Description, "-")),
			"EquipmentPath": equipment.GetPath(),
		})
		// Keep nested slice keyed by a synthetic index path.
		key := equipment.GetPath()
		for _, row := range portRows {
			row["EquipmentPath"] = key
		}
	}
	equipmentPorts := map[string][]map[string]string{}
	for _, menuItem := range menuItems.GetChilds(equipmentMenuItem) {
		equipment, ok := menuItem.(*Equipment)
		if !ok {
			continue
		}
		rows := []map[string]string{}
		for _, child := range menuItems.GetChilds(equipment) {
			port, ok := child.(*Port)
			if !ok {
				continue
			}
			portName := "-"
			if strings.TrimSpace(port.Name) != "" {
				portName = port.Name
			}
			portType := strings.Join(strings.Fields(strings.Join([]string{port.PortType, port.PoE, port.Speed}, " ")), " ")
			tagged := "-"
			switch port.TaggedVLANMode {
			case TaggedVLANModeAllowAll:
				tagged = "Allow All"
			case TaggedVLANModeBlockAll:
				tagged = "Block All"
			case TaggedVLANModeCustom:
				values := make([]string, 0, len(port.TaggedVLANIDs))
				for _, vlanID := range port.TaggedVLANIDs {
					values = append(values, renderVLANID(vlanID))
				}
				tagged = strings.Join(values, ", ")
			}
			destinationParts := []string{}
			if port.ConnectedTo != "" {
				destinationParts = append(destinationParts, renderPortLink(port.ConnectedTo))
			}
			if port.Description != "" {
				destinationParts = append(destinationParts, port.Description)
			}
			destination := "-"
			if len(destinationParts) > 0 {
				destination = strings.Join(destinationParts, " | ")
			}
			rows = append(rows, map[string]string{
				"Number":      markdownCode(port.ID),
				"Name":        markdownTableCell(portName),
				"Type":        markdownTableCell(portType),
				"Networks":    markdownTableCell(fmt.Sprintf("Native: %s Tagged: %s", renderVLANID(port.NativeVLANID), tagged)),
				"Destination": markdownTableCell(destination),
			})
		}
		equipmentPorts[equipment.GetPath()] = rows
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
		"VLANRows":               vlanRows,
		"SSIDRows":               ssidRows,
		"ZoneRows":               zoneRows,
		"EquipmentRows":          equipmentRows,
		"EquipmentPorts":         equipmentPorts,
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

func markdownCode(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "`", "'")
	return "`" + value + "`"
}

func markdownTableCell(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", "<br>")
	return value
}

func clampOverviewDescription(value string, maxLen int) string {
	if maxLen < 1 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxLen {
		return value
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

func defaultIfEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func safeFileNameSegment(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "item"
	}
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, trimmed)
	return strings.Trim(safe, "_")
}

func parseOptionalVLANID(value string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	vlanID, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, err
	}
	if vlanID < 1 || vlanID > 4094 {
		return 0, fmt.Errorf("must be in range 1-4094")
	}
	return vlanID, nil
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
