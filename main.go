package main

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/pterm/pterm"
	"github.com/rivo/tview"
)

// Show a navigable tree view of the current directory.
func main() {
	root := tview.NewTreeNode("100.64.0.1/10").
		SetColor(tcell.ColorRed)
	tree := tview.NewTreeView().
		SetRoot(root).
		SetCurrentNode(root)

	add := func(target *tview.TreeNode, cidr string) {
		node := tview.NewTreeNode(cidr).
			SetReference(cidr).
			SetSelectable(true)
		if false {
			node.SetColor(tcell.ColorGreen)
		}
		target.AddChild(node)
	}

	// Add the current directory to the root node.
	add(root, "100.64.0.1/11")

	// If a directory was selected, open it.
	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		reference := node.GetReference()
		children := node.GetChildren()
		if len(children) == 0 {
			// Load and show files in this directory.
			cidr := reference.(string)
			add(node, cidr)
		} else {
			// Collapse if visible, expand if collapsed.
			node.SetExpanded(!node.IsExpanded())
		}
	})

	tree.SetBorder(true).SetTitle("Networks")

	app := tview.NewApplication()
	flex := tview.NewFlex().SetDirection(tview.FlexRow)

	positionLine := tview.NewTextView().SetText("Networks -> 100.64.0.1/10 -> 100.64.0.1/11")
	positionLine.SetBorder(true).SetTitle("Position")
	flex.AddItem(positionLine, 3, 1, false)

	navigationFlex := tview.NewFlex().SetDirection(tview.FlexColumn)

	navigationPanel := tree
	navigationFlex.AddItem(navigationPanel, 0, 2, true)

	usagePanel := tview.NewTextView()
	usagePanel.SetDynamicColors(true).SetScrollable(true).
		SetChangedFunc(func() {
			app.Draw()
		})
	usagePanel.SetBorder(true).SetTitle("Usage")
	barData := []pterm.Bar{
		{Label: "A", Value: 10},
		{Label: "B", Value: 20},
		{Label: "C", Value: 30},
		{Label: "D", Value: 40},
		{Label: "E", Value: 50},
		{Label: "F", Value: 40},
		{Label: "G", Value: 30},
		{Label: "H", Value: 20},
		{Label: "I", Value: 10},
		{Label: "J", Value: 100},
		{Label: "K", Value: 0},
		{Label: "L", Value: 0},
		{Label: "M", Value: 0},
		{Label: "N", Value: 0},
		{Label: "O", Value: 0},
		{Label: "P", Value: 0},
		{Label: "Q", Value: 0},
		{Label: "R", Value: 0},
		{Label: "S", Value: 0},
		{Label: "T", Value: 0},
		{Label: "U", Value: 0},
		{Label: "V", Value: 0},
		{Label: "W", Value: 0},
		{Label: "X", Value: 0},
		{Label: "Y", Value: 0},
		{Label: "Z", Value: 0},
		{Label: "AA", Value: 0},
		{Label: "AB", Value: 0},
		{Label: "AC", Value: 0},
		{Label: "AD", Value: 0},
		{Label: "AE", Value: 0},
	}
	go func() {
		for {
			barData[0].Value = (barData[0].Value + 1) % 100
			usagePanel.Clear()
			_ = pterm.DefaultBarChart.WithBars(barData).WithHorizontal().WithWidth(pterm.GetTerminalWidth()/2 - 11).WithShowValue().WithWriter(tview.ANSIWriter(usagePanel)).Render()
			time.Sleep(1 * time.Second)
		}
	}()
	navigationFlex.AddItem(usagePanel, 0, 4, false)

	detailsPanel := tview.NewTextView().SetText("Details")
	detailsPanel.SetBorder(true).SetTitle("Details")
	navigationFlex.AddItem(detailsPanel, 0, 2, false)

	flex.AddItem(navigationFlex, 0, 2, false)

	statusLine := tview.NewTextView().SetText("Status")
	statusLine.SetBorder(true).SetTitle("Status")
	flex.AddItem(statusLine, 3, 1, false)

	if err := app.SetRoot(flex, true).SetFocus(flex).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
