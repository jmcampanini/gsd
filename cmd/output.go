package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/logbook"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/search"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/jmcampanini/gsd/internal/tui"
	"github.com/spf13/cobra"
)

const (
	glyphTaskOpen    = "•"
	glyphProjectOpen = "◆"
	glyphAreaActive  = "●"
	glyphDone        = "✓"
	glyphCancelled   = "✗"
	glyphAdded       = "+"
	glyphDeleted     = "−"
	glyphNeutral     = "~"
	glyphTag         = "#"
	glyphBranch      = "├"
	glyphLastBranch  = "└"
)

type glyphAccent int

const (
	accentNone glyphAccent = iota
	accentGreen
	accentRed
)

// mutationVerb pairs a mutation's human label with its glyph class so a verb
// cannot be reworded without carrying its glyph along.
type mutationVerb struct {
	label  string
	glyph  string
	accent glyphAccent
}

var (
	verbDeleted    = mutationVerb{label: "Deleted", glyph: glyphDeleted, accent: accentRed}
	verbDone       = mutationVerb{label: "Done", glyph: glyphDone, accent: accentGreen}
	verbCancelled  = mutationVerb{label: "Cancelled", glyph: glyphCancelled, accent: accentRed}
	verbArchived   = mutationVerb{label: "Archived", glyph: glyphCancelled, accent: accentRed}
	verbEdited     = mutationVerb{label: "Edited", glyph: glyphNeutral}
	verbReopened   = mutationVerb{label: "Reopened", glyph: glyphNeutral}
	verbReordered  = mutationVerb{label: "Reordered", glyph: glyphNeutral}
	verbUnarchived = mutationVerb{label: "Unarchived", glyph: glyphNeutral}
	verbTagged     = mutationVerb{label: "Tagged", glyph: glyphAdded, accent: accentGreen}
	verbUntagged   = mutationVerb{label: "Untagged", glyph: glyphDeleted, accent: accentRed}
)

func (o humanOutput) verbGlyph(verb mutationVerb) string {
	switch verb.accent {
	case accentGreen:
		return o.styles.green.Render(verb.glyph)
	case accentRed:
		return o.styles.red.Render(verb.glyph)
	default:
		return verb.glyph
	}
}

type errorEnvelope struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    apperr.Code `json:"code"`
	Message string      `json:"message"`
}

type humanStyles struct {
	faint      lipgloss.Style
	green      lipgloss.Style
	red        lipgloss.Style
	faintGreen lipgloss.Style
	faintRed   lipgloss.Style
	boldRed    lipgloss.Style
}

type humanOutput struct {
	writer io.Writer
	styles humanStyles
	today  string
}

func newHumanOutput(writer io.Writer, dark bool, today string) humanOutput {
	theme := tui.ThemeForBackground(dark)
	return humanOutput{
		writer: writer,
		styles: humanStyles{
			faint:      lipgloss.NewStyle().Faint(true),
			green:      lipgloss.NewStyle().Foreground(theme.Green),
			red:        lipgloss.NewStyle().Foreground(theme.Red),
			faintGreen: lipgloss.NewStyle().Faint(true).Foreground(theme.Green),
			faintRed:   lipgloss.NewStyle().Faint(true).Foreground(theme.Red),
			boldRed:    lipgloss.NewStyle().Bold(true).Foreground(theme.Red),
		},
		today: today,
	}
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("write JSON output: %w", err)
	}
	return nil
}

// writeCommandOutput is the JSON/human dispatch point for every site whose two
// modes share one payload; only tagging, recursive delete, and tag rename
// branch by hand because their JSON payloads differ. If a cross-cutting output
// policy lands (--quiet, another machine surface such as CSV), the next
// refactor is moving dispatch into the with*Application wrappers so it exists
// in exactly one place.
func writeCommandOutput[T any](
	command *cobra.Command,
	options *rootOptions,
	value T,
	writeHuman func(humanOutput, T) error,
) error {
	if options.json {
		return writeJSON(command.OutOrStdout(), value)
	}
	return writeHuman(options.presentation.output(command), value)
}

func taskMutationWriter(verb mutationVerb) func(humanOutput, task.Task) error {
	return func(output humanOutput, current task.Task) error {
		return output.writeTaskMutation(verb, current)
	}
}

