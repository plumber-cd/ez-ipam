package ui

import (
	"slices"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/plumber-cd/ez-ipam/internal/domain"
	"github.com/rivo/tview"
)

func (a *App) setupLayout() {
	a.TviewApp = tview.NewApplication()
	a.Pages = tview.NewPages()
	rootFlex := tview.NewFlex().SetDirection(tview.FlexRow)

	a.PositionLine = tview.NewTextView()
	a.PositionLine.SetBorder(true)
	a.PositionLine.SetTitle("Navigation")
	a.PositionLine.SetText("Home")
	rootFlex.AddItem(a.PositionLine, 3, 1, false)

	middleFlex := tview.NewFlex().SetDirection(tview.FlexColumn)
	rootFlex.AddItem(middleFlex, 0, 2, false)

	a.NavPanel = tview.NewList()
	a.NavPanel.ShowSecondaryText(false)
	a.NavPanel.SetBorder(true).SetTitle("Menu")
	middleFlex.AddItem(a.NavPanel, 0, 1, false)

	a.DetailsFlex = tview.NewFlex().SetDirection(tview.FlexRow)
	middleFlex.AddItem(a.DetailsFlex, 0, 2, false)

	a.DetailsPanel = tview.NewTextView()
	a.DetailsPanel.SetBorder(true).SetTitle("Details")
	a.DetailsFlex.AddItem(a.DetailsPanel, 0, 1, false)

	a.KeysLine = tview.NewTextView()
	a.KeysLine.SetBorder(false)
	a.UpdateKeysLine()
	rootFlex.AddItem(a.KeysLine, 1, 1, false)

	a.StatusLine = tview.NewTextView()
	a.StatusLine.SetBorder(true)
	a.StatusLine.SetTitle("Status")
	a.StatusLine.SetWrap(true)
	a.StatusLine.SetWordWrap(true)
	a.StatusLine.SetChangedFunc(func() {
		a.resizeStatusLine()
	})
	a.DetailsFlex.AddItem(a.StatusLine, 3, 0, false)

	a.Pages.AddPage(mainPageName, rootFlex, true, true)

	// Redirect focus from non-interactive panels to nav panel.
	a.PositionLine.SetFocusFunc(func() { a.TviewApp.SetFocus(a.NavPanel) })
	a.DetailsPanel.SetFocusFunc(func() { a.TviewApp.SetFocus(a.NavPanel) })
	a.StatusLine.SetFocusFunc(func() { a.TviewApp.SetFocus(a.NavPanel) })
	a.KeysLine.SetFocusFunc(func() { a.TviewApp.SetFocus(a.NavPanel) })

	// Mouse capture: single click only highlights, double click navigates.
	a.NavPanel.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		switch action {
		case tview.MouseLeftClick:
			a.mouseSelectArmed = false
		case tview.MouseLeftDoubleClick:
			a.mouseSelectArmed = true
			return tview.MouseLeftClick, event
		}
		return action, event
	})

	// Navigation changed callback.
	a.NavPanel.SetChangedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		selected := a.Catalog.GetByParentAndDisplayID(a.CurrentItem, mainText)
		if selected == nil {
			return
		}
		a.CurrentFocus = selected
		a.onItemChanged(selected)
		a.UpdateKeysLine()
	})

	// Navigation selected callback (enter / double-click).
	a.NavPanel.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		if !a.mouseSelectArmed {
			a.mouseSelectArmed = true
			return
		}

		selected := a.Catalog.GetByParentAndDisplayID(a.CurrentItem, mainText)
		if selected == nil {
			return
		}

		oldMenuItem := a.CurrentItem
		a.CurrentItem = selected

		a.ReloadMenu(oldMenuItem)
		a.onItemSelected(selected)
		a.UpdateKeysLine()
	})

	// Navigation done callback (escape / backspace).
	a.NavPanel.SetDoneFunc(func() {
		if a.CurrentItem == nil {
			return
		}

		a.onItemDone(a.CurrentItem)

		oldMenuItem := a.CurrentItem
		parent := a.Catalog.Get(a.CurrentItem.GetParentPath())
		a.CurrentItem = parent

		a.ReloadMenu(oldMenuItem)

		if a.CurrentItem == nil {
			a.PositionLine.Clear()
			a.PositionLine.SetText("Home")
		} else {
			a.onItemSelected(a.CurrentItem)
		}

		a.UpdateKeysLine()
	})

	// Input capture on nav panel for vim keys and global shortcuts.
	a.NavPanel.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
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
				a.Pages.ShowPage(quitPageName)
				a.quitDialog.SetFocus(1)
				a.TviewApp.SetFocus(a.quitDialog)
				return nil
			case '?':
				a.showHelpPopup()
				return nil
			}
		}

		if a.CurrentItem != nil {
			if e := a.onMenuKeyPress(a.CurrentItem, event); e != event {
				return e
			}
		}
		if a.CurrentFocus != nil {
			if e := a.onFocusKeyPress(a.CurrentFocus, event); e != event {
				return e
			}
		}

		return event
	})

	// ---- All dialogs ----
	// Helper to create and register a simple form dialog.
	makeFormDialog := func(pageName, title string, buildForm func(form *tview.Form)) {
		form := tview.NewForm().SetButtonsAlign(tview.AlignCenter)
		buildForm(form)
		form.SetBorder(true).SetTitle(title)
		a.dialogForms[pageName] = form
		cancelDialog := func() {
			// Clear all input fields.
			for i := range form.GetFormItemCount() {
				item := form.GetFormItem(i)
				if input, ok := item.(*tview.InputField); ok {
					input.SetText("")
				}
				if ta, ok := item.(*hintedTextArea); ok {
					ta.SetText("", false)
				}
			}
			a.Pages.SwitchToPage(mainPageName)
			a.TviewApp.SetFocus(a.NavPanel)
		}
		a.wireDialogFormKeys(form, cancelDialog)
		a.Pages.AddPage(pageName, a.createDialogPage(form, computeFormDialogWidth(form), computeFormDialogHeight(form)), true, false)
	}

	// New Network dialog.
	makeFormDialog("*new_network*", "Add Network", func(form *tview.Form) {
		form.AddInputField("CIDR", "", FormFieldWidth, nil, nil).
			AddButton("Save", func() {
				a.AddNewNetwork(getAndClearTextFromInputField(form, "CIDR"))
				a.Pages.SwitchToPage(mainPageName)
				a.TviewApp.SetFocus(a.NavPanel)
			}).
			AddButton("Cancel", func() {
				getAndClearTextFromInputField(form, "CIDR")
				a.Pages.SwitchToPage(mainPageName)
				a.TviewApp.SetFocus(a.NavPanel)
			})
	})

	// Split Network dialog.
	makeFormDialog("*split_network*", "Split Network", func(form *tview.Form) {
		form.AddInputField("New Prefix Length", "", FormFieldWidth, nil, nil).
			AddButton("Save", func() {
				newPrefix := getAndClearTextFromInputField(form, "New Prefix Length")
				newPrefix = strings.TrimLeft(newPrefix, "/")
				if newPrefix == "" {
					a.SplitNetwork(0)
				} else {
					newPrefixInt, err := strconv.Atoi(newPrefix)
					if err != nil {
						a.setStatus("Invalid prefix length. Enter a number larger than the current prefix: " + err.Error())
						return
					}
					a.SplitNetwork(newPrefixInt)
				}
				a.Pages.SwitchToPage(mainPageName)
				a.TviewApp.SetFocus(a.NavPanel)
			}).
			AddButton("Cancel", func() {
				getAndClearTextFromInputField(form, "New Prefix Length")
				a.Pages.SwitchToPage(mainPageName)
				a.TviewApp.SetFocus(a.NavPanel)
			})
	})

	// Network allocation dialogs are dynamic (see showNetworkAllocDialog).

	// Reserve IP dialog.
	makeFormDialog("*reserve_ip*", "Reserve IP", func(form *tview.Form) {
		form.AddInputField("IP Address", "", FormFieldWidth, nil, nil).
			AddInputField("Name", "", FormFieldWidth, nil, nil).
			AddInputField("MAC Address", "", FormFieldWidth, nil, nil).
			AddFormItem(newHintedTextArea("Description", "", FormFieldWidth, 3, descriptionHint)).
			AddButton("Save", func() {
				address := getAndClearTextFromInputField(form, "IP Address")
				displayName := getAndClearTextFromInputField(form, "Name")
				macAddress := getAndClearTextFromInputField(form, "MAC Address")
				description := getAndClearTextFromTextArea(form, "Description")
				a.ReserveIP(address, displayName, macAddress, description)
				a.Pages.SwitchToPage(mainPageName)
				a.TviewApp.SetFocus(a.NavPanel)
			}).
			AddButton("Cancel", func() {
				getAndClearTextFromInputField(form, "IP Address")
				getAndClearTextFromInputField(form, "Name")
				getAndClearTextFromInputField(form, "MAC Address")
				getAndClearTextFromTextArea(form, "Description")
				a.Pages.SwitchToPage(mainPageName)
				a.TviewApp.SetFocus(a.NavPanel)
			})
	})

	// Update IP Reservation dialog.
	makeFormDialog("*update_ip_reservation*", "Update IP Reservation", func(form *tview.Form) {
		form.AddInputField("Name", "", FormFieldWidth, nil, nil).
			AddInputField("MAC Address", "", FormFieldWidth, nil, nil).
			AddFormItem(newHintedTextArea("Description", "", FormFieldWidth, 3, descriptionHint)).
			AddButton("Save", func() {
				displayName := getAndClearTextFromInputField(form, "Name")
				macAddress := getAndClearTextFromInputField(form, "MAC Address")
				description := getAndClearTextFromTextArea(form, "Description")
				a.UpdateIPReservation(displayName, macAddress, description)
				a.Pages.SwitchToPage(mainPageName)
				a.TviewApp.SetFocus(a.NavPanel)
			}).
			AddButton("Cancel", func() {
				getAndClearTextFromInputField(form, "Name")
				getAndClearTextFromInputField(form, "MAC Address")
				getAndClearTextFromTextArea(form, "Description")
				a.Pages.SwitchToPage(mainPageName)
				a.TviewApp.SetFocus(a.NavPanel)
			})
	})

	// VLAN add/update dialogs are dynamic (see showVLANDialog).

	// Add SSID dialog.
	makeFormDialog("*add_ssid*", "Add WiFi SSID", func(form *tview.Form) {
		form.AddInputField("SSID", "", FormFieldWidth, nil, nil).
			AddFormItem(newHintedTextArea("Description", "", FormFieldWidth, 3, descriptionHint)).
			AddButton("Save", func() {
				ssidID := getAndClearTextFromInputField(form, "SSID")
				description := getAndClearTextFromTextArea(form, "Description")
				a.AddSSID(ssidID, description)
				a.Pages.SwitchToPage(mainPageName)
				a.TviewApp.SetFocus(a.NavPanel)
			}).
			AddButton("Cancel", func() {
				getAndClearTextFromInputField(form, "SSID")
				getAndClearTextFromTextArea(form, "Description")
				a.Pages.SwitchToPage(mainPageName)
				a.TviewApp.SetFocus(a.NavPanel)
			})
	})

	// Update SSID dialog.
	makeFormDialog("*update_ssid*", "Update WiFi SSID", func(form *tview.Form) {
		form.AddFormItem(newHintedTextArea("Description", "", FormFieldWidth, 3, descriptionHint)).
			AddButton("Save", func() {
				description := getAndClearTextFromTextArea(form, "Description")
				a.UpdateSSID(description)
				a.Pages.SwitchToPage(mainPageName)
				a.TviewApp.SetFocus(a.NavPanel)
			}).
			AddButton("Cancel", func() {
				getAndClearTextFromTextArea(form, "Description")
				a.Pages.SwitchToPage(mainPageName)
				a.TviewApp.SetFocus(a.NavPanel)
			})
	})

	// Zone add/update dialogs are dynamic (see showZoneDialog).

	// Add Equipment dialog.
	makeFormDialog("*add_equipment*", "Add Equipment", func(form *tview.Form) {
		form.AddInputField("Name", "", FormFieldWidth, nil, nil).
			AddInputField("Model", "", FormFieldWidth, nil, nil).
			AddFormItem(newHintedTextArea("Description", "", FormFieldWidth, 3, descriptionHint)).
			AddButton("Save", func() {
				name := getAndClearTextFromInputField(form, "Name")
				model := getAndClearTextFromInputField(form, "Model")
				description := getAndClearTextFromTextArea(form, "Description")
				a.AddEquipment(name, model, description)
				a.Pages.SwitchToPage(mainPageName)
				a.TviewApp.SetFocus(a.NavPanel)
			}).
			AddButton("Cancel", func() {
				getAndClearTextFromInputField(form, "Name")
				getAndClearTextFromInputField(form, "Model")
				getAndClearTextFromTextArea(form, "Description")
				a.Pages.SwitchToPage(mainPageName)
				a.TviewApp.SetFocus(a.NavPanel)
			})
	})

	// Update Equipment dialog.
	makeFormDialog("*update_equipment*", "Update Equipment", func(form *tview.Form) {
		form.AddInputField("Name", "", FormFieldWidth, nil, nil).
			AddInputField("Model", "", FormFieldWidth, nil, nil).
			AddFormItem(newHintedTextArea("Description", "", FormFieldWidth, 3, descriptionHint)).
			AddButton("Save", func() {
				name := getAndClearTextFromInputField(form, "Name")
				model := getAndClearTextFromInputField(form, "Model")
				description := getAndClearTextFromTextArea(form, "Description")
				a.UpdateEquipment(name, model, description)
				a.Pages.SwitchToPage(mainPageName)
				a.TviewApp.SetFocus(a.NavPanel)
			}).
			AddButton("Cancel", func() {
				getAndClearTextFromInputField(form, "Name")
				getAndClearTextFromInputField(form, "Model")
				getAndClearTextFromTextArea(form, "Description")
				a.Pages.SwitchToPage(mainPageName)
				a.TviewApp.SetFocus(a.NavPanel)
			})
	})

	// Confirm modals.
	a.setupModals()

	// Quit dialog.
	{
		a.quitDialog = tview.NewModal().SetText("Do you want to quit? All unsaved changes will be lost.").
			AddButtons([]string{"Quit", "Cancel"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				switch buttonLabel {
				case "Quit":
					a.TviewApp.Stop()
				case "Cancel":
					fallthrough
				default:
					a.Pages.SwitchToPage(mainPageName)
					a.TviewApp.SetFocus(a.NavPanel)
				}
			})
		a.Pages.AddPage(quitPageName, a.quitDialog, true, false)
	}

	// Root setup.
	a.TviewApp.SetRoot(a.Pages, true)
	a.TviewApp.EnableMouse(true)
	a.TviewApp.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		a.resizeStatusLine()
		a.UpdateKeysLine()
		return false
	})
	a.TviewApp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Test sentinel: if a sentinel channel is set and the sentinel key is received,
		// signal completion and consume the event.
		if a.SentinelCh != nil && event.Key() == tcell.KeyF63 {
			ch := a.SentinelCh
			a.SentinelCh = nil
			close(ch)
			return nil
		}
		switch event.Key() {
		case tcell.KeyCtrlC:
			a.Pages.ShowPage(quitPageName)
			a.quitDialog.SetFocus(1)
			a.TviewApp.SetFocus(a.quitDialog)
			return nil
		case tcell.KeyCtrlS:
			a.Save()
			return nil
		case tcell.KeyCtrlQ:
			a.TviewApp.Stop()
			return nil
		}
		return event
	})
	a.Pages.SwitchToPage(mainPageName)
	a.TviewApp.SetFocus(a.NavPanel)
}

