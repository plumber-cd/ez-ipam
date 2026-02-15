package ui

import (
	"slices"
	"strconv"
	"strings"

	"github.com/plumber-cd/ez-ipam/internal/domain"
	"github.com/rivo/tview"
)

// getFormItemByLabel finds a form item by its label (supports prefix matching for decorated labels).
func getFormItemByLabel(form *tview.Form, label string) tview.FormItem {
	formItemIndex := form.GetFormItemIndex(label)
	if formItemIndex < 0 {
		for i := range form.GetFormItemCount() {
			formItem := form.GetFormItem(i)
			if formItem == nil {
				continue
			}
			if strings.HasPrefix(formItem.GetLabel(), label) {
				formItemIndex = i
				break
			}
		}
	}
	if formItemIndex < 0 {
		panic("Failed to find " + label + " form item index")
	}

	formItem := form.GetFormItem(formItemIndex)
	if formItem == nil {
		panic("Failed to find " + label + " form item")
	}

	return formItem
}

// getFormItemByLabelIfPresent finds a form item by its label, returning false if not found.
func getFormItemByLabelIfPresent(form *tview.Form, label string) (tview.FormItem, bool) {
	formItemIndex := form.GetFormItemIndex(label)
	if formItemIndex < 0 {
		for i := range form.GetFormItemCount() {
			formItem := form.GetFormItem(i)
			if formItem == nil {
				continue
			}
			if strings.HasPrefix(formItem.GetLabel(), label) {
				formItemIndex = i
				break
			}
		}
	}
	if formItemIndex < 0 {
		return nil, false
	}

	formItem := form.GetFormItem(formItemIndex)
	if formItem == nil {
		return nil, false
	}
	return formItem, true
}

func getAndClearTextFromInputField(form *tview.Form, label string) string {
	formItem := getFormItemByLabel(form, label)

	inputField, ok := formItem.(*tview.InputField)
	if !ok {
		panic("Failed to cast " + label + " input field")
	}

	text := inputField.GetText()
	inputField.SetText("")

	return text
}

func setTextFromInputField(form *tview.Form, label, value string) {
	formItem := getFormItemByLabel(form, label)

	inputField, ok := formItem.(*tview.InputField)
	if !ok {
		panic("Failed to cast " + label + " input field")
	}

	inputField.SetText(value)
}

func getAndClearTextFromTextArea(form *tview.Form, label string) string { //nolint:unparam // label is always "Description" today but the function is general-purpose
	formItem := getFormItemByLabel(form, label)

	textArea, ok := formItem.(*hintedTextArea)
	if !ok {
		panic("Failed to cast " + label + " text area")
	}

	text := textArea.GetText()
	textArea.SetText("", false)

	return text
}

func setTextFromTextArea(form *tview.Form, label, value string) { //nolint:unparam // label is always "Description" today but the function is general-purpose
	formItem := getFormItemByLabel(form, label)

	textArea, ok := formItem.(*hintedTextArea)
	if !ok {
		panic("Failed to cast " + label + " text area")
	}

	textArea.SetText(value, true)
}

func getTextFromInputFieldIfPresent(form *tview.Form, label string) string {
	formItem, ok := getFormItemByLabelIfPresent(form, label)
	if !ok {
		return ""
	}
	inputField, ok := formItem.(*tview.InputField)
	if !ok {
		panic("Failed to cast " + label + " input field")
	}
	return inputField.GetText()
}

func getTextFromTextAreaIfPresent(form *tview.Form, label string) string {
	formItem, ok := getFormItemByLabelIfPresent(form, label)
	if !ok {
		return ""
	}
	textArea, ok := formItem.(*hintedTextArea)
	if !ok {
		panic("Failed to cast " + label + " text area")
	}
	return textArea.GetText()
}

func getDropDownOptionIfPresent(form *tview.Form, label, fallback string) string {
	formItem, ok := getFormItemByLabelIfPresent(form, label)
	if !ok {
		return fallback
	}
	dropdown, ok := formItem.(*tview.DropDown)
	if !ok {
		panic("Failed to cast " + label + " dropdown")
	}
	_, option := dropdown.GetCurrentOption()
	if option == "" {
		return fallback
	}
	return option
}

