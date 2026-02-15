package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	helpPageName = "*help*"

	FormFieldWidth          = 42
	descriptionHint         = "Ctrl+E: edit in $EDITOR"
	maxDialogViewportHeight = 23

	LagModeDisabledOption = "Disabled"
	LagGroupSelfOption    = "Self"
	TaggedModeNoneOption  = "None"
	NoneVLANOption        = "<none>"
	DNSModeRecord         = "Record"
	DNSModeAlias          = "Alias"
)

var (
	GlobalKeys        = []string{"<q> Quit", "<ctrl+s> Save"}
	LagModeOptions    = []string{LagModeDisabledOption, "802.3ad"}
	TaggedModeOptions = []string{TaggedModeNoneOption, "AllowAll", "BlockAll", "Custom"}
	DNSModeOptions    = []string{DNSModeRecord, DNSModeAlias}
)

// portDialogValues captures port form field values during dialog construction and rebuild.
type portDialogValues struct {
	PortNumber       string
	Enabled          bool
	Name             string
	PortType         string
	Speed            string
	PoE              string
	LAGMode          string
	LAGGroup         string
	NativeVLANID     string
	TaggedMode       string
	TaggedVLANIDs    string
	DestinationNotes string
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

// dnsRecordDialogValues captures DNS record form values during dialog construction.
type dnsRecordDialogValues struct {
	FQDN           string
	Mode           string
	RecordType     string
	RecordValue    string
	ReservedIPPath string
	Description    string
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

	pendingDNSDeletesOnUnreserve []string
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
	if a.KeysLine == nil {
		return
	}

	mandatoryHelpKey := "<?> Help"
	keys := append(append(append([]string{}, GlobalKeys...), a.CurrentMenuItemKeys...), a.CurrentFocusKeys...)
	visibleKeys := append(append([]string{}, keys...), mandatoryHelpKey)
	text := " " + strings.Join(visibleKeys, " | ")

	_, _, innerWidth, _ := a.KeysLine.GetInnerRect()
	if innerWidth > 0 {
		for len(visibleKeys) > 1 && len(text) > innerWidth {
			visibleKeys = visibleKeys[:len(visibleKeys)-2]
			visibleKeys = append(visibleKeys, mandatoryHelpKey)
			text = " " + strings.Join(visibleKeys, " | ")
		}
		if len(visibleKeys) == 1 {
			text = " " + mandatoryHelpKey
		}
	}

	a.KeysLine.SetText(text)
}

func (a *App) showHelpPopup() {
	var content strings.Builder
	content.WriteString("Full keyboard shortcuts\n\n")
	content.WriteString("Navigation\n")
	content.WriteString("- h / Left Arrow: Back\n")
	content.WriteString("- j / Down Arrow: Move down\n")
	content.WriteString("- k / Up Arrow: Move up\n")
	content.WriteString("- l / Right Arrow / Enter: Open or select\n")
	content.WriteString("- Backspace: Go up one level\n")
	content.WriteString("- Ctrl+U: Page up\n")
	content.WriteString("- Ctrl+D: Page down\n\n")
	content.WriteString("Global\n")
	content.WriteString("- q: Quit (with confirmation)\n")
	content.WriteString("- Ctrl+S: Save\n")
	content.WriteString("- Ctrl+Q: Force quit\n")
	content.WriteString("- ?: Show this help\n\n")
	content.WriteString("Current context\n")
	for _, key := range a.CurrentMenuItemKeys {
		content.WriteString("- ")
		content.WriteString(key)
		content.WriteString("\n")
	}
	for _, key := range a.CurrentFocusKeys {
		content.WriteString("- ")
		content.WriteString(key)
		content.WriteString("\n")
	}

	helpText := tview.NewTextView().
		SetText(content.String()).
		SetScrollable(true).
		SetWrap(true).
		SetWordWrap(true)
	helpText.SetBorder(true).SetTitle("Keyboard Shortcuts (scroll: Up/Down, PgUp/PgDn)")
	helpText.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape, tcell.KeyEnter, tcell.KeyBS, tcell.KeyBackspace2:
			a.dismissHelpPopup()
			return nil
		}
		return event
	})

	dialogWidth := 58
	dialogHeight := 20

	a.Pages.RemovePage(helpPageName)
	a.Pages.AddPage(helpPageName, a.createDialogPage(helpText, dialogWidth, dialogHeight), true, true)
	a.Pages.ShowPage(helpPageName)
	a.TviewApp.SetFocus(helpText)
}

