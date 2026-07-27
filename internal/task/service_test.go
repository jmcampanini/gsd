package task

import (
	"context"
	"testing"
	"time"
)

type recordingRepository struct {
	addCalls  int
	title     string
	note      string
	timestamp string
	findCalls int
}

func (r *recordingRepository) Add(
	_ context.Context,
	title string,
	note string,
	timestamp string,
) (Task, error) {
	r.addCalls++
	r.title = title
	r.note = note
	r.timestamp = timestamp

	return Task{ID: 1, Title: title, Note: note, CreatedAt: timestamp, UpdatedAt: timestamp}, nil
}

func (*recordingRepository) Inbox(context.Context) ([]Task, error) {
	return []Task{}, nil
}

func (r *recordingRepository) Find(_ context.Context, id int64) (Task, error) {
	r.findCalls++
	return Task{ID: id}, nil
}

func TestAddPreservesAcceptedTextAndNormalizesTimestamp(t *testing.T) {
	t.Parallel()

	repository := &recordingRepository{}
	service := NewService(repository)
	service.now = func() time.Time {
		return time.Date(2026, time.July, 27, 12, 34, 56, 987654321, time.FixedZone("offset", -4*60*60))
	}

	title := "  Keep surrounding space  "
	note := "line one\nline two\n"
	created, err := service.Add(context.Background(), title, note)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if repository.title != title || created.Title != title {
		t.Errorf("title = %q, want exact %q", created.Title, title)
	}
	if repository.note != note || created.Note != note {
		t.Errorf("note = %q, want exact %q", created.Note, note)
	}
	if repository.timestamp != "2026-07-27T16:34:56.987Z" {
		t.Errorf("timestamp = %q, want UTC milliseconds", repository.timestamp)
	}
}

func TestAddRejectsInvalidTextBeforePersistence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		title string
		note  string
	}{
		{name: "blank title", title: " \t\n"},
		{name: "invalid title UTF-8", title: string([]byte{0xff})},
		{name: "invalid note UTF-8", title: "valid", note: string([]byte{0xff})},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			repository := &recordingRepository{}
			service := NewService(repository)
			_, err := service.Add(context.Background(), test.title, test.note)
			if err == nil {
				t.Fatal("Add() error = nil, want invalid_argument")
			}
			code, ok := ErrorCodeOf(err)
			if !ok || code != ErrorInvalidArgument {
				t.Errorf("Add() error = %v, want invalid_argument", err)
			}
			if repository.addCalls != 0 {
				t.Errorf("repository Add() calls = %d, want 0", repository.addCalls)
			}
		})
	}
}

func TestParseID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  int64
		valid bool
	}{
		{value: "1", want: 1, valid: true},
		{value: "001", want: 1, valid: true},
		{value: "0"},
		{value: "-1"},
		{value: "+1"},
		{value: "1.0"},
		{value: "１２"},
		{value: "9223372036854775808"},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			t.Parallel()

			got, err := ParseID(test.value)
			if test.valid {
				if err != nil {
					t.Fatalf("ParseID() error = %v", err)
				}
				if got != test.want {
					t.Errorf("ParseID() = %d, want %d", got, test.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("ParseID() = %d, want invalid_argument", got)
			}
			code, ok := ErrorCodeOf(err)
			if !ok || code != ErrorInvalidArgument {
				t.Errorf("ParseID() error = %v, want invalid_argument", err)
			}
		})
	}
}

func TestShowRejectsNonpositiveIDBeforePersistence(t *testing.T) {
	t.Parallel()

	repository := &recordingRepository{}
	service := NewService(repository)
	_, err := service.Show(context.Background(), 0)
	if err == nil {
		t.Fatal("Show() error = nil, want invalid_argument")
	}
	if repository.findCalls != 0 {
		t.Errorf("repository Find() calls = %d, want 0", repository.findCalls)
	}
}