func projectMutationWriter(verb mutationVerb) func(humanOutput, project.Project) error {
	return func(output humanOutput, current project.Project) error {
		return output.writeProjectMutation(verb, current)
	}
}

func projectResolutionWriter(verb mutationVerb) func(humanOutput, project.Resolution) error {
	return func(output humanOutput, resolution project.Resolution) error {
		return output.writeProjectResolution(verb, resolution)
	}
}

func areaMutationWriter(verb mutationVerb) func(humanOutput, area.Area) error {
	return func(output humanOutput, current area.Area) error {
		return output.writeAreaMutation(verb, current)
	}
}

func logbookWriter(location *time.Location) func(humanOutput, []logbook.Entry) error {
	return func(output humanOutput, entries []logbook.Entry) error {
		return output.writeLogbook(entries, location)
	}
}

func writeCommandError(writer io.Writer, jsonMode bool, err error) error {
	if code, ok := apperr.CodeOf(err); ok && jsonMode {
		return writeJSON(writer, errorEnvelope{Error: errorPayload{Code: code, Message: err.Error()}})
	}
	_, writeErr := fmt.Fprintf(writer, "Error: %s\n", humanText(err.Error(), false))
	return writeErr
}

func (o humanOutput) writeAddedTask(created task.Task) error {
	return o.writeAddedEntity("task", created.ID, created.Title, created.Tags)
}

func (o humanOutput) writeAddedProject(created project.Project) error {
	return o.writeAddedEntity("project", created.ID, created.Title, created.Tags)
}

func (o humanOutput) writeAddedArea(created area.Area) error {
	return o.writeAddedEntity("area", created.ID, created.Title, created.Tags)
}

func (o humanOutput) writeAddedEntity(noun string, id int64, title string, tags []string) error {
	_, err := fmt.Fprintf(
		o.writer,
		"%s Added %s %s: %s%s\n",
		o.styles.green.Render(glyphAdded),
		noun,
		o.styles.faint.Render(strconv.FormatInt(id, 10)),
		humanText(title, false),
		o.addedTagSuffix(tags),
	)
	return err
}

func (o humanOutput) addedTagSuffix(titles []string) string {
	if len(titles) == 0 {
		return ""
	}
	return "  " + o.humanTagTitles(titles)
}

func (o humanOutput) writeAddedTag(created tag.Tag) error {
	_, err := fmt.Fprintf(
		o.writer,
		"%s Added tag %s\n",
		o.styles.green.Render(glyphAdded),
		humanText(created.Title, false),
	)
	return err
}

func (o humanOutput) writeRenamedTag(oldName, newName string) error {
	_, err := fmt.Fprintf(
		o.writer,
		"%s Renamed tag %s to %s\n",
		glyphNeutral,
		humanText(oldName, false),
		humanText(newName, false),
	)
	return err
}

func (o humanOutput) writeTagDeletion(deletion tag.Deletion) error {
	plural := ""
	if deletion.Detached != 1 {
		plural = "s"
	}
	_, err := fmt.Fprintf(
		o.writer,
		"%s Deleted tag %s (detached from %s item%s)\n",
		o.styles.red.Render(glyphDeleted),
		humanText(deletion.Tag.Title, false),
		o.styles.faint.Render(strconv.FormatInt(deletion.Detached, 10)),
		plural,
	)
	return err
}

func (o humanOutput) writeTaskTagging(verb mutationVerb, tagging task.Tagging) error {
	return o.writeEntityTagging(verb, "task", tagging.Task.ID, tagging.TagTitles)
}

func (o humanOutput) writeProjectTagging(verb mutationVerb, tagging project.Tagging) error {
	return o.writeEntityTagging(verb, "project", tagging.Project.ID, tagging.TagTitles)
}

func (o humanOutput) writeAreaTagging(verb mutationVerb, tagging area.Tagging) error {
	return o.writeEntityTagging(verb, "area", tagging.Area.ID, tagging.TagTitles)
}

