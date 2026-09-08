package main

import (
	"image/color"

	"mc/internal/paint"
)

type theme struct {
	baseStyle      paint.Style
	emptyStyle     paint.Style
	cursorStyle    paint.Style
	selectionStyle paint.Style

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
	nordTheme
	tokyonightTheme
)

const defaultTheme = draculaTheme

type themeItem struct {
	name   string
	preset colorPreset
}

var themeList = []themeItem{
	{"autumn", autumnTheme},
	{"base16", base16Theme},
	{"dracula", draculaTheme},
	{"ferra", ferraTheme},
	{"github", githubTheme},
	{"monokai", monokaiTheme},
	{"nord", nordTheme},
	{"tokyonight", tokyonightTheme},
}

func findPreset(name string) colorPreset {
	for i := range themeList {
		if themeList[i].name == name {
			return themeList[i].preset
		}
	}
	return defaultTheme
}

func findPresetIndex(name string) int {
	for i := range themeList {
		if themeList[i].name == name {
			return i
		}
	}
	return 0
}

func (m *model) updateTheme() {
	setTextinputStyle(&m.input, m.theme)
	setTextinputStyle(&m.pathInput, m.theme)
	setSpinnerStyle(&m.spinner, m.theme)
	if m.search != nil {
		setTextinputStyle(&m.search.filename, m.theme)
		setTextinputStyle(&m.search.text, m.theme)
	}
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

	preset := findPreset(name)

	switch preset {
	case autumnTheme:
		base = paint.Color("#232323")
		empty = paint.Color("#212121")
		cursor = paint.Color("#404040")

		white = paint.Color("#F3F2CC")
		black = paint.Color("#212121")
		gray = paint.Color("#646f69")
		green = paint.Color("#99be70")
		red = paint.Color("#F05E48")
		accent1 = paint.Color("#86c1b9")
		accent2 = paint.Color("#727ca5")
		accent3 = paint.Color("#72a59e")
		accent4 = paint.Color("#cfba8b")
		accent5 = paint.Color("#FAD566")

	case base16Theme:
		base = paint.NoColor{}
		empty = paint.NoColor{}
		cursor = paint.NoColor{}

		white = paint.NoColor{}
		black = paint.Color("#000000") // there is no other way
		gray = paint.BrightBlack
		green = paint.Green
		red = paint.Red
		accent1 = paint.Cyan
		accent2 = paint.BrightCyan
		accent3 = paint.BrightBlue
		accent4 = paint.Yellow
		accent5 = paint.BrightRed

	case draculaTheme:
		base = paint.Color("#282a36")
		empty = paint.Color("#222430")
		cursor = paint.Color("#44475a")

		white = paint.Color("#ffffff")
		black = paint.Color("#000000")
		gray = paint.Color("#6272a4")
		green = paint.Color("#94d716")
		red = paint.Color("#ea1212")
		accent1 = paint.Color("#ff79c6")
		accent2 = paint.Color("#bd93f9")
		accent3 = paint.Color("#8be9fd")
		accent4 = paint.Color("#f1fa8c")
		accent5 = paint.Color("#ffb86c")

	case ferraTheme:
		base = paint.Color("#2b292d")
		empty = paint.Color("#2b292d")
		cursor = paint.Color("#383539")

		white = paint.Color("#D1D1E0")
		black = paint.Color("#000000")
		gray = paint.Color("#4d424b")
		green = paint.Color("#B1B695")
		red = paint.Color("#e06b75")
		accent1 = paint.Color("#F5D76E")
		accent2 = paint.Color("#F6B6C9")
		accent3 = paint.Color("#D1D1E0")
		accent4 = paint.Color("#fecdb2")
		accent5 = paint.Color("#ffa07a")

	case githubTheme:
		base = paint.Color("#22272e")
		empty = paint.Color("#22272e")
		cursor = paint.Color("#373e47")

		white = paint.Color("#adbac7")
		black = paint.Color("#1c2128")
		gray = paint.Color("#768390")
		green = paint.Color("#57ab5a")
		red = paint.Color("#e5534b")
		accent1 = paint.Color("#c96198")
		accent2 = paint.Color("#8256d0")
		accent3 = paint.Color("#96d0ff")
		accent4 = paint.Color("#eac55f")
		accent5 = paint.Color("#f69d50")

	case monokaiTheme:
		base = paint.Color("#272822")
		empty = paint.Color("#1e1f1c")
		cursor = paint.Color("#414339")

		white = paint.Color("#f8f8f2")
		black = paint.Color("#1c2128")
		gray = paint.Color("#878b91")
		green = paint.Color("#a6e22e")
		red = paint.Color("#f48771")
		accent1 = paint.Color("#F92672")
		accent2 = paint.Color("#C586C0")
		accent3 = paint.Color("#75beff")
		accent4 = paint.Color("#e6db74")
		accent5 = paint.Color("#fd971f")

	case nordTheme:
		base = paint.Color("#2e3440")
		empty = paint.Color("#2e3440")
		cursor = paint.Color("#3b4252")

		white = paint.Color("#ECEFF4")
		black = paint.Color("#1e2129")
		gray = paint.Color("#4C566A")
		green = paint.Color("#A3BE8C")
		red = paint.Color("#BF616A")
		accent1 = paint.Color("#5E81AC")
		accent2 = paint.Color("#81A1C1")
		accent3 = paint.Color("#D08770")
		accent4 = paint.Color("#88C0D0")
		accent5 = paint.Color("#B48EAD")

	case tokyonightTheme:
		base = paint.Color("#222436")
		empty = paint.Color("#1e2030")
		cursor = paint.Color("#2f334d")

		white = paint.Color("#c8d3f5")
		black = paint.Color("#1b1d2b")
		gray = paint.Color("#636da6")
		green = paint.Color("#4fd6be")
		red = paint.Color("#ff757f")
		accent1 = paint.Color("#ff966c")
		accent2 = paint.Color("#ffc777")
		accent3 = paint.Color("#4fd6be")
		accent4 = paint.Color("#65bcff")
		accent5 = paint.Color("#c099ff")
	}

	defaultStyle := paint.NewStyle().Foreground(white)
	selection := blendColor(base, cursor, 0.35)

	return &theme{
		baseStyle:      defaultStyle.Background(base),
		emptyStyle:     defaultStyle.Background(empty),
		cursorStyle:    defaultStyle.Background(cursor),
		selectionStyle: defaultStyle.Background(selection),

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

func blendColor(base, accent color.Color, amount float64) color.Color {
	br, bg, bb, ba := base.RGBA()
	ar, ag, ab, aa := accent.RGBA()
	if ba == 0 || aa == 0 {
		return base
	}
	blend := func(a, b uint32) uint8 {
		return uint8((float64(a)*(1-amount) + float64(b)*amount) / 257)
	}
	return color.RGBA{blend(br, ar), blend(bg, ag), blend(bb, ab), 255}
}
