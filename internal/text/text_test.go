package text

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestEllipsize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		width int
		want  string
	}{
		{name: "ASCII shorter than width", value: "task", width: 5, want: "task"},
		{name: "ASCII exact width", value: "task", width: 4, want: "task"},
		{name: "ASCII truncated", value: "tasks", width: 4, want: "tas…"},
		{name: "wide Unicode", value: "界界界", width: 4, want: "界…"},
		{
			name:  "ANSI style preserved",
			value: "\x1b[31mtasks\x1b[0m",
			width: 4,
			want:  "\x1b[31mtas…\x1b[0m",
		},
		{name: "zero width", value: "task", width: 0, want: ""},
		{name: "negative width", value: "task", width: -1, want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := Ellipsize(test.value, test.width)
			if got != test.want {
				t.Errorf("Ellipsize(%q, %d) = %q, want %q", test.value, test.width, got, test.want)
			}
			if width := ansi.StringWidth(got); width > max(test.width, 0) {
				t.Errorf("Ellipsize(%q, %d) visible width = %d, want at most %d", test.value, test.width, width, max(test.width, 0))
			}
		})
	}
}

func TestHuman(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		value             string
		preserveLineFeeds bool
		want              string
	}{
		{name: "plain text passes through", value: "Call plumber é 界", want: "Call plumber é 界"},
		{
			name:  "control characters become visible escapes",
			value: "bad\nvalue\x1b]8;;https://example.com\a",
			want:  `bad\nvalue\x1b]8;;https://example.com\a`,
		},
		{
			name:              "preserved line feeds keep other escapes",
			value:             "line one\nline\ttwo",
			preserveLineFeeds: true,
			want:              "line one\nline\\ttwo",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := Human(test.value, test.preserveLineFeeds); got != test.want {
				t.Errorf("Human(%q, %t) = %q, want %q", test.value, test.preserveLineFeeds, got, test.want)
			}
		})
	}
}
