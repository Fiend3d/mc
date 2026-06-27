package textinput

import (
	"reflect"
	"slices"
	"strings"
	"unicode"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"mc/widgets/cursor"
	"mc/widgets/key"
	"mc/widgets/runeutil"
	"github.com/atotto/clipboard"
	rw "github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
)

type (
	pasteMsg    string
	pasteErrMsg struct{ error }
)

type EchoMode int

const (
	EchoNormal EchoMode = iota
	EchoPassword
	EchoNone
)

type ValidateFunc func(string) error

type KeyMap struct {
	CharacterForward        key.Binding
	CharacterBackward       key.Binding
	WordForward             key.Binding
	WordBackward            key.Binding
	DeleteWordBackward      key.Binding
	DeleteWordForward       key.Binding
	DeleteAfterCursor       key.Binding
	DeleteBeforeCursor      key.Binding
	DeleteCharacterBackward key.Binding
	DeleteCharacterForward  key.Binding
	LineStart               key.Binding
	LineEnd                 key.Binding
	Paste                   key.Binding
	AcceptSuggestion        key.Binding
	NextSuggestion          key.Binding
	PrevSuggestion          key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		CharacterForward:        key.NewBinding(key.WithKeys("right", "ctrl+f")),
		CharacterBackward:       key.NewBinding(key.WithKeys("left", "ctrl+b")),
		WordForward:             key.NewBinding(key.WithKeys("alt+right", "ctrl+right", "alt+f")),
		WordBackward:            key.NewBinding(key.WithKeys("alt+left", "ctrl+left", "alt+b")),
		DeleteWordBackward:      key.NewBinding(key.WithKeys("alt+backspace", "ctrl+w")),
		DeleteWordForward:       key.NewBinding(key.WithKeys("alt+delete", "alt+d")),
		DeleteAfterCursor:       key.NewBinding(key.WithKeys("ctrl+k")),
		DeleteBeforeCursor:      key.NewBinding(key.WithKeys("ctrl+u")),
		DeleteCharacterBackward: key.NewBinding(key.WithKeys("backspace", "ctrl+h")),
		DeleteCharacterForward:  key.NewBinding(key.WithKeys("delete", "ctrl+d")),
		LineStart:               key.NewBinding(key.WithKeys("home", "ctrl+a")),
		LineEnd:                 key.NewBinding(key.WithKeys("end", "ctrl+e")),
		Paste:                   key.NewBinding(key.WithKeys("ctrl+v")),
		AcceptSuggestion:        key.NewBinding(key.WithKeys("tab")),
		NextSuggestion:          key.NewBinding(key.WithKeys("down", "ctrl+n")),
		PrevSuggestion:          key.NewBinding(key.WithKeys("up", "ctrl+p")),
	}
}

type Model struct {
	Err error

	Prompt        string
	Placeholder   string
	EchoMode      EchoMode
	EchoCharacter rune

	useVirtualCursor bool

	virtualCursor cursor.Model

	CharLimit int

	styles Styles

	width int

	KeyMap KeyMap

	value []rune

	focus bool

	pos int

	offset      int
	offsetRight int

	Validate ValidateFunc

	rsan runeutil.Sanitizer

	ShowSuggestions bool

	suggestions            [][]rune
	matchedSuggestions     [][]rune
	currentSuggestionIndex int
}

func New() Model {
	m := Model{
		Prompt:           "> ",
		EchoCharacter:    '*',
		CharLimit:        0,
		styles:           DefaultDarkStyles(),
		ShowSuggestions:  false,
		useVirtualCursor: true,
		virtualCursor:    cursor.New(),
		KeyMap:           DefaultKeyMap(),
		suggestions:      [][]rune{},
		value:            nil,
		focus:            false,
		pos:              0,
	}
	m.updateVirtualCursorStyle()
	return m
}

func (m Model) VirtualCursor() bool {
	return m.useVirtualCursor
}

func (m *Model) SetVirtualCursor(v bool) {
	m.useVirtualCursor = v
	m.updateVirtualCursorStyle()
}

func (m Model) Styles() Styles {
	return m.styles
}

func (m *Model) SetStyles(s Styles) {
	m.styles = s
	m.updateVirtualCursorStyle()
}

func (m Model) Width() int {
	return m.width
}

func (m *Model) SetWidth(w int) {
	m.width = w
}