func (o humanOutput) writeEntityTagging(verb mutationVerb, noun string, id int64, titles []string) error {
	_, err := fmt.Fprintf(
		o.writer,
		"%s%s %s: %s %s  %s\n",
		o.verbGlyph(verb),
		o.styles.faint.Render(glyphTag),
		verb.label,
		o.styles.faint.Render(noun),
		o.styles.faint.Render(strconv.FormatInt(id, 10)),
		o.humanTagTitles(titles),
	)
	return err
}

func (o humanOutput) writeAreaMutation(verb mutationVerb, current area.Area) error {
	_, err := fmt.Fprintf(
		o.writer,
		"%s %s: area %s  %s\n",
		o.verbGlyph(verb),
		verb.label,
		o.styles.faint.Render(strconv.FormatInt(current.ID, 10)),
		humanText(current.Title, false),
	)
	return err
}

func (o humanOutput) writeAreaDeletion(deletion area.Deletion) error {
	if err := o.writeAreaMutation(verbDeleted, deletion.Area); err != nil {
		return err
	}
	if err := o.writeNarratedProjects(deletion.DeletedProjects); err != nil {
		return err
	}
	return o.writeNarratedTasks("Deleted", "task", deletion.DeletedTasks)
}

func (o humanOutput) writeNarratedProjects(projects []project.Project) error {
	rows := make([]narratedRow, 0, len(projects))
	for _, current := range projects {
		rows = append(rows, narratedRow{ID: current.ID, Title: current.Title})
	}
	return o.writeNarration("Deleted", "project", rows)
}

func (o humanOutput) writeOpenTaskList(tasks []task.ViewTask) error {
	rows := make([][]string, 0, len(tasks))
	for _, current := range tasks {
		rows = append(rows, []string{
			strconv.FormatInt(current.ID, 10),
			humanText(current.Title, false),
			o.taskDateTokens(current.Task, true),
		})
	}
	return o.writeCollection(
		[]string{"id", "title", "dates"},
		rows,
		0,
		0,
	)
}

func (o humanOutput) writeTaskList(tasks []task.Task) error {
	rows := make([][]string, 0, len(tasks))
	for _, current := range tasks {
		rows = append(rows, []string{
			strconv.FormatInt(current.ID, 10),
			humanText(current.Title, false),
			o.statusWord(current.Status),
			o.taskDateTokens(current, current.Status == string(task.ListStatusOpen)),
		})
	}
	return o.writeCollection(
		[]string{"id", "title", "status", "dates"},
		rows,
		0,
		0,
	)
}

func (o humanOutput) writeTaskMutation(verb mutationVerb, current task.Task) error {
	_, err := fmt.Fprintf(
		o.writer,
		"%s %s: %s  %s\n",
		o.verbGlyph(verb),
		verb.label,
		o.styles.faint.Render(strconv.FormatInt(current.ID, 10)),
		humanText(current.Title, false),
	)
	return err
}

func (o humanOutput) writeProjectMutation(verb mutationVerb, current project.Project) error {
	_, err := fmt.Fprintf(
		o.writer,
		"%s %s: project %s  %s\n",
		o.verbGlyph(verb),
		verb.label,
		o.styles.faint.Render(strconv.FormatInt(current.ID, 10)),
		humanText(current.Title, false),
	)
	return err
}

func (o humanOutput) writeProjectResolution(verb mutationVerb, resolution project.Resolution) error {
	if err := o.writeProjectMutation(verb, resolution.Project); err != nil {
		return err
	}
	return o.writeNarratedTasks("Cancelled", "open task", resolution.CancelledTasks)
}

func (o humanOutput) writeProjectDeletion(deletion project.Deletion) error {
	if err := o.writeProjectMutation(verbDeleted, deletion.Project); err != nil {
		return err
	}
	return o.writeNarratedTasks("Deleted", "task", deletion.DeletedTasks)
}

func (o humanOutput) writeNarratedTasks(action, noun string, tasks []task.Task) error {
	rows := make([]narratedRow, 0, len(tasks))
	for _, current := range tasks {
		rows = append(rows, narratedRow{ID: current.ID, Title: current.Title})
	}
	return o.writeNarration(action, noun, rows)
}

type narratedRow struct {
	ID    int64
	Title string
}

