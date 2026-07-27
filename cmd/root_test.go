package cmd

import (
	"errors"
	"fmt"
	"testing"
)

type testApplicationError struct{}

func (testApplicationError) Error() string {
	return "application error"
}

func (testApplicationError) exitCategory() errorCategory {
	return errorCategoryApplication
}

func TestExitCodeForError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "success", want: 0},
		{name: "application", err: testApplicationError{}, want: 1},
		{name: "wrapped application", err: fmt.Errorf("context: %w", testApplicationError{}), want: 1},
		{name: "usage", err: errors.New("usage error"), want: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := exitCodeForError(test.err); got != test.want {
				t.Fatalf("exitCodeForError() = %d, want %d", got, test.want)
			}
		})
	}
}
