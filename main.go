package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	mainPage = "*main*"
)

var (
	app   *tview.Application
	pages *tview.Pages
	focus tview.Primitive

	positionLine    *tview.TextView
	navigationPanel *tview.List
	detailsPanel    *tview.TextView
	statusLine      *tview.TextView
)

func main() {
	app := tview.NewApplication()
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

	detailsPanel = tview.NewTextView().SetText("Details")
	detailsPanel.SetBorder(true).SetTitle("Details")
	middleFlex.AddItem(detailsPanel, 0, 5, false)

	statusLine = tview.NewTextView()
	statusLine.SetBorder(true)
	rootFlex.AddItem(statusLine, 3, 1, false)

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
		selected := menuItems.GetByParentAndName(currentMenuItem, mainText)
		if selected == nil {
			statusLine.Clear()
			statusLine.SetText("Failed to find currently changed menu item!")
			return
		}

		detailsPanel.Clear()
		detailsPanel.SetText("Details about " + selected.GetName())

		statusLine.Clear()
		statusLine.SetText("Changed: " + selected.GetPath())
	})

	navigationPanel.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		selected := menuItems.GetByParentAndName(currentMenuItem, mainText)
		if selected == nil {
			statusLine.Clear()
			statusLine.SetText("Failed to find currently selected menu item!")
			return
		}

		newChilds := menuItems.GetChilds(selected)
		if len(newChilds) > 0 {
			currentMenuItem = selected

			navigationPanel.Clear()
			for _, menuItem := range menuItems.GetChilds(selected) {
				navigationPanel.AddItem(menuItem.GetName(), menuItem.GetPath(), 0, nil)
			}

			positionLine.Clear()
			positionLine.SetText(selected.GetPath())

			detailsPanel.Clear()
			detailsPanel.SetText("Details about " + selected.GetName())

			statusLine.Clear()
			statusLine.SetText("Selected: " + selected.GetPath())
		} else {
			statusLine.Clear()
			statusLine.SetText("No child items for " + selected.GetPath())
		}
	})

	navigationPanel.SetDoneFunc(func() {
		var selected MenuItem
		if currentMenuItem != nil {
			selected = currentMenuItem.GetParent()
			currentMenuItem = selected
		}

		navigationPanel.Clear()
		for _, menuItem := range menuItems.GetChilds(selected) {
			navigationPanel.AddItem(menuItem.GetName(), menuItem.GetPath(), 0, nil)
		}

		if selected == nil {
			positionLine.Clear()
			positionLine.SetText("Home")

			statusLine.Clear()
			statusLine.SetText("Navigated back to Home")
		} else {
			positionLine.Clear()
			positionLine.SetText(selected.GetPath())

			statusLine.Clear()
			statusLine.SetText("Back to: " + selected.GetPath())
		}
	})

	navigationPanel.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
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
			}
		case tcell.KeyCtrlU:
			return tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone)
		case tcell.KeyCtrlD:
			return tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone)
		}

		return event
	})

	pages = tview.NewPages().
		AddPage(mainPage, rootFlex, true, true)
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
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q':
				app.Stop()
				return nil
			}
		}

		return event
	})
	app.SetFocus(navigationPanel)
	focus = app.GetFocus()

	load()

	if err := app.Run(); err != nil {
		panic(err)
	}
}

func load() {
	menuItems.Add(
		&MenuFolder{
			Name: "Netowrks",
		},
	)

	menuItems.Add(
		&MenuFolder{
			Name: "IPs",
		},
	)

	navigationPanel.Clear()
	for _, menuItem := range menuItems.GetChilds(nil) {
		navigationPanel.AddItem(menuItem.GetName(), menuItem.GetPath(), 0, nil)
	}
}
