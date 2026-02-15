package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/plumber-cd/ez-ipam/internal/domain"
	"github.com/plumber-cd/ez-ipam/internal/export"
	"github.com/plumber-cd/ez-ipam/internal/store"
)

const (
	mainPageName = "*main*"
	quitPageName = "*quit*"

	FormFieldWidth          = 42
	descriptionHint         = "Ctrl+E: edit in $EDITOR"
	maxDialogViewportHeight = 23

	LagModeDisabledOption = "Disabled"
	TaggedModeNoneOption  = "None"
	NoneVLANOption        = "<none>"
)

var (
	GlobalKeys        = []string{"<q> Quit", "<ctrl+s> Save"}
	LagModeOptions    = []string{LagModeDisabledOption, "802.3ad"}
	TaggedModeOptions = []string{TaggedModeNoneOption, "AllowAll", "BlockAll", "Custom"}
)

// portDialogValues captures port form field values during dialog construction and rebuild.
type portDialogValues struct {
	PortNumber    string
	Name          string
	PortType      string
	Speed         string
	PoE           string
	LAGMode       string
	LAGGroup      string
	NativeVLANID  string
	TaggedMode    string
	TaggedVLANIDs string
	Description   string
}

// vlanDialogValues captures VLAN form field values during dialog construction.
type vlanDialogValues struct {
	VLANIDText   string
	Name         string
	Description  string
	SelectedZone string
}

// zoneDialogValues captures zone form field values during dialog construction.
type zoneDialogValues struct {
	Name          string
	Description   string
	SelectedVLANs map[int]bool
}

// networkAllocDialogValues captures network allocation form field values.
type networkAllocDialogValues struct {
	Name           string
	Description    string
	VLANID         string // numeric VLAN ID as string, or "" for none
	ChildPrefixLen string // only for subnets mode
}

// App holds all UI state for the EZ-IPAM application.
type App struct {
	Catalog *domain.Catalog
	WorkDir string

	TviewApp *tview.Application
	Pages    *tview.Pages

	// Layout widgets.
	PositionLine *tview.TextView
	NavPanel     *tview.List
	DetailsPanel *tview.TextView
	StatusLine   *tview.TextView
	KeysLine     *tview.TextView
	DetailsFlex  *tview.Flex

	// Navigation state.
	CurrentItem         domain.Item
	CurrentFocus        domain.Item
	CurrentMenuItemKeys []string
	CurrentFocusKeys    []string

	// mouseSelectArmed distinguishes single-click (highlight) from double-click
	// (navigate). tview translates DoubleClick -> Click before calling
	// SetSelectedFunc, so we suppress the single click's selected callback
	// and only act on the double-click.
	mouseSelectArmed bool

	// Quit dialog reference.
	quitDialog *tview.Modal

	// dialogForms maps page names to their forms for static dialogs created at init.
	dialogForms map[string]*tview.Form

	// Test synchronization: if non-nil, closed when a sentinel key is received.
	SentinelCh chan struct{}
}

// New creates a new App, loads state from dir, and sets up the UI.
func New(dir string) (*App, error) {
	a := &App{
		WorkDir:          dir,
		mouseSelectArmed: true,
		dialogForms:      make(map[string]*tview.Form),
	}

	catalog, err := store.Load(a.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("load data: %w", err)
	}
	a.Catalog = catalog
	a.setupLayout()
	a.ReloadMenu(nil)

	return a, nil
}

// Run starts the tview application loop.
func (a *App) Run() error {
	return a.TviewApp.Run()
}

// Stop stops the application.
func (a *App) Stop() {
	if a.TviewApp != nil {
		a.TviewApp.Stop()
	}
}

// setStatus updates the status line text.
func (a *App) setStatus(text string) {
	a.StatusLine.Clear()
	a.StatusLine.SetText(text)
}

// Save persists data to YAML files and renders the markdown report.
func (a *App) Save() {
	if err := store.Save(a.WorkDir, a.Catalog); err != nil {
		a.setStatus("Error saving data: " + err.Error())
		return
	}

	md, err := export.RenderMarkdown(a.Catalog)
	if err != nil {
		a.setStatus("Error rendering markdown: " + err.Error())
		return
	}
	mdPath := filepath.Join(a.WorkDir, store.MarkdownFileName)
	if err := os.WriteFile(mdPath, []byte(md), 0644); err != nil {
		a.setStatus("Error writing markdown: " + err.Error())
		return
	}

	a.setStatus("Saved to .ez-ipam/ and EZ-IPAM.md")
}