func (a *App) setupModals() {
	makeModal := func(pageName, text string, onYes func()) {
		modal := tview.NewModal().SetText(text).AddButtons([]string{"Yes", "No"})
		modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			switch buttonLabel {
			case "Yes":
				onYes()
				fallthrough
			case "No":
				fallthrough
			default:
				modal.SetText("")
				a.Pages.SwitchToPage(mainPageName)
				a.TviewApp.SetFocus(a.NavPanel)
			}
		})
		a.Pages.AddPage(pageName, modal, true, false)
	}

	makeModal("*unreserve_ip*", "Unreserve this IP address?", func() { a.UnreserveIP() })
	makeModal("*delete_vlan*", "Delete this VLAN?", func() { a.DeleteVLAN() })
	makeModal("*delete_ssid*", "Delete this WiFi SSID?", func() { a.DeleteSSID() })
	makeModal("*delete_zone*", "Delete this zone?", func() { a.DeleteZone() })
	makeModal("*delete_equipment*", "Delete this equipment and all ports?", func() { a.DeleteEquipment() })
	makeModal("*disconnect_port*", "Disconnect this port?", func() { a.DisconnectPort() })
	makeModal("*delete_port*", "Delete this port?", func() { a.DeletePort() })
	makeModal("*deallocate_network*", "Deallocate this network and remove its subnets?", func() { a.DeallocateNetwork() })
	makeModal("*delete_network*", "Delete this top-level network and all of its subnets?", func() { a.DeleteNetwork() })
	makeModal("*delete_dns_record*", "Delete this DNS record?", func() { a.DeleteDNSRecord() })
}

