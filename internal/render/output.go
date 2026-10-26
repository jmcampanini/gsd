package render

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/logbook"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/search"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/jmcampanini/gsd/internal/text"
	"github.com/jmcampanini/gsd/internal/tui"
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

// MutationVerb pairs a mutation's human label with its glyph class so a verb
// cannot be reworded without carrying its glyph along.
type MutationVerb struct {
	label  string
	glyph  string
	accent glyphAccent
}

// The mutation verbs commands pass to the writers. Deleted, Cancelled,
// Archived, and Untagged render red; Done and Tagged render green.
var (
	VerbDeleted    = MutationVerb{label: "Deleted", glyph: glyphDeleted, accent: accentRed}
	VerbDone       = MutationVerb{label: "Done", glyph: glyphDone, accent: accentGreen}
	VerbCancelled  = MutationVerb{label: "Cancelled", glyph: glyphCancelled, accent: accentRed}
	VerbArchived   = MutationVerb{label: "Archived", glyph: glyphCancelled, accent: accentRed}
	VerbEdited     = MutationVerb{label: "Edited", glyph: glyphNeutral}
	verbMoved      = MutationVerb{label: "Moved", glyph: glyphNeutral}
	VerbReopened   = MutationVerb{label: "Reopened", glyph: glyphNeutral}
	VerbReordered  = MutationVerb{label: "Reordered", glyph: glyphNeutral}
	VerbUnarchived = MutationVerb{label: "Unarchived", glyph: glyphNeutral}
	VerbTagged     = MutationVerb{label: "Tagged", glyph: glyphAdded, accent: accentGreen}
	VerbUntagged   = MutationVerb{label: "Untagged", glyph: glyphDeleted, accent: accentRed}
)

func (o HumanOutput) verbGlyph(verb MutationVerb) string {
	switch verb.accent {
	case accentGreen:
		return o.styles.green.Render(verb.glyph)
	case accentRed:
		return o.styles.red.Render(verb.glyph)
	default:
		return verb.glyph
	}
}

// ErrorEnvelope is the JSON envelope for application errors.
type ErrorEnvelope struct {
	Error ErrorPayload `json:"error"`
}

