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

func newTheme() *theme {
	white := lipgloss.Color("#ffffff")
	black := lipgloss.Color("#000000")
	gray := lipgloss.Color("#6272a4")
	green := lipgloss.Color("#94d716")
	red := lipgloss.Color("#ea1212")
	accent1 := lipgloss.Color("#ff79c6")
	accent2 := lipgloss.Color("#bd93f9")
	accent3 := lipgloss.Color("#8be9fd")
	accent4 := lipgloss.Color("#f1fa8c")
	accent5 := lipgloss.Color("#ffb86c")

	defaultStyle := lipgloss.NewStyle().Foreground(white)
	return &theme{
		baseStyle:   defaultStyle.Background(lipgloss.Color("#282a36")),
		emptyStyle:  defaultStyle.Background(lipgloss.Color("#222430")),
		cursorStyle: defaultStyle.Background(lipgloss.Color("#44475a")),

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