// mouseBlocker returns a box that absorbs mouse events (prevents clicking through dialog overlays).
func mouseBlocker() *tview.Box {
	box := tview.NewBox()
	box.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		return action, nil
	})
	return box
}

// createDialogPage wraps a form or content primitive in a centered dialog overlay.
func (a *App) createDialogPage(content tview.Primitive, width, height int) tview.Primitive {
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

// submitPrimaryFormButton programmatically activates the first button in a form.
func submitPrimaryFormButton(form *tview.Form, setFocus func(p tview.Primitive)) {
	if form.GetButtonCount() == 0 {
		return
	}
	handler := form.GetButton(0).InputHandler()
	if handler == nil {
		return
	}
	handler(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), setFocus)
}

// wireDialogFormKeys sets up standard keyboard handling for a dialog form.
func (a *App) wireDialogFormKeys(form *tview.Form, onCancel func()) {
	form.SetCancelFunc(onCancel)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		formItemIndex, _ := form.GetFocusedItemIndex()
		var focusedFormItem tview.FormItem
		if formItemIndex >= 0 {
			focusedFormItem = form.GetFormItem(formItemIndex)
			if _, ok := focusedFormItem.(*tview.DropDown); ok {
				return event
			}
		}

		switch event.Key() {
		case tcell.KeyEscape:
			if sd, ok := focusedFormItem.(*searchableDropdown); ok && sd.isOpen {
				return event
			}
			onCancel()
			return nil
		case tcell.KeyCtrlE:
			textArea, ok := focusedFormItem.(*hintedTextArea)
			if !ok {
				return event
			}
			updatedText, err := a.openInExternalEditor(textArea.GetText())
			if err != nil {
				a.setStatus("Failed to open external editor: " + err.Error())
				return nil
			}
			textArea.SetText(updatedText, true)
			return nil
		case tcell.KeyEnter:
			if formItemIndex >= 0 {
				if _, ok := focusedFormItem.(*searchableDropdown); ok {
					return event
				}
				if _, ok := focusedFormItem.(*hintedTextArea); ok {
					return event
				}
				if _, ok := focusedFormItem.(*tview.Checkbox); ok {
					// Toggle checkbox on Enter for better UX.
					return tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone)
				}
			}
			submitPrimaryFormButton(form, func(p tview.Primitive) {
				a.TviewApp.SetFocus(p)
			})
			return nil
		}
		return event
	})
}

