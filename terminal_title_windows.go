package main

import (
	"runtime"
	"strings"
	"unicode"
	"unsafe"

	"golang.org/x/sys/windows"
)

var titleKernel32 = windows.NewLazySystemDLL("kernel32.dll")
var getConsoleTitleW = titleKernel32.NewProc("GetConsoleTitleW")
var setConsoleTitleW = titleKernel32.NewProc("SetConsoleTitleW")

func readConsoleTitle() (string, error) {
	// Console titles are limited to fewer than 64K characters.
	buffer := make([]uint16, 65536)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	_, _, _ = titleKernel32.NewProc("SetLastError").Call(0)
	n, _, err := getConsoleTitleW.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if n == 0 && err != windows.ERROR_SUCCESS {
		return "", err
	}
	return windows.UTF16ToString(buffer), nil
}

func writeConsoleTitle(title string) error {
	ptr, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return err
	}
	ok, _, err := setConsoleTitleW.Call(uintptr(unsafe.Pointer(ptr)))
	if ok == 0 {
		return err
	}
	return nil
}

func terminalTitle(dir string) string {
	dir = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, dir)
	if dir == "" {
		return "mc"
	}
	return "mc - " + dir
}

// Title ownership follows the runtime, never background workers.
type terminalTitleState struct {
	original string
	captured bool
	last     string
	applied  bool
	set      func(string) error
}

func newTerminalTitleState(get func() (string, error), set func(string) error) *terminalTitleState {
	original, err := get()
	return &terminalTitleState{original: original, captured: err == nil, set: set}
}

func (s *terminalTitleState) update(dir string) {
	title := terminalTitle(dir)
	if s.applied && s.last == title {
		return
	}
	if s.set(title) == nil {
		s.last = title
		s.applied = true
	}
}

func (s *terminalTitleState) restore() {
	if s.captured {
		_ = s.set(s.original)
	}
	s.applied = false
}
