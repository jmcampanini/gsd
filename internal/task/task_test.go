package task

import "testing"

func TestOverdueRequiresOpenStatusAndArrivedDueDate(t *testing.T) {
	t.Parallel()

	due := func(value string) *string { return &value }
	today := "2026-08-15"
	tests := []struct {
		name    string
		current Task
		want    bool
	}{
		{name: "open due today", current: Task{Status: StatusOpen, DueOn: due("2026-08-15")}, want: true},
		{name: "open past due", current: Task{Status: StatusOpen, DueOn: due("2026-08-01")}, want: true},
		{name: "open future due", current: Task{Status: StatusOpen, DueOn: due("2026-08-16")}},
		{name: "open without due date", current: Task{Status: StatusOpen}},
		{name: "done past due", current: Task{Status: StatusDone, DueOn: due("2026-08-01")}},
		{name: "cancelled past due", current: Task{Status: StatusCancelled, DueOn: due("2026-08-01")}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := Overdue(test.current, today); got != test.want {
				t.Errorf("Overdue() = %t, want %t", got, test.want)
			}
		})
	}
}