// dismissDialog removes a dialog page and returns to main.
func (a *App) dismissDialog(pageName string) {
	a.Pages.RemovePage(pageName)
	a.Pages.SwitchToPage(mainPageName)
	a.TviewApp.SetFocus(a.NavPanel)
}

// showPortDialog creates a fresh port dialog. Dropdown callbacks are set after
// all fields are added so that tview's initial SetCurrentOption does not trigger
// a cascading rebuild.
func (a *App) showPortDialog(pageName, title string, vals portDialogValues, parent *domain.Equipment, focusLabel string, onSave func(portDialogValues)) {
	a.Pages.RemovePage(pageName)

	form := tview.NewForm().SetButtonsAlign(tview.AlignCenter)

	if vals.PortNumber == "" && !vals.Enabled {
		vals.Enabled = true
	}
	vals.LAGMode = normalizeLagModeOption(vals.LAGMode)
	vals.TaggedMode = normalizeTaggedModeOption(vals.TaggedMode)
	if !vals.Enabled {
		vals.Name = ""
		vals.LAGMode = LagModeDisabledOption
		vals.LAGGroup = ""
		vals.NativeVLANID = ""
		vals.TaggedMode = TaggedModeNoneOption
		vals.TaggedVLANIDs = ""
	} else {
		if vals.LAGMode == LagModeDisabledOption {
			vals.LAGGroup = ""
		} else {
			if vals.LAGGroup == vals.PortNumber && strings.TrimSpace(vals.PortNumber) != "" {
				vals.LAGGroup = "self"
			}
			if strings.TrimSpace(vals.LAGGroup) == "" {
				vals.LAGGroup = "self"
			}
		}
		if vals.TaggedMode != string(domain.TaggedVLANModeCustom) {
			vals.TaggedVLANIDs = ""
		}
	}
	isLagMember := vals.Enabled && vals.LAGMode != LagModeDisabledOption && vals.LAGGroup != "" && !strings.EqualFold(vals.LAGGroup, "self")

	form.AddInputField("Port Number", vals.PortNumber, FormFieldWidth, nil, nil)
	form.AddCheckbox("Enabled", vals.Enabled, nil)
	if vals.Enabled {
		form.AddInputField("Name", vals.Name, FormFieldWidth, nil, nil)
	}
	form.AddInputField("Port Type", vals.PortType, FormFieldWidth, nil, nil)
	form.AddInputField("Speed", vals.Speed, FormFieldWidth, nil, nil)
	form.AddInputField("PoE", vals.PoE, FormFieldWidth, nil, nil)
	if vals.Enabled {
		lagModeIndex := slices.Index(LagModeOptions, vals.LAGMode)
		if lagModeIndex < 0 {
			lagModeIndex = 0
		}
		form.AddDropDown("LAG Mode", LagModeOptions, lagModeIndex, nil)
		lagModeItem := getFormItemByLabel(form, "LAG Mode")
		lagModeDropdown := lagModeItem.(*tview.DropDown)
		lagModeDropdown.SetSelectedFunc(func(option string, _ int) {
			newVals := capturePortFormValues(form)
			newVals.LAGMode = normalizeLagModeOption(option)
			a.showPortDialog(pageName, title, newVals, parent, "LAG Mode", onSave)
		})
		if vals.LAGMode != LagModeDisabledOption {
			lagGroupOptions := a.getLAGGroupDropdownOptions(parent, vals.PortNumber)
			lagGroupCurrent := findLAGGroupDropdownOption(lagGroupOptions, vals.LAGGroup)
			lagGroupIndex := slices.Index(lagGroupOptions, lagGroupCurrent)
			if lagGroupIndex < 0 {
				lagGroupIndex = 0
			}
			form.AddDropDown("LAG Group", lagGroupOptions, lagGroupIndex, nil)
			lagGroupItem := getFormItemByLabel(form, "LAG Group")
			lagGroupDropdown := lagGroupItem.(*tview.DropDown)
			lagGroupDropdown.SetSelectedFunc(func(option string, _ int) {
				newVals := capturePortFormValues(form)
				newVals.LAGGroup = parseLAGGroupFromDropdownOption(option)
				a.showPortDialog(pageName, title, newVals, parent, "LAG Group", onSave)
			})
		}
		if !isLagMember {
			nativeVLANOptions := a.getVLANDropdownOptions()
			nativeCurrent := findVLANDropdownOption(nativeVLANOptions, vals.NativeVLANID)
			form.AddFormItem(newSearchableDropdown("Native VLAN ID", nativeVLANOptions, nativeCurrent, true, nil))
			taggedModeIndex := slices.Index(TaggedModeOptions, vals.TaggedMode)
			if taggedModeIndex < 0 {
				taggedModeIndex = 0
			}
			form.AddDropDown("Tagged VLAN Mode", TaggedModeOptions, taggedModeIndex, nil)
			taggedModeItem := getFormItemByLabel(form, "Tagged VLAN Mode")
			taggedModeDropdown := taggedModeItem.(*tview.DropDown)
			taggedModeDropdown.SetSelectedFunc(func(option string, _ int) {
				newVals := capturePortFormValues(form)
				newVals.TaggedMode = normalizeTaggedModeOption(option)
				a.showPortDialog(pageName, title, newVals, parent, "Tagged VLAN Mode", onSave)
			})
			if vals.TaggedMode == string(domain.TaggedVLANModeCustom) {
				taggedSet := parseVLANIDSet(vals.TaggedVLANIDs)
				for _, v := range a.getVLANOptions() {
					form.AddCheckbox(v.label, taggedSet[v.id], nil)
				}
			}
		}
	}
	form.AddFormItem(newHintedTextArea("Destination Notes", vals.DestinationNotes, FormFieldWidth, 3, descriptionHint))

	cancel := func() {
		a.dismissDialog(pageName)
	}

	form.AddButton("Save", func() {
		result := capturePortFormValues(form)
		onSave(result)
		a.dismissDialog(pageName)
	})
	form.AddButton("Cancel", cancel)

	enabledItem := getFormItemByLabel(form, "Enabled")
	enabledCheckbox := enabledItem.(*tview.Checkbox)
	enabledCheckbox.SetChangedFunc(func(checked bool) {
		newVals := capturePortFormValues(form)
		newVals.Enabled = checked
		a.showPortDialog(pageName, title, newVals, parent, "Enabled", onSave)
	})

	form.SetBorder(true).SetTitle(title)
	a.wireDialogFormKeys(form, cancel)
	a.Pages.AddPage(pageName, a.createDialogPage(form, computeFormDialogWidth(form), computeFormDialogHeight(form)), true, false)
	a.Pages.ShowPage(pageName)
	focusIndex := 0
	if focusLabel != "" {
		if idx := form.GetFormItemIndex(focusLabel); idx >= 0 {
			focusIndex = idx
		}
	}
	form.SetFocus(focusIndex)
	a.TviewApp.SetFocus(form)
}

