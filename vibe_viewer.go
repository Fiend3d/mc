package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rivo/uniseg"
	"mc/internal/event"
)

type vibeViewerReadyMsg struct {
	generation       uint64
	path, root, text string
	line             int
	err              error
}
type vibeViewerDoneMsg struct {
	generation uint64
	err        error
}

// vibeViewLine picks the current-file line a row opens at. Deleted lines no
// longer exist, so they open where they used to be: at the next surviving line
// (usually their replacement), or the previous one at the end of a hunk.
func vibeViewLine(h vibeHunk, index int) vibeLine {
	current := func(l vibeLine) bool { return l.kind == ' ' || l.kind == '+' }
	for i := index; i < len(h.lines); i++ {
		if current(h.lines[i]) {
			return h.lines[i]
		}
	}
	for i := min(index, len(h.lines)) - 1; i >= 0; i-- {
		if current(h.lines[i]) {
			return h.lines[i]
		}
	}
	return vibeLine{new: max(1, h.new)}
}

func (m *model) viewVibe() event.Cmd {
	v := &m.vibe
	r := v.current()
	if r == nil || r.file < 0 || r.kind == 'd' || v.viewing {
		return nil
	}
	if m.cfg.F3 == nil {
		return m.addMessage(msgWarning, "F3 viewer is not configured")
	}
	f := v.snapshot.files[r.file]
	if f.status == "D" {
		v.viewerErr = compareLiteral(f.path) + " was deleted; there is no current file to open"
		return nil
	}
	line := vibeLine{new: 1}
	if len(f.hunks) > 0 {
		hi, li := 0, 0
		if r.kind == 'h' || r.kind == 'l' {
			hi = r.hunk
		}
		h := f.hunks[hi]
		if r.kind == 'l' {
			li = r.line
		} else {
			// A file or hunk opens at its first change.
			li = slices.IndexFunc(h.lines, func(l vibeLine) bool { return l.kind == '+' || l.kind == '-' })
			li = max(0, li)
		}
		line = vibeViewLine(h, li)
	}
	path := filepath.Join(v.root, filepath.FromSlash(f.path))
	request := vibeViewerReadyMsg{generation: v.generation, path: path, root: v.root, line: max(1, line.new), text: line.text}
	v.viewing = true
	v.viewerErr = ""
	v.pending = true
	if v.loading {
		v.invalid = true
		v.stop()
	}
	return func() event.Msg {
		info, err := os.Stat(path)
		request.err = err
		if err == nil && !info.Mode().IsRegular() {
			request.err = fmt.Errorf("%s is not a regular file", path)
		}
		return request
	}
}

func (m *model) startVibeViewer(msg vibeViewerReadyMsg) event.Cmd {
	v := &m.vibe
	if m.mode != vibeMode || msg.generation != v.generation {
		return nil
	}
	if msg.err != nil {
		v.viewing = false
		v.viewerErr = msg.err.Error()
		return nil
	}
	cmd := vibeViewerCommand(*m.cfg.F3, m.cfg.Theme, msg)
	return event.ExecProcess(cmd, func(err error) event.Msg { return vibeViewerDoneMsg{msg.generation, err} })
}

func vibeViewerCommand(tool ToolConfig, theme string, msg vibeViewerReadyMsg) *exec.Cmd {
	args := slices.Clone(tool.Args)
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(tool.Command)), ".exe")
	if name == "koneko" && tool.Type == "path" {
		// The selected row owns the initial selection; remove competing
		// search/selection flags while preserving the user's other options.
		filtered := args[:0]
		for i := 0; i < len(args); i++ {
			a := args[i]
			if a == "-search" || a == "-select" {
				i++
				continue
			}
			if strings.HasPrefix(a, "-search=") || strings.HasPrefix(a, "-select=") {
				continue
			}
			filtered = append(filtered, a)
		}
		args = filtered
		args = append(args, "-theme="+theme, fmt.Sprintf("-select=%d:1-%d:%d", msg.line, msg.line, uniseg.GraphemeClusterCount(msg.text)+1))
	}
	switch tool.Type {
	case "dir":
		args = append(args, filepath.Dir(msg.path))
	case "none":
	default:
		args = append(args, msg.path)
	}
	cmd := exec.Command(tool.Command, args...)
	cmd.Dir = msg.root
	return cmd
}