func (a *App) dismissHelpPopup() {
	a.Pages.RemovePage(helpPageName)
	a.Pages.SwitchToPage(mainPageName)
	a.TviewApp.SetFocus(a.NavPanel)
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
		if ok {
			itemLabel := strings.TrimSpace(dd.GetLabel())
			if itemLabel != label && !strings.HasPrefix(itemLabel, label) {
				continue
			}
			currentIdx, currentText := dd.GetCurrentOption()
			if strings.Contains(currentText, optionContains) {
				return true
			}
			for idx := range 32 {
				dd.SetCurrentOption(idx)
				_, optionText := dd.GetCurrentOption()
				if strings.Contains(optionText, optionContains) {
					return true
				}
			}
			dd.SetCurrentOption(currentIdx)
			return false
		}

		sd, ok := item.(*searchableDropdown)
		if !ok {
			continue
		}
		itemLabel := strings.TrimSpace(sd.GetLabel())
		if itemLabel != label && !strings.HasPrefix(itemLabel, label) {
			continue
		}
		for idx, option := range sd.options {
			if !strings.Contains(option, optionContains) {
				continue
			}
			sd.SetText(option)
			sd.lastValid = option
			sd.isOpen = false
			if sd.onSelected != nil {
				sd.onSelected(option, idx)
			}
			return true
		}
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

// searchableDropdown is an InputField with autocomplete-backed option selection.
type searchableDropdown struct {
	*tview.InputField
	options            []string
	lastValid          string
	allowEmpty         bool
	isOpen             bool
	autocompleteActive bool
	onSelected         func(text string, index int)
}

func newSearchableDropdown(label string, options []string, currentValue string, allowEmpty bool, onSelected func(string, int)) *searchableDropdown {
	input := tview.NewInputField().SetLabel(label).SetFieldWidth(FormFieldWidth)
	if allowEmpty {
		input.SetPlaceholder(NoneVLANOption)
	}
	input.SetText(currentValue)
	sd := &searchableDropdown{
		InputField: input,
		options:    append([]string(nil), options...),
		lastValid:  currentValue,
		allowEmpty: allowEmpty,
		onSelected: onSelected,
	}
	sd.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyDown, tcell.KeyUp, tcell.KeyPgDn, tcell.KeyPgUp, tcell.KeyRune, tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyDelete:
			sd.autocompleteActive = true
		case tcell.KeyEscape:
			sd.autocompleteActive = false
			sd.isOpen = false
		}
		return event
	})

	sd.SetAutocompleteFunc(func(currentText string) []string {
		if !sd.autocompleteActive {
			sd.isOpen = false
			return nil
		}
		query := strings.TrimSpace(currentText)
		if query == "" {
			sd.isOpen = len(sd.options) > 0
			return append([]string(nil), sd.options...)
		}
		q := strings.ToLower(query)
		filtered := make([]string, 0, len(sd.options))
		for _, option := range sd.options {
			if strings.Contains(strings.ToLower(option), q) {
				filtered = append(filtered, option)
			}
		}
		sd.isOpen = len(filtered) > 0
		return filtered
	})
	sd.SetAutocompletedFunc(func(text string, index int, source int) bool {
		switch source {
		case tview.AutocompletedNavigate:
			return false
		default:
			sd.SetText(text)
			sd.lastValid = text
			sd.isOpen = false
			sd.autocompleteActive = false
			if sd.onSelected != nil {
				sd.onSelected(text, index)
			}
			return true
		}
	})
	return sd
}

func (sd *searchableDropdown) SetFinishedFunc(handler func(key tcell.Key)) tview.FormItem {
	sd.InputField.SetFinishedFunc(func(key tcell.Key) {
		text := strings.TrimSpace(sd.GetText())
		switch {
		case sd.allowEmpty && text == "":
			sd.SetText("")
			sd.lastValid = ""
		case !slices.Contains(sd.options, text):
			sd.SetText(sd.lastValid)
		default:
			sd.lastValid = text
		}
		sd.isOpen = false
		sd.autocompleteActive = false
		if handler != nil {
			handler(key)
		}
	})
	return sd
}

func newHintedTextArea(label, text string, fieldWidth, fieldHeight int, hint string) *hintedTextArea { //nolint:unparam // fieldWidth currently passed as FormFieldWidth in all call sites; keep signature general for future dialogs
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