func normalizeLagModeOption(value string) string {
	if strings.TrimSpace(value) == "" {
		return LagModeDisabledOption
	}
	return value
}

func normalizeTaggedModeOption(value string) string {
	if strings.TrimSpace(value) == "" {
		return TaggedModeNoneOption
	}
	return value
}

func findOptionIndex(options []string, value string) int {
	for i, option := range options {
		if option == value {
			return i
		}
	}
	return 0
}

// capturePortFormValues reads all port form fields and returns them as a value struct.
func capturePortFormValues(form *tview.Form) portDialogValues {
	return portDialogValues{
		PortNumber:    getTextFromInputFieldIfPresent(form, "Port Number"),
		Name:          getTextFromInputFieldIfPresent(form, "Name"),
		PortType:      getTextFromInputFieldIfPresent(form, "Port Type"),
		Speed:         getTextFromInputFieldIfPresent(form, "Speed"),
		PoE:           getTextFromInputFieldIfPresent(form, "PoE"),
		LAGMode:       normalizeLagModeOption(getDropDownOptionIfPresent(form, "LAG Mode", LagModeDisabledOption)),
		LAGGroup:      getTextFromInputFieldIfPresent(form, "LAG Group"),
		NativeVLANID:  parseVLANIDFromDropdownOption(getDropDownOptionIfPresent(form, "Native VLAN ID", NoneVLANOption)),
		TaggedMode:    normalizeTaggedModeOption(getDropDownOptionIfPresent(form, "Tagged VLAN Mode", TaggedModeNoneOption)),
		TaggedVLANIDs: collectCheckedVLANIDsFromForm(form),
		Description:   getTextFromTextAreaIfPresent(form, "Description"),
	}
}

func computeFormDialogHeight(form *tview.Form) int {
	itemCount := form.GetFormItemCount()
	totalItemHeight := 0
	for i := range itemCount {
		itemHeight := form.GetFormItem(i).GetFieldHeight()
		if itemHeight <= 0 {
			itemHeight = tview.DefaultFormFieldHeight
		}
		totalItemHeight += itemHeight
	}

	paddingBetweenItems := 0
	if itemCount > 1 {
		paddingBetweenItems = itemCount - 1
	}

	buttonRows := 0
	if form.GetButtonCount() > 0 {
		buttonRows = 2
	}

	borderRows := 2
	paddingRows := 2
	totalRows := borderRows + paddingRows + totalItemHeight + paddingBetweenItems + buttonRows
	return min(totalRows, maxDialogViewportHeight)
}

func computeFormDialogWidth(form *tview.Form) int {
	maxLabelWidth := 0
	for i := range form.GetFormItemCount() {
		formItem := form.GetFormItem(i)
		if formItem == nil {
			continue
		}
		labelWidth := tview.TaggedStringWidth(formItem.GetLabel())
		if labelWidth > maxLabelWidth {
			maxLabelWidth = labelWidth
		}
	}

	return 2 + 2 + maxLabelWidth + 1 + FormFieldWidth
}

// ---------- VLAN / Zone option helpers ----------

// vlanOption holds a VLAN's data for building dropdowns and checkboxes.
type vlanOption struct {
	id    int
	label string
}

// getVLANOptions returns all VLANs from the catalog, sorted.
func (a *App) getVLANOptions() []vlanOption {
	vlansFolder := a.Catalog.GetByParentAndDisplayID(nil, domain.FolderVLANs)
	if vlansFolder == nil {
		return nil
	}
	var options []vlanOption
	for _, item := range a.Catalog.GetChildren(vlansFolder) {
		vlan, ok := item.(*domain.VLAN)
		if !ok {
			continue
		}
		id, err := strconv.Atoi(vlan.ID)
		if err != nil {
			continue
		}
		options = append(options, vlanOption{id: id, label: vlan.DisplayID()})
	}
	return options
}

// getVLANDropdownOptions returns a dropdown option list with <none> as the first entry.
func (a *App) getVLANDropdownOptions() []string {
	vlans := a.getVLANOptions()
	options := make([]string, 0, len(vlans)+1)
	options = append(options, NoneVLANOption)
	for _, v := range vlans {
		options = append(options, v.label)
	}
	return options
}

