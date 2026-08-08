package task

import (
	"context"
	"fmt"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/dates"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/tag"
)

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) Add(ctx context.Context, request AddRequest) (Task, error) {
	if err := domain.ValidateTitle(request.Title); err != nil {
		return Task{}, err
	}
	if err := domain.ValidateNote(request.Note); err != nil {
		return Task{}, err
	}
	if err := domain.ValidateOptionalID("project", request.ProjectID); err != nil {
		return Task{}, err
	}
	if err := domain.ValidateOptionalID("area", request.AreaID); err != nil {
		return Task{}, err
	}
	if request.ProjectID != nil && request.AreaID != nil {
		return Task{}, apperr.New(apperr.InvalidArgument, "task cannot belong to both a project and an area", nil)
	}
	if request.DeferStage != nil {
		if err := domain.ValidateTitle(*request.DeferStage); err != nil {
			return Task{}, err
		}
	}

	var err error
	request.Tags, err = domain.NormalizeTagNames(request.Tags)
	if err != nil {
		return Task{}, err
	}

	reference := s.now()
	request.DueOn, err = canonicalizeDate(request.DueOn, reference)
	if err != nil {
		return Task{}, err
	}
	request.DeferUntil, err = canonicalizeDate(request.DeferUntil, reference)
	if err != nil {
		return Task{}, err
	}

	fields := AddFields{
		ProjectID:  request.ProjectID,
		AreaID:     request.AreaID,
		Title:      request.Title,
		Note:       request.Note,
		DeferUntil: request.DeferUntil,
		DueOn:      request.DueOn,
		Promotes:   request.Promotes,
		Tags:       request.Tags,
	}
	timestamp := domain.FormatTimestamp(reference)
	if len(fields.Tags) == 0 && request.DeferStage == nil {
		return s.store.Add(ctx, fields, timestamp)
	}

	var added Task
	err = s.store.WithinTransaction(ctx, func(store Transaction) error {
		if request.DeferStage != nil {
			stage, err := resolveDeferStage(ctx, store, request.ProjectID, *request.DeferStage)
			if err != nil {
				return err
			}
			fields.DeferStageID = &stage.ID
		}

		added, err = store.Add(ctx, fields, timestamp)
		if err != nil {
			return err
		}
		if len(fields.Tags) == 0 {
			return nil
		}

		resolved, err := store.ResolveTags(ctx, fields.Tags)
		if err != nil {
			return err
		}
		if err := store.AttachTags(ctx, added.ID, resolved); err != nil {
			return err
		}

		added, err = store.Find(ctx, added.ID)
		return err
	})
	if err != nil {
		return Task{}, err
	}

	return added, nil
}

func (s *Service) Inbox(ctx context.Context) ([]ViewTask, error) {
	return domain.NormalizeSliceResult(s.store.Inbox(ctx))
}

func (s *Service) Available(ctx context.Context) ([]ViewTask, error) {
	return domain.NormalizeSliceResult(s.store.Available(ctx))
}

func (s *Service) Show(ctx context.Context, id int64) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}

	return s.store.Find(ctx, id)
}

func (s *Service) List(ctx context.Context, options ListOptions) ([]Task, error) {
	if !validListStatus(options.Status) {
		return nil, apperr.New(apperr.InvalidArgument, fmt.Sprintf("invalid list status %q", options.Status), nil)
	}
	if !validDateSelector(options.Date) {
		return nil, apperr.New(apperr.InvalidArgument, fmt.Sprintf("invalid date selector %q", options.Date), nil)
	}
	if err := domain.ValidateOptionalID("project", options.ProjectID); err != nil {
		return nil, err
	}
	if err := domain.ValidateOptionalID("area", options.AreaID); err != nil {
		return nil, err
	}
	if options.ProjectID != nil && options.AreaID != nil {
		return nil, apperr.New(apperr.InvalidArgument, "cannot filter tasks by both project and area", nil)
	}
	if options.Tag != nil {
		if err := domain.ValidateTitle(*options.Tag); err != nil {
			return nil, err
		}
	}

	filter := ListFilter{
		Status:    options.Status,
		Date:      options.Date,
		ProjectID: options.ProjectID,
		AreaID:    options.AreaID,
	}
	if options.ProjectID == nil && options.AreaID == nil && options.Tag == nil {
		return domain.NormalizeSliceResult(s.store.List(ctx, filter))
	}

	var listed []Task
	err := s.store.WithinReadTransaction(ctx, func(transaction Transaction) error {
		if options.ProjectID != nil {
			if err := transaction.ProjectExists(ctx, *options.ProjectID); err != nil {
				return err
			}
		}
		if options.AreaID != nil {
			if err := transaction.AreaExists(ctx, *options.AreaID); err != nil {
				return err
			}
		}
		if options.Tag != nil {
			resolved, err := transaction.ResolveTags(ctx, []string{*options.Tag})
			if err != nil {
				return err
			}
			filter.TagID = &resolved[0].ID
		}

		var err error
		listed, err = transaction.List(ctx, filter)
		return err
	})
	if err != nil {
		return nil, err
	}

	return domain.NormalizeSliceResult(listed, nil)
}

