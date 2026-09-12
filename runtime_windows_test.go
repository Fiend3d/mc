package main

import (
	"bytes"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"
	"unsafe"
)

// This helper runs in a real Windows pseudoconsole, including terminal restore
// and a child process reading from stdin. No desktop window is created.
func TestConPTYChild(t *testing.T) {
	if os.Getenv("MC_TEST_CHILD") != "1" {
		return
	}
	input, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	os.Stdin, os.Stdout, os.Stderr = input, output, output

	m := initialModel([]string{os.Getenv("MC_TEST_LEFT"), os.Getenv("MC_TEST_RIGHT")})
	executable, _ := os.Executable()
	m.cfg.F4 = &ToolConfig{Command: executable, Type: "none", Args: []string{"-test.run=^TestConPTYTool$"}}
	if os.Getenv("MC_TEST_TOOL_KIND") == "powershell" {
		m.cfg.F4 = &ToolConfig{Command: "powershell", Type: "none", Args: []string{"-NoProfile", "-Command", `Write-Host MC-TOOL-READY; $line=[Console]::ReadLine(); [IO.File]::WriteAllText($env:MC_TEST_TOOL_RESULT,$line)`}}
	}

	if err := run(&m); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("MC_TEST_RESULT"), []byte(m.result), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestConPTYTool(t *testing.T) {
	if os.Getenv("MC_TEST_CHILD") != "1" {
		return
	}
	fmt.Println("MC-TOOL-READY")
	buf := make([]byte, 1024)
	n, err := os.Stdin.Read(buf)
	text := strings.TrimSpace(string(buf[:n]))
	if err != nil {
		text = "ERROR: " + err.Error()
	}
	if err := os.WriteFile(os.Getenv("MC_TEST_TOOL_RESULT"), []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestConPTYNavigationAndProcessHandoff(t *testing.T) {
	for _, kind := range []string{"go", "powershell"} {
		t.Run(kind, func(t *testing.T) { t.Setenv("MC_TEST_TOOL_KIND", kind); testConPTYHandoff(t) })
	}
}
func testConPTYHandoff(t *testing.T) {
	if os.Getenv("MC_TEST_CHILD") == "1" {
		return
	}
	dir := t.TempDir()
	left, right := filepath.Join(dir, "left"), filepath.Join(dir, "right")
	for _, p := range []string{left, right} {
		if err := os.Mkdir(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	touch(t, filepath.Join(left, "sample.txt"))
	touch(t, filepath.Join(right, "sample.txt"))
	t.Setenv("APPDATA", dir)
	t.Setenv("MC_TEST_CHILD", "1")
	t.Setenv("MC_TEST_LEFT", left)
	t.Setenv("MC_TEST_RIGHT", right)
	result, toolResult := filepath.Join(dir, "result"), filepath.Join(dir, "tool-result")
	t.Setenv("MC_TEST_RESULT", result)
	t.Setenv("MC_TEST_TOOL_RESULT", toolResult)
	inputRead, inputWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer inputRead.Close()
	defer inputWrite.Close()
	outputRead, outputWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer outputRead.Close()
	defer outputWrite.Close()
	var console windows.Handle
	if err = windows.CreatePseudoConsole(windows.Coord{X: 100, Y: 24}, windows.Handle(inputRead.Fd()), windows.Handle(outputWrite.Fd()), 0, &console); err != nil {
		t.Skipf("ConPTY unavailable: %v", err)
	}
	defer windows.ClosePseudoConsole(console)
	attributes, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		t.Fatal(err)
	}
	defer attributes.Delete()
	consolePointer := *(*unsafe.Pointer)(unsafe.Pointer(&console))
	if err = attributes.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, consolePointer, unsafe.Sizeof(console)); err != nil {
		t.Fatal(err)
	}
	startup := windows.StartupInfoEx{StartupInfo: windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfoEx{}))}, ProcThreadAttributeList: attributes.List()}
	executable, _ := os.Executable()
	command, _ := windows.UTF16PtrFromString(windows.ComposeCommandLine([]string{executable, "-test.run=^TestConPTYChild$"}))
	var process windows.ProcessInformation
	if err = windows.CreateProcess(nil, command, nil, nil, false, windows.EXTENDED_STARTUPINFO_PRESENT, nil, nil, &startup.StartupInfo, &process); err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(process.Process)
	defer windows.CloseHandle(process.Thread)
	defer windows.TerminateProcess(process.Process, 1)
	var mu sync.Mutex
	var output bytes.Buffer
	go func() {
		buf := make([]byte, 16384)
		for {
			n, e := outputRead.Read(buf)
			mu.Lock()
			output.Write(buf[:n])
			mu.Unlock()
			if e != nil {
				return
			}
		}
	}()
	wait := func(label string, ready func() bool) {
		t.Helper()
		deadline := time.Now().Add(12 * time.Second)
		for time.Now().Before(deadline) {
			if ready() {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		mu.Lock()
		log := output.String()
		mu.Unlock()
		b, e := os.ReadFile(toolResult)
		if i := strings.Index(log, "MC-TOOL-READY"); i >= 0 {
			log = log[max(0, i-300):min(len(log), i+1800)]
		} else if len(log) > 2400 {
			log = log[len(log)-2400:]
		}
		t.Fatalf("timeout waiting for %s; tool result: %q (%v); output tail: %s", label, b, e, log)
	}
	contains := func(s string) func() bool {
		return func() bool { mu.Lock(); defer mu.Unlock(); return bytes.Contains(output.Bytes(), []byte(s)) }
	}
	send := func(s string) {
		t.Helper()
		if _, err := fmt.Fprint(inputWrite, s); err != nil {
			t.Fatal(err)
		}
	}
	wait("first frame", contains("sample.txt"))
	// Exercise the actual Windows key-record path: Ctrl+J must remain
	// distinct from Enter, which legacy LF-only input cannot express.
	send("\x1b[74;36;10;1;8;1_\x1b[74;36;10;0;8;1_")
	wait("Jump shortcut", contains("JUMP"))
	send("\t") // Tab leaves Jump and switches panes.
	send("\x1b[68;32;68;1;16;1_\x1b[68;32;68;0;16;1_")
	wait("Compare shortcut", contains("Differences"))
	wait("comparison loaded", contains("Files are identical"))
	send("q") // Close Compare before exercising the editor handoff.
	send("\x1b[115;62;0;1;0;1_\x1b[115;62;0;0;0;1_") // F4 down/up
	wait("external tool", contains("MC-TOOL-READY"))
	for _, r := range "handoff works\r" {
		send(fmt.Sprintf("\x1b[%d;0;%d;1;0;1_\x1b[%d;0;%d;0;0;1_", unicode.ToUpper(r), r, unicode.ToUpper(r), r))
	}
	wait("tool input", func() bool { b, _ := os.ReadFile(toolResult); return string(b) == "handoff works" })
	// Use actual Ctrl+H key records: the legacy byte 8 means Backspace.
	count := func(s string) int { mu.Lock(); defer mu.Unlock(); return bytes.Count(output.Bytes(), []byte(s)) }
	wait("resumed frame", func() bool { return count("?1049h") >= 2 })
	before := count("?1049l")
	ctrlH := "\x1b[72;35;8;1;8;1_\x1b[72;35;8;0;8;1_"
	send(ctrlH)
	wait("hidden terminal", func() bool { return count("?1049l") > before })
	send(ctrlH)
	wait("visible terminal", func() bool { return count("?1049h") >= 3 })
	send("\x1b[<0;70;3M\x1b[<0;70;3m") // click right pane through the console mouse driver
	send("q")

	wait("quit output", func() bool { b, _ := os.ReadFile(result); return string(b) == right })
	status, err := windows.WaitForSingleObject(process.Process, 3000)
	if err != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatalf("child did not exit: %v %v", status, err)
	}
	var code uint32
	if err = windows.GetExitCodeProcess(process.Process, &code); err != nil || code != 0 {
		t.Fatalf("child exit: %d %v", code, err)
	}
}
