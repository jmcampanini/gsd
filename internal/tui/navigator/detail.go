package navigator

import (
	"strconv"
	"strings"

	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

func taskDetail(current task.Task) detailView {
	fields := make([]detailField, 0, 14)
	fields = appendDetailField(fields, "project", nullableInt64(current.ProjectID), false)
	fields = appendDetailField(fields, "area", nullableInt64(current.AreaID), false)
	fields = appendDetailField(fields, "note", current.Note, true)
	fields = appendDetailField(fields, "due on", nullableString(current.DueOn), false)
	fields = appendDetailField(fields, "defer until", nullableString(current.DeferUntil), false)
	fields = appendDetailField(fields, "defer stage", nullableString(current.DeferStageTitle), false)
	fields = appendDetailField(fields, "promotes", strconv.FormatBool(current.Promotes), false)
	fields = appendDetailField(fields, "done at", nullableString(current.DoneAt), false)
	fields = appendDetailField(fields, "cancelled at", nullableString(current.CancelledAt), false)
	fields = appendDetailField(fields, "status", current.Status, false)
	fields = appendDetailField(fields, "position", strconv.FormatInt(current.Position, 10), false)
	fields = appendDetailField(fields, "created at", current.CreatedAt, false)
	fields = appendDetailField(fields, "updated at", current.UpdatedAt, false)
	fields = appendDetailField(fields, "tags", tagTitles(current.Tags), false)
	return detailView{
		kind:     detailTask,
		id:       current.ID,
		title:    current.Title,
		status:   current.Status,
		promotes: current.Promotes,
		fields:   fields,
	}
}

func projectDetail(shown project.Detail) detailView {
	current := shown.Project
	fields := make([]detailField, 0, 10)
	fields = appendDetailField(fields, "area", nullableInt64(current.AreaID), false)
	location := ""
	if shown.Location != nil {
		location = shown.Location.BoardTitle + "/" + shown.Location.StageTitle
	}
	fields = appendDetailField(fields, "board", location, false)
	fields = appendDetailField(fields, "note", current.Note, true)
	fields = appendDetailField(fields, "done at", nullableString(current.DoneAt), false)
	fields = appendDetailField(fields, "cancelled at", nullableString(current.CancelledAt), false)
	fields = appendDetailField(fields, "status", current.Status, false)
	fields = appendDetailField(fields, "position", strconv.FormatInt(current.Position, 10), false)
	fields = appendDetailField(fields, "created at", current.CreatedAt, false)
	fields = appendDetailField(fields, "updated at", current.UpdatedAt, false)
	fields = appendDetailField(fields, "tags", tagTitles(current.Tags), false)
	return detailView{
		kind:   detailProject,
		id:     current.ID,
		title:  current.Title,
		status: current.Status,
		fields: fields,
	}
}

func areaDetail(current area.Area) detailView {
	fields := make([]detailField, 0, 6)
	fields = appendDetailField(fields, "note", current.Note, true)
	fields = appendDetailField(fields, "archived at", nullableString(current.ArchivedAt), false)
	fields = appendDetailField(fields, "position", strconv.FormatInt(current.Position, 10), false)
	fields = appendDetailField(fields, "created at", current.CreatedAt, false)
	fields = appendDetailField(fields, "updated at", current.UpdatedAt, false)
	fields = appendDetailField(fields, "tags", tagTitles(current.Tags), false)
	return detailView{
		kind:   detailArea,
		id:     current.ID,
		title:  current.Title,
		status: archivedStatus(current.ArchivedAt),
		fields: fields,
	}
}

func boardDetail(shown board.Show) detailView {
	current := shown.Board
	stages := make([]string, len(shown.Stages))
	for index := range shown.Stages {
		stages[index] = shown.Stages[index].Title
	}
	fields := make([]detailField, 0, 5)
	fields = appendDetailField(fields, "note", current.Note, true)
	fields = appendDetailField(fields, "position", strconv.FormatInt(current.Position, 10), false)
	fields = appendDetailField(fields, "stages", strings.Join(stages, " → "), false)
	fields = appendDetailField(fields, "created at", current.CreatedAt, false)
	fields = appendDetailField(fields, "updated at", current.UpdatedAt, false)
	return detailView{kind: detailBoard, id: current.ID, title: current.Title, fields: fields}
}

func appendDetailField(fields []detailField, label, value string, preserveLineFeeds bool) []detailField {
	if value == "" {
		return fields
	}
	return append(fields, detailField{label: label, value: value, preserveLineFeeds: preserveLineFeeds})
}

func nullableString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func nullableInt64(value *int64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatInt(*value, 10)
}

func tagTitles(titles []string) string {
	visible := make([]string, len(titles))
	for index := range titles {
		visible[index] = "#" + titles[index]
	}
	return strings.Join(visible, " ")
}

func archivedStatus(archivedAt *string) string {
	if archivedAt != nil {
		return "archived"
	}
	return ""
}
