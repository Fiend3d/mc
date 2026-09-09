package main

import (
	"context"
	"errors"
	"fmt"
	"mc/internal/event"
	"mc/shutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type task struct {
	id       int
	cmd      command
	action   string
	state    string
	progress shutil.Progress
	err      error
	cancel   context.CancelFunc
}
type taskProgressMsg struct {
	id       int
	progress shutil.Progress
}
type taskDoneMsg struct {
	id       int
	err      error
	progress shutil.Progress
}

func (m *model) enqueue(cmd command, action string) event.Cmd {
	t := &task{id: len(m.taskList) + 1, cmd: cmd, action: action, state: "queued"}
	m.taskList = append(m.taskList, t)
	m.startTask()
	return nil
}
func (m *model) startTask() {
	for _, t := range m.taskList {
		if t.state == "running" || t.state == "scanning" || t.state == "cancelling" {
			return
		}
	}
	for _, t := range m.taskList {
		if t.state != "queued" {
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		t.cancel = cancel
		t.state = "scanning"
		out := m.taskEvents
		cmd := t.cmd
		action := t.action
		id := t.id
		m.taskWorkers.Add(1)
		go func() {
			defer m.taskWorkers.Done()
			var last time.Time
			var latest shutil.Progress
			report := func(p shutil.Progress) {
				latest = p
				if time.Since(last) < 100*time.Millisecond {
					return
				}
				last = time.Now()
				select {
				case out <- taskProgressMsg{id, p}:
				default:
				}
			}
			var err error
			if action == "undo" {
				if c, ok := cmd.(*fileActionCommand); ok {
					err = shutil.UndoJournal(ctx, &c.journal, &c.redoJournal, report)
				} else {
					err = cmd.undo()
				}
			} else {
				switch c := cmd.(type) {
				case *fileActionCommand:
					if action == "redo" {
						err = c.redoTask(ctx, report)
					} else {
						err = c.executeTask(ctx, report)
					}
				case *deleteCommand:
					err = c.executeTask(ctx, report)
				default:
					if err = ctx.Err(); err == nil {
						err = cmd.execute()
					}
				}
			}
			out <- taskDoneMsg{id, err, latest}
		}()
		return
	}
}
func (m *model) cancelTask(t *task) {
	if t.state == "queued" {
		t.state = "cancelled"
		m.jobDone()
	} else if t.cancel != nil && (t.state == "running" || t.state == "scanning") {
		t.state = "cancelling"
		t.cancel()
	}
}
func (m *model) tasksPending() bool {
	for _, t := range m.taskList {
		switch t.state {
		case "queued", "running", "scanning", "cancelling":
			return true
		}
	}
	return false
}

func (m *model) taskStripVisible() bool {
	for _, t := range m.taskList {
		switch t.state {
		case "running", "scanning", "cancelling":
			return true
		}
	}
	return false
}
func (m *model) finishTask(msg taskDoneMsg) event.Cmd {
	t := m.taskList[msg.id-1]
	t.cancel()
	t.err = msg.err
	t.progress = msg.progress
	t.state = "completed"
	if errors.Is(msg.err, context.Canceled) {
		t.state = "cancelled"
	} else if msg.err != nil {
		t.state = "failed"
	}
	if t.action == "undo" {
		if msg.err == nil {
			m.cm.commitUndo()
		}
	} else if t.action == "redo" {
		partial := false
		if c, ok := t.cmd.(*fileActionCommand); ok {
			partial = len(c.journal) > 0
		}
		if msg.err == nil || partial {
			m.cm.commitRedo()
		}
	} else if t.cmd.undoable() {
		if c, ok := t.cmd.(*fileActionCommand); !ok && msg.err == nil || ok && len(c.journal) > 0 {
			m.cm.pushHistory(t.cmd)
		}
	}
	m.jobDone()
	m.startTask()
	message := fmt.Sprintf("Task %d %s: %s", t.id, t.state, t.cmd)
	if msg.err != nil {
		message += " — " + msg.err.Error()
	}
	cmds := []event.Cmd{m.addMessage(msgInfo, message)}
	for _, p := range m.panes {
		for _, tab := range p.tabs {
			cmds = append(cmds, m.readTab(tab))
		}
	}
	if m.quitting && !m.tasksPending() && !m.hasJobs() {
		if m.quitResult {
			m.result = m.currentDir()
		}
		cmds = append(cmds, event.Quit)
	}
	return event.Batch(cmds...)
}

func (c *fileActionCommand) executeTask(ctx context.Context, report shutil.ProgressFunc) error {
	c.journal = nil
	c.redoJournal = nil
	var total int64
	files := 0
	for _, pair := range c.pairs {
		if pair.src == pair.dst {
			continue
		}
		err := filepath.Walk(pair.src, func(p string, i os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if err = ctx.Err(); err != nil {
				return err
			}
			if err = shutil.CheckPath(p); err != nil {
				return err
			}
			if !i.IsDir() {
				total += i.Size()
				files++
			}
			if report != nil {
				report(shutil.Progress{Path: p, Total: total, TotalFiles: files, Scanning: true})
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	var bytesDone int64
	filesDone := 0
	for i := range c.pairs {
		pair := &c.pairs[i]
		if pair.src == pair.dst {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !c.overwrite && shutil.PathExists(pair.dst) && !strings.EqualFold(pair.src, pair.dst) {
			pair.dst = shutil.UniquePath(nil, nil, pair.dst)
		}
		if c.overwrite && shutil.PathExists(pair.dst) && !c.overwriteExisting[pair.dst] {
			return fmt.Errorf("destination appeared after submission; confirm overwrite again: %s", pair.dst)
		}
		baseBytes, baseFiles := bytesDone, filesDone
		records, err := shutil.Transfer(ctx, pair.src, pair.dst, c.action != copyFileAction, c.overwrite, func(p shutil.Progress) {
			bytesDone = baseBytes + p.Bytes
			filesDone = baseFiles + p.Files
			p.Bytes = bytesDone
			p.Files = filesDone
			p.Total = total
			p.TotalFiles = files
			p.Scanning = false
			if report != nil {
				report(p)
			}
		})
		c.journal = append(c.journal, records...)
		if err != nil {
			return err
		}
	}
	return nil
}
func (c *fileActionCommand) redoTask(ctx context.Context, report shutil.ProgressFunc) error {
	// Reapply precisely the work that was undone, including a partial original task.
	for _, r := range c.redoJournal {
		if len(r.Snapshot) > 0 {
			if err := shutil.Unchanged(r.Source, r.Snapshot); err != nil {
				return err
			}
		}
		if !r.RemovedSourceDir && shutil.PathExists(r.Destination) && !strings.EqualFold(r.Source, r.Destination) {
			return fmt.Errorf("redo destination already exists: %s", r.Destination)
		}
	}
	c.journal = nil
	for i := len(c.redoJournal) - 1; i >= 0; i-- {
		r := c.redoJournal[i]
		if err := ctx.Err(); err != nil {
			return err
		}
		if r.RemovedSourceDir {
			if err := os.Remove(r.Source); err != nil {
				return err
			}
			c.journal = append(c.journal, r)
		} else if r.Directory && !r.Move {
			if err := os.MkdirAll(r.Destination, 0755); err != nil {
				return err
			}
			c.journal = append(c.journal, r)
		} else {
			records, err := shutil.Transfer(ctx, r.Source, r.Destination, r.Move, false, report)
			c.journal = append(c.journal, records...)
			if err != nil {
				return err
			}
		}
	}
	c.redoJournal = nil
	return nil
}

func (c *deleteCommand) executeTask(ctx context.Context, report shutil.ProgressFunc) error {
	var paths []string
	for _, root := range c.paths {
		err := filepath.Walk(root, func(p string, i os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if err = ctx.Err(); err != nil {
				return err
			}
			if err = shutil.CheckPath(p); err != nil {
				return err
			}
			paths = append(paths, p)
			return nil
		})
		if err != nil {
			return err
		}
	}
	for i := len(paths) - 1; i >= 0; i-- {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := os.Remove(paths[i]); err != nil {
			return err
		}
		if report != nil {
			report(shutil.Progress{Path: paths[i], Files: len(paths) - i, TotalFiles: len(paths)})
		}
	}
	return nil
}

func (m *model) updateV2(msg event.Msg) (bool, event.Cmd) {
	switch msg := msg.(type) {
	case taskProgressMsg:
		t := m.taskList[msg.id-1]
		t.progress = msg.progress
		if t.state != "cancelling" {
			if msg.progress.Scanning {
				t.state = "scanning"
			} else {
				t.state = "running"
			}
		}
		return true, nil
	case taskDoneMsg:
		return true, m.finishTask(msg)
	case event.PasteMsg:
		if m.mode == transferMode {
			var cmd event.Cmd
			m.input, cmd = m.input.Update(msg)
			return true, cmd
		}
	case event.KeyMsg:
		key := msg.String()
		if m.quitting {
			switch key {
			case "esc", "n":
				m.quitting = false
			case "y", "enter":
				for _, t := range m.taskList {
					m.cancelTask(t)
				}
				if m.search != nil {
					m.search.stop()
				}
				if !m.tasksPending() {
					if m.quitResult {
						m.result = m.currentDir()
					}
					return true, event.Quit
				}
			}
			return true, nil
		}
		if m.taskView {
			switch key {
			case "esc", "w":
				m.taskView = false
			case "j", "down":
				m.taskCursor = min(len(m.taskList)-1, m.taskCursor+1)
			case "k", "up":
				m.taskCursor = max(0, m.taskCursor-1)
			case "c":
				if m.taskCursor >= 0 && m.taskCursor < len(m.taskList) {
					m.cancelTask(m.taskList[m.taskCursor])
				}
			}
			return true, nil
		}
		if m.mode == transferMode {
			switch key {
			case "esc":
				m.mode = normalMode
				return true, nil
			case "enter":
				dst, err := expandWindowsEnv(m.input.Value())
				if err != nil {
					return true, m.addMessage(msgError, err.Error())
				}
				if !filepath.IsAbs(dst) {
					dst = filepath.Join(m.getTab().dir, dst)
				}
				info, err := os.Stat(dst)
				if err != nil || !info.IsDir() {
					return true, m.addMessage(msgError, "Destination must be an existing directory")
				}
				action := copyFileAction
				if m.transferMove {
					action = cutFileAction
				}
				cmd := newFileActionCommand(action, m.transferPaths, dst, false)
				m.mode = normalMode
				m.addJob()
				return true, m.addCommand(cmd)
			}
			var cmd event.Cmd
			m.input, cmd = m.input.Update(msg)
			return true, cmd
		}
		if m.mode == normalMode || m.mode == jumpMode {
			switch key {
			case "tab":
				m.activePane = 1 - m.activePane
				m.pane = m.panes[m.activePane]
				m.mode = normalMode
				m.click = mouseClick{}
				return true, nil
			case "ctrl+left":
				return m.sendTab(0, false)
			case "ctrl+right":
				return m.sendTab(1, false)
			case "shift+left":
				return m.sendTab(0, true)
			case "shift+right":
				return m.sendTab(1, true)
			case "w":
				m.taskView = true
				m.taskCursor = max(0, len(m.taskList)-1)
				return true, nil
			case "Y", "X":
				m.transferPaths = append([]string(nil), m.getPaths()...)
				if len(m.transferPaths) == 0 {
					return true, m.addMessage(msgWarning, "Nothing selected")
				}
				m.transferMove = key == "X"
				m.resetInput("Destination directory")
				m.input.SetValue(m.paneDir(1 - m.activePane))
				m.mode = transferMode
				return true, nil
			case "u", "U":
				if m.tasksPending() {
					return true, m.addMessage(msgWarning, "Wait for pending file operations before undo/redo")
				}
			}
		}
	}
	return false, nil
}
