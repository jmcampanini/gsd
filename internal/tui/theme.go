package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

type Theme struct {
	Accent     color.Color
	AccentText color.Color
	InputBg    color.Color
	Text       color.Color
	Dim        color.Color
	Cursor     color.Color
	Green      color.Color
	Red        color.Color
}

func ThemeForBackground(isDark bool) Theme {
	lightDark := lipgloss.LightDark(isDark)
	return Theme{
		Accent:     lightDark(lipgloss.Color("#8839ef"), lipgloss.Color("#ca9ee6")),
		AccentText: lightDark(lipgloss.Color("#eff1f5"), lipgloss.Color("#303446")),
		InputBg:    lightDark(lipgloss.Color("#dce0e8"), lipgloss.Color("#414559")),
		Text:       lightDark(lipgloss.Color("#4c4f69"), lipgloss.Color("#c6d0f5")),
		Dim:        lightDark(lipgloss.Color("#8c8fa1"), lipgloss.Color("#838ba7")),
		Green:      lightDark(lipgloss.Color("#40a02b"), lipgloss.Color("#a6d189")),
		Red:        lightDark(lipgloss.Color("#d20f39"), lipgloss.Color("#e78284")),
	}
}