func (s *Service) Edit(ctx context.Context, id int64, request EditRequest) (Edition, error) {
	if err := validateID(id); err != nil {
		return Edition{}, err
	}
	if request.DueOn.Set != nil && request.DueOn.Clear {
		return Edition{}, apperr.New(apperr.InvalidArgument, "due date cannot be set and cleared", nil)
	}
	if request.DeferUntil.Set != nil && request.DeferUntil.Clear {
		return Edition{}, apperr.New(apperr.InvalidArgument, "defer date cannot be set and cleared", nil)
	}
	if request.DeferStage.Set != nil && request.DeferStage.Clear {
		return Edition{}, apperr.New(apperr.InvalidArgument, "defer stage cannot be set and cleared", nil)
	}
	if request.Project.Set != nil && request.Project.Clear {
		return Edition{}, apperr.New(apperr.InvalidArgument, "project cannot be set and cleared", nil)
	}
	if err := domain.ValidateOptionalID("project", request.Project.Set); err != nil {
		return Edition{}, err
	}
	if request.Area.Set != nil && request.Area.Clear {
		return Edition{}, apperr.New(apperr.InvalidArgument, "area cannot be set and cleared", nil)
	}
	if err := domain.ValidateOptionalID("area", request.Area.Set); err != nil {
		return Edition{}, err
	}
	if request.Project.Set != nil && request.Area.Set != nil {
		return Edition{}, apperr.New(apperr.InvalidArgument, "task cannot be moved to both a project and an area", nil)
	}
	if request.Title == nil && request.Note == nil && request.Promotes == nil &&
		request.DueOn.Set == nil && !request.DueOn.Clear &&
		request.DeferUntil.Set == nil && !request.DeferUntil.Clear &&
		request.DeferStage.Set == nil && !request.DeferStage.Clear &&
		request.Project.Set == nil && !request.Project.Clear &&
		request.Area.Set == nil && !request.Area.Clear {
		return Edition{}, apperr.New(
			apperr.InvalidArgument,
			"task edit requires at least one field",
			nil,
		)
	}
	if request.Title != nil {
		if err := domain.ValidateTitle(*request.Title); err != nil {
			return Edition{}, err
		}
	}
	if request.Note != nil {
		if err := domain.ValidateNote(*request.Note); err != nil {
			return Edition{}, err
		}
	}
	if request.DeferStage.Set != nil {
		if err := domain.ValidateTitle(*request.DeferStage.Set); err != nil {
			return Edition{}, err
		}
	}

	reference := s.now()
	var err error
	request.DueOn.Set, err = canonicalizeDate(request.DueOn.Set, reference)
	if err != nil {
		return Edition{}, err
	}
	request.DeferUntil.Set, err = canonicalizeDate(request.DeferUntil.Set, reference)
	if err != nil {
		return Edition{}, err
	}

	fields := EditFields{
		Project:    request.Project,
		Area:       request.Area,
		Title:      request.Title,
		Note:       request.Note,
		DeferUntil: request.DeferUntil,
		DueOn:      request.DueOn,
		Promotes:   request.Promotes,
	}
	result := Edition{ClearedDefers: []Task{}}
	membershipRequested := request.Project.Set != nil || request.Project.Clear ||
		request.Area.Set != nil || request.Area.Clear
	stageRequested := request.DeferStage.Set != nil || request.DeferStage.Clear
	timestamp := domain.FormatTimestamp(reference)
	if !membershipRequested && !stageRequested {
		result.Task, err = s.store.Edit(ctx, id, fields, timestamp)
		if err != nil {
			return Edition{}, err
		}
		return result, nil
	}

	err = s.store.WithinTransaction(ctx, func(store Transaction) error {
		current, err := store.Find(ctx, id)
		if err != nil {
			return err
		}
		destinationProjectID := projectAfterEdit(current.ProjectID, current.AreaID, request)
		projectChanged := !domain.SameOptionalID(current.ProjectID, destinationProjectID)

		switch {
		case request.DeferStage.Set != nil:
			stage, err := resolveDeferStage(ctx, store, destinationProjectID, *request.DeferStage.Set)
			if err != nil {
				return err
			}
			fields.DeferStageID.Set = &stage.ID
		case request.DeferStage.Clear:
			fields.DeferStageID.Clear = true
		case projectChanged && current.DeferStageID != nil:
			fields.DeferStageID.Clear = true
		}

		result.Task, err = store.Edit(ctx, id, fields, timestamp)
		if err != nil {
			return err
		}
		if projectChanged && request.DeferStage.Set == nil && current.DeferStageID != nil {
			result.ClearedDefers = append(result.ClearedDefers, result.Task)
		}
		return nil
	})
	if err != nil {
		return Edition{}, err
	}
	return result, nil
}

