package domain

import (
	"reflect"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
)

func TestTagNamesMarshalJSONAlwaysEmitsAnArray(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		value TagNames
		want  string
	}{
		{name: "nil", want: `[]`},
		{name: "non-nil", value: TagNames{"home", "errands"}, want: `["home","errands"]`},
		{
			name:  "HTML characters are not escaped",
			value: TagNames{"<repair> & upkeep"},
			want:  `["<repair> & upkeep"]`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := MarshalCompactJSON(test.value)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != test.want {
				t.Errorf("Marshal() = %s, want %s", got, test.want)
			}
		})
	}
}

func TestNormalizeTagNamesMatchesSQLiteNoCaseAndPreservesFirstSpelling(t *testing.T) {
	t.Parallel()

	names := []string{"Errands", "ERRANDS", "é", "É", "home", "Home"}
	got, err := NormalizeTagNames(names)
	if err != nil {
		t.Fatalf("NormalizeTagNames() error = %v", err)
	}
	want := []string{"Errands", "é", "É", "home"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("NormalizeTagNames() = %q, want %q", got, want)
	}
}

func TestNormalizeTagNamesValidatesWithoutChangingAcceptedText(t *testing.T) {
	t.Parallel()

	got, err := NormalizeTagNames([]string{"  errands  "})
	if err != nil {
		t.Fatalf("NormalizeTagNames() error = %v", err)
	}
	if !reflect.DeepEqual(got, []string{"  errands  "}) {
		t.Errorf("NormalizeTagNames() = %q, want stored text unchanged", got)
	}

	for _, names := range [][]string{{"   "}, {string([]byte{0xff})}} {
		if _, err := NormalizeTagNames(names); errorCode(err) != apperr.InvalidArgument {
			t.Errorf("NormalizeTagNames(%q) error = %v, want invalid_argument", names, err)
		}
	}
}

func errorCode(err error) apperr.Code {
	code, _ := apperr.CodeOf(err)
	return code
}