func (o humanOutput) writeNarration(action, noun string, rows []narratedRow) error {
	if len(rows) == 0 {
		return nil
	}
	plural := ""
	if len(rows) != 1 {
		plural = "s"
	}
	if _, err := fmt.Fprintf(o.writer, "%s %d %s%s:\n", action, len(rows), noun, plural); err != nil {
		return err
	}
	idWidth := 0
	for _, row := range rows {
		idWidth = max(idWidth, len(strconv.FormatInt(row.ID, 10)))
	}
	for index, row := range rows {
		branch := glyphBranch
		if index == len(rows)-1 {
			branch = glyphLastBranch
		}
		id := padRight(strconv.FormatInt(row.ID, 10), idWidth)
		if _, err := fmt.Fprintf(
			o.writer,
			"  %s %s  %s\n",
			o.styles.faint.Render(branch),
			o.styles.faint.Render(id),
			humanText(row.Title, false),
		); err != nil {
			return err
		}
	}
	return nil
}

func (o humanOutput) writeProjectList(projects []project.Project) error {
	rows := make([][]string, 0, len(projects))
	for _, current := range projects {
		rows = append(rows, []string{
			strconv.FormatInt(current.ID, 10),
			humanText(current.Title, false),
			o.statusWord(current.Status),
		})
	}
	return o.writeCollection(
		[]string{"id", "title", "status"},
		rows,
		0,
		0,
	)
}

func (o humanOutput) writeTagList(tags []tag.ListedTag) error {
	nameWidth := 0
	visible := make([]string, len(tags))
	for index, current := range tags {
		visible[index] = humanText(current.Title, false)
		nameWidth = max(nameWidth, lipgloss.Width(visible[index]))
	}
	for index, current := range tags {
		if _, err := fmt.Fprintf(
			o.writer,
			"%s%s  %s\n",
			o.styles.faint.Render(glyphTag),
			padRight(visible[index], nameWidth),
			o.styles.faint.Render(strconv.FormatInt(current.UsageCount, 10)),
		); err != nil {
			return err
		}
	}
	return nil
}

func (o humanOutput) writeAreaList(areas []area.Area) error {
	rows := make([][]string, 0, len(areas))
	for _, current := range areas {
		state := ""
		if current.ArchivedAt != nil {
			state = o.styles.faintRed.Render("archived")
		}
		rows = append(rows, []string{
			strconv.FormatInt(current.ID, 10),
			humanText(current.Title, false),
			state,
		})
	}
	return o.writeCollection(
		[]string{"id", "title", "state"},
		rows,
		0,
		0,
	)
}

func (o humanOutput) writeSearchHits(hits []search.Hit) error {
	rows := make([][]string, 0, len(hits))
	for _, hit := range hits {
		var id int64
		var title string
		var status string
		switch hit.Kind {
		case search.KindTask:
			if hit.Task == nil {
				return fmt.Errorf("render search hit: task row is missing")
			}
			id = hit.Task.ID
			title = hit.Task.Title
			status = o.statusWord(hit.Task.Status)
		case search.KindProject:
			if hit.Project == nil {
				return fmt.Errorf("render search hit: project row is missing")
			}
			id = hit.Project.ID
			title = hit.Project.Title
			status = o.statusWord(hit.Project.Status)
		case search.KindArea:
			if hit.Area == nil {
				return fmt.Errorf("render search hit: area row is missing")
			}
			id = hit.Area.ID
			title = hit.Area.Title
			if hit.Area.ArchivedAt != nil {
				status = o.styles.faintRed.Render("archived")
			}
		default:
			return fmt.Errorf("render search hit: unknown kind %q", hit.Kind)
		}

		contextTitles := make([]string, 0, 2)
		if hit.ProjectTitle != nil {
			contextTitles = append(contextTitles, humanText(*hit.ProjectTitle, false))
		}
		if hit.GoverningAreaTitle != nil {
			contextTitles = append(contextTitles, humanText(*hit.GoverningAreaTitle, false))
		}
		rows = append(rows, []string{
			humanText(hit.Kind, false),
			strconv.FormatInt(id, 10),
			humanText(title, false),
			status,
			strings.Join(contextTitles, " · "),
		})
	}

	return o.writeCollection(
		[]string{"kind", "id", "title", "status", "context"},
		rows,
		1,
		0, 1, 4,
	)
}