// ErrorPayload contains the stable code and diagnostic message.
type ErrorPayload struct {
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

// HumanOutput writes human-readable entity views to a configured stream.
type HumanOutput struct {
	writer io.Writer
	styles humanStyles
	today  string
}

// newHumanOutput selects a palette and uses today for relative date styling.
func newHumanOutput(writer io.Writer, dark bool, today string) HumanOutput {
	theme := tui.ThemeForBackground(dark)
	return HumanOutput{
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

// WriteJSON writes compact, newline-terminated JSON without HTML escaping.
func WriteJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("write JSON output: %w", err)
	}
	return nil
}

// SamePayload selects a result unchanged for JSON output.
func SamePayload[T any](value T) any {
	return value
}

// TaskMutationWriter returns a writer that prints one task mutation line
// with verb.
func TaskMutationWriter(verb MutationVerb) func(HumanOutput, task.Task) error {
	return func(output HumanOutput, current task.Task) error {
		return output.writeTaskMutation(verb, current)
	}
}

// ProjectMutationWriter returns a writer that prints one project mutation
// line with verb.
func ProjectMutationWriter(verb MutationVerb) func(HumanOutput, project.Project) error {
	return func(output HumanOutput, current project.Project) error {
		return output.WriteProjectMutation(verb, current)
	}
}

// ProjectResolutionWriter returns a writer that prints the project mutation
// line followed by the open tasks the resolution cancelled.
func ProjectResolutionWriter(verb MutationVerb) func(HumanOutput, project.Resolution) error {
	return func(output HumanOutput, resolution project.Resolution) error {
		return output.writeProjectResolution(verb, resolution)
	}
}

// AreaMutationWriter returns a writer that prints one area mutation line
// with verb.
func AreaMutationWriter(verb MutationVerb) func(HumanOutput, area.Area) error {
	return func(output HumanOutput, current area.Area) error {
		return output.writeAreaMutation(verb, current)
	}
}

// BoardMutationWriter returns a writer that prints one board mutation line
// with verb.
func BoardMutationWriter(verb MutationVerb) func(HumanOutput, board.Board) error {
	return func(output HumanOutput, current board.Board) error {
		return output.writeBoardMutation(verb, current)
	}
}

// StageMutationWriter returns a writer that prints one board/stage mutation
// line with verb.
func StageMutationWriter(verb MutationVerb) func(HumanOutput, board.StageResult) error {
	return func(output HumanOutput, result board.StageResult) error {
		return output.writeStageMutation(verb, result)
	}
}

// LogbookWriter returns a writer that prints logbook entries with their
// resolution times shown in location.
func LogbookWriter(location *time.Location) func(HumanOutput, []logbook.Entry) error {
	return func(output HumanOutput, entries []logbook.Entry) error {
		return output.writeLogbook(entries, location)
	}
}

// WriteCommandError writes application errors as JSON when requested and usage errors as text.
func WriteCommandError(writer io.Writer, jsonMode bool, err error) error {
	if code, ok := apperr.CodeOf(err); ok && jsonMode {
		return WriteJSON(writer, ErrorEnvelope{Error: ErrorPayload{Code: code, Message: err.Error()}})
	}
	_, writeErr := fmt.Fprintf(writer, "Error: %s\n", text.Human(err.Error(), false))
	return writeErr
}

// WriteAddedTask prints the added line for a task with its tags.
func (o HumanOutput) WriteAddedTask(created task.Task) error {
	return o.writeAddedEntity("task", created.ID, created.Title, created.Tags)
}

// WriteAddedProject prints the added line for a project with its tags.
func (o HumanOutput) WriteAddedProject(created project.Project) error {
	return o.writeAddedEntity("project", created.ID, created.Title, created.Tags)
}

// WriteAddedArea prints the added line for an area with its tags.
func (o HumanOutput) WriteAddedArea(created area.Area) error {
	return o.writeAddedEntity("area", created.ID, created.Title, created.Tags)
}

func (o HumanOutput) writeAddedEntity(noun string, id int64, title string, tags []string) error {
	_, err := fmt.Fprintf(
		o.writer,
		"%s Added %s %s: %s%s\n",
		o.styles.green.Render(glyphAdded),
		noun,
		o.styles.faint.Render(strconv.FormatInt(id, 10)),
		text.Human(title, false),
		o.addedTagSuffix(tags),
	)
	return err
}

func (o HumanOutput) addedTagSuffix(titles []string) string {
	if len(titles) == 0 {
		return ""
	}
	return "  " + o.humanTagTitles(titles)
}

// WriteAddedBoard prints the board title and its stages joined by arrows.
func (o HumanOutput) WriteAddedBoard(addition board.Addition) error {
	stages := make([]string, len(addition.Stages))
	for index, stage := range addition.Stages {
		stages[index] = text.Human(stage.Title, false)
	}
	_, err := fmt.Fprintf(
		o.writer,
		"%s Board: %s (%s)\n",
		o.styles.green.Render(glyphAdded),
		text.Human(addition.Board.Title, false),
		strings.Join(stages, " → "),
	)
	return err
}

// WriteAddedStage prints the added line for a stage as board/stage.
func (o HumanOutput) WriteAddedStage(result board.StageResult) error {
	_, err := fmt.Fprintf(
		o.writer,
		"%s Added stage %s/%s\n",
		o.styles.green.Render(glyphAdded),
		text.Human(result.Board.Title, false),
		text.Human(result.Stage.Title, false),
	)
	return err
}

// WriteRenamedStage prints the old and new board/stage names.
func (o HumanOutput) WriteRenamedStage(result board.StageRenameResult) error {
	_, err := fmt.Fprintf(
		o.writer,
		"%s Renamed stage %s/%s to %s/%s\n",
		glyphNeutral,
		text.Human(result.Board.Title, false),
		text.Human(result.PreviousTitle, false),
		text.Human(result.Board.Title, false),
		text.Human(result.Stage.Title, false),
	)
	return err
}

func (o HumanOutput) writeStageMutation(verb MutationVerb, result board.StageResult) error {
	return o.writeStageMutationValues(verb, result.Board, result.Stage)
}

// WriteStageDeletion prints the deleted stage and then the tasks whose
// defer stage it cleared.
func (o HumanOutput) WriteStageDeletion(deletion board.StageDeletion) error {
	if err := o.writeStageMutationValues(VerbDeleted, deletion.Board, deletion.Stage); err != nil {
		return err
	}
	return o.writeClearedStageDefers(deletion.ClearedDefers)
}

func (o HumanOutput) writeStageMutationValues(verb MutationVerb, currentBoard board.Board, stage board.Stage) error {
	_, err := fmt.Fprintf(
		o.writer,
		"%s %s: stage %s/%s\n",
		o.verbGlyph(verb),
		verb.label,
		text.Human(currentBoard.Title, false),
		text.Human(stage.Title, false),
	)
	return err
}

// WriteAddedTag prints the added line for a tag.
func (o HumanOutput) WriteAddedTag(created tag.Tag) error {
	_, err := fmt.Fprintf(
		o.writer,
		"%s Added tag %s\n",
		o.styles.green.Render(glyphAdded),
		text.Human(created.Title, false),
	)
	return err
}

// WriteRenamedTag prints the old and new tag titles.
func (o HumanOutput) WriteRenamedTag(oldName, newName string) error {
	_, err := fmt.Fprintf(
		o.writer,
		"%s Renamed tag %s to %s\n",
		glyphNeutral,
		text.Human(oldName, false),
		text.Human(newName, false),
	)
	return err
}

// WriteTagDeletion prints the deleted tag and how many items it was
// detached from.
func (o HumanOutput) WriteTagDeletion(deletion tag.Deletion) error {
	plural := ""
	if deletion.Detached != 1 {
		plural = "s"
	}
	_, err := fmt.Fprintf(
		o.writer,
		"%s Deleted tag %s (detached from %s item%s)\n",
		o.styles.red.Render(glyphDeleted),
		text.Human(deletion.Tag.Title, false),
		o.styles.faint.Render(strconv.FormatInt(deletion.Detached, 10)),
		plural,
	)
	return err
}

// WriteTaskTagging prints the task ID and the tags named in the request.
func (o HumanOutput) WriteTaskTagging(verb MutationVerb, tagging task.Tagging) error {
	return o.writeEntityTagging(verb, "task", tagging.Task.ID, tagging.TagTitles)
}

// WriteProjectTagging prints the project ID and the tags named in the
// request.
func (o HumanOutput) WriteProjectTagging(verb MutationVerb, tagging project.Tagging) error {
	return o.writeEntityTagging(verb, "project", tagging.Project.ID, tagging.TagTitles)
}

// WriteAreaTagging prints the area ID and the tags named in the request.
func (o HumanOutput) WriteAreaTagging(verb MutationVerb, tagging area.Tagging) error {
	return o.writeEntityTagging(verb, "area", tagging.Area.ID, tagging.TagTitles)
}

func (o HumanOutput) writeEntityTagging(verb MutationVerb, noun string, id int64, titles []string) error {
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

func (o HumanOutput) writeAreaMutation(verb MutationVerb, current area.Area) error {
	_, err := fmt.Fprintf(
		o.writer,
		"%s %s: area %s  %s\n",
		o.verbGlyph(verb),
		verb.label,
		o.styles.faint.Render(strconv.FormatInt(current.ID, 10)),
		text.Human(current.Title, false),
	)
	return err
}

func (o HumanOutput) writeBoardMutation(verb MutationVerb, current board.Board) error {
	_, err := fmt.Fprintf(
		o.writer,
		"%s %s: board %s\n",
		o.verbGlyph(verb),
		verb.label,
		text.Human(current.Title, false),
	)
	return err
}

// WriteBoardDeletion prints the deleted line for a board.
func (o HumanOutput) WriteBoardDeletion(deletion board.Deletion) error {
	return o.writeBoardMutation(VerbDeleted, deletion.Board)
}

// WriteAreaDeletion prints the deleted area followed by the projects and
// tasks a recursive delete removed.
func (o HumanOutput) WriteAreaDeletion(deletion area.Deletion) error {
	if err := o.writeAreaMutation(VerbDeleted, deletion.Area); err != nil {
		return err
	}
	if err := o.writeNarratedProjects(deletion.DeletedProjects); err != nil {
		return err
	}
	return o.writeNarratedTasks("Deleted", "task", deletion.DeletedTasks)
}

func (o HumanOutput) writeNarratedProjects(projects []project.Project) error {
	rows := make([]narratedRow, 0, len(projects))
	for _, current := range projects {
		rows = append(rows, narratedRow{ID: current.ID, Title: current.Title})
	}
	return o.writeNarration("Deleted", "project", rows)
}

// WriteOpenTaskList prints an id, title, dates table for the inbox and
// available views, which hold only open tasks.
func (o HumanOutput) WriteOpenTaskList(tasks []task.ViewTask) error {
	rows := make([][]string, 0, len(tasks))
	for _, current := range tasks {
		rows = append(rows, []string{
			strconv.FormatInt(current.ID, 10),
			o.taskTitle(current.Task),
			o.taskDateTokens(current.Task),
		})
	}
	return o.writeCollection(
		[]string{"id", "title", "dates"},
		rows,
		0,
		0,
	)
}

// WriteTaskList prints an id, title, status, dates table.
func (o HumanOutput) WriteTaskList(tasks []task.Task) error {
	rows := make([][]string, 0, len(tasks))
	for _, current := range tasks {
		rows = append(rows, []string{
			strconv.FormatInt(current.ID, 10),
			o.taskTitle(current),
			o.statusWord(current.Status),
			o.taskDateTokens(current),
		})
	}
	return o.writeCollection(
		[]string{"id", "title", "status", "dates"},
		rows,
		0,
		0,
	)
}

// writeTaskMutation writes task mutation.
func (o HumanOutput) writeTaskMutation(verb MutationVerb, current task.Task) error {
	_, err := fmt.Fprintf(
		o.writer,
		"%s %s: %s  %s\n",
		o.verbGlyph(verb),
		verb.label,
		o.styles.faint.Render(strconv.FormatInt(current.ID, 10)),
		text.Human(current.Title, false),
	)
	return err
}

// WriteTaskEdition prints the edited line and then any cleared defer stage.
func (o HumanOutput) WriteTaskEdition(edition task.Edition) error {
	if err := o.writeTaskMutation(VerbEdited, edition.Task); err != nil {
		return err
	}
	return o.writeClearedStageDefers(edition.ClearedDefers)
}

// WriteTaskCompletion prints the done line and, when the task promoted its
// project, the stage it moved to or that it was already last.
func (o HumanOutput) WriteTaskCompletion(completion task.Completion) error {
	if err := o.writeTaskMutation(VerbDone, completion.Task); err != nil {
		return err
	}
	if completion.Promotion == nil {
		return nil
	}
	promotion := completion.Promotion
	suffix := ""
	if promotion.LastStage {
		suffix = " (already at last stage)"
	}
	_, err := fmt.Fprintf(
		o.writer,
		"%s Promoted: %s %s  %s → %s%s\n",
		glyphNeutral,
		glyphProjectOpen,
		o.styles.faint.Render(strconv.FormatInt(promotion.Project.ID, 10)),
		text.Human(promotion.Project.Title, false),
		text.Human(promotion.StageTitle, false),
		suffix,
	)
	return err
}

func (o HumanOutput) writeClearedStageDefers(tasks []task.Task) error {
	idWidth := 0
	for _, current := range tasks {
		idWidth = max(idWidth, len(strconv.FormatInt(current.ID, 10)))
	}
	for index, current := range tasks {
		branch := glyphBranch
		if index == len(tasks)-1 {
			branch = glyphLastBranch
		}
		id := padRight(strconv.FormatInt(current.ID, 10), idWidth)
		if _, err := fmt.Fprintf(
			o.writer,
			"  %s Cleared stage defer: %s  %s\n",
			o.styles.faint.Render(branch),
			o.styles.faint.Render(id),
			text.Human(current.Title, false),
		); err != nil {
			return err
		}
	}
	return nil
}

// WriteProjectMutation prints one project mutation line with verb.
func (o HumanOutput) WriteProjectMutation(verb MutationVerb, current project.Project) error {
	_, err := fmt.Fprintf(
		o.writer,
		"%s %s: project %s  %s\n",
		o.verbGlyph(verb),
		verb.label,
		o.styles.faint.Render(strconv.FormatInt(current.ID, 10)),
		text.Human(current.Title, false),
	)
	return err
}

// WriteProjectBoardEdit prints the project's new board/stage, or no board,
// and then any cleared task defers.
func (o HumanOutput) WriteProjectBoardEdit(edition project.Edition) error {
	destination := "(no board)"
	if edition.Location != nil {
		destination = text.Human(edition.Location.BoardTitle, false) + "/" +
			text.Human(edition.Location.StageTitle, false)
	}
	if err := o.writeProjectBoardMutation(VerbEdited, edition.Project, destination); err != nil {
		return err
	}
	return o.writeClearedStageDefers(edition.ClearedDefers)
}

// WriteProjectMovement prints the moved line with the destination stage.
func (o HumanOutput) WriteProjectMovement(movement project.Movement) error {
	return o.writeProjectBoardMutation(
		verbMoved,
		movement.Project,
		text.Human(movement.StageTitle, false),
	)
}

func (o HumanOutput) writeProjectBoardMutation(
	verb MutationVerb,
	current project.Project,
	destination string,
) error {
	_, err := fmt.Fprintf(
		o.writer,
		"%s %s: %s %s  %s → %s\n",
		o.verbGlyph(verb),
		verb.label,
		o.projectGlyph(current.Status),
		o.styles.faint.Render(strconv.FormatInt(current.ID, 10)),
		text.Human(current.Title, false),
		destination,
	)
	return err
}

func (o HumanOutput) writeProjectResolution(verb MutationVerb, resolution project.Resolution) error {
	if err := o.WriteProjectMutation(verb, resolution.Project); err != nil {
		return err
	}
	return o.writeNarratedTasks("Cancelled", "open task", resolution.CancelledTasks)
}

// WriteProjectDeletion prints the deleted project followed by the tasks a
// recursive delete removed.
func (o HumanOutput) WriteProjectDeletion(deletion project.Deletion) error {
	if err := o.WriteProjectMutation(VerbDeleted, deletion.Project); err != nil {
		return err
	}
	return o.writeNarratedTasks("Deleted", "task", deletion.DeletedTasks)
}

func (o HumanOutput) writeNarratedTasks(action, noun string, tasks []task.Task) error {
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

func (o HumanOutput) writeNarration(action, noun string, rows []narratedRow) error {
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
			text.Human(row.Title, false),
		); err != nil {
			return err
		}
	}
	return nil
}

// WriteProjectList prints an id, title, status table.
func (o HumanOutput) WriteProjectList(projects []project.Project) error {
	rows := make([][]string, 0, len(projects))
	for _, current := range projects {
		rows = append(rows, []string{
			strconv.FormatInt(current.ID, 10),
			text.Human(current.Title, false),
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

// WriteBoardList prints each board with its stages joined by arrows.
func (o HumanOutput) WriteBoardList(boards []board.ListedBoard) error {
	rows := make([][]string, 0, len(boards))
	for _, current := range boards {
		stages := make([]string, len(current.Stages))
		for index, stage := range current.Stages {
			stages[index] = text.Human(stage.Title, false)
		}
		rows = append(rows, []string{
			text.Human(current.Title, false),
			strings.Join(stages, " → "),
		})
	}
	return o.writeCollection([]string{"board", "stages"}, rows, -1, 1)
}

// WriteBoard prints the board's stages with the projects on each and their
// task progress.
func (o HumanOutput) WriteBoard(shown board.Show) error {
	stageTitles := make([]string, len(shown.Stages))
	stageWidth := 0
	projectIDWidth := 0
	projectTitleWidth := 0
	for index, stage := range shown.Stages {
		stageTitles[index] = text.Human(stage.Title, false)
		stageWidth = max(stageWidth, lipgloss.Width(stageTitles[index]))
		for _, current := range stage.Projects {
			projectIDWidth = max(projectIDWidth, len(strconv.FormatInt(current.ID, 10)))
			projectTitleWidth = max(projectTitleWidth, lipgloss.Width(text.Human(current.Title, false)))
		}
	}

	headline := "(no stages)"
	if len(stageTitles) > 0 {
		headline = strings.Join(stageTitles, " → ")
	}
	if _, err := fmt.Fprintf(o.writer, "%s  %s\n", text.Human(shown.Board.Title, false), headline); err != nil {
		return err
	}
	for index, stage := range shown.Stages {
		stageTitle := padRight(stageTitles[index], stageWidth)
		if len(stage.Projects) == 0 {
			if _, err := fmt.Fprintf(o.writer, "  %s  %s\n", stageTitle, o.styles.faint.Render("(empty)")); err != nil {
				return err
			}
			continue
		}
		for projectIndex, current := range stage.Projects {
			visibleStage := strings.Repeat(" ", stageWidth)
			if projectIndex == 0 {
				visibleStage = stageTitle
			}
			id := padRight(strconv.FormatInt(current.ID, 10), projectIDWidth)
			title := padRight(text.Human(current.Title, false), projectTitleWidth)
			if _, err := fmt.Fprintf(
				o.writer,
				"  %s  %s %s  %s  %d/%d\n",
				visibleStage,
				glyphProjectOpen,
				o.styles.faint.Render(id),
				title,
				current.Progress.Done,
				current.Progress.Total,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

// WriteTagList prints each tag with its usage count.
func (o HumanOutput) WriteTagList(tags []tag.ListedTag) error {
	nameWidth := 0
	visible := make([]string, len(tags))
	for index, current := range tags {
		visible[index] = text.Human(current.Title, false)
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

// WriteAreaList prints an id, title, state table; state is archived or
// blank.
func (o HumanOutput) WriteAreaList(areas []area.Area) error {
	rows := make([][]string, 0, len(areas))
	for _, current := range areas {
		state := ""
		if current.ArchivedAt != nil {
			state = o.styles.faintRed.Render("archived")
		}
		rows = append(rows, []string{
			strconv.FormatInt(current.ID, 10),
			text.Human(current.Title, false),
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

// WriteSearchHits prints a kind, id, title, status, context table in rank
// order, where context is the hit's project and area titles; a hit missing
// its entity row is an error.
func (o HumanOutput) WriteSearchHits(hits []search.Hit) error {
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
			title = o.taskTitle(*hit.Task)
			status = o.statusWord(hit.Task.Status)
		case search.KindProject:
			if hit.Project == nil {
				return fmt.Errorf("render search hit: project row is missing")
			}
			id = hit.Project.ID
			title = text.Human(hit.Project.Title, false)
			status = o.statusWord(hit.Project.Status)
		case search.KindArea:
			if hit.Area == nil {
				return fmt.Errorf("render search hit: area row is missing")
			}
			id = hit.Area.ID
			title = text.Human(hit.Area.Title, false)
			if hit.Area.ArchivedAt != nil {
				status = o.styles.faintRed.Render("archived")
			}
		default:
			return fmt.Errorf("render search hit: unknown kind %q", hit.Kind)
		}

		contextTitles := make([]string, 0, 2)
		if hit.ProjectTitle != nil {
			contextTitles = append(contextTitles, text.Human(*hit.ProjectTitle, false))
		}
		if hit.GoverningAreaTitle != nil {
			contextTitles = append(contextTitles, text.Human(*hit.GoverningAreaTitle, false))
		}
		rows = append(rows, []string{
			text.Human(hit.Kind, false),
			strconv.FormatInt(id, 10),
			title,
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

func (o HumanOutput) writeLogbook(entries []logbook.Entry, location *time.Location) error {
	rows := make([][]string, 0, len(entries))
	for _, entry := range entries {
		resolvedAt, err := time.Parse(time.RFC3339Nano, entry.ResolvedAt)
		if err != nil {
			return fmt.Errorf("parse logbook resolved_at for %s %d: %w", entry.Kind, entry.ID, err)
		}
		rows = append(rows, []string{
			text.Human(entry.Kind, false),
			strconv.FormatInt(entry.ID, 10),
			text.Human(entry.Title, false),
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

// WriteTask prints the task detail view with a glyph for its status.
func (o HumanOutput) WriteTask(current task.Task) error {
	glyph := glyphTaskOpen
	switch current.Status {
	case task.StatusDone:
		glyph = o.styles.green.Render(glyphDone)
	case task.StatusCancelled:
		glyph = o.styles.red.Render(glyphCancelled)
	}
	fields := []detailField{
		{Label: "project", Value: o.metadata(nullableInt64(current.ProjectID))},
		{Label: "area", Value: o.metadata(nullableInt64(current.AreaID))},
		{Label: "note", Value: text.Human(current.Note, true)},
		{Label: "due on", Value: o.detailDueDate(current)},
		{Label: "defer until", Value: o.metadata(text.Human(nullableString(current.DeferUntil), false))},
		{Label: "defer stage", Value: o.metadata(text.Human(nullableString(current.DeferStageTitle), false))},
		{Label: "promotes", Value: o.metadata(strconv.FormatBool(current.Promotes))},
		{Label: "done at", Value: o.metadata(text.Human(nullableString(current.DoneAt), false))},
		{Label: "cancelled at", Value: o.metadata(text.Human(nullableString(current.CancelledAt), false))},
		{Label: "status", Value: o.statusWord(current.Status)},
		{Label: "position", Value: o.metadata(strconv.FormatInt(current.Position, 10))},
		{Label: "created at", Value: o.metadata(text.Human(current.CreatedAt, false))},
		{Label: "updated at", Value: o.metadata(text.Human(current.UpdatedAt, false))},
		{Label: "tags", Value: o.humanTagTitles(current.Tags)},
	}
	return o.writeDetail(glyph, current.ID, o.taskTitle(current), fields)
}

// WriteProject prints the project detail view including its board location.
func (o HumanOutput) WriteProject(detail project.Detail) error {
	current := detail.Project
	boardLocation := ""
	if detail.Location != nil {
		boardLocation = text.Human(detail.Location.BoardTitle, false) + "/" +
			text.Human(detail.Location.StageTitle, false)
	}
	fields := []detailField{
		{Label: "area", Value: o.metadata(nullableInt64(current.AreaID))},
		{Label: "board", Value: o.metadata(boardLocation)},
		{Label: "note", Value: text.Human(current.Note, true)},
		{Label: "done at", Value: o.metadata(text.Human(nullableString(current.DoneAt), false))},
		{Label: "cancelled at", Value: o.metadata(text.Human(nullableString(current.CancelledAt), false))},
		{Label: "status", Value: o.statusWord(current.Status)},
		{Label: "position", Value: o.metadata(strconv.FormatInt(current.Position, 10))},
		{Label: "created at", Value: o.metadata(text.Human(current.CreatedAt, false))},
		{Label: "updated at", Value: o.metadata(text.Human(current.UpdatedAt, false))},
		{Label: "tags", Value: o.humanTagTitles(current.Tags)},
	}
	return o.writeDetail(o.projectGlyph(current.Status), current.ID, text.Human(current.Title, false), fields)
}

func (o HumanOutput) projectGlyph(status string) string {
	switch status {
	case project.StatusDone:
		return o.styles.green.Render(glyphDone)
	case project.StatusCancelled:
		return o.styles.red.Render(glyphCancelled)
	default:
		return glyphProjectOpen
	}
}

// WriteArea prints the area detail view with a glyph for its archive state.
func (o HumanOutput) WriteArea(current area.Area) error {
	glyph := glyphAreaActive
	if current.ArchivedAt != nil {
		glyph = o.styles.red.Render(glyphCancelled)
	}
	fields := []detailField{
		{Label: "note", Value: text.Human(current.Note, true)},
		{Label: "archived at", Value: o.metadata(text.Human(nullableString(current.ArchivedAt), false))},
		{Label: "position", Value: o.metadata(strconv.FormatInt(current.Position, 10))},
		{Label: "created at", Value: o.metadata(text.Human(current.CreatedAt, false))},
		{Label: "updated at", Value: o.metadata(text.Human(current.UpdatedAt, false))},
		{Label: "tags", Value: o.humanTagTitles(current.Tags)},
	}
	return o.writeDetail(glyph, current.ID, text.Human(current.Title, false), fields)
}

type detailField struct {
	Label string
	Value string
}

func (o HumanOutput) writeDetail(glyph string, id int64, title string, fields []detailField) error {
	if _, err := fmt.Fprintf(
		o.writer,
		"%s %s  %s\n",
		glyph,
		o.styles.faint.Render(strconv.FormatInt(id, 10)),
		title,
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

func (o HumanOutput) humanTagTitles(titles []string) string {
	visible := make([]string, len(titles))
	for index := range titles {
		visible[index] = o.styles.faint.Render(glyphTag) + text.Human(titles[index], false)
	}
	return strings.Join(visible, " ")
}

func (o HumanOutput) statusWord(status string) string {
	visible := text.Human(status, false)
	switch status {
	case task.StatusDone:
		return o.styles.faintGreen.Render(visible)
	case task.StatusCancelled:
		return o.styles.faintRed.Render(visible)
	default:
		return o.styles.faint.Render(visible)
	}
}

func (o HumanOutput) metadata(value string) string {
	if value == "" {
		return ""
	}
	return o.styles.faint.Render(value)
}

func (o HumanOutput) detailDueDate(current task.Task) string {
	value := text.Human(nullableString(current.DueOn), false)
	if value == "" {
		return ""
	}
	if task.Overdue(current, o.today) {
		return o.styles.boldRed.Render(value)
	}
	return o.styles.faint.Render(value)
}

func (o HumanOutput) writeCollection(
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

func (o HumanOutput) taskTitle(current task.Task) string {
	title := text.Human(current.Title, false)
	if current.Promotes {
		title += " " + o.styles.faint.Render("↑")
	}
	return title
}

func (o HumanOutput) taskDateTokens(current task.Task) string {
	tokens := make([]string, 0, 3)
	if current.DueOn != nil {
		value := "due " + text.Human(*current.DueOn, false)
		if task.Overdue(current, o.today) {
			tokens = append(tokens, o.styles.boldRed.Render(value))
		} else {
			tokens = append(tokens, o.styles.faint.Render(value))
		}
	}
	if current.DeferUntil != nil {
		tokens = append(tokens, o.styles.faint.Render("defer "+text.Human(*current.DeferUntil, false)))
	}
	if current.DeferStageTitle != nil {
		tokens = append(tokens, o.styles.faint.Render("defer→"+text.Human(*current.DeferStageTitle, false)))
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