func (s *Service) Reorder(ctx context.Context, id int64, placement domain.Placement) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}
	if err := domain.ValidatePlacement(placement); err != nil {
		return Task{}, err
	}

	return s.store.Reorder(ctx, id, placement, domain.FormatTimestamp(s.now()))
}

func (s *Service) Done(ctx context.Context, id int64) (Completion, error) {
	if err := validateID(id); err != nil {
		return Completion{}, err
	}

	timestamp := domain.FormatTimestamp(s.now())
	var result Completion
	err := s.store.WithinTransaction(ctx, func(store Transaction) error {
		completed, err := store.Done(ctx, id, timestamp)
		if err != nil {
			return err
		}
		result.Task = completed
		if !completed.Promotes || completed.ProjectID == nil {
			return nil
		}

		currentProject, err := store.FindProject(ctx, *completed.ProjectID)
		if err != nil {
			return err
		}
		if currentProject.StageID == nil {
			return nil
		}
		currentStage, err := store.FindStageByID(ctx, *currentProject.StageID)
		if err != nil {
			return err
		}
		nextStage, err := store.FindNextStage(ctx, currentStage.BoardID, currentStage.Position)
		if err != nil {
			return err
		}
		if nextStage == nil {
			result.Promotion = &Promotion{
				Project: currentProject, StageTitle: currentStage.Title, LastStage: true,
			}
			return nil
		}

		promoted, err := store.MoveProjectStage(ctx, currentProject.ID, nextStage.ID, timestamp)
		if err != nil {
			return err
		}
		result.PromotedProject = &promoted
		result.Promotion = &Promotion{Project: promoted, StageTitle: nextStage.Title}
		return nil
	})
	if err != nil {
		return Completion{}, err
	}
	return result, nil
}

func (s *Service) Cancel(ctx context.Context, id int64) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}

	return s.store.Cancel(ctx, id, domain.FormatTimestamp(s.now()))
}

func (s *Service) Reopen(ctx context.Context, id int64) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}

	return s.store.Reopen(ctx, id, domain.FormatTimestamp(s.now()))
}

func (s *Service) Tag(ctx context.Context, id int64, names []string) (Tagging, error) {
	return s.changeTags(ctx, id, names, true)
}

func (s *Service) Untag(ctx context.Context, id int64, names []string) (Tagging, error) {
	return s.changeTags(ctx, id, names, false)
}