func (m *Model) SetValue(s string) {
	runes := m.san().Sanitize([]rune(s))
	err := m.validate(runes)
	m.setValueInternal(runes, err)
}

func (m *Model) setValueInternal(runes []rune, err error) {
	m.Err = err

	empty := len(m.value) == 0

	if m.CharLimit > 0 && len(runes) > m.CharLimit {
		m.value = runes[:m.CharLimit]
	} else {
		m.value = runes
	}
	if (m.pos == 0 && empty) || m.pos > len(m.value) {
		m.SetCursor(len(m.value))
	}
	m.handleOverflow()
}

func (m Model) Value() string {
	return string(m.value)
}

func (m Model) Position() int {
	return m.pos
}

func (m *Model) SetCursor(pos int) {
	m.pos = clamp(pos, 0, len(m.value))
	m.handleOverflow()
}

func (m *Model) CursorStart() {
	m.SetCursor(0)
}

func (m *Model) CursorEnd() {
	m.SetCursor(len(m.value))
}

func (m Model) Focused() bool {
	return m.focus
}

func (m *Model) Focus() tea.Cmd {
	m.focus = true
	return m.virtualCursor.Focus()
}

func (m *Model) Blur() {
	m.focus = false
	m.virtualCursor.Blur()
}

func (m *Model) Reset() {
	m.value = nil
	m.SetCursor(0)
}

func (m *Model) SetSuggestions(suggestions []string) {
	m.suggestions = make([][]rune, len(suggestions))
	for i, s := range suggestions {
		m.suggestions[i] = []rune(s)
	}

	m.updateSuggestions()
}

func (m *Model) san() runeutil.Sanitizer {
	if m.rsan == nil {
		m.rsan = runeutil.NewSanitizer(
			runeutil.ReplaceTabs(" "), runeutil.ReplaceNewlines(" "))
	}
	return m.rsan
}

func (m *Model) insertRunesFromUserInput(v []rune) {
	paste := m.san().Sanitize(v)

	var availSpace int
	if m.CharLimit > 0 {
		availSpace = m.CharLimit - len(m.value)

		if availSpace <= 0 {
			return
		}

		if availSpace < len(paste) {
			paste = paste[:availSpace]
		}
	}

	head := m.value[:m.pos]
	tailSrc := m.value[m.pos:]
	tail := make([]rune, len(tailSrc))
	copy(tail, tailSrc)

	for _, r := range paste {
		head = append(head, r)
		m.pos++
		if m.CharLimit > 0 {
			availSpace--
			if availSpace <= 0 {
				break
			}
		}
	}

	value := append(head, tail...)
	inputErr := m.validate(value)
	m.setValueInternal(value, inputErr)
}

func (m *Model) handleOverflow() {
	if m.Width() <= 0 || uniseg.StringWidth(string(m.value)) <= m.Width() {
		m.offset = 0
		m.offsetRight = len(m.value)
		return
	}

	m.offsetRight = min(m.offsetRight, len(m.value))

	if m.pos < m.offset {
		m.offset = m.pos

		w := 0
		i := 0
		runes := m.value[m.offset:]

		for i < len(runes) && w <= m.Width() {
			w += rw.RuneWidth(runes[i])
			if w <= m.Width()+1 {
				i++
			}
		}

		m.offsetRight = m.offset + i
	} else if m.pos >= m.offsetRight {
		m.offsetRight = m.pos

		w := 0
		runes := m.value[:m.offsetRight]
		i := len(runes) - 1

		for i > 0 && w < m.Width() {
			w += rw.RuneWidth(runes[i])
			if w <= m.Width() {
				i--
			}
		}

		m.offset = m.offsetRight - (len(runes) - 1 - i)
	}
}

func (m *Model) deleteBeforeCursor() {
	m.value = m.value[m.pos:]
	m.Err = m.validate(m.value)
	m.offset = 0
	m.SetCursor(0)
}

func (m *Model) deleteAfterCursor() {
	m.value = m.value[:m.pos]
	m.Err = m.validate(m.value)
	m.SetCursor(len(m.value))
}

