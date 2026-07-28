package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/jmcampanini/gsd/internal/task"
)

type errorEnvelope struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    task.ErrorCode `json:"code"`
	Message string         `json:"message"`
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("write JSON output: %w", err)
	}

	return nil
}

func writeCommandError(writer io.Writer, jsonMode bool, err error) error {
	code, ok := task.ErrorCodeOf(err)
	if !ok {
		code = task.ErrorUsage
	}
	if jsonMode {
		return writeJSON(writer, errorEnvelope{Error: errorPayload{Code: code, Message: err.Error()}})
	}

	visibleError := err
	if code == task.ErrorInternal {
		if cause := errors.Unwrap(err); cause != nil {
			visibleError = cause
		}
	}
	_, writeErr := fmt.Fprintf(writer, "Error: %v\n", visibleError)
	return writeErr
}

func writeAddedTask(writer io.Writer, created task.Task) error {
	_, err := fmt.Fprintf(writer, "Added task %d: %s\n", created.ID, humanText(created.Title, false))
	return err
}

func writeInbox(writer io.Writer, tasks []task.Task) error {
	if len(tasks) == 0 {
		return nil
	}

	rows := make([][]string, 0, len(tasks))
	for _, current := range tasks {
		rows = append(rows, []string{strconv.FormatInt(current.ID, 10), humanText(current.Title, false)})
	}

	return writeTable(writer, rows)
}

func writeTaskList(writer io.Writer, tasks []task.Task) error {
	if len(tasks) == 0 {
		return nil
	}

	rows := make([][]string, 0, len(tasks))
	for _, current := range tasks {
		rows = append(rows, []string{
			strconv.FormatInt(current.ID, 10),
			humanText(current.Title, false),
			humanText(current.Status, false),
		})
	}

	return writeTable(writer, rows)
}

func writeTaskMutation(writer io.Writer, action string, current task.Task) error {
	_, err := fmt.Fprintf(
		writer,
		"%s: %d  %s\n",
		action,
		current.ID,
		humanText(current.Title, false),
	)
	return err
}

func writeTask(writer io.Writer, current task.Task) error {
	rows := [][]string{
		{"ID", strconv.FormatInt(current.ID, 10)},
		{"Title", humanText(current.Title, false)},
		{"Note", humanText(current.Note, true)},
		{"Done at", humanText(nullableString(current.DoneAt), false)},
		{"Cancelled at", humanText(nullableString(current.CancelledAt), false)},
		{"Status", humanText(current.Status, false)},
		{"Position", strconv.FormatInt(current.Position, 10)},
		{"Created at", humanText(current.CreatedAt, false)},
		{"Updated at", humanText(current.UpdatedAt, false)},
	}

	return writeTable(writer, rows)
}

func writeTable(writer io.Writer, rows [][]string) error {
	columnCount := len(rows[0])
	renderer := table.New().
		Rows(rows...).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderHeader(false).
		BorderColumn(false).
		BorderRow(false).
		StyleFunc(func(_ int, column int) lipgloss.Style {
			style := lipgloss.NewStyle()
			if column == 0 {
				style = style.PaddingLeft(2)
			}
			if column < columnCount-1 {
				style = style.PaddingRight(2)
			}

			return style
		})

	lines := strings.Split(renderer.String(), "\n")
	for index, line := range lines {
		lines[index] = strings.TrimRight(line, " ")
	}

	_, err := fmt.Fprintln(writer, strings.Join(lines, "\n"))
	return err
}

func nullableString(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func humanText(value string, preserveLineFeeds bool) string {
	var visible strings.Builder
	visible.Grow(len(value))
	for _, character := range value {
		if character == '\n' && preserveLineFeeds {
			visible.WriteRune(character)
			continue
		}
		if unicode.IsControl(character) {
			quoted := strconv.QuoteRune(character)
			visible.WriteString(quoted[1 : len(quoted)-1])
			continue
		}
		visible.WriteRune(character)
	}

	return visible.String()
}