func (s *Service) changeTags(
	ctx context.Context,
	id int64,
	names []string,
	attach bool,
) (Tagging, error) {
	if err := validateID(id); err != nil {
		return Tagging{}, err
	}
	if len(names) == 0 {
		return Tagging{}, apperr.New(
			apperr.InvalidArgument,
			"task tagging requires at least one tag",
			nil,
		)
	}

	normalizedNames, normalizeErr := domain.NormalizeTagNames(names)
	if normalizeErr != nil {
		return Tagging{}, normalizeErr
	}

	var result Tagging
	transactionErr := s.store.WithinTransaction(ctx, func(store Transaction) error {
		if _, err := store.Find(ctx, id); err != nil {
			return err
		}

		resolvedTags, err := store.ResolveTags(ctx, normalizedNames)
		if err != nil {
			return err
		}
		if attach {
			if err := store.AttachTags(ctx, id, resolvedTags); err != nil {
				return err
			}
		} else if err := store.DetachTags(ctx, id, resolvedTags); err != nil {
			return err
		}

		refreshed, err := store.Find(ctx, id)
		if err != nil {
			return err
		}
		result = Tagging{Task: refreshed, TagTitles: tag.Titles(resolvedTags)}
		return nil
	})
	if transactionErr != nil {
		return Tagging{}, transactionErr
	}

	return result, nil
}

func (s *Service) Delete(ctx context.Context, id int64) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}

	return s.store.Delete(ctx, id)
}

func resolveDeferStage(
	ctx context.Context,
	store Transaction,
	projectID *int64,
	title string,
) (StageReference, error) {
	if projectID == nil {
		return StageReference{}, apperr.New(
			apperr.InvalidArgument,
			"defer stage requires a task project on a board",
			nil,
		)
	}
	foundProject, err := store.FindProject(ctx, *projectID)
	if err != nil {
		return StageReference{}, err
	}
	if foundProject.StageID == nil {
		return StageReference{}, apperr.New(
			apperr.InvalidArgument,
			fmt.Sprintf("defer stage requires project %d to be on a board", foundProject.ID),
			nil,
		)
	}
	currentStage, err := store.FindStageByID(ctx, *foundProject.StageID)
	if err != nil {
		return StageReference{}, err
	}
	target, err := store.FindStage(ctx, currentStage.BoardID, title)
	if err == nil {
		return target, nil
	}
	if code, coded := apperr.CodeOf(err); !coded || code != apperr.NotFound {
		return StageReference{}, err
	}
	exists, existsErr := store.StageExists(ctx, title)
	if existsErr != nil {
		return StageReference{}, existsErr
	}
	if exists {
		return StageReference{}, apperr.New(
			apperr.InvalidArgument,
			fmt.Sprintf("stage %s is not on project %d's board", title, foundProject.ID),
			nil,
		)
	}
	return StageReference{}, err
}

func projectAfterEdit(currentProjectID, currentAreaID *int64, request EditRequest) *int64 {
	switch {
	case request.Project.Set != nil:
		return request.Project.Set
	case request.Area.Set != nil:
		return nil
	case request.Project.Clear && currentProjectID != nil:
		return nil
	case request.Area.Clear && currentAreaID != nil:
		return nil
	default:
		return currentProjectID
	}
}

func ParseListStatus(value string) (ListStatus, error) {
	status := ListStatus(value)
	if !validListStatus(status) {
		return "", apperr.New(apperr.InvalidArgument, fmt.Sprintf("invalid list status %q", value), nil)
	}

	return status, nil
}

func ParseID(value string) (int64, error) {
	return domain.ParseID("task", value)
}

func validListStatus(status ListStatus) bool {
	switch status {
	case ListStatusOpen, ListStatusDone, ListStatusCancelled, ListStatusAll:
		return true
	default:
		return false
	}
}

func validDateSelector(selector DateSelector) bool {
	switch selector {
	case DateSelectorNone, DateSelectorDue, DateSelectorOverdue, DateSelectorDeferred:
		return true
	default:
		return false
	}
}

func canonicalizeDate(value *string, reference time.Time) (*string, error) {
	if value == nil {
		return nil, nil
	}

	canonical, err := dates.Parse(*value, reference)
	if err != nil {
		return nil, apperr.New(apperr.InvalidArgument, fmt.Sprintf("invalid date %q", *value), err)
	}

	return &canonical, nil
}

func validateID(id int64) error {
	return domain.ValidateID("task", id)
}
