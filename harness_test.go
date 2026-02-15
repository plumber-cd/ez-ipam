package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/plumber-cd/ez-ipam/internal/ui"
)

var updateGolden = flag.Bool("update", false, "update golden snapshot files")

// sentinelSyncKey must match the key checked in App.SetInputCapture.
const sentinelSyncKey = tcell.KeyF63

type TestHarness struct {
	t       *testing.T
	App     *ui.App
	screen  tcell.SimulationScreen
	repoDir string
	workDir string
	oldDir  string
	runErr  chan error
	once    sync.Once
}

func NewTestHarness(t *testing.T) *TestHarness {
	t.Helper()
	return NewTestHarnessInDir(t, t.TempDir())
}

func NewTestHarnessInDir(t *testing.T, dir string) *TestHarness {
	t.Helper()

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir work dir: %v", err)
	}

	application, err := ui.New(dir)
	if err != nil {
		t.Fatalf("init app: %v", err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("init simulation screen: %v", err)
	}
	screen.SetSize(80, 25)
	application.TviewApp.SetScreen(screen)

	h := &TestHarness{
		t:       t,
		App:     application,
		screen:  screen,
		repoDir: oldDir,
		workDir: dir,
		oldDir:  oldDir,
		runErr:  make(chan error, 1),
	}
	t.Cleanup(h.Close)

	go func() {
		h.runErr <- application.Run()
	}()

	h.WaitForDraw()
	return h
}

func (h *TestHarness) Close() {
	h.once.Do(func() {
		if h.App != nil {
			h.App.Stop()
		}
		select {
		case err := <-h.runErr:
			if err != nil {
				h.t.Fatalf("app run failed: %v", err)
			}
		case <-time.After(2 * time.Second):
		}
	})
}

func (h *TestHarness) WaitForDraw() {
	done := make(chan struct{})
	h.App.TviewApp.QueueUpdateDraw(func() {
		close(done)
	})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		h.t.Fatalf("timed out waiting for draw")
	}
}

func (h *TestHarness) PressKey(key tcell.Key, r rune, mod tcell.ModMask) {
	// Set up the sentinel channel on the event loop goroutine to avoid races.
	ready := make(chan struct{})
	done := make(chan struct{})
	h.App.TviewApp.QueueUpdate(func() {
		h.App.SentinelCh = done
		close(ready)
	})
	<-ready

	// Inject the key followed by a sentinel. Both go into the screen event
	// channel, so they are processed in order by the event loop.
	h.screen.InjectKey(key, r, mod)
	h.screen.InjectKey(sentinelSyncKey, 0, tcell.ModNone)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		h.t.Fatalf("timed out waiting for key processing")
	}
}

func (h *TestHarness) InjectKeyNoWait(key tcell.Key, r rune, mod tcell.ModMask) {
	h.screen.InjectKey(key, r, mod)
}

func (h *TestHarness) PressRune(r rune) {
	h.PressKey(tcell.KeyRune, r, tcell.ModNone)
}

func (h *TestHarness) TypeText(s string) {
	for _, r := range s {
		h.PressRune(r)
	}
}

func (h *TestHarness) PressEnter() {
	h.PressKey(tcell.KeyEnter, 0, tcell.ModNone)
}

func (h *TestHarness) PressEscape() {
	h.PressKey(tcell.KeyEscape, 0, tcell.ModNone)
}

func (h *TestHarness) PressBackspace() {
	h.PressKey(tcell.KeyBackspace2, 0, tcell.ModNone)
}

func (h *TestHarness) PressTab() {
	h.PressKey(tcell.KeyTab, 0, tcell.ModNone)
}

func (h *TestHarness) PressUp() {
	h.PressKey(tcell.KeyUp, 0, tcell.ModNone)
}

func (h *TestHarness) PressDown() {
	h.PressKey(tcell.KeyDown, 0, tcell.ModNone)
}

