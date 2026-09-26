package main

import (
	"context"
	"fmt"
	"github.com/Fiend3d/catatui"
	"github.com/Fiend3d/catatui/term"
	"github.com/fsnotify/fsnotify"
	"maps"
	"mc/internal/event"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type delivered struct {
	msg event.Msg
	ack chan struct{}
}

type watcherReadyMsg struct{ watcher *fsnotify.Watcher }

var newWatcher = fsnotify.NewWatcher

// A watch registration can block on a slow volume, so only this worker calls
// Add and Remove. Filesystem events still reach the UI loop directly.
func runWatchUpdates(ctx context.Context, watcher *fsnotify.Watcher, updates <-chan map[string]bool) {
	registered := make(map[string]bool)
	for {
		select {
		case <-ctx.Done():
			return
		case wanted := <-updates:
			for dir := range registered {
				if !wanted[dir] {
					_ = watcher.Remove(dir)
					delete(registered, dir)
				}
			}
			live := make(map[string]bool)
			for _, dir := range watcher.WatchList() {
				live[dir] = true
			}
			for dir := range wanted {
				if ctx.Err() != nil {
					return
				}
				if !registered[dir] || !live[dir] {
					registered[dir] = watcher.Add(dir) == nil
				}
			}
		}
	}
}

func run(m *model) (err error) {
	defer term.RecoverAndRestore()
	defer m.saveTabs()
	title := newTerminalTitleState(readConsoleTitle, writeConsoleTitle)
	defer title.restore()
	var terminal *catatui.Terminal
	var restore func()
	var reader *term.EventReader
	start := func() error {
		var e error
		opts := []term.Option{term.WithBracketedPaste(), term.WithFocusReporting()}
		if m.mode == hiddenMode {
			opts = append(opts, term.WithoutAlternateScreen())
		} else {
			opts = append(opts, term.WithMouse())
		}
		terminal, restore, e = term.Init(opts...)
		if e != nil {
			return e
		}
		reader = term.NewEventReader(os.Stdin, os.Stdout)
		title.update(m.currentDir())
		return nil
	}
	stop := func() {
		if reader != nil {
			reader.Close()
		}
		if restore != nil {
			restore()
		}
	}
	if err = start(); err != nil {
		return err
	}
	defer stop()
	ctx, cancel := context.WithCancel(context.Background())
	var workers sync.WaitGroup
	defer func() {
		cancel()
		m.compare.stop()
		m.vibe.stop()
		for _, t := range m.taskList {
			if t.cancel != nil {
				t.cancel()
			}
		}
		if m.search != nil {
			m.search.stop()
		}
		workers.Wait()
		done := make(chan struct{})
		go func() { m.taskWorkers.Wait(); close(done) }()
		for {
			select {
			case <-done:
				return
			case <-m.taskEvents:
			}
		}
	}()
	inbox := make(chan delivered, 128)
	var execute func(event.Cmd)
	execute = func(cmd event.Cmd) {
		if cmd == nil {
			return
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			var effect func(event.Msg)
			effect = func(msg event.Msg) {
				switch v := msg.(type) {
				case nil:
					return
				case event.BatchMsg:
					for _, c := range v {
						execute(c)
					}
				case event.SequenceMsg:
					for _, c := range v {
						if ctx.Err() != nil {
							return
						}
						if c != nil {
							effect(c())
						}
					}
				case event.Delay:
					timer := time.NewTimer(v.Duration)
					defer timer.Stop()
					select {
					case now := <-timer.C:
						effect(v.Next(now))
					case <-ctx.Done():
					}
				default:
					ack := make(chan struct{})
					select {
					case inbox <- delivered{msg, ack}:
					case <-ctx.Done():
						return
					}
					select {
					case <-ack:
					case <-ctx.Done():
					}
				}
			}
			effect(cmd())
		}()
	}
	for _, p := range m.panes {
		for _, t := range p.tabs {
			execute(m.readTab(t))
		}
	}
	var watcher *fsnotify.Watcher
	var watchUpdates chan map[string]bool
	watched := make(map[string]bool)
	pendingRefresh := make(refreshQueue)
	lastFallback := time.Now()
	reconcileWatches := func(retry bool) {
		if watchUpdates == nil {
			return
		}
		wanted := make(map[string]bool)
		for _, p := range m.panes {
			for _, t := range p.tabs {
				if t.dir != "" {
					wanted[filepath.Clean(t.dir)] = true
				}
			}
		}
		for dir := range watched {
			if !wanted[dir] {
				delete(pendingRefresh, dir)
			}
		}
		if !retry && maps.Equal(watched, wanted) {
			return
		}
		watched = wanted
		select {
		case watchUpdates <- wanted:
		default:
			<-watchUpdates
			watchUpdates <- wanted
		}
	}
	watcherStarted := false
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	dirty := true
	for {
		if dirty && m.mode != hiddenMode {
			if err = terminal.Draw(func(f *catatui.Frame) {
				m.screenWidth = int(f.Area().Width)
				m.screenHeight = int(f.Area().Height)
				m.draw(f)
			}); err != nil {
				return err
			}
			dirty = false
		}
		if !watcherStarted {
			watcherStarted = true
			execute(func() event.Msg {
				w, _ := newWatcher()
				return watcherReadyMsg{watcher: w}
			})
		}
		var msg event.Msg
		var ack chan struct{}
		select {
		case d := <-inbox:
			msg, ack = d.msg, d.ack
		case msg = <-m.taskEvents:
		case e, ok := <-reader.Events():
			if !ok {
				return reader.Err()
			}
			msg = m.inputEvent(e)
		case change := <-watcherEvents(watcher):
			pendingRefresh.changed(change, watched, time.Now())
			continue
		case <-watcherErrors(watcher):
			for dir := range watched {
				pendingRefresh[dir] = time.Now()
			}
			continue
		case <-ticker.C:
			now := time.Now()
			retry := now.Sub(lastFallback) >= 2*time.Second
			reconcileWatches(retry)
			if retry {
				lastFallback = now
				if m.mode == vibeMode && !m.taskView {
					execute(m.refreshVibe())
					dirty = true
				}
				for _, p := range m.panes {
					for _, t := range p.tabs {
						// Also catches missed notifications and clipboard changes.
						if _, pending := pendingRefresh[t.dir]; !pending {
							pendingRefresh[t.dir] = now
						}
					}
				}
			}
			for _, cmd := range pendingRefresh.drain(m, now) {
				execute(cmd)
			}
			if m.tasksPending() {
				dirty = true
			}
			continue
		}

		switch v := msg.(type) {
		case watcherReadyMsg:
			if v.watcher != nil {
				watcher = v.watcher
				defer watcher.Close()
				watchUpdates = make(chan map[string]bool, 1)
				workers.Add(1)
				go func() {
					defer workers.Done()
					runWatchUpdates(ctx, watcher, watchUpdates)
				}()
				reconcileWatches(true)
			}
		case event.QuitMsg:
			if ack != nil {
				close(ack)
			}
			return nil
		case event.ProcessMsg:
			stop()
			title.restore()
			v.Command.Stdin = os.Stdin
			v.Command.Stdout = os.Stdout
			v.Command.Stderr = os.Stderr
			processErr := v.Command.Run()
			if err = start(); err != nil {
				if ack != nil {
					close(ack)
				}
				return err
			}
			if v.Next != nil {
				_, cmd := m.Update(v.Next(processErr))
				execute(cmd)
			}
			if processErr != nil {
				execute(m.addMessage(msgError, processErr.Error()))
			}
		default:
			if msg != nil {
				wasHidden := m.mode == hiddenMode
				m.dimensions()
				_, cmd := m.Update(msg)
				execute(cmd)
				if wasHidden != (m.mode == hiddenMode) {
					stop()
					if err = start(); err != nil {
						return err
					}
				}
			}
		}
		if ack != nil {
			close(ack)
		}
		title.update(m.currentDir())
		dirty = true
	}
}

func watcherEvents(w *fsnotify.Watcher) <-chan fsnotify.Event {
	if w == nil {
		return nil
	}
	return w.Events
}

func watcherErrors(w *fsnotify.Watcher) <-chan error {
	if w == nil {
		return nil
	}
	return w.Errors
}

func samePath(a, b string) bool { return strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) }

