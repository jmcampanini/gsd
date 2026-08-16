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
	fields := make([]detailField, 0, 15)
	fields = appendDetailField(fields, "id", strconv.FormatInt(current.ID, 10))
	fields = appendDetailField(fields, "project", nullableInt64(current.ProjectID))
	fields = appendDetailField(fields, "area", nullableInt64(current.AreaID))
	fields = appendDetailNote(fields, current.Note)
	fields = appendDetailField(fields, "due on", nullableString(current.DueOn))
	fields = appendDetailField(fields, "defer until", nullableString(current.DeferUntil))
	fields = appendDetailField(fields, "defer stage", nullableString(current.DeferStageTitle))
	fields = appendDetailField(fields, "promotes", strconv.FormatBool(current.Promotes))
	fields = appendDetailField(fields, "done at", nullableString(current.DoneAt))
	fields = appendDetailField(fields, "cancelled at", nullableString(current.CancelledAt))
	fields = appendDetailField(fields, "status", current.Status)
	fields = appendDetailField(fields, "position", strconv.FormatInt(current.Position, 10))
	fields = appendDetailField(fields, "created at", current.CreatedAt)
	fields = appendDetailField(fields, "updated at", current.UpdatedAt)
	fields = appendDetailField(fields, "tags", tagTitles(current.Tags))
	return detailView{
		kind:     detailTask,
		title:    current.Title,
		status:   current.Status,
		promotes: current.Promotes,
		fields:   fields,
	}
}

func projectDetail(shown project.Detail) detailView {
	current := shown.Project
	fields := make([]detailField, 0, 11)
	fields = appendDetailField(fields, "id", strconv.FormatInt(current.ID, 10))
	fields = appendDetailField(fields, "area", nullableInt64(current.AreaID))
	location := ""
	if shown.Location != nil {
		location = shown.Location.BoardTitle + "/" + shown.Location.StageTitle
	}
	fields = appendDetailField(fields, "board", location)
	fields = appendDetailNote(fields, current.Note)
	fields = appendDetailField(fields, "done at", nullableString(current.DoneAt))
	fields = appendDetailField(fields, "cancelled at", nullableString(current.CancelledAt))
	fields = appendDetailField(fields, "status", current.Status)
	fields = appendDetailField(fields, "position", strconv.FormatInt(current.Position, 10))
	fields = appendDetailField(fields, "created at", current.CreatedAt)
	fields = appendDetailField(fields, "updated at", current.UpdatedAt)
	fields = appendDetailField(fields, "tags", tagTitles(current.Tags))
	return detailView{
		kind:   detailProject,
		title:  current.Title,
		status: current.Status,
		fields: fields,
	}
}

func areaDetail(current area.Area) detailView {
	fields := make([]detailField, 0, 7)
	fields = appendDetailField(fields, "id", strconv.FormatInt(current.ID, 10))
	fields = appendDetailNote(fields, current.Note)
	fields = appendDetailField(fields, "archived at", nullableString(current.ArchivedAt))
	fields = appendDetailField(fields, "position", strconv.FormatInt(current.Position, 10))
	fields = appendDetailField(fields, "created at", current.CreatedAt)
	fields = appendDetailField(fields, "updated at", current.UpdatedAt)
	fields = appendDetailField(fields, "tags", tagTitles(current.Tags))
	return detailView{
		kind:   detailArea,
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
	fields := make([]detailField, 0, 6)
	fields = appendDetailField(fields, "id", strconv.FormatInt(current.ID, 10))
	fields = appendDetailNote(fields, current.Note)
	fields = appendDetailField(fields, "position", strconv.FormatInt(current.Position, 10))
	fields = appendDetailField(fields, "stages", joinStages(stages))
	fields = appendDetailField(fields, "created at", current.CreatedAt)
	fields = appendDetailField(fields, "updated at", current.UpdatedAt)
	return detailView{kind: detailBoard, title: current.Title, fields: fields}
}

func appendDetailField(fields []detailField, label, value string) []detailField {
	if value == "" {
		return fields
	}
	return append(fields, detailField{label: label, value: value})
}

func appendDetailNote(fields []detailField, value string) []detailField {
	if value == "" {
		return fields
	}
	return append(fields, detailField{label: "note", value: value, preserveLineFeeds: true})
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

// statusArchived is navigator vocabulary: areas have no stored status, so the
// detail headline derives one from the archival stamp.
const statusArchived = "archived"

func archivedStatus(archivedAt *string) string {
	if archivedAt != nil {
		return statusArchived
	}
	return ""
}
