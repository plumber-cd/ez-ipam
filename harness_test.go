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
)

var updateGolden = flag.Bool("update", false, "update golden snapshot files")

const sentinelSyncKey = tcell.KeyF63

type TestHarness struct {
	t       *testing.T
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
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir work dir: %v", err)
	}

	resetState()
	setupApp()

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("init simulation screen: %v", err)
	}
	screen.SetSize(80, 25)
	app.SetScreen(screen)

	h := &TestHarness{
		t:       t,
		screen:  screen,
		repoDir: oldDir,
		workDir: dir,
		oldDir:  oldDir,
		runErr:  make(chan error, 1),
	}
	t.Cleanup(h.Close)

	go func() {
		h.runErr <- app.Run()
	}()

	h.WaitForDraw()
	return h
}

func (h *TestHarness) Close() {
	h.once.Do(func() {
		if app != nil {
			app.Stop()
		}
		select {
		case err := <-h.runErr:
			if err != nil {
				h.t.Fatalf("app run failed: %v", err)
			}
		case <-time.After(2 * time.Second):
		}
		_ = os.Chdir(h.oldDir)
	})
}

func (h *TestHarness) WaitForDraw() {
	done := make(chan struct{})
	app.QueueUpdateDraw(func() {
		close(done)
	})
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		h.t.Fatalf("timed out waiting for draw")
	}
}

func (h *TestHarness) PressKey(key tcell.Key, r rune, mod tcell.ModMask) {
	done := make(chan struct{})
	oldCapture := app.GetInputCapture()
	var once sync.Once
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == sentinelSyncKey {
			once.Do(func() {
				close(done)
			})
			return nil
		}
		if oldCapture != nil {
			return oldCapture(event)
		}
		return event
	})

	h.screen.InjectKey(key, r, mod)
	h.screen.InjectKey(sentinelSyncKey, 0, tcell.ModNone)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		app.SetInputCapture(oldCapture)
		h.t.Fatalf("timed out waiting for key processing")
	}
	app.SetInputCapture(oldCapture)
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
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
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

func (h *TestHarness) AssertStatusContains(substr string) {
	h.t.Helper()
	h.AssertScreenContains(substr)
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
	for i := 0; i < max; i++ {
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
	srcData := filepath.Join(h.workDir, dataDirName)
	srcMD := filepath.Join(h.workDir, markdownFileName)
	dstData := filepath.Join(h.repoDir, dataDirName)
	dstMD := filepath.Join(h.repoDir, markdownFileName)

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
