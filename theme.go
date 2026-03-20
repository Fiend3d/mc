package main

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

type theme struct {
	baseStyle   lipgloss.Style
	emptyStyle  lipgloss.Style
	cursorStyle lipgloss.Style

	whiteColor color.Color
	blackColor color.Color
	grayColor  color.Color
	greenColor color.Color
	redColor   color.Color

	accentColor1 color.Color
	accentColor2 color.Color
	accentColor3 color.Color
	accentColor4 color.Color
	accentColor5 color.Color
}

type colorPreset int

const (
	autumnTheme colorPreset = iota
	base16Theme
	draculaTheme
	ferraTheme
	githubTheme
	monokaiTheme
)

const defaultTheme = draculaTheme

var themeMap = map[string]colorPreset{
	"autumn":  autumnTheme,
	"base16":  base16Theme,
	"dracula": draculaTheme,
	"ferra":   ferraTheme,
	"github":  githubTheme,
	"monokai": monokaiTheme,
}

func newTheme(name string) *theme {
	var base color.Color
	var empty color.Color
	var cursor color.Color

	var white color.Color
	var black color.Color
	var gray color.Color
	var green color.Color
	var red color.Color
	var accent1 color.Color
	var accent2 color.Color
	var accent3 color.Color
	var accent4 color.Color
	var accent5 color.Color

	preset, ok := themeMap[name]
	if !ok {
		preset = defaultTheme
	}

	switch preset {
	case autumnTheme:
		base = lipgloss.Color("#232323")
		empty = lipgloss.Color("#212121")
		cursor = lipgloss.Color("#404040")

		white = lipgloss.Color("#F3F2CC")
		black = lipgloss.Color("#212121")
		gray = lipgloss.Color("#646f69")
		green = lipgloss.Color("#99be70")
		red = lipgloss.Color("#F05E48")
		accent1 = lipgloss.Color("#86c1b9")
		accent2 = lipgloss.Color("#727ca5")
		accent3 = lipgloss.Color("#72a59e")
		accent4 = lipgloss.Color("#cfba8b")
		accent5 = lipgloss.Color("#FAD566")

	case base16Theme:
		base = lipgloss.NoColor{}
		empty = lipgloss.NoColor{}
		cursor = lipgloss.NoColor{}

		white = lipgloss.NoColor{}
		black = lipgloss.Color("#000000") // there is no other way
		gray = lipgloss.BrightBlack
		green = lipgloss.Green
		red = lipgloss.Red
		accent1 = lipgloss.Cyan
		accent2 = lipgloss.BrightCyan
		accent3 = lipgloss.BrightBlue
		accent4 = lipgloss.Yellow
		accent5 = lipgloss.BrightRed

	case draculaTheme:
		base = lipgloss.Color("#282a36")
		empty = lipgloss.Color("#222430")
		cursor = lipgloss.Color("#44475a")

		white = lipgloss.Color("#ffffff")
		black = lipgloss.Color("#000000")
		gray = lipgloss.Color("#6272a4")
		green = lipgloss.Color("#94d716")
		red = lipgloss.Color("#ea1212")
		accent1 = lipgloss.Color("#ff79c6")
		accent2 = lipgloss.Color("#bd93f9")
		accent3 = lipgloss.Color("#8be9fd")
		accent4 = lipgloss.Color("#f1fa8c")
		accent5 = lipgloss.Color("#ffb86c")

	case ferraTheme:
		base = lipgloss.Color("#2b292d")
		empty = lipgloss.Color("#2b292d")
		cursor = lipgloss.Color("#383539")

		white = lipgloss.Color("#D1D1E0")
		black = lipgloss.Color("#000000")
		gray = lipgloss.Color("#4d424b")
		green = lipgloss.Color("#B1B695")
		red = lipgloss.Color("#e06b75")
		accent1 = lipgloss.Color("#F5D76E")
		accent2 = lipgloss.Color("#F6B6C9")
		accent3 = lipgloss.Color("#D1D1E0")
		accent4 = lipgloss.Color("#fecdb2")
		accent5 = lipgloss.Color("#ffa07a")

	case githubTheme:
		base = lipgloss.Color("#22272e")
		empty = lipgloss.Color("#22272e")
		cursor = lipgloss.Color("#373e47")

		white = lipgloss.Color("#adbac7")
		black = lipgloss.Color("#1c2128")
		gray = lipgloss.Color("#768390")
		green = lipgloss.Color("#57ab5a")
		red = lipgloss.Color("#e5534b")
		accent1 = lipgloss.Color("#c96198")
		accent2 = lipgloss.Color("#8256d0")
		accent3 = lipgloss.Color("#96d0ff")
		accent4 = lipgloss.Color("#eac55f")
		accent5 = lipgloss.Color("#f69d50")

	case monokaiTheme:
		base = lipgloss.Color("#272822")
		empty = lipgloss.Color("#1e1f1c")
		cursor = lipgloss.Color("#414339")

		white = lipgloss.Color("#f8f8f2")
		black = lipgloss.Color("#1c2128")
		gray = lipgloss.Color("#878b91")
		green = lipgloss.Color("#a6e22e")
		red = lipgloss.Color("#f48771")
		accent1 = lipgloss.Color("#C586C0")
		accent2 = lipgloss.Color("#F92672")
		accent3 = lipgloss.Color("#75beff")
		accent4 = lipgloss.Color("#e6db74")
		accent5 = lipgloss.Color("#fd971f")
	}

	defaultStyle := lipgloss.NewStyle().Foreground(white)
	return &theme{
		baseStyle:   defaultStyle.Background(base),
		emptyStyle:  defaultStyle.Background(empty),
		cursorStyle: defaultStyle.Background(cursor),

		whiteColor: white,
		blackColor: black,
		grayColor:  gray,
		greenColor: green,
		redColor:   red,

		accentColor1: accent1,
		accentColor2: accent2,
		accentColor3: accent3,
		accentColor4: accent4,
		accentColor5: accent5,
	}
}
