package cursor

import (
	"context"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const defaultBlinkSpeed = time.Millisecond * 530

var lastID int64

func nextID() int {
	return int(atomic.AddInt64(&lastID, 1))
}

type initialBlinkMsg struct{}

type BlinkMsg struct {
	id  int
	tag int
}

type blinkCanceled struct{}

type blinkCtx struct {
	ctx    context.Context
	cancel context.CancelFunc
}

type Mode int

const (
	CursorBlink Mode = iota
	CursorStatic
	CursorHide
)

func (c Mode) String() string {
	return [...]string{
		"blink",
		"static",
		"hidden",
	}[c]
}

type Model struct {
	Style      lipgloss.Style
	TextStyle  lipgloss.Style
	BlinkSpeed time.Duration
	IsBlinked  bool
	char       string
	id         int
	focus      bool
	blinkCtx   *blinkCtx
	blinkTag   int
	mode       Mode
}

func New() Model {
	return Model{
		id:         nextID(),
		BlinkSpeed: defaultBlinkSpeed,
		IsBlinked:  true,
		mode:       CursorBlink,
		blinkCtx: &blinkCtx{
			ctx: context.Background(),
		},
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case initialBlinkMsg:
		if m.mode != CursorBlink || !m.focus {
			return m, nil
		}
		cmd := m.Blink()
		return m, cmd

	case tea.FocusMsg:
		return m, m.Focus()

	case tea.BlurMsg:
		m.Blur()
		return m, nil

	case BlinkMsg:
		if m.mode != CursorBlink || !m.focus {
			return m, nil
		}
		if msg.id != m.id || msg.tag != m.blinkTag {
			return m, nil
		}
		var cmd tea.Cmd
		if m.mode == CursorBlink {
			m.IsBlinked = !m.IsBlinked
			cmd = m.Blink()
		}
		return m, cmd

	case blinkCanceled:
		return m, nil
	}
	return m, nil
}

func (m Model) Mode() Mode {
	return m.mode
}

func (m *Model) SetMode(mode Mode) tea.Cmd {
	if mode < CursorBlink || mode > CursorHide {
		return nil
	}
	m.mode = mode
	m.IsBlinked = m.mode == CursorHide || !m.focus
	if mode == CursorBlink {
		return Blink
	}
	return nil
}

func (m *Model) Blink() tea.Cmd {
	if m.mode != CursorBlink {
		return nil
	}

	if m.blinkCtx != nil && m.blinkCtx.cancel != nil {
		m.blinkCtx.cancel()
	}

	ctx, cancel := context.WithTimeout(m.blinkCtx.ctx, m.BlinkSpeed)
	m.blinkCtx.cancel = cancel

	m.blinkTag++
	blinkMsg := BlinkMsg{id: m.id, tag: m.blinkTag}

	return func() tea.Msg {
		defer cancel()
		<-ctx.Done()
		if ctx.Err() == context.DeadlineExceeded {
			return blinkMsg
		}
		return blinkCanceled{}
	}
}

func Blink() tea.Msg {
	return initialBlinkMsg{}
}

func (m *Model) Focus() tea.Cmd {
	m.focus = true
	m.IsBlinked = m.mode == CursorHide

	if m.mode == CursorBlink && m.focus {
		return m.Blink()
	}
	return nil
}

func (m *Model) Blur() {
	m.focus = false
	m.IsBlinked = true
}

func (m *Model) SetChar(char string) {
	m.char = char
}

func (m Model) View() string {
	if m.IsBlinked {
		return m.TextStyle.Inline(true).Render(m.char)
	}
	return m.Style.Inline(true).Reverse(true).Render(m.char)
}
