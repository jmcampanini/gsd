package dates

import (
	"testing"
	"time"
	_ "time/tzdata"
)

func TestParseAcceptedGrammarAndCanonicalOutput(t *testing.T) {
	t.Parallel()

	reference := time.Date(2024, time.January, 30, 18, 45, 0, 0, time.UTC)
	tests := []struct {
		value string
		want  string
	}{
		{value: "0000-02-29", want: "0000-02-29"},
		{value: "2024-02-29", want: "2024-02-29"},
		{value: "9999-12-31", want: "9999-12-31"},
		{value: "today", want: "2024-01-30"},
		{value: "tomorrow", want: "2024-01-31"},
		{value: "+0d", want: "2024-01-30"},
		{value: "+0002d", want: "2024-02-01"},
		{value: "+0w", want: "2024-01-30"},
		{value: "+01w", want: "2024-02-06"},
		{value: "+0000000000000000000000000000001d", want: "2024-01-31"},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			t.Parallel()

			got, err := Parse(test.value, reference)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", test.value, err)
			}
			if got != test.want {
				t.Errorf("Parse(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestParseUsesReferenceLocalCalendarAcrossDST(t *testing.T) {
	t.Parallel()

	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v", err)
	}
	reference := time.Date(2024, time.March, 9, 23, 30, 0, 0, location)

	for _, test := range []struct {
		value string
		want  string
	}{
		{value: "today", want: "2024-03-09"},
		{value: "tomorrow", want: "2024-03-10"},
		{value: "+1d", want: "2024-03-10"},
		{value: "+1w", want: "2024-03-16"},
	} {
		got, parseErr := Parse(test.value, reference)
		if parseErr != nil {
			t.Fatalf("Parse(%q) error = %v", test.value, parseErr)
		}
		if got != test.want {
			t.Errorf("Parse(%q) = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestParseWeekdaysAreStrictlyNext(t *testing.T) {
	t.Parallel()

	reference := time.Date(2024, time.February, 28, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		value string
		want  string
	}{
		{value: "sun", want: "2024-03-03"},
		{value: "mon", want: "2024-03-04"},
		{value: "tue", want: "2024-03-05"},
		{value: "wed", want: "2024-03-06"},
		{value: "thu", want: "2024-02-29"},
		{value: "fri", want: "2024-03-01"},
		{value: "sat", want: "2024-03-02"},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			t.Parallel()

			got, err := Parse(test.value, reference)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", test.value, err)
			}
			if got != test.want {
				t.Errorf("Parse(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestParseDayArithmeticAcrossCalendarBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		reference time.Time
		value     string
		want      string
	}{
		{
			name:      "common year February",
			reference: time.Date(2023, time.February, 28, 0, 0, 0, 0, time.UTC),
			value:     "+1d",
			want:      "2023-03-01",
		},
		{
			name:      "leap day",
			reference: time.Date(2024, time.February, 28, 0, 0, 0, 0, time.UTC),
			value:     "+1d",
			want:      "2024-02-29",
		},
		{
			name:      "end of leap month",
			reference: time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC),
			value:     "+1d",
			want:      "2024-03-01",
		},
		{
			name:      "end of year",
			reference: time.Date(2023, time.December, 31, 0, 0, 0, 0, time.UTC),
			value:     "+1d",
			want:      "2024-01-01",
		},
		{
			name:      "week over year",
			reference: time.Date(2024, time.December, 30, 0, 0, 0, 0, time.UTC),
			value:     "+1w",
			want:      "2025-01-06",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := Parse(test.value, test.reference)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", test.value, err)
			}
			if got != test.want {
				t.Errorf("Parse(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestParseRejectsUnsupportedAndMalformedValues(t *testing.T) {
	t.Parallel()

	reference := time.Date(2024, time.June, 15, 0, 0, 0, 0, time.UTC)
	values := []string{
		"",
		" today",
		"today ",
		"Today",
		"TOMORROW",
		"next monday",
		"monday",
		"MON",
		"2024-2-01",
		"24-02-01",
		"02024-02-01",
		"2024-02-1",
		"2024/02/01",
		"2024-00-01",
		"2024-13-01",
		"2024-02-30",
		"2023-02-29",
		"10000-01-01",
		"-001-01-01",
		"２０２４-０２-０１",
		"+d",
		"+1",
		"+1D",
		"+1m",
		"+ 1d",
		"+1 d",
		"1d",
		"++1d",
		"+-1d",
		"+1.0d",
		"+１d",
		"-1d",
		"+1dd",
		"+18446744073709551616d",
		"+2635249153387078803w",
	}

	for _, value := range values {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			got, err := Parse(value, reference)
			if err == nil {
				t.Fatalf("Parse(%q) = %q, want error", value, got)
			}
			if got != "" {
				t.Errorf("Parse(%q) result = %q on error, want empty", value, got)
			}
		})
	}
}

func TestParseRejectsArithmeticOutsideCanonicalYears(t *testing.T) {
	t.Parallel()

	maximum := time.Date(9999, time.December, 31, 0, 0, 0, 0, time.UTC)
	for _, value := range []string{"tomorrow", "+1d", "+1w", "sat"} {
		got, err := Parse(value, maximum)
		if err == nil {
			t.Fatalf("Parse(%q, maximum) = %q, want error", value, got)
		}
	}

	got, err := Parse("+1d", time.Date(9999, time.December, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Parse(+1d, day before maximum) error = %v", err)
	}
	if got != "9999-12-31" {
		t.Errorf("Parse(+1d, day before maximum) = %q, want 9999-12-31", got)
	}

	for _, reference := range []time.Time{
		time.Date(-1, time.December, 31, 0, 0, 0, 0, time.UTC),
		time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC),
	} {
		if result, parseErr := Parse("today", reference); parseErr == nil {
			t.Errorf("Parse(today, year %d) = %q, want error", reference.Year(), result)
		}
	}
}