// findVLANDropdownIndex finds the dropdown index for a given VLAN ID string.
func findVLANDropdownIndex(options []string, vlanIDStr string) int {
	vlanIDStr = strings.TrimSpace(vlanIDStr)
	if vlanIDStr == "" {
		return 0
	}
	for i, option := range options {
		parts := strings.SplitN(option, " ", 2)
		if len(parts) > 0 && parts[0] == vlanIDStr {
			return i
		}
	}
	return 0
}

// parseVLANIDFromDropdownOption extracts the VLAN ID string from a dropdown option.
// Returns "" for <none> or unrecognized options.
func parseVLANIDFromDropdownOption(option string) string {
	if option == NoneVLANOption || option == "" {
		return ""
	}
	parts := strings.SplitN(option, " ", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// getZoneNames returns all zone display names from the catalog.
func (a *App) getZoneNames() []string {
	zonesFolder := a.Catalog.GetByParentAndDisplayID(nil, domain.FolderZones)
	if zonesFolder == nil {
		return nil
	}
	var names []string
	for _, item := range a.Catalog.GetChildren(zonesFolder) {
		zone, ok := item.(*domain.Zone)
		if !ok {
			continue
		}
		names = append(names, zone.DisplayName)
	}
	return names
}

// getZoneDropdownOptions returns a dropdown option list with <none> as first entry.
func (a *App) getZoneDropdownOptions() []string {
	names := a.getZoneNames()
	options := make([]string, 0, len(names)+1)
	options = append(options, NoneVLANOption)
	options = append(options, names...)
	return options
}

// findZoneDropdownIndex finds the dropdown index for a given zone name.
func findZoneDropdownIndex(options []string, zoneName string) int {
	zoneName = strings.TrimSpace(zoneName)
	if zoneName == "" {
		return 0
	}
	for i, option := range options {
		if option == zoneName {
			return i
		}
	}
	return 0
}

// parseZoneFromDropdownOption returns zone name or "" for <none>.
func parseZoneFromDropdownOption(option string) string {
	option = strings.TrimSpace(option)
	if option == "" || option == NoneVLANOption {
		return ""
	}
	return option
}

// getZoneContainingVLAN returns the zone name that contains the given VLAN ID.
func (a *App) getZoneContainingVLAN(vlanID int) string {
	zonesFolder := a.Catalog.GetByParentAndDisplayID(nil, domain.FolderZones)
	if zonesFolder == nil {
		return ""
	}
	for _, item := range a.Catalog.GetChildren(zonesFolder) {
		zone, ok := item.(*domain.Zone)
		if !ok {
			continue
		}
		for _, id := range zone.VLANIDs {
			if id == vlanID {
				return zone.DisplayName
			}
		}
	}
	return ""
}

// collectCheckedVLANIDsFromForm reads checkbox form items and returns a CSV of checked VLAN IDs.
// It identifies VLAN checkboxes by parsing numeric prefixes from their labels.
func collectCheckedVLANIDsFromForm(form *tview.Form) string {
	var ids []string
	for i := range form.GetFormItemCount() {
		item := form.GetFormItem(i)
		cb, ok := item.(*tview.Checkbox)
		if !ok || !cb.IsChecked() {
			continue
		}
		label := strings.TrimSpace(cb.GetLabel())
		parts := strings.SplitN(label, " ", 2)
		if len(parts) > 0 {
			if _, err := strconv.Atoi(parts[0]); err == nil {
				ids = append(ids, parts[0])
			}
		}
	}
	return strings.Join(ids, ",")
}

// parseVLANIDSet parses a comma-separated VLAN ID string into a set.
func parseVLANIDSet(csv string) map[int]bool {
	result := make(map[int]bool)
	for _, part := range strings.Split(csv, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.Atoi(part)
		if err == nil {
			result[id] = true
		}
	}
	return result
}

// buildVLANIDsCSV converts a set of selected VLAN IDs to a sorted CSV string.
func buildVLANIDsCSV(selected map[int]bool) string {
	var ids []int
	for id, checked := range selected {
		if checked {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	strs := make([]string, 0, len(ids))
	for _, id := range ids {
		strs = append(strs, strconv.Itoa(id))
	}
	return strings.Join(strs, ",")
}