// showVLANDialog creates a fresh VLAN add/update dialog with a single zone dropdown.
func (a *App) showVLANDialog(pageName, title string, vals vlanDialogValues, showVLANID bool, onSave func(vlanDialogValues)) {
	a.Pages.RemovePage(pageName)

	form := tview.NewForm().SetButtonsAlign(tview.AlignCenter)

	if showVLANID {
		form.AddInputField("VLAN ID", vals.VLANIDText, FormFieldWidth, nil, nil)
	}
	form.AddInputField("Name", vals.Name, FormFieldWidth, nil, nil)
	form.AddFormItem(newHintedTextArea("Description", vals.Description, FormFieldWidth, 3, descriptionHint))
	zoneOptions := a.getZoneDropdownOptions()
	zoneCurrent := findZoneDropdownOption(zoneOptions, vals.SelectedZone)
	form.AddFormItem(newSearchableDropdown("Zone", zoneOptions, zoneCurrent, true, nil))

	cancel := func() { a.dismissDialog(pageName) }

	form.AddButton("Save", func() {
		result := vlanDialogValues{
			VLANIDText:   getTextFromInputFieldIfPresent(form, "VLAN ID"),
			Name:         getTextFromInputFieldIfPresent(form, "Name"),
			Description:  getTextFromTextAreaIfPresent(form, "Description"),
			SelectedZone: parseZoneFromDropdownOption(getSearchableDropdownValue(form, "Zone", NoneVLANOption)),
		}
		onSave(result)
		a.dismissDialog(pageName)
	})
	form.AddButton("Cancel", cancel)

	form.SetBorder(true).SetTitle(title)
	a.wireDialogFormKeys(form, cancel)
	a.Pages.AddPage(pageName, a.createDialogPage(form, computeFormDialogWidth(form), computeFormDialogHeight(form)), true, false)
	a.Pages.ShowPage(pageName)
	form.SetFocus(0)
	a.TviewApp.SetFocus(form)
}

