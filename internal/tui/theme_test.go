package tui

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestThemeForBackground(t *testing.T) {
	tests := []struct {
		name string
		dark bool
		want Theme
	}{
		{
			name: "Latte",
			want: Theme{
				Accent:     lipgloss.Color("#8839ef"),
				AccentText: lipgloss.Color("#eff1f5"),
				InputBg:    lipgloss.Color("#dce0e8"),
				Text:       lipgloss.Color("#4c4f69"),
				Dim:        lipgloss.Color("#8c8fa1"),
				Green:      lipgloss.Color("#40a02b"),
				Red:        lipgloss.Color("#d20f39"),
				Yellow:     lipgloss.Color("#df8e1d"),
			},
		},
		{
			name: "Frappe",
			dark: true,
			want: Theme{
				Accent:     lipgloss.Color("#ca9ee6"),
				AccentText: lipgloss.Color("#303446"),
				InputBg:    lipgloss.Color("#414559"),
				Text:       lipgloss.Color("#c6d0f5"),
				Dim:        lipgloss.Color("#838ba7"),
				Green:      lipgloss.Color("#a6d189"),
				Red:        lipgloss.Color("#e78284"),
				Yellow:     lipgloss.Color("#e5c890"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ThemeForBackground(test.dark)
			assertColor(t, "Accent", got.Accent, test.want.Accent)
			assertColor(t, "AccentText", got.AccentText, test.want.AccentText)
			assertColor(t, "InputBg", got.InputBg, test.want.InputBg)
			assertColor(t, "Text", got.Text, test.want.Text)
			assertColor(t, "Dim", got.Dim, test.want.Dim)
			if got.Cursor != nil {
				t.Errorf("Cursor = %v, want terminal default", got.Cursor)
			}
			assertColor(t, "Green", got.Green, test.want.Green)
			assertColor(t, "Red", got.Red, test.want.Red)
			assertColor(t, "Yellow", got.Yellow, test.want.Yellow)
		})
	}
}

func assertColor(t *testing.T, token string, got, want color.Color) {
	t.Helper()

	if got == nil || want == nil {
		if got != want {
			t.Errorf("%s = %v, want %v", token, got, want)
		}
		return
	}

	gotR, gotG, gotB, gotA := got.RGBA()
	wantR, wantG, wantB, wantA := want.RGBA()
	if gotR != wantR || gotG != wantG || gotB != wantB || gotA != wantA {
		t.Errorf(
			"%s RGBA = (%d, %d, %d, %d), want (%d, %d, %d, %d)",
			token,
			gotR,
			gotG,
			gotB,
			gotA,
			wantR,
			wantG,
			wantB,
			wantA,
		)
	}
}
