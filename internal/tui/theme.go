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
	if isDark {
		return Theme{
			Accent:     lipgloss.Color("#ca9ee6"),
			AccentText: lipgloss.Color("#303446"),
			InputBg:    lipgloss.Color("#414559"),
			Text:       lipgloss.Color("#c6d0f5"),
			Dim:        lipgloss.Color("#838ba7"),
			Green:      lipgloss.Color("#a6d189"),
			Red:        lipgloss.Color("#e78284"),
		}
	}

	return Theme{
		Accent:     lipgloss.Color("#8839ef"),
		AccentText: lipgloss.Color("#eff1f5"),
		InputBg:    lipgloss.Color("#dce0e8"),
		Text:       lipgloss.Color("#4c4f69"),
		Dim:        lipgloss.Color("#8c8fa1"),
		Green:      lipgloss.Color("#40a02b"),
		Red:        lipgloss.Color("#d20f39"),
	}
}