func (m *Model) deleteWordBackward() {
	if m.pos == 0 || len(m.value) == 0 {
		return
	}

	if m.EchoMode != EchoNormal {
		m.deleteBeforeCursor()
		return
	}

	oldPos := m.pos

	m.SetCursor(m.pos - 1)
	for unicode.IsSpace(m.value[m.pos]) {
		if m.pos <= 0 {
			break
		}
		m.SetCursor(m.pos - 1)
	}

	for m.pos > 0 {
		if !unicode.IsSpace(m.value[m.pos]) {
			m.SetCursor(m.pos - 1)
		} else {
			if m.pos > 0 {
				m.SetCursor(m.pos + 1)
			}
			break
		}
	}

	if oldPos > len(m.value) {
		m.value = m.value[:m.pos]
	} else {
		m.value = append(m.value[:m.pos], m.value[oldPos:]...)
	}
	m.Err = m.validate(m.value)
}

func (m *Model) deleteWordForward() {
	if m.pos >= len(m.value) || len(m.value) == 0 {
		return
	}

	if m.EchoMode != EchoNormal {
		m.deleteAfterCursor()
		return
	}

	oldPos := m.pos
	m.SetCursor(m.pos + 1)
	for unicode.IsSpace(m.value[m.pos]) {
		m.SetCursor(m.pos + 1)

		if m.pos >= len(m.value) {
			break
		}
	}

	for m.pos < len(m.value) {
		if !unicode.IsSpace(m.value[m.pos]) {
			m.SetCursor(m.pos + 1)
		} else {
			break
		}
	}

	if m.pos > len(m.value) {
		m.value = m.value[:oldPos]
	} else {
		m.value = append(m.value[:oldPos], m.value[m.pos:]...)
	}
	m.Err = m.validate(m.value)

	m.SetCursor(oldPos)
}

func (m *Model) wordBackward() {
	if m.pos == 0 || len(m.value) == 0 {
		return
	}

	if m.EchoMode != EchoNormal {
		m.CursorStart()
		return
	}

	i := m.pos - 1
	for i >= 0 {
		if unicode.IsSpace(m.value[i]) {
			m.SetCursor(m.pos - 1)
			i--
		} else {
			break
		}
	}

	for i >= 0 {
		if !unicode.IsSpace(m.value[i]) {
			m.SetCursor(m.pos - 1)
			i--
		} else {
			break
		}
	}
}

func (m *Model) wordForward() {
	if m.pos >= len(m.value) || len(m.value) == 0 {
		return
	}

	if m.EchoMode != EchoNormal {
		m.CursorEnd()
		return
	}

	i := m.pos
	for i < len(m.value) {
		if unicode.IsSpace(m.value[i]) {
			m.SetCursor(m.pos + 1)
			i++
		} else {
			break
		}
	}

	for i < len(m.value) {
		if !unicode.IsSpace(m.value[i]) {
			m.SetCursor(m.pos + 1)
			i++
		} else {
			break
		}
	}
}

