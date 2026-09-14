package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestTerminalTitle(t *testing.T) {
	for _, dir := range []string{"", `C:\projects\mc`, `C:\`, `\\server\share`, `C:\проект\日本語`} {
		want := "mc"
		if dir != "" {
			want += " - " + dir
		}
		if got := terminalTitle(dir); got != want {
			t.Errorf("title(%q) = %q, want %q", dir, got, want)
		}
	}
	if got := terminalTitle("C:x\x00\x1b\n"); got != "mc - C:x" {
		t.Errorf("unsafe title: %q", got)
	}
}

func TestTerminalTitleLifecycle(t *testing.T) {
	var writes []string
	state := newTerminalTitleState(func() (string, error) { return "PowerShell", nil }, func(s string) error { writes = append(writes, s); return nil })
	state.update(`C:\left`)
	state.update(`C:\left`)
	state.update(`C:\right`)
	state.update("")
	state.restore()  // tool handoff
	state.update("") // tool returned, same path must still reapply
	state.restore()  // exit or error cleanup
	state.restore()  // cleanup still restores if a child changed the title before restart failed
	want := []string{`mc - C:\left`, `mc - C:\right`, "mc", "PowerShell", "mc", "PowerShell", "PowerShell"}
	if !reflect.DeepEqual(writes, want) {
		t.Fatalf("writes = %q, want %q", writes, want)
	}
}

func TestTerminalTitleFailures(t *testing.T) {
	calls := 0
	state := newTerminalTitleState(func() (string, error) { return "", errors.New("unavailable") }, func(string) error { calls++; return errors.New("unavailable") })
	state.update(`C:\left`)
	state.update(`C:\left`)
	state.restore()
	if calls != 2 || state.applied {
		t.Fatalf("failed writes suppressed retries or restored unknown title: %+v", state)
	}
}
