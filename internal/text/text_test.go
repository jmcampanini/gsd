package text

import "testing"

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
