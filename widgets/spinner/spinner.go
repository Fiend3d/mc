package spinner

import (
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var lastID int64

func nextID() int {
	return int(atomic.AddInt64(&lastID, 1))
}

type Spinner struct {
	Frames []string
	FPS    time.Duration
}

var (
	Line = Spinner{
		Frames: []string{"|", "/", "-", "\\"},
		FPS:    time.Second / 10,
	}
	Dot = Spinner{
		Frames: []string{"⣾ ", "⣽ ", "⣻ ", "⢿ ", "⡿ ", "⣟ ", "⣯ ", "⣷ "},
		FPS:    time.Second / 10,
	}
	MiniDot = Spinner{
		Frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		FPS:    time.Second / 12,
	}
	Jump = Spinner{
		Frames: []string{"⢄", "⢂", "⢁", "⡁", "⡈", "⡐", "⡠"},
		FPS:    time.Second / 10,
	}
	Pulse = Spinner{
		Frames: []string{"█", "▓", "▒", "░"},
		FPS:    time.Second / 8,
	}
	Points = Spinner{
		Frames: []string{"∙∙∙", "●∙∙", "∙●∙", "∙∙●"},
		FPS:    time.Second / 7,
	}
	Globe = Spinner{
		Frames: []string{"🌍", "🌎", "🌏"},
		FPS:    time.Second / 4,
	}
	Moon = Spinner{
		Frames: []string{"🌑", "🌒", "🌓", "🌔", "🌕", "🌖", "🌗", "🌘"},
		FPS:    time.Second / 8,
	}
	Monkey = Spinner{
		Frames: []string{"🙈", "🙉", "🙊"},
		FPS:    time.Second / 3,
	}
	Meter = Spinner{
		Frames: []string{
			"▱▱▱",
			"▰▱▱",
			"▰▰▱",
			"▰▰▰",
			"▰▰▱",
			"▱▰▱",
			"▱▱▱",
		},
		FPS: time.Second / 7,
	}
	Hamburger = Spinner{
		Frames: []string{"☱", "☲", "☴", "☲"},
		FPS:    time.Second / 3,
	}
	Ellipsis = Spinner{
		Frames: []string{"", ".", "..", "..."},
		FPS:    time.Second / 3,
	}
)

type Model struct {
	Spinner Spinner
	Style   lipgloss.Style
	frame   int
	id      int
	tag     int
}

func (m Model) ID() int {
	return m.id
}

func New(opts ...Option) Model {
	m := Model{
		Spinner: Line,
		id:      nextID(),
	}

	for _, opt := range opts {
		opt(&m)
	}

	return m
}

type TickMsg struct {
	Time time.Time
	tag  int
	ID   int
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TickMsg:
		if msg.ID > 0 && msg.ID != m.id {
			return m, nil
		}
		if msg.tag > 0 && msg.tag != m.tag {
			return m, nil
		}
		m.frame++
		if m.frame >= len(m.Spinner.Frames) {
			m.frame = 0
		}
		m.tag++
		return m, m.tick(m.id, m.tag)
	default:
		return m, nil
	}
}

func (m Model) View() string {
	if m.frame >= len(m.Spinner.Frames) {
		return "(error)"
	}
	return m.Style.Render(m.Spinner.Frames[m.frame])
}

func (m Model) Tick() tea.Msg {
	return TickMsg{
		Time: time.Now(),
		ID:   m.id,
		tag:  m.tag,
	}
}

func (m Model) tick(id, tag int) tea.Cmd {
	return tea.Tick(m.Spinner.FPS, func(t time.Time) tea.Msg {
		return TickMsg{
			Time: t,
			ID:   id,
			tag:  tag,
		}
	})
}

type Option func(*Model)

func WithSpinner(spinner Spinner) Option {
	return func(m *Model) {
		m.Spinner = spinner
	}
}

func WithStyle(style lipgloss.Style) Option {
	return func(m *Model) {
		m.Style = style
	}
}