func (o humanOutput) writeLogbook(entries []logbook.Entry, location *time.Location) error {
	rows := make([][]string, 0, len(entries))
	for _, entry := range entries {
		resolvedAt, err := time.Parse(time.RFC3339Nano, entry.ResolvedAt)
		if err != nil {
			return fmt.Errorf("parse logbook resolved_at for %s %d: %w", entry.Kind, entry.ID, err)
		}
		rows = append(rows, []string{
			humanText(entry.Kind, false),
			strconv.FormatInt(entry.ID, 10),
			humanText(entry.Title, false),
			o.statusWord(entry.Status),
			resolvedAt.In(location).Format(time.DateOnly),
		})
	}

	return o.writeCollection(
		[]string{"kind", "id", "title", "status", "date"},
		rows,
		1,
		0, 1, 4,
	)
}

func (o humanOutput) writeTask(current task.Task) error {
	glyph := glyphTaskOpen
	if current.Status == string(task.ListStatusDone) {
		glyph = o.styles.green.Render(glyphDone)
	} else if current.Status == string(task.ListStatusCancelled) {
		glyph = o.styles.red.Render(glyphCancelled)
	}
	fields := []detailField{
		{Label: "project", Value: o.metadata(nullableInt64(current.ProjectID))},
		{Label: "area", Value: o.metadata(nullableInt64(current.AreaID))},
		{Label: "note", Value: humanText(current.Note, true)},
		{Label: "due on", Value: o.detailDueDate(current)},
		{Label: "defer until", Value: o.metadata(humanText(nullableString(current.DeferUntil), false))},
		{Label: "done at", Value: o.metadata(humanText(nullableString(current.DoneAt), false))},
		{Label: "cancelled at", Value: o.metadata(humanText(nullableString(current.CancelledAt), false))},
		{Label: "status", Value: o.statusWord(current.Status)},
		{Label: "position", Value: o.metadata(strconv.FormatInt(current.Position, 10))},
		{Label: "created at", Value: o.metadata(humanText(current.CreatedAt, false))},
		{Label: "updated at", Value: o.metadata(humanText(current.UpdatedAt, false))},
		{Label: "tags", Value: o.humanTagTitles(current.Tags)},
	}
	return o.writeDetail(glyph, current.ID, current.Title, fields)
}

func (o humanOutput) writeProject(current project.Project) error {
	glyph := glyphProjectOpen
	if current.Status == string(project.ListStatusDone) {
		glyph = o.styles.green.Render(glyphDone)
	} else if current.Status == string(project.ListStatusCancelled) {
		glyph = o.styles.red.Render(glyphCancelled)
	}
	fields := []detailField{
		{Label: "area", Value: o.metadata(nullableInt64(current.AreaID))},
		{Label: "note", Value: humanText(current.Note, true)},
		{Label: "done at", Value: o.metadata(humanText(nullableString(current.DoneAt), false))},
		{Label: "cancelled at", Value: o.metadata(humanText(nullableString(current.CancelledAt), false))},
		{Label: "status", Value: o.statusWord(current.Status)},
		{Label: "position", Value: o.metadata(strconv.FormatInt(current.Position, 10))},
		{Label: "created at", Value: o.metadata(humanText(current.CreatedAt, false))},
		{Label: "updated at", Value: o.metadata(humanText(current.UpdatedAt, false))},
		{Label: "tags", Value: o.humanTagTitles(current.Tags)},
	}
	return o.writeDetail(glyph, current.ID, current.Title, fields)
}

func (o humanOutput) writeArea(current area.Area) error {
	glyph := glyphAreaActive
	if current.ArchivedAt != nil {
		glyph = o.styles.red.Render(glyphCancelled)
	}
	fields := []detailField{
		{Label: "note", Value: humanText(current.Note, true)},
		{Label: "archived at", Value: o.metadata(humanText(nullableString(current.ArchivedAt), false))},
		{Label: "position", Value: o.metadata(strconv.FormatInt(current.Position, 10))},
		{Label: "created at", Value: o.metadata(humanText(current.CreatedAt, false))},
		{Label: "updated at", Value: o.metadata(humanText(current.UpdatedAt, false))},
		{Label: "tags", Value: o.humanTagTitles(current.Tags)},
	}
	return o.writeDetail(glyph, current.ID, current.Title, fields)
}

