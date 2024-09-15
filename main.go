package main

import (
	_ "embed"
	"html/template"

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
	quitPage                       = "*quit*"

	dataDirName      = ".ez-ipam"
	networksDirName  = "networks"
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

	newNetworkDialog                 *tview.Form
	splitNetworkDialog               *tview.Form
	summarizeNetworkDialog           *tview.Modal
	allocateNetworkSubnetsModeDialog *tview.Form
	allocateNetworkHostsModeDialog   *tview.Form
	updateNetworkAllocationDialog    *tview.Form
	deallocateNetworkDialog          *tview.Modal
	deleteNetworkDialog              *tview.Modal
	quitDialog                       *tview.Modal
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
		allocateNetworkSubnetsModeDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("Display Name", "", 40, nil, nil).
			AddTextArea("Description", "", 48, 5, 0, nil).
			AddInputField("Subnets Prefix", "", 40, nil, nil).
			AddButton("Save", func() {
				displayName := getAndClearTextFromInputField(allocateNetworkSubnetsModeDialog, "Display Name")
				description := getAndClearTextFromTextArea(allocateNetworkSubnetsModeDialog, "Description")
				subnetsPrefix := getAndClearTextFromInputField(allocateNetworkSubnetsModeDialog, "Subnets Prefix")
				subnetsPrefix = strings.TrimLeft(subnetsPrefix, "/")
				subnetsPrefixInt, err := strconv.Atoi(subnetsPrefix)
				if err != nil {
					statusLine.Clear()
					statusLine.SetText("Invalid subnets prefix, should be a number representing smaller networks than this parent " + err.Error())
				}
				AllocateNetworkInSubnetsMode(displayName, description, subnetsPrefixInt)

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", func() {
				getAndClearTextFromInputField(allocateNetworkSubnetsModeDialog, "Display Name")
				getAndClearTextFromTextArea(allocateNetworkSubnetsModeDialog, "Description")
				getAndClearTextFromInputField(allocateNetworkSubnetsModeDialog, "Subnets Prefix")

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			})
		allocateNetworkSubnetsModeDialog.SetBorder(true)
		allocateNetworkSubnetsModeFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(
				tview.NewFlex().SetDirection(tview.FlexRow).
					AddItem(nil, 0, 1, false).
					AddItem(allocateNetworkSubnetsModeDialog, height, 1, false).
					AddItem(nil, 0, 1, false),
				width, 1, false).
			AddItem(nil, 0, 1, false)
		pages.AddPage(allocateNetworkSubnetsModePage, allocateNetworkSubnetsModeFlex, true, false)
	}

	{
		height := 13
		width := 57
		allocateNetworkHostsModeDialog = tview.NewForm().SetButtonsAlign(tview.AlignCenter).
			AddInputField("Display Name", "", 40, nil, nil).
			AddTextArea("Description", "", 48, 5, 0, nil).
			AddButton("Save", func() {
				displayName := getAndClearTextFromInputField(allocateNetworkHostsModeDialog, "Display Name")
				description := getAndClearTextFromTextArea(allocateNetworkHostsModeDialog, "Description")

				AllocateNetworkInHostsMode(displayName, description)

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			}).
			AddButton("Cancel", func() {
				getAndClearTextFromInputField(allocateNetworkHostsModeDialog, "Display Name")
				getAndClearTextFromTextArea(allocateNetworkHostsModeDialog, "Description")

				pages.SwitchToPage(mainPage)
				app.SetFocus(navigationPanel)
			})
		allocateNetworkHostsModeDialog.SetBorder(true)
		allocateNetworkHostsModeFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(
				tview.NewFlex().SetDirection(tview.FlexRow).
					AddItem(nil, 0, 1, false).
					AddItem(allocateNetworkHostsModeDialog, height, 1, false).
					AddItem(nil, 0, 1, false),
				width, 1, false).
			AddItem(nil, 0, 1, false)
		pages.AddPage(allocateNetworkHostsModePage, allocateNetworkHostsModeFlex, true, false)
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

	currentDir, err := os.Getwd()
	if err != nil {
		panic("Failed to get current directory: " + err.Error())
	}

	dataDir := filepath.Join(currentDir, dataDirName)
	networkDir := filepath.Join(dataDir, networksDirName)

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
	if len(networkFiles) == 0 {
		menuItems.MustAdd(
			&Network{
				MenuFolder: &MenuFolder{
					ID:         "192.168.0.0/16",
					ParentPath: networks.GetPath(),
				},
			},
		)
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

	networksTmpDir := filepath.Join(dataTmpDir, networksDirName)
	if err := os.RemoveAll(networksTmpDir); err != nil {
		panic("Failed to remove " + networksTmpDir + " directory: " + err.Error())
	}
	if err := os.MkdirAll(networksTmpDir, 0755); err != nil {
		panic("Failed to create " + networksTmpDir + " directory: " + err.Error())
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
		default:
		}
	}

	if err := os.RemoveAll(dataDir); err != nil {
		panic("Failed to remove " + dataDir + " directory: " + err.Error())
	}
	if err := os.Rename(dataTmpDir, dataDir); err != nil {
		panic("Failed to rename " + dataTmpDir + " to " + dataDir + " directory: " + err.Error())
	}

	networksIndex := []string{}
	networksDisplayNames := map[string]string{}
	networksDataIndex := map[string][]string{}
	networksData := map[string]map[string]string{}
	networksOpts := map[string]map[string]int{}
	var recursivelyPopulateNetworksData func(n *Network)
	recursivelyPopulateNetworksData = func(n *Network) {
		index, data, err := n.RenderDetailsMap()
		if err != nil {
			panic("Failed to render details map for " + n.GetPath() + ": " + err.Error())
		}
		networksIndex = append(networksIndex, n.GetID())
		networksDisplayNames[n.GetID()] = n.GetID()
		networksDataIndex[n.GetID()] = index
		networksData[n.GetID()] = data
		networksOpts[n.GetID()] = map[string]int{
			"AllocationMode": int(n.AllocationMode),
		}
		childs := menuItems.GetChilds(n)
		for _, child := range childs {
			nn, ok := child.(*Network)
			if !ok {
				panic("Failed to cast " + child.GetPath() + " to network")
			}
			recursivelyPopulateNetworksData(nn)
		}
	}

	networksMenuItem := menuItems.GetByParentAndID(nil, "Networks")
	for _, menuItem := range menuItems.GetChilds(networksMenuItem) {
		n, ok := menuItem.(*Network)
		if !ok {
			panic("Failed to cast " + menuItem.GetPath() + " to network")
		}
		recursivelyPopulateNetworksData(n)
	}

	template := template.Must(template.New(markdownFileName).Parse(markdownTmpl))
	input := map[string]interface{}{
		"NetworksIndex":        networksIndex,
		"NetworksDisplayNames": networksDisplayNames,
		"NetworksDataIndex":    networksDataIndex,
		"NetworksData":         networksData,
		"NetworksOpts":         networksOpts,
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
	statusLine.SetText("Saved")
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
