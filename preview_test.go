package main

import (
	"encoding/json"
	"fmt"
	"github.com/Fiend3d/catatui"
	"mc/shutil"
	"os"
	"testing"
	"time"
)

// Opt-in export for visual review; ordinary test runs create no artifacts.
func TestExportNativePreview(t *testing.T) {
	path := os.Getenv("MC_RENDER_PREVIEW")
	if path == "" {
		t.Skip("set MC_RENDER_PREVIEW to export native cells")
	}
	m := testModel(t, `C:\Projects\mc`, `D:\Archive`)
	m.screenWidth, m.screenHeight = 110, 26
	m.panes[0].tabs = append(m.panes[0].tabs, newTab(`C:\Downloads`, &page{}))
	for i, name := range []string{"assets", "internal", "shutil", "widgets", "README.md", "go.mod", "main.go", "model.go", "runtime.go", "tasks.go", "view_native.go"} {
		m.panes[0].tabs[0].page.items = append(m.panes[0].tabs[0].page.items, &filepathItem{name: name, isDir: i < 4, size: uint64(2048 + i*512), selected: i == 4 || i == 5, modTime: time.Date(2026, 9, 6, 12, 0, 0, 0, time.Local)})
	}
	m.panes[0].tabs[0].getPageSettings().cursor = 4
	for _, name := range []string{"mc-v1", "screenshots", "backups"} {
		m.panes[1].tabs[0].page.items = append(m.panes[1].tabs[0].page.items, &filepathItem{name: name, isDir: true})
	}
	m.taskList = []*task{{id: 1, cmd: &fileActionCommand{action: copyFileAction, pairs: []pathPair{{src: `C:\Downloads\archive.zip`, dst: `D:\Archive\archive.zip`}}}, action: "execute", state: "running", progress: shutil.Progress{Bytes: 42 * 1024 * 1024, Total: 100 * 1024 * 1024, Files: 3, TotalFiles: 8}}}
	backend := catatui.NewTestBackend(110, 26)
	terminal, _ := catatui.NewTerminal(backend)
	if err := terminal.Draw(m.draw); err != nil {
		t.Fatal(err)
	}
	type cell struct{ Symbol, Fg, Bg string }
	cells := make([]cell, len(backend.Buffer().Content))
	rgb := func(c catatui.Color, fallback string) string {
		r, g, b, ok := c.RGB()
		if !ok {
			return fallback
		}
		return fmt.Sprintf("#%02x%02x%02x", r, g, b)
	}
	for i, c := range backend.Buffer().Content {
		cells[i] = cell{c.GetSymbol(), rgb(c.Fg, "#ffffff"), rgb(c.Bg, "#222430")}
		if c.Modifier.Contains(catatui.ModifierReversed) {
			cells[i].Fg, cells[i].Bg = cells[i].Bg, cells[i].Fg
		}
	}
	data, err := json.Marshal(struct {
		Width, Height int
		Cells         []cell
	}{110, 26, cells})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
