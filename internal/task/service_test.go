package task

import (
	"context"
	"testing"
	"time"
)

type recordingRepository struct {
	addCalls           int
	title              string
	note               string
	timestamp          string
	findCalls          int
	listCalls          int
	listedStatus       ListStatus
	listResult         []Task
	doneCalls          int
	cancelCalls        int
	reopenCalls        int
	deleteCalls        int
	lifecycleID        int64
	lifecycleTimestamp string
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

func (r *recordingRepository) List(_ context.Context, status ListStatus) ([]Task, error) {
	r.listCalls++
	r.listedStatus = status
	return r.listResult, nil
}

func (r *recordingRepository) Done(_ context.Context, id int64, timestamp string) (Task, error) {
	r.doneCalls++
	r.recordLifecycle(id, timestamp)
	return Task{ID: id, DoneAt: &timestamp}, nil
}

func (r *recordingRepository) Cancel(_ context.Context, id int64, timestamp string) (Task, error) {
	r.cancelCalls++
	r.recordLifecycle(id, timestamp)
	return Task{ID: id, CancelledAt: &timestamp}, nil
}

func (r *recordingRepository) Reopen(_ context.Context, id int64, timestamp string) (Task, error) {
	r.reopenCalls++
	r.recordLifecycle(id, timestamp)
	return Task{ID: id}, nil
}

func (r *recordingRepository) Delete(_ context.Context, id int64) (Task, error) {
	r.deleteCalls++
	r.lifecycleID = id
	return Task{ID: id}, nil
}

func (r *recordingRepository) recordLifecycle(id int64, timestamp string) {
	r.lifecycleID = id
	r.lifecycleTimestamp = timestamp
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

func TestParseListStatus(t *testing.T) {
	t.Parallel()

	for _, value := range []ListStatus{
		ListStatusOpen,
		ListStatusDone,
		ListStatusCancelled,
		ListStatusAll,
	} {
		value := value
		t.Run(string(value), func(t *testing.T) {
			t.Parallel()
			got, err := ParseListStatus(string(value))
			if err != nil {
				t.Fatalf("ParseListStatus() error = %v", err)
			}
			if got != value {
				t.Errorf("ParseListStatus() = %q, want %q", got, value)
			}
		})
	}

	if _, err := ParseListStatus("OPEN"); err == nil {
		t.Fatal("ParseListStatus(OPEN) error = nil, want invalid_argument")
	} else if code, ok := ErrorCodeOf(err); !ok || code != ErrorInvalidArgument {
		t.Errorf("ParseListStatus(OPEN) error = %v, want invalid_argument", err)
	}
}

func TestListValidatesStatusAndNormalizesNil(t *testing.T) {
	t.Parallel()

	repository := &recordingRepository{}
	service := NewService(repository)

	listed, err := service.List(context.Background(), ListStatusDone)
	if err != nil {
		t.Fatalf("List(done) error = %v", err)
	}
	if listed == nil || len(listed) != 0 {
		t.Errorf("List(done) = %#v, want non-nil empty list", listed)
	}
	if repository.listCalls != 1 || repository.listedStatus != ListStatusDone {
		t.Errorf("repository List() calls/status = %d/%q, want 1/%q", repository.listCalls, repository.listedStatus, ListStatusDone)
	}

	_, err = service.List(context.Background(), ListStatus("invalid"))
	if err == nil {
		t.Fatal("List(invalid) error = nil, want invalid_argument")
	}
	if code, ok := ErrorCodeOf(err); !ok || code != ErrorInvalidArgument {
		t.Errorf("List(invalid) error = %v, want invalid_argument", err)
	}
	if repository.listCalls != 1 {
		t.Errorf("repository List() calls = %d, want 1", repository.listCalls)
	}
}

func TestLifecycleValidatesIDBeforePersistence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		apply func(*Service) error
		calls func(*recordingRepository) int
	}{
		{name: "done", apply: func(service *Service) error { _, err := service.Done(context.Background(), 0); return err }, calls: func(repository *recordingRepository) int { return repository.doneCalls }},
		{name: "cancel", apply: func(service *Service) error { _, err := service.Cancel(context.Background(), 0); return err }, calls: func(repository *recordingRepository) int { return repository.cancelCalls }},
		{name: "reopen", apply: func(service *Service) error { _, err := service.Reopen(context.Background(), 0); return err }, calls: func(repository *recordingRepository) int { return repository.reopenCalls }},
		{name: "delete", apply: func(service *Service) error { _, err := service.Delete(context.Background(), 0); return err }, calls: func(repository *recordingRepository) int { return repository.deleteCalls }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &recordingRepository{}
			err := test.apply(NewService(repository))
			if err == nil {
				t.Fatal("lifecycle error = nil, want invalid_argument")
			}
			if code, ok := ErrorCodeOf(err); !ok || code != ErrorInvalidArgument {
				t.Errorf("lifecycle error = %v, want invalid_argument", err)
			}
			if calls := test.calls(repository); calls != 0 {
				t.Errorf("repository calls = %d, want 0", calls)
			}
		})
	}
}

func TestLifecycleDelegatesOneNormalizedTimestampPerAttempt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		apply func(*Service) error
		calls func(*recordingRepository) int
	}{
		{name: "done", apply: func(service *Service) error { _, err := service.Done(context.Background(), 7); return err }, calls: func(repository *recordingRepository) int { return repository.doneCalls }},
		{name: "cancel", apply: func(service *Service) error { _, err := service.Cancel(context.Background(), 7); return err }, calls: func(repository *recordingRepository) int { return repository.cancelCalls }},
		{name: "reopen", apply: func(service *Service) error { _, err := service.Reopen(context.Background(), 7); return err }, calls: func(repository *recordingRepository) int { return repository.reopenCalls }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := &recordingRepository{}
			service := NewService(repository)
			nowCalls := 0
			service.now = func() time.Time {
				nowCalls++
				return time.Date(2026, time.July, 27, 12, 34, 56, 987654321, time.FixedZone("offset", -4*60*60))
			}

			if err := test.apply(service); err != nil {
				t.Fatalf("lifecycle error = %v", err)
			}
			if test.calls(repository) != 1 || repository.lifecycleID != 7 {
				t.Errorf("repository calls/ID = %d/%d, want 1/7", test.calls(repository), repository.lifecycleID)
			}
			if nowCalls != 1 || repository.lifecycleTimestamp != "2026-07-27T16:34:56.987Z" {
				t.Errorf("now calls/timestamp = %d/%q, want 1/UTC milliseconds", nowCalls, repository.lifecycleTimestamp)
			}
		})
	}
}