// openInExternalEditor opens the text in $EDITOR and returns the result.
func (a *App) openInExternalEditor(currentText string) (string, error) {
	tmpFile, err := os.CreateTemp("", "ez-ipam-*.txt")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.WriteString(currentText); err != nil {
		_ = tmpFile.Close()
		return "", err
	}
	if err := tmpFile.Close(); err != nil {
		return "", err
	}

	editor := strings.TrimSpace(os.Getenv("VISUAL"))
	if editor == "" {
		editor = strings.TrimSpace(os.Getenv("EDITOR"))
	}
	if editor == "" {
		editor = "vi"
	}

	var runErr error
	ok := a.TviewApp.Suspend(func() {
		cmd := exec.Command("sh", "-c", editor+` "$@"`, "ez-ipam-editor", tmpFile.Name())
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		runErr = cmd.Run()
	})
	if !ok {
		return "", fmt.Errorf("failed to suspend terminal UI")
	}
	if runErr != nil {
		return "", runErr
	}

	updatedText, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", err
	}
	return string(updatedText), nil
}

// ReloadMenu rebuilds the navigation list, optionally preserving focus on focusedItem.
func (a *App) ReloadMenu(focusedItem domain.Item) {
	a.NavPanel.Clear()

	newMenuItems := a.Catalog.GetChildren(a.CurrentItem)
	fromIndex := -1
	for i, item := range newMenuItems {
		if focusedItem != nil && focusedItem.DisplayID() == item.DisplayID() {
			fromIndex = i
		}
		a.NavPanel.AddItem(item.DisplayID(), item.GetPath(), 0, nil)
	}

	if fromIndex >= 0 {
		a.NavPanel.SetCurrentItem(fromIndex)
	}
}

// UpdateKeysLine refreshes the keyboard shortcuts help line.
func (a *App) UpdateKeysLine() {
	a.KeysLine.Clear()
	a.KeysLine.SetText(" " + strings.Join(append(append(GlobalKeys, a.CurrentMenuItemKeys...), a.CurrentFocusKeys...), " | "))
}

// resizeStatusLine adjusts the status panel height to fit its text content.
func (a *App) resizeStatusLine() {
	if a.StatusLine == nil || a.DetailsFlex == nil {
		return
	}

	_, _, innerWidth, _ := a.StatusLine.GetInnerRect()
	if innerWidth <= 0 {
		a.DetailsFlex.ResizeItem(a.StatusLine, 3, 0)
		return
	}

	text := a.StatusLine.GetText(false)
	requiredLines := wrappedLineCount(text, innerWidth)
	height := requiredLines + 2 // top and bottom border
	if height < 3 {
		height = 3
	}
	a.DetailsFlex.ResizeItem(a.StatusLine, height, 0)
}

// wrappedLineCount returns the number of visual lines after word wrapping.
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

// SelectDialogDropdownOption selects an option in the currently visible dialog form.
// It matches the dropdown by label prefix and option by substring containment.
func (a *App) SelectDialogDropdownOption(label, optionContains string) bool {
	_, front := a.Pages.GetFrontPage()
	if front == nil {
		return false
	}
	form := findFormInPrimitive(front)
	if form == nil {
		return false
	}
	for i := range form.GetFormItemCount() {
		item := form.GetFormItem(i)
		dd, ok := item.(*tview.DropDown)
		if !ok {
			continue
		}
		itemLabel := strings.TrimSpace(dd.GetLabel())
		if itemLabel != label && !strings.HasPrefix(itemLabel, label) {
			continue
		}
		currentIdx, currentText := dd.GetCurrentOption()
		if strings.Contains(currentText, optionContains) {
			return true
		}
		for idx := 0; idx < 32; idx++ {
			dd.SetCurrentOption(idx)
			_, optionText := dd.GetCurrentOption()
			if strings.Contains(optionText, optionContains) {
				return true
			}
		}
		dd.SetCurrentOption(currentIdx)
		return false
	}
	return false
}

// hintedTextArea keeps a textarea and its hint in one FormItem.
type hintedTextArea struct {
	*tview.TextArea
	hint       string
	labelWidth int
}

func newHintedTextArea(label, text string, fieldWidth, fieldHeight int, hint string) *hintedTextArea { //nolint:unparam // label is always "Description" today but the function is general-purpose
	textArea := tview.NewTextArea().SetLabel(label).SetSize(fieldHeight, fieldWidth)
	textArea.SetText(text, false)
	return &hintedTextArea{
		TextArea: textArea,
		hint:     hint,
	}
}

func (h *hintedTextArea) GetFieldHeight() int {
	return h.TextArea.GetFieldHeight()
}

func (h *hintedTextArea) SetFormAttributes(labelWidth int, labelColor, bgColor, fieldTextColor, fieldBgColor tcell.Color) tview.FormItem {
	h.labelWidth = labelWidth
	h.TextArea.SetFormAttributes(labelWidth, labelColor, bgColor, fieldTextColor, fieldBgColor)
	return h
}

func (h *hintedTextArea) Draw(screen tcell.Screen) {
	x, y, width, height := h.GetRect()
	if height <= 0 {
		return
	}
	h.SetRect(x, y, width, max(height-1, 0))
	h.TextArea.Draw(screen)

	fieldX := x + h.labelWidth
	fieldW := max(width-h.labelWidth, 0)
	tview.Print(screen, h.hint, fieldX, y+height-1, fieldW, tview.AlignLeft, tcell.ColorGray)
}
