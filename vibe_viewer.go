package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rivo/uniseg"
	"mc/internal/event"
)

type vibeViewerReadyMsg struct {
	generation       uint64
	path, root, text string
	line             int
	historical       bool
	data             []byte
	err              error
}
type vibeViewerDoneMsg struct {
	generation uint64
	err        error
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
	line := vibeLine{new: 1}
	if len(f.hunks) > 0 {
		hi := 0
		if r.kind == 'h' || r.kind == 'l' {
			hi = r.hunk
		}
		h := f.hunks[hi]
		if r.kind == 'l' {
			line = h.lines[r.line]
		} else {
			for _, l := range h.lines {
				if l.kind == '+' || l.kind == '-' {
					line = l
					break
				}
			}
		}
	}
	historical := line.kind == '-' || f.status == "D"
	path := filepath.Join(v.root, filepath.FromSlash(f.path))
	number := line.new
	if historical {
		path = filepath.Join(v.root, filepath.FromSlash(f.oldPath))
		number = line.old
	}
	request := vibeViewerReadyMsg{generation: v.generation, path: path, root: v.root, line: max(1, number), text: line.text, historical: historical}
	v.viewing = true
	v.viewerErr = ""
	v.pending = true
	if v.loading {
		v.invalid = true
		v.stop()
	}
	blob := f.blob
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	v.viewerCancel = cancel
	return func() event.Msg {
		defer cancel()
		if historical {
			if blob == "" {
				request.err = fmt.Errorf("no historical version is available for %s", f.path)
				return request
			}
			request.data, request.err = vibeGit(ctx, request.root, compareTextLimit, "cat-file", "blob", blob)
			if errors.Is(request.err, errVibeLimit) {
				request.err = fmt.Errorf("historical file exceeds the 5 MiB viewing limit")
			}
		} else {
			info, err := os.Stat(path)
			request.err = err
			if err == nil && !info.Mode().IsRegular() {
				request.err = fmt.Errorf("%s is not a regular file", path)
			}
		}
		return request
	}
}

func cleanupVibeTemp(path string) {
	if path != "" {
		_ = os.Chmod(path, 0600)
		_ = os.Remove(path)
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
	path := msg.path
	if msg.historical {
		file, err := os.CreateTemp("", "mc-vibe-*"+filepath.Ext(path))
		if err != nil {
			v.viewing = false
			v.viewerErr = err.Error()
			return nil
		}
		path = file.Name()
		_, err = file.Write(msg.data)
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Chmod(path, 0444)
		}
		if err != nil {
			cleanupVibeTemp(path)
			v.viewing = false
			v.viewerErr = err.Error()
			return nil
		}
		v.tempPath = path
	}
	tool := *m.cfg.F3
	cmd := vibeViewerCommand(tool, m.cfg.Theme, msg, path)
	temp := v.tempPath
	return event.ExecProcess(cmd, func(err error) event.Msg { cleanupVibeTemp(temp); return vibeViewerDoneMsg{msg.generation, err} })
}

func vibeViewerCommand(tool ToolConfig, theme string, msg vibeViewerReadyMsg, path string) *exec.Cmd {
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
		if msg.historical {
			args = append(args, "-no-git")
		}
	}
	switch tool.Type {
	case "dir":
		args = append(args, filepath.Dir(msg.path))
	case "none":
	default:
		args = append(args, path)
	}
	cmd := exec.Command(tool.Command, args...)
	cmd.Dir = msg.root
	return cmd
}