// showZoneDialog creates a fresh zone add/update dialog with VLAN checkboxes.
func (a *App) showZoneDialog(pageName, title string, vals zoneDialogValues, onSave func(zoneDialogValues)) {
	a.Pages.RemovePage(pageName)

	form := tview.NewForm().SetButtonsAlign(tview.AlignCenter)

	form.AddInputField("Name", vals.Name, FormFieldWidth, nil, nil)
	form.AddFormItem(newHintedTextArea("Description", vals.Description, FormFieldWidth, 3, descriptionHint))

	// VLAN checkboxes.
	if vals.SelectedVLANs == nil {
		vals.SelectedVLANs = make(map[int]bool)
	}
	for _, v := range a.getVLANOptions() {
		vlanID := v.id // capture for closure
		checked := vals.SelectedVLANs[vlanID]
		form.AddCheckbox(v.label, checked, func(c bool) {
			vals.SelectedVLANs[vlanID] = c
		})
	}

	cancel := func() { a.dismissDialog(pageName) }

	form.AddButton("Save", func() {
		result := zoneDialogValues{
			Name:          getTextFromInputFieldIfPresent(form, "Name"),
			Description:   getTextFromTextAreaIfPresent(form, "Description"),
			SelectedVLANs: vals.SelectedVLANs,
		}
		onSave(result)
		a.dismissDialog(pageName)
	})
	form.AddButton("Cancel", cancel)

	form.SetBorder(true).SetTitle(title)
	a.wireDialogFormKeys(form, cancel)
	a.Pages.AddPage(pageName, a.createDialogPage(form, computeFormDialogWidth(form), computeFormDialogHeight(form)), true, false)
	a.Pages.ShowPage(pageName)
	form.SetFocus(0)
	a.TviewApp.SetFocus(form)
}

