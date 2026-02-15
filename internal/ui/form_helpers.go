package ui

import (
	"fmt"
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

func getSearchableDropdownValue(form *tview.Form, label, fallback string) string {
	formItem, ok := getFormItemByLabelIfPresent(form, label)
	if !ok {
		return fallback
	}
	if dropdown, ok := formItem.(*tview.DropDown); ok {
		_, value := dropdown.GetCurrentOption()
		value = strings.TrimSpace(value)
		if value == "" {
			return fallback
		}
		return value
	}
	searchable, ok := formItem.(*searchableDropdown)
	if !ok {
		return getTextFromInputFieldIfPresent(form, label)
	}
	value := strings.TrimSpace(searchable.GetText())
	if value == "" {
		return fallback
	}
	return value
}

func getCheckboxValueIfPresent(form *tview.Form, label string, fallback bool) bool {
	formItem, ok := getFormItemByLabelIfPresent(form, label)
	if !ok {
		return fallback
	}
	checkbox, ok := formItem.(*tview.Checkbox)
	if !ok {
		panic("Failed to cast " + label + " checkbox")
	}
	return checkbox.IsChecked()
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

// capturePortFormValues reads all port form fields and returns them as a value struct.
func capturePortFormValues(form *tview.Form) portDialogValues {
	return portDialogValues{
		PortNumber:       getTextFromInputFieldIfPresent(form, "Port Number"),
		Enabled:          getCheckboxValueIfPresent(form, "Enabled", true),
		Name:             getTextFromInputFieldIfPresent(form, "Name"),
		PortType:         getTextFromInputFieldIfPresent(form, "Port Type"),
		Speed:            getTextFromInputFieldIfPresent(form, "Speed"),
		PoE:              getTextFromInputFieldIfPresent(form, "PoE"),
		LAGMode:          normalizeLagModeOption(getSearchableDropdownValue(form, "LAG Mode", LagModeDisabledOption)),
		LAGGroup:         parseLAGGroupFromDropdownOption(getSearchableDropdownValue(form, "LAG Group", "")),
		NativeVLANID:     parseVLANIDFromDropdownOption(getSearchableDropdownValue(form, "Native VLAN ID", NoneVLANOption)),
		TaggedMode:       normalizeTaggedModeOption(getSearchableDropdownValue(form, "Tagged VLAN Mode", TaggedModeNoneOption)),
		TaggedVLANIDs:    collectCheckedVLANIDsFromForm(form),
		DestinationNotes: getTextFromTextAreaIfPresent(form, "Destination Notes"),
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

// getVLANDropdownOptions returns all selectable VLAN dropdown options.
func (a *App) getVLANDropdownOptions() []string {
	vlans := a.getVLANOptions()
	options := make([]string, 0, len(vlans))
	for _, v := range vlans {
		options = append(options, v.label)
	}
	return options
}

// findVLANDropdownOption finds the matching dropdown option for a VLAN ID string.
func findVLANDropdownOption(options []string, vlanIDStr string) string {
	vlanIDStr = strings.TrimSpace(vlanIDStr)
	if vlanIDStr == "" {
		return ""
	}
	for _, option := range options {
		parts := strings.SplitN(option, " ", 2)
		if len(parts) > 0 && parts[0] == vlanIDStr {
			return option
		}
	}
	return ""
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

// getLAGGroupDropdownOptions returns Self plus existing LAG master ports.
func (a *App) getLAGGroupDropdownOptions(parent *domain.Equipment, currentPortNumber string) []string {
	options := []string{LagGroupSelfOption}
	for _, child := range a.Catalog.GetChildren(parent) {
		port, ok := child.(*domain.Port)
		if !ok || port.ID == currentPortNumber {
			continue
		}
		// Only existing masters are valid member targets.
		if strings.TrimSpace(port.LAGMode) == "" || port.LAGGroup != port.Number() || port.Disabled {
			continue
		}
		if strings.TrimSpace(port.Name) != "" {
			options = append(options, port.ID+": "+port.Name)
			continue
		}
		options = append(options, port.ID)
	}
	return options
}

// findLAGGroupDropdownOption finds the matching dropdown option for a lag group value.
func findLAGGroupDropdownOption(options []string, lagGroupStr string) string {
	lagGroupStr = strings.TrimSpace(lagGroupStr)
	if lagGroupStr == "" || strings.EqualFold(lagGroupStr, "self") {
		return LagGroupSelfOption
	}
	for _, option := range options {
		if parseLAGGroupFromDropdownOption(option) == lagGroupStr {
			return option
		}
	}
	return LagGroupSelfOption
}

// parseLAGGroupFromDropdownOption extracts "self" or port number from dropdown option.
func parseLAGGroupFromDropdownOption(option string) string {
	option = strings.TrimSpace(option)
	if option == "" {
		return ""
	}
	if option == LagGroupSelfOption {
		return "self"
	}
	parts := strings.SplitN(option, ":", 2)
	return strings.TrimSpace(parts[0])
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

// getZoneDropdownOptions returns all selectable zone dropdown options.
func (a *App) getZoneDropdownOptions() []string {
	names := a.getZoneNames()
	options := make([]string, 0, len(names))
	options = append(options, names...)
	return options
}

// findZoneDropdownOption finds the matching dropdown option for a zone name.
func findZoneDropdownOption(options []string, zoneName string) string {
	zoneName = strings.TrimSpace(zoneName)
	if zoneName == "" {
		return ""
	}
	for _, option := range options {
		if option == zoneName {
			return option
		}
	}
	return ""
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

func formatReservedIPDisplayOption(ip *domain.IP) string {
	if ip == nil {
		return ""
	}
	base := fmt.Sprintf("%s (%s)", ip.ID, ip.DisplayName)
	if strings.TrimSpace(ip.MACAddress) != "" {
		return fmt.Sprintf("%s (%s %s)", ip.ID, ip.DisplayName, ip.MACAddress)
	}
	return base
}

// getReservedIPDropdownOptions returns all reserved IP choices and their paths.
func (a *App) getReservedIPDropdownOptions() (options []string, paths []string) {
	type optionRow struct {
		option string
		path   string
	}
	rows := []optionRow{}
	for _, item := range a.Catalog.All() {
		ip, ok := item.(*domain.IP)
		if !ok {
			continue
		}
		rows = append(rows, optionRow{
			option: formatReservedIPDisplayOption(ip),
			path:   ip.GetPath(),
		})
	}
	slices.SortFunc(rows, func(a, b optionRow) int {
		return strings.Compare(strings.ToLower(a.option), strings.ToLower(b.option))
	})
	options = make([]string, 0, len(rows))
	paths = make([]string, 0, len(rows))
	for _, row := range rows {
		options = append(options, row.option)
		paths = append(paths, row.path)
	}
	return options, paths
}

func findReservedIPDropdownOption(options, paths []string, reservedIPPath string) string {
	for i, path := range paths {
		if path == strings.TrimSpace(reservedIPPath) && i < len(options) {
			return options[i]
		}
	}
	return ""
}