func (m *model) inputEvent(e term.Event) event.Msg {
	switch e.Kind {
	case term.EventResize:
		m.screenWidth, m.screenHeight = int(e.Size.Width), int(e.Size.Height)
		m.dimensions()
		return event.WindowSizeMsg{Width: m.width, Height: m.height}
	case term.EventPaste:
		return event.PasteMsg{Content: e.Text}
	case term.EventFocus:
		if e.Focused {
			return event.FocusMsg{}
		}
		return event.BlurMsg{}
	case term.EventKey:
		names := map[term.KeyCode]string{term.KeyEnter: "enter", term.KeyEscape: "esc", term.KeyBackspace: "backspace", term.KeyTab: "tab", term.KeyBackTab: "shift+tab", term.KeyDelete: "delete", term.KeyInsert: "insert", term.KeyLeft: "left", term.KeyRight: "right", term.KeyUp: "up", term.KeyDown: "down", term.KeyHome: "home", term.KeyEnd: "end", term.KeyPageUp: "pgup", term.KeyPageDown: "pgdown"}
		name := names[e.Key]
		text := ""
		if e.Key == term.KeyRune {
			name = string(e.Rune)
			if e.Mods&(term.ModCtrl|term.ModAlt) == 0 {
				text = name
			}
			if e.Rune == ' ' {
				name = "space"
			}
		}
		if e.Key >= term.KeyF1 && e.Key <= term.KeyF12 {
			name = fmt.Sprintf("f%d", e.Key-term.KeyF1+1)
		}
		if e.Mods.Contains(term.ModShift) && e.Key != term.KeyRune && e.Key != term.KeyBackTab {
			name = "shift+" + name
		}
		if e.Mods.Contains(term.ModCtrl) {
			name = "ctrl+" + strings.ToLower(name)
		}
		if e.Mods.Contains(term.ModAlt) {
			name = "alt+" + name
		}
		return event.KeyMsg{Name: name, Text: text}
	case term.EventMouse:
		// Overlays own input before any pane focus or tab mutation occurs.
		if m.taskView || m.quitting || (e.MouseKind == term.MouseDown && e.Button != term.MouseButtonLeft) {
			return nil
		}
		x, y := int(e.X), int(e.Y)
		if m.mode == vibeMode {
			if e.MouseKind == term.MouseDrag {
				return event.MouseDragMsg{X: x, Y: y, Button: event.MouseLeft}
			}
			if e.MouseKind == term.MouseUp {
				return event.MouseUpMsg{X: x, Y: y, Button: event.MouseLeft}
			}
		}
		if m.mode == vibeMode && e.MouseKind == term.MouseMove {
			m.syncVibeLayout()
			_, leftWidth, _, _ := m.vibePaneLayout()
			if x >= leftWidth-1 {
				return event.MouseHoverMsg{Index: -1}
			}
			return event.MouseHoverMsg{Index: m.vibe.rowAtY(y, m.vibeHeight())}
		}
		// Help owns drag and release so its scrollbar can be dragged; other
		// modes never see them, keeping their coordinate handling untouched.
		if m.mode == helpMode || m.mode == helpFilterMode {
			switch e.MouseKind {
			case term.MouseDrag:
				return event.MouseDragMsg{X: x, Y: y, Button: event.MouseLeft}
			case term.MouseUp:
				return event.MouseUpMsg{X: x, Y: y, Button: event.MouseLeft}
			}
		}
		if m.mode == gitListMode && e.MouseKind == term.MouseMove {
			return event.MouseHoverMsg{Index: m.gitListRowAtY(y), Git: true}
		}
		if m.mode == searchMode && e.MouseKind == term.MouseMove {
			index := y - 3 + m.search.start
			if y >= 3 && y < m.height-2 && index >= 0 && index < m.search.length() {
				return event.MouseHoverMsg{Index: index, Search: true}
			}
			return event.MouseHoverMsg{Pane: -1, Index: -1, Search: true}
		}
		if m.mode == normalMode || m.mode == jumpMode {
			left := (m.screenWidth - 1) / 2
			pane := 0
			if x > left {
				pane = 1
				x -= left + 1
			}
			if x == left && pane == 0 {
				if e.MouseKind == term.MouseMove {
					return event.MouseHoverMsg{Pane: -1, Index: -1}
				}
				return nil
			}
			if e.MouseKind == term.MouseDown || e.MouseKind == term.MouseScrollDown || e.MouseKind == term.MouseScrollUp {
				if pane != m.activePane {
					m.finishRangeSelection()
					m.click = mouseClick{}
				}
				m.activePane = pane
				m.pane = m.panes[pane]
			}
			// The native pane has a separate tab row above its path.
			if y == 0 {
				if e.MouseKind == term.MouseDown {
					m.finishRangeSelection()
					if index := paneTabAtX(m.panes[pane], x); index >= 0 {
						return event.MouseTabMsg{Pane: pane, Index: index}
					}
				}
				if e.MouseKind == term.MouseMove {
					return event.MouseHoverMsg{Pane: pane, Index: paneTabAtX(m.panes[pane], x), Tab: true}
				}
				return nil
			}
			y--
			if !m.panes[pane].hasTabs() {
				if e.MouseKind == term.MouseMove {
					return event.MouseHoverMsg{Pane: -1, Index: -1}
				}
				return nil
			}
			if e.MouseKind == term.MouseMove {
				if y == 0 {
					if m.panes[pane].tabs[m.panes[pane].currentTab].dir != "" {
						return event.MouseHoverMsg{Pane: pane, Index: -1, Path: true, X: x}
					}
					return event.MouseHoverMsg{Pane: -1, Index: -1}
				}
				t := m.panes[pane].tabs[m.panes[pane].currentTab]
				settings := t.getPageSettings()
				index := y - 1 + settings.start
				if y >= 1 && y < m.height-2 && index >= 0 && index < len(t.page.getItems()) {
					return event.MouseHoverMsg{Pane: pane, Index: index}
				}
				return event.MouseHoverMsg{Pane: -1, Index: -1}
			}
		}
		if m.taskView || m.quitting {
			return nil
		}
		switch e.MouseKind {
		case term.MouseMove:
			return event.MouseHoverMsg{Pane: -1, Index: -1}
		case term.MouseDown:
			if e.Button == term.MouseButtonLeft {
				return event.MouseClickMsg{X: x, Y: y, Button: event.MouseLeft, Shift: e.Mods.Contains(term.ModShift), Ctrl: e.Mods.Contains(term.ModCtrl)}
			}
		case term.MouseScrollUp:
			return event.MouseWheelMsg{X: x, Y: y, Button: event.MouseWheelUp}
		case term.MouseScrollDown:
			return event.MouseWheelMsg{X: x, Y: y, Button: event.MouseWheelDown}
		}
	}
	return nil
}