type detailField struct {
	Label string
	Value string
}

func (o humanOutput) writeDetail(glyph string, id int64, title string, fields []detailField) error {
	if _, err := fmt.Fprintf(
		o.writer,
		"%s %s  %s\n",
		glyph,
		o.styles.faint.Render(strconv.FormatInt(id, 10)),
		humanText(title, false),
	); err != nil {
		return err
	}
	labelWidth := 0
	for _, field := range fields {
		labelWidth = max(labelWidth, len(field.Label))
	}
	for _, field := range fields {
		if field.Value == "" {
			if _, err := fmt.Fprintf(o.writer, "    %s\n", o.styles.faint.Render(field.Label)); err != nil {
				return err
			}
			continue
		}
		lines := strings.Split(field.Value, "\n")
		if lines[0] == "" {
			if _, err := fmt.Fprintf(o.writer, "    %s\n", o.styles.faint.Render(field.Label)); err != nil {
				return err
			}
		} else {
			label := o.styles.faint.Render(padRight(field.Label, labelWidth))
			if _, err := fmt.Fprintf(o.writer, "    %s  %s\n", label, lines[0]); err != nil {
				return err
			}
		}
		indent := strings.Repeat(" ", 4+labelWidth+2)
		for _, line := range lines[1:] {
			if line == "" {
				if _, err := fmt.Fprintln(o.writer); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintf(o.writer, "%s%s\n", indent, line); err != nil {
				return err
			}
		}
	}
	return nil
}

func (o humanOutput) humanTagTitles(titles []string) string {
	visible := make([]string, len(titles))
	for index := range titles {
		visible[index] = o.styles.faint.Render(glyphTag) + humanText(titles[index], false)
	}
	return strings.Join(visible, " ")
}

func (o humanOutput) statusWord(status string) string {
	visible := humanText(status, false)
	switch status {
	case string(task.ListStatusDone):
		return o.styles.faintGreen.Render(visible)
	case string(task.ListStatusCancelled):
		return o.styles.faintRed.Render(visible)
	default:
		return o.styles.faint.Render(visible)
	}
}

func (o humanOutput) metadata(value string) string {
	if value == "" {
		return ""
	}
	return o.styles.faint.Render(value)
}

func (o humanOutput) detailDueDate(current task.Task) string {
	value := humanText(nullableString(current.DueOn), false)
	if value == "" {
		return ""
	}
	if current.Status == string(task.ListStatusOpen) && value <= o.today {
		return o.styles.boldRed.Render(value)
	}
	return o.styles.faint.Render(value)
}

func (o humanOutput) writeCollection(
	headers []string,
	rows [][]string,
	rightAlignedColumn int,
	faintColumns ...int,
) error {
	if len(rows) == 0 {
		return nil
	}
	columnCount := len(headers)
	renderer := table.New().
		Headers(headers...).
		Rows(rows...).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderHeader(false).
		BorderColumn(false).
		BorderRow(false).
		Wrap(false).
		StyleFunc(func(row, column int) lipgloss.Style {
			var style lipgloss.Style
			if row == table.HeaderRow || slices.Contains(faintColumns, column) {
				style = o.styles.faint
			}
			if column == rightAlignedColumn {
				style = style.Align(lipgloss.Right)
			}
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
	_, err := fmt.Fprintln(o.writer, strings.Join(lines, "\n"))
	return err
}

func (o humanOutput) taskDateTokens(current task.Task, urgent bool) string {
	tokens := make([]string, 0, 2)
	if current.DueOn != nil {
		value := "due " + humanText(*current.DueOn, false)
		if urgent && *current.DueOn <= o.today {
			tokens = append(tokens, o.styles.boldRed.Render(value))
		} else {
			tokens = append(tokens, o.styles.faint.Render(value))
		}
	}
	if current.DeferUntil != nil {
		tokens = append(tokens, o.styles.faint.Render("defer "+humanText(*current.DeferUntil, false)))
	}
	return strings.Join(tokens, " ")
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

func padRight(value string, width int) string {
	if padding := width - lipgloss.Width(value); padding > 0 {
		return value + strings.Repeat(" ", padding)
	}
	return value
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
