// Package event contains mc's application events and asynchronous effects.
// Terminal ownership and scheduling live in the application's catatui loop.
package event

import (
	"image/color"
	"os/exec"
	"time"
)

type Msg = any
type Cmd func() Msg
type Model interface{ Update(Msg) (Model, Cmd) }
type BatchMsg []Cmd
type SequenceMsg []Cmd

func Batch(cmds ...Cmd) Cmd    { return func() Msg { return BatchMsg(cmds) } }
func Sequence(cmds ...Cmd) Cmd { return func() Msg { return SequenceMsg(cmds) } }

type Delay struct {
	Duration time.Duration
	Next     func(time.Time) Msg
}

func Tick(d time.Duration, next func(time.Time) Msg) Cmd { return func() Msg { return Delay{d, next} } }

type QuitMsg struct{}

func Quit() Msg { return QuitMsg{} }

type ProcessMsg struct {
	Command *exec.Cmd
	Next    func(error) Msg
}

func ExecProcess(c *exec.Cmd, next func(error) Msg) Cmd {
	return func() Msg { return ProcessMsg{c, next} }
}

type KeyPressMsg struct{ Name, Text string }
type KeyMsg = KeyPressMsg

func (k KeyPressMsg) String() string { return k.Name }

type PasteMsg struct{ Content string }
type FocusMsg struct{}
type BlurMsg struct{}
type WindowSizeMsg struct{ Width, Height int }
type Mouse struct {
	X, Y, Button int
	Shift        bool
	Ctrl         bool
}

const (
	MouseLeft = iota
	MouseWheelUp
	MouseWheelDown
)

type MouseClickMsg Mouse

func (m MouseClickMsg) Mouse() Mouse { return Mouse(m) }

type MouseWheelMsg Mouse

func (m MouseWheelMsg) Mouse() Mouse { return Mouse(m) }

// MouseDragMsg is the pointer moving with a button held down.
type MouseDragMsg Mouse

func (m MouseDragMsg) Mouse() Mouse { return Mouse(m) }

// MouseUpMsg is the button being released, which ends any drag in progress.
type MouseUpMsg Mouse

func (m MouseUpMsg) Mouse() Mouse { return Mouse(m) }

type MouseHoverMsg struct {
	Pane, Index int
	Search      bool
	Tab         bool
	Path        bool
	X           int
}

type MouseTabMsg struct{ Pane, Index int }

type CursorShape int

const (
	CursorBlock CursorShape = iota
	CursorBar
)

type Cursor struct {
	X, Y  int
	Blink bool
	Color color.Color
	Shape CursorShape
}

func NewCursor(x, y int) *Cursor { return &Cursor{X: x, Y: y} }
