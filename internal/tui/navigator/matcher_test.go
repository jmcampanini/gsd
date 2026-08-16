package navigator

import (
	"slices"
	"testing"
)

func TestMatchRowsUsesSubsequencesAndPreservesSourceOrder(t *testing.T) {
	source := []string{"Call plumber", "plmb exactly", "unrelated"}

	got := matchRows("plmb", source)
	if len(got) != 2 {
		t.Fatalf("matches = %#v, want two", got)
	}
	if got[0].index != 0 || !slices.Equal(got[0].positions, []int{5, 6, 8, 9}) {
		t.Errorf("first match = %#v, want source 0 at byte positions [5 6 8 9]", got[0])
	}
	if got[1].index != 1 || !slices.Equal(got[1].positions, []int{0, 1, 2, 3}) {
		t.Errorf("second match = %#v, want source 1 at byte positions [0 1 2 3]", got[1])
	}
}

func TestMatchRowsUsesSmartCase(t *testing.T) {
	source := []string{"Call plumber", "call plumber", "Call pLumber"}

	tests := []struct {
		name    string
		pattern string
		want    []int
	}{
		{name: "lowercase is insensitive", pattern: "call", want: []int{0, 1, 2}},
		{name: "uppercase makes match sensitive", pattern: "Call", want: []int{0, 2}},
		{name: "mixed case is sensitive throughout", pattern: "pLmb", want: []int{2}},
		{name: "wrong uppercase does not match", pattern: "Plmb", want: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := matchRows(test.pattern, source)
			indexes := make([]int, len(got))
			for index := range got {
				indexes[index] = got[index].index
			}
			if !slices.Equal(indexes, test.want) {
				t.Errorf("source indexes = %v, want %v", indexes, test.want)
			}
		})
	}
}

func TestMatchRowsReportsUnicodeBytePositions(t *testing.T) {
	got := matchRows("é界", []string{"no match", "Café 世界"})

	if len(got) != 1 {
		t.Fatalf("matches = %#v, want one", got)
	}
	if got[0].index != 1 || !slices.Equal(got[0].positions, []int{3, 9}) {
		t.Errorf("match = %#v, want source 1 at UTF-8 byte positions [3 9]", got[0])
	}
}