func (h *TestHarness) PressCtrl(r rune) {
	switch r {
	case 'c', 'C':
		h.PressKey(tcell.KeyCtrlC, 0, tcell.ModNone)
	case 'd', 'D':
		h.PressKey(tcell.KeyCtrlD, 0, tcell.ModNone)
	case 'q', 'Q':
		h.PressKey(tcell.KeyCtrlQ, 0, tcell.ModNone)
	case 's', 'S':
		h.PressKey(tcell.KeyCtrlS, 0, tcell.ModNone)
	case 'u', 'U':
		h.PressKey(tcell.KeyCtrlU, 0, tcell.ModNone)
	default:
		h.t.Fatalf("unsupported ctrl key: %q", r)
	}
}

func (h *TestHarness) NavigateToNetworks() {
	h.PressEnter()
	h.AssertScreenContains("│Networks")
}

func (h *TestHarness) ConfirmModal() {
	h.PressTab()
	h.PressEnter()
}

func (h *TestHarness) CancelModal() {
	h.PressEnter()
}

func (h *TestHarness) OpenAddNetworkDialog() {
	h.PressRune('n')
	h.AssertScreenContains("Add Network")
}

func (h *TestHarness) WaitForExit(timeout time.Duration) error {
	select {
	case err := <-h.runErr:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for app exit")
	}
}

func (h *TestHarness) GetScreenText() string {
	cells, width, height := h.screen.GetContents()
	var sb strings.Builder
	for row := range height {
		for col := range width {
			cell := cells[row*width+col]
			if len(cell.Runes) > 0 && cell.Runes[0] != 0 {
				sb.WriteRune(cell.Runes[0])
			} else {
				sb.WriteRune(' ')
			}
		}
		sb.WriteRune('\n')
	}
	return sb.String()
}

func (h *TestHarness) GetScreenRegion(x, y, w, hh int) string {
	cells, width, _ := h.screen.GetContents()
	var sb strings.Builder
	for row := y; row < y+hh; row++ {
		for col := x; col < x+w; col++ {
			cell := cells[row*width+col]
			if len(cell.Runes) > 0 && cell.Runes[0] != 0 {
				sb.WriteRune(cell.Runes[0])
			} else {
				sb.WriteRune(' ')
			}
		}
		sb.WriteRune('\n')
	}
	return sb.String()
}

func (h *TestHarness) DumpScreen() {
	h.t.Logf("\n%s", h.GetScreenText())
}

func (h *TestHarness) AssertScreenContains(substr string) {
	h.t.Helper()
	text := h.GetScreenText()
	if !strings.Contains(text, substr) {
		h.DumpScreen()
		h.t.Fatalf("screen does not contain %q", substr)
	}
}

func (h *TestHarness) AssertScreenNotContains(substr string) {
	h.t.Helper()
	text := h.GetScreenText()
	if strings.Contains(text, substr) {
		h.DumpScreen()
		h.t.Fatalf("screen unexpectedly contains %q", substr)
	}
}

func (h *TestHarness) AssertScreenNotContainsAny(substrs ...string) {
	h.t.Helper()
	text := h.GetScreenText()
	for _, substr := range substrs {
		if strings.Contains(text, substr) {
			h.DumpScreen()
			h.t.Fatalf("screen unexpectedly contains %q", substr)
		}
	}
}

func (h *TestHarness) AssertStatusContains(substr string) {
	h.t.Helper()
	h.AssertScreenContains(substr)
}

func (h *TestHarness) PressPageUp() {
	h.PressCtrl('u')
}

func (h *TestHarness) PressPageDown() {
	h.PressCtrl('d')
}

func (h *TestHarness) AssertNoModal() {
	h.t.Helper()
	done := make(chan bool, 1)
	h.App.TviewApp.QueueUpdateDraw(func() {
		name, _ := h.App.Pages.GetFrontPage()
		done <- name == "*main*"
	})
	if !<-done {
		h.t.Fatalf("expected no modal/dialog overlay; front page is not main")
	}
}

func (h *TestHarness) AssertGoldenSnapshot(name string) {
	h.t.Helper()
	actual := h.GetScreenText()
	goldenPath := filepath.Join(h.repoDir, "testdata", name+".golden")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			h.t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(actual), 0o644); err != nil {
			h.t.Fatalf("write golden file: %v", err)
		}
		return
	}

	expectedBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		h.t.Fatalf("read golden %s: %v (run with -args -update)", goldenPath, err)
	}
	expected := string(expectedBytes)
	if expected != actual {
		h.DumpScreen()
		h.t.Fatalf("golden mismatch for %s\n%s", name, renderLineDiff(expected, actual))
	}
}

