package domain

import (
	"reflect"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
)

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