// showNetworkAllocDialog creates a fresh network allocation dialog with a VLAN dropdown.
func (a *App) showNetworkAllocDialog(pageName, title string, vals networkAllocDialogValues, showChildPrefix bool, onSave func(networkAllocDialogValues)) {
	a.Pages.RemovePage(pageName)

	form := tview.NewForm().SetButtonsAlign(tview.AlignCenter)

	vlanOptions := a.getVLANDropdownOptions()

	form.AddInputField("Name", vals.Name, FormFieldWidth, nil, nil)
	form.AddFormItem(newHintedTextArea("Description", vals.Description, FormFieldWidth, 3, descriptionHint))
	vlanCurrent := findVLANDropdownOption(vlanOptions, vals.VLANID)
	form.AddFormItem(newSearchableDropdown("VLAN ID", vlanOptions, vlanCurrent, true, nil))
	if showChildPrefix {
		form.AddInputField("Child Prefix Len", vals.ChildPrefixLen, FormFieldWidth, nil, nil)
	}

	cancel := func() { a.dismissDialog(pageName) }

	form.AddButton("Save", func() {
		result := networkAllocDialogValues{
			Name:        getTextFromInputFieldIfPresent(form, "Name"),
			Description: getTextFromTextAreaIfPresent(form, "Description"),
			VLANID:      parseVLANIDFromDropdownOption(getSearchableDropdownValue(form, "VLAN ID", NoneVLANOption)),
		}
		if showChildPrefix {
			result.ChildPrefixLen = getTextFromInputFieldIfPresent(form, "Child Prefix Len")
		}
		onSave(result)
		a.dismissDialog(pageName)
	})
	form.AddButton("Cancel", cancel)

	form.SetBorder(true).SetTitle(title)
	a.wireDialogFormKeys(form, cancel)
	a.Pages.AddPage(pageName, a.createDialogPage(form, computeFormDialogWidth(form), computeFormDialogHeight(form)), true, false)
	a.Pages.ShowPage(pageName)
	form.SetFocus(0)
	a.TviewApp.SetFocus(form)
}

// showDNSRecordDialog creates a fresh DNS add/update dialog.
// focusLabel preserves focus across dialog rebuilds.
func (a *App) showDNSRecordDialog(pageName, title string, vals dnsRecordDialogValues, allowFQDNEdit bool, focusLabel string, onSave func(dnsRecordDialogValues)) {
	a.Pages.RemovePage(pageName)

	form := tview.NewForm().SetButtonsAlign(tview.AlignCenter)
	if vals.Mode == "" {
		vals.Mode = DNSModeRecord
	}

	form.AddInputField("FQDN", vals.FQDN, FormFieldWidth, nil, nil)

	modeIndex := slices.Index(DNSModeOptions, vals.Mode)
	if modeIndex < 0 {
		modeIndex = 0
	}
	form.AddDropDown("Mode", DNSModeOptions, modeIndex, nil)
	modeItem := getFormItemByLabel(form, "Mode")
	modeDropdown := modeItem.(*tview.DropDown)

	if vals.Mode == DNSModeAlias {
		options, paths := a.getReservedIPDropdownOptions()
		current := findReservedIPDropdownOption(options, paths, vals.ReservedIPPath)
		form.AddFormItem(newSearchableDropdown("Reserved IP", options, current, false, nil))
	} else {
		form.AddInputField("Record Type", vals.RecordType, FormFieldWidth, nil, nil)
		form.AddInputField("Record Value", vals.RecordValue, FormFieldWidth, nil, nil)
	}
	form.AddFormItem(newHintedTextArea("Description", vals.Description, FormFieldWidth, 3, descriptionHint))

	cancel := func() { a.dismissDialog(pageName) }
	form.AddButton("Save", func() {
		result := dnsRecordDialogValues{
			FQDN:        strings.TrimSpace(getTextFromInputFieldIfPresent(form, "FQDN")),
			Mode:        vals.Mode,
			RecordType:  strings.TrimSpace(getTextFromInputFieldIfPresent(form, "Record Type")),
			RecordValue: strings.TrimSpace(getTextFromInputFieldIfPresent(form, "Record Value")),
			Description: getTextFromTextAreaIfPresent(form, "Description"),
		}
		if result.Mode == DNSModeAlias {
			options, paths := a.getReservedIPDropdownOptions()
			selected := getSearchableDropdownValue(form, "Reserved IP", "")
			for i, option := range options {
				if option == selected && i < len(paths) {
					result.ReservedIPPath = paths[i]
					break
				}
			}
			result.RecordType = ""
			result.RecordValue = ""
		} else {
			result.ReservedIPPath = ""
		}
		if !allowFQDNEdit {
			result.FQDN = vals.FQDN
		}
		onSave(result)
		a.dismissDialog(pageName)
	})
	form.AddButton("Cancel", cancel)

	// Register mode callback after form construction. This avoids firing on initial
	// SetCurrentOption and accidentally rebuilding to focus index 0.
	modeDropdown.SetSelectedFunc(func(option string, _ int) {
		newVals := vals
		newVals.FQDN = getTextFromInputFieldIfPresent(form, "FQDN")
		newVals.Mode = option
		newVals.RecordType = getTextFromInputFieldIfPresent(form, "Record Type")
		newVals.RecordValue = getTextFromInputFieldIfPresent(form, "Record Value")
		newVals.Description = getTextFromTextAreaIfPresent(form, "Description")
		if option == DNSModeAlias {
			options, paths := a.getReservedIPDropdownOptions()
			selected := getSearchableDropdownValue(form, "Reserved IP", "")
			for i := range options {
				if options[i] == selected && i < len(paths) {
					newVals.ReservedIPPath = paths[i]
					break
				}
			}
		}
		a.showDNSRecordDialog(pageName, title, newVals, allowFQDNEdit, "Mode", onSave)
	})

	form.SetBorder(true).SetTitle(title)
	a.wireDialogFormKeys(form, cancel)
	a.Pages.AddPage(pageName, a.createDialogPage(form, computeFormDialogWidth(form), computeFormDialogHeight(form)), true, false)
	a.Pages.ShowPage(pageName)
	focusIndex := 0
	if focusLabel != "" {
		if idx := form.GetFormItemIndex(focusLabel); idx >= 0 {
			focusIndex = idx
		}
	}
	form.SetFocus(focusIndex)
	a.TviewApp.SetFocus(form)
}