func renderLineDiff(expected, actual string) string {
	exp := strings.Split(expected, "\n")
	act := strings.Split(actual, "\n")
	max := len(exp)
	if len(act) > max {
		max = len(act)
	}
	for i := range max {
		var e, a string
		if i < len(exp) {
			e = exp[i]
		}
		if i < len(act) {
			a = act[i]
		}
		if e != a {
			return fmt.Sprintf("first diff at line %d\nexpected: %q\nactual  : %q", i+1, e, a)
		}
	}
	return "content differs"
}

func (h *TestHarness) SaveDemoArtifactsToRepo() {
	h.t.Helper()
	srcData := filepath.Join(h.workDir, ".ez-ipam")
	srcMD := filepath.Join(h.workDir, "EZ-IPAM.md")
	dstData := filepath.Join(h.repoDir, ".ez-ipam")
	dstMD := filepath.Join(h.repoDir, "EZ-IPAM.md")

	if err := os.RemoveAll(dstData); err != nil {
		h.t.Fatalf("remove old data dir: %v", err)
	}
	if err := os.Remove(dstMD); err != nil && !os.IsNotExist(err) {
		h.t.Fatalf("remove old markdown: %v", err)
	}
	if err := copyDir(srcData, dstData); err != nil {
		h.t.Fatalf("copy data dir: %v", err)
	}
	md, err := os.ReadFile(srcMD)
	if err != nil {
		h.t.Fatalf("read source markdown: %v", err)
	}
	if err := os.WriteFile(dstMD, md, 0o644); err != nil {
		h.t.Fatalf("write markdown: %v", err)
	}
}

// CurrentFocusID returns the DisplayID of the currently focused item (thread-safe).
func (h *TestHarness) CurrentFocusID() string {
	done := make(chan string, 1)
	h.App.TviewApp.QueueUpdateDraw(func() {
		if h.App.CurrentFocus == nil {
			done <- ""
			return
		}
		done <- h.App.CurrentFocus.DisplayID()
	})
	return <-done
}

// CurrentKeys returns current menu and focus key lists (thread-safe).
func (h *TestHarness) CurrentKeys() ([]string, []string) {
	type result struct {
		menu  []string
		focus []string
	}
	done := make(chan result, 1)
	h.App.TviewApp.QueueUpdateDraw(func() {
		menuCopy := append([]string{}, h.App.CurrentMenuItemKeys...)
		focusCopy := append([]string{}, h.App.CurrentFocusKeys...)
		done <- result{menu: menuCopy, focus: focusCopy}
	})
	v := <-done
	return v.menu, v.focus
}

// CurrentStatusText returns the current status line text (thread-safe).
func (h *TestHarness) CurrentStatusText() string {
	done := make(chan string, 1)
	h.App.TviewApp.QueueUpdateDraw(func() {
		done <- h.App.StatusLine.GetText(true)
	})
	return <-done
}

// FocusMatches checks whether the current focus starts with the given id.
func (h *TestHarness) FocusMatches(id string) bool {
	return strings.HasPrefix(h.CurrentFocusID(), id)
}

// MoveFocusToID scrolls up then down until the named item is focused, or fails.
func (h *TestHarness) MoveFocusToID(t *testing.T, id string) {
	t.Helper()
	for range 40 {
		h.PressRune('k')
	}
	if h.FocusMatches(id) {
		return
	}
	for range 200 {
		if h.FocusMatches(id) {
			return
		}
		h.PressRune('j')
	}
	t.Fatalf("could not focus item %q; current=%q", id, h.CurrentFocusID())
}

func (h *TestHarness) SelectDropdownOption(label string, optionContains string) {
	h.t.Helper()
	if strings.TrimSpace(optionContains) == "" {
		return
	}
	done := make(chan bool, 1)
	h.App.TviewApp.QueueUpdateDraw(func() {
		done <- h.App.SelectDialogDropdownOption(label, optionContains)
	})
	if !<-done {
		h.t.Fatalf("dropdown %q option containing %q not found", label, optionContains)
	}
}

func (h *TestHarness) ToggleCheckbox() {
	h.PressRune(' ')
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