func (m Model) echoTransform(v string) string {
	switch m.EchoMode {
	case EchoPassword:
		return strings.Repeat(string(m.EchoCharacter), uniseg.StringWidth(v))
	case EchoNone:
		return ""
	case EchoNormal:
		return v
	default:
		return v
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focus {
		return m, nil
	}

	keyMsg, ok := msg.(tea.KeyPressMsg)
	if ok && key.Matches(keyMsg, m.KeyMap.AcceptSuggestion) {
		if m.canAcceptSuggestion() {
			m.value = append(m.value, m.matchedSuggestions[m.currentSuggestionIndex][len(m.value):]...)
			m.CursorEnd()
		}
	}

	oldPos := m.pos

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.KeyMap.DeleteWordBackward):
			m.deleteWordBackward()
		case key.Matches(msg, m.KeyMap.DeleteCharacterBackward):
			m.Err = nil
			if len(m.value) > 0 {
				m.value = append(m.value[:max(0, m.pos-1)], m.value[m.pos:]...)
				m.Err = m.validate(m.value)
				if m.pos > 0 {
					m.SetCursor(m.pos - 1)
				}
			}
		case key.Matches(msg, m.KeyMap.WordBackward):
			m.wordBackward()
		case key.Matches(msg, m.KeyMap.CharacterBackward):
			if m.pos > 0 {
				m.SetCursor(m.pos - 1)
			}
		case key.Matches(msg, m.KeyMap.WordForward):
			m.wordForward()
		case key.Matches(msg, m.KeyMap.CharacterForward):
			if m.pos < len(m.value) {
				m.SetCursor(m.pos + 1)
			}
		case key.Matches(msg, m.KeyMap.LineStart):
			m.CursorStart()
		case key.Matches(msg, m.KeyMap.DeleteCharacterForward):
			if len(m.value) > 0 && m.pos < len(m.value) {
				m.value = slices.Delete(m.value, m.pos, m.pos+1)
				m.Err = m.validate(m.value)
			}
		case key.Matches(msg, m.KeyMap.LineEnd):
			m.CursorEnd()
		case key.Matches(msg, m.KeyMap.DeleteAfterCursor):
			m.deleteAfterCursor()
		case key.Matches(msg, m.KeyMap.DeleteBeforeCursor):
			m.deleteBeforeCursor()
		case key.Matches(msg, m.KeyMap.Paste):
			return m, Paste
		case key.Matches(msg, m.KeyMap.DeleteWordForward):
			m.deleteWordForward()
		case key.Matches(msg, m.KeyMap.NextSuggestion):
			m.nextSuggestion()
		case key.Matches(msg, m.KeyMap.PrevSuggestion):
			m.previousSuggestion()
		default:
			m.insertRunesFromUserInput([]rune(msg.Text))
		}

		m.updateSuggestions()

	case tea.PasteMsg:
		m.insertRunesFromUserInput([]rune(msg.Content))

	case pasteMsg:
		m.insertRunesFromUserInput([]rune(msg))

	case pasteErrMsg:
		m.Err = msg
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd

	if m.useVirtualCursor {
		m.virtualCursor, cmd = m.virtualCursor.Update(msg)
		cmds = append(cmds, cmd)

		if oldPos != m.pos && m.virtualCursor.Mode() == cursor.CursorBlink {
			m.virtualCursor.IsBlinked = false
			cmds = append(cmds, m.virtualCursor.Blink())
		}
	}

	m.handleOverflow()
	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if len(m.value) == 0 && m.Placeholder != "" {
		return m.placeholderView()
	}

	styles := m.activeStyle()

	styleText := styles.Text.Inline(true).Render

	value := m.value[m.offset:m.offsetRight]
	pos := max(0, m.pos-m.offset)
	v := styleText(m.echoTransform(string(value[:pos])))

	if pos < len(value) {
		char := m.echoTransform(string(value[pos]))
		m.virtualCursor.TextStyle = styles.Text
		m.virtualCursor.SetChar(char)
		v += m.virtualCursor.View()
		v += styleText(m.echoTransform(string(value[pos+1:])))
		v += m.completionView(0)
	} else {
		if m.focus && m.canAcceptSuggestion() {
			suggestion := m.matchedSuggestions[m.currentSuggestionIndex]
			if len(value) < len(suggestion) {
				m.virtualCursor.TextStyle = styles.Suggestion
				m.virtualCursor.SetChar(m.echoTransform(string(suggestion[pos])))
				v += m.virtualCursor.View()
				v += m.completionView(1)
			} else {
				m.virtualCursor.TextStyle = styles.Text
				m.virtualCursor.SetChar(" ")
				v += m.virtualCursor.View()
			}
		} else {
			m.virtualCursor.TextStyle = styles.Text
			m.virtualCursor.SetChar(" ")
			v += m.virtualCursor.View()
		}
	}

	valWidth := uniseg.StringWidth(string(value))
	if m.Width() > 0 && valWidth <= m.Width() {
		padding := max(0, m.Width()-valWidth)
		if valWidth+padding <= m.Width() && pos < len(value) {
			padding++
		}
		v += styleText(strings.Repeat(" ", padding))
	}

	return m.promptView() + v
}

func (m Model) promptView() string {
	return m.activeStyle().Prompt.Render(m.Prompt)
}

func (m Model) placeholderView() string {
	var (
		v      string
		styles = m.activeStyle()
		render = styles.Placeholder.Render
	)

	p := make([]rune, max(m.Width()+1, len(m.Placeholder)))
	copy(p, []rune(m.Placeholder))

	m.virtualCursor.TextStyle = styles.Placeholder
	m.virtualCursor.SetChar(string(p[:1]))
	v += m.virtualCursor.View()

	if m.Width() < 1 && len(p) <= 1 {
		return styles.Prompt.Render(m.Prompt) + v
	}

	if m.Width() > 0 {
		minWidth := lipgloss.Width(m.Placeholder)
		availWidth := m.Width() - minWidth + 1

		if availWidth < 0 {
			minWidth += availWidth
			availWidth = 0
		}
		v += render(string(p[1:minWidth]))
		v += render(strings.Repeat(" ", availWidth))
	} else {
		v += render(string(p[1:]))
	}

	return styles.Prompt.Render(m.Prompt) + v
}