// showSummarizeDialog creates a summarize dialog for the given candidates.
func (a *App) showSummarizeDialog(candidates []*domain.Network, fromIndex, toIndex int, parentDisplayID string) {
	const pageName = "*summarize_network*"
	a.Pages.RemovePage(pageName)

	options := make([]string, 0, len(candidates))
	for _, c := range candidates {
		options = append(options, c.DisplayID())
	}

	fromIdx := fromIndex
	toIdx := toIndex

	form := tview.NewForm().SetButtonsAlign(tview.AlignCenter)
	form.AddDropDown("From", options, fromIdx, func(_ string, idx int) {
		if idx >= 0 && idx < len(options) {
			fromIdx = idx
		}
	})
	form.AddDropDown("To", options, toIdx, func(_ string, idx int) {
		if idx >= 0 && idx < len(options) {
			toIdx = idx
		}
	})
	form.AddButton("Summarize", func() {
		a.SummarizeNetworkSelection(candidates, fromIdx, toIdx)
		a.dismissDialog(pageName)
	})
	form.AddButton("Cancel", func() {
		a.dismissDialog(pageName)
	})

	form.SetBorder(true).SetTitle("Summarize in " + parentDisplayID)
	a.wireDialogFormKeys(form, func() { a.dismissDialog(pageName) })
	a.Pages.AddPage(pageName, a.createDialogPage(form, computeFormDialogWidth(form), computeFormDialogHeight(form)), true, false)
	a.Pages.ShowPage(pageName)
	form.SetFocus(0)
	a.TviewApp.SetFocus(form)
}

// showConnectPortDialog creates a connect port dialog with available targets.
func (a *App) showConnectPortDialog(portDisplayID string, options []string, paths []string) {
	const pageName = "*connect_port*"
	a.Pages.RemovePage(pageName)

	indexByOption := make(map[string]int, len(options))
	for i, option := range options {
		if _, exists := indexByOption[option]; !exists {
			indexByOption[option] = i
		}
	}
	selection := ""
	selectedOption := ""

	form := tview.NewForm().SetButtonsAlign(tview.AlignCenter)
	form.AddFormItem(newSearchableDropdown("Target", options, selectedOption, false, func(option string, _ int) {
		if optionIndex, ok := indexByOption[option]; ok && optionIndex >= 0 && optionIndex < len(paths) {
			selection = paths[optionIndex]
		}
	}))
	form.AddButton("Connect", func() {
		if selection == "" {
			selectedText := strings.TrimSpace(getSearchableDropdownValue(form, "Target", ""))
			if optionIndex, ok := indexByOption[selectedText]; ok && optionIndex >= 0 && optionIndex < len(paths) {
				selection = paths[optionIndex]
			}
		}
		if selection == "" {
			a.dismissDialog(pageName)
			a.setStatus("Error connecting Port: select a target")
			return
		}
		a.ConnectPort(selection)
		a.dismissDialog(pageName)
	})
	form.AddButton("Cancel", func() {
		a.dismissDialog(pageName)
	})

	form.SetBorder(true).SetTitle("Connect " + portDisplayID)
	a.wireDialogFormKeys(form, func() { a.dismissDialog(pageName) })
	a.Pages.AddPage(pageName, a.createDialogPage(form, computeFormDialogWidth(form), computeFormDialogHeight(form)), true, false)
	a.Pages.ShowPage(pageName)
	form.SetFocus(0)
	a.TviewApp.SetFocus(form)
}