func Blink() tea.Msg {
	return cursor.Blink()
}

func Paste() tea.Msg {
	str, err := clipboard.ReadAll()
	if err != nil {
		return pasteErrMsg{err}
	}
	return pasteMsg(str)
}

func clamp(v, low, high int) int {
	if high < low {
		low, high = high, low
	}
	return min(high, max(low, v))
}

func (m Model) completionView(offset int) string {
	if !m.canAcceptSuggestion() {
		return ""
	}
	value := m.value
	suggestion := m.matchedSuggestions[m.currentSuggestionIndex]
	if len(value) < len(suggestion) {
		return m.activeStyle().Suggestion.Inline(true).
			Render(string(suggestion[len(value)+offset:]))
	}
	return ""
}

func (m *Model) getSuggestions(sugs [][]rune) []string {
	suggestions := make([]string, len(sugs))
	for i, s := range sugs {
		suggestions[i] = string(s)
	}
	return suggestions
}

func (m *Model) AvailableSuggestions() []string {
	return m.getSuggestions(m.suggestions)
}

func (m *Model) MatchedSuggestions() []string {
	return m.getSuggestions(m.matchedSuggestions)
}

func (m *Model) CurrentSuggestionIndex() int {
	return m.currentSuggestionIndex
}

func (m *Model) CurrentSuggestion() string {
	if m.currentSuggestionIndex >= len(m.matchedSuggestions) {
		return ""
	}

	return string(m.matchedSuggestions[m.currentSuggestionIndex])
}

func (m *Model) canAcceptSuggestion() bool {
	return len(m.matchedSuggestions) > 0
}

func (m *Model) updateSuggestions() {
	if !m.ShowSuggestions {
		return
	}

	if len(m.value) <= 0 || len(m.suggestions) <= 0 {
		m.matchedSuggestions = [][]rune{}
		return
	}

	matches := [][]rune{}
	for _, s := range m.suggestions {
		suggestion := string(s)

		if strings.HasPrefix(strings.ToLower(suggestion), strings.ToLower(string(m.value))) {
			matches = append(matches, []rune(suggestion))
		}
	}
	if !reflect.DeepEqual(matches, m.matchedSuggestions) {
		m.currentSuggestionIndex = 0
	}

	m.matchedSuggestions = matches
}

func (m *Model) nextSuggestion() {
	m.currentSuggestionIndex = (m.currentSuggestionIndex + 1)
	if m.currentSuggestionIndex >= len(m.matchedSuggestions) {
		m.currentSuggestionIndex = 0
	}
}

func (m *Model) previousSuggestion() {
	m.currentSuggestionIndex = (m.currentSuggestionIndex - 1)
	if m.currentSuggestionIndex < 0 {
		m.currentSuggestionIndex = len(m.matchedSuggestions) - 1
	}
}

func (m Model) validate(v []rune) error {
	if m.Validate != nil {
		return m.Validate(string(v))
	}
	return nil
}

func (m Model) Cursor() *tea.Cursor {
	if m.useVirtualCursor || !m.Focused() {
		return nil
	}

	w := lipgloss.Width

	promptWidth := w(m.promptView())
	xOffset := m.Position() +
		promptWidth
	if m.width > 0 {
		xOffset = min(xOffset, m.width+promptWidth)
	}

	style := m.styles.Cursor
	c := tea.NewCursor(xOffset, 0)
	c.Blink = style.Blink
	c.Color = style.Color
	c.Shape = style.Shape
	return c
}

func (m *Model) updateVirtualCursorStyle() {
	if !m.useVirtualCursor {
		m.virtualCursor.SetMode(cursor.CursorHide)
		return
	}

	m.virtualCursor.Style = lipgloss.NewStyle().Foreground(m.styles.Cursor.Color)

	if m.styles.Cursor.Blink {
		if m.styles.Cursor.BlinkSpeed > 0 {
			m.virtualCursor.BlinkSpeed = m.styles.Cursor.BlinkSpeed
		}
		m.virtualCursor.SetMode(cursor.CursorBlink)
		return
	}
	m.virtualCursor.SetMode(cursor.CursorStatic)
}

func (m Model) activeStyle() *StyleState {
	if m.focus {
		return &m.styles.Focused
	}
	return &m.styles.Blurred
}
