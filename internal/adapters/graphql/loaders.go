package graphql

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/graph-gophers/dataloader/v7"

	"github.com/Matthew11K/Comments-Service/internal/adapters/graphql/graph/model"
	"github.com/Matthew11K/Comments-Service/internal/application"
	"github.com/Matthew11K/Comments-Service/internal/domain"
)

const (
	scopePost    = "post"
	scopeComment = "comment"
)

type commentConnectionKey struct {
	Scope string
	ID    uuid.UUID
	First int
	After string
}

type Loaders struct {
	service            *application.CommentsService
	commentConnections *dataloader.Loader[commentConnectionKey, *model.CommentConnection]
	topLevelCounts     *dataloader.Loader[uuid.UUID, int]
	replyCounts        *dataloader.Loader[uuid.UUID, int]
}

func NewLoaders(service *application.CommentsService) *Loaders {
	loaders := &Loaders{service: service}
	loaders.commentConnections = dataloader.NewBatchedLoader(
		loaders.batchCommentConnections,
		dataloader.WithWait[commentConnectionKey, *model.CommentConnection](time.Millisecond),
	)
	loaders.topLevelCounts = dataloader.NewBatchedLoader(
		loaders.batchTopLevelCounts,
		dataloader.WithWait[uuid.UUID, int](time.Millisecond),
	)
	loaders.replyCounts = dataloader.NewBatchedLoader(
		loaders.batchReplyCounts,
		dataloader.WithWait[uuid.UUID, int](time.Millisecond),
	)

	return loaders
}

func (l *Loaders) batchCommentConnections(
	ctx context.Context,
	keys []commentConnectionKey,
) []*dataloader.Result[*model.CommentConnection] {
	results := make([]*dataloader.Result[*model.CommentConnection], len(keys))
	for group, indexes := range groupCommentConnectionKeys(keys) {
		l.loadCommentConnectionGroup(ctx, results, keys, group, indexes)
	}

	return results
}

type commentConnectionGroup struct {
	Scope string
	First int
	After string
}

func groupCommentConnectionKeys(keys []commentConnectionKey) map[commentConnectionGroup][]int {
	grouped := make(map[commentConnectionGroup][]int)
	for idx, key := range keys {
		group := commentConnectionGroup{
			Scope: key.Scope,
			First: key.First,
			After: key.After,
		}
		grouped[group] = append(grouped[group], idx)
	}

	return grouped
}

func (l *Loaders) loadCommentConnectionGroup(
	ctx context.Context,
	results []*dataloader.Result[*model.CommentConnection],
	keys []commentConnectionKey,
	group commentConnectionGroup,
	indexes []int,
) {
	page, err := domain.NewPageInput(group.First, optionalString(group.After))
	if err != nil {
		fillCommentConnectionErrors(results, indexes, err)
		return
	}

	switch group.Scope {
	case scopePost:
		l.loadPostCommentConnections(ctx, results, keys, indexes, page)
	case scopeComment:
		l.loadReplyCommentConnections(ctx, results, keys, indexes, page)
	default:
		fillCommentConnectionErrors(results, indexes, unsupportedCommentConnectionScopeError())
	}
}

func (l *Loaders) loadPostCommentConnections(
	ctx context.Context,
	results []*dataloader.Result[*model.CommentConnection],
	keys []commentConnectionKey,
	indexes []int,
	page domain.PageInput,
) {
	postIDs := make([]domain.PostID, 0, len(indexes))
	for _, idx := range indexes {
		postIDs = append(postIDs, domain.NewPostID(keys[idx].ID))
	}

	pages, err := l.service.BatchListTopLevelComments(ctx, postIDs, page)
	if err != nil {
		fillCommentConnectionErrors(results, indexes, err)
		return
	}

	for _, idx := range indexes {
		postID := domain.NewPostID(keys[idx].ID)
		connection, buildErr := newCommentConnection(pages[postID])
		results[idx] = &dataloader.Result[*model.CommentConnection]{Data: connection, Error: buildErr}
	}
}

func (l *Loaders) loadReplyCommentConnections(
	ctx context.Context,
	results []*dataloader.Result[*model.CommentConnection],
	keys []commentConnectionKey,
	indexes []int,
	page domain.PageInput,
) {
	parentIDs := make([]domain.CommentID, 0, len(indexes))
	for _, idx := range indexes {
		parentIDs = append(parentIDs, domain.NewCommentID(keys[idx].ID))
	}

	pages, err := l.service.BatchListReplies(ctx, parentIDs, page)
	if err != nil {
		fillCommentConnectionErrors(results, indexes, err)
		return
	}

	for _, idx := range indexes {
		parentID := domain.NewCommentID(keys[idx].ID)
		connection, buildErr := newCommentConnection(pages[parentID])
		results[idx] = &dataloader.Result[*model.CommentConnection]{Data: connection, Error: buildErr}
	}
}

func fillCommentConnectionErrors(
	results []*dataloader.Result[*model.CommentConnection],
	indexes []int,
	err error,
) {
	for _, idx := range indexes {
		results[idx] = &dataloader.Result[*model.CommentConnection]{Error: err}
	}
}

func unsupportedCommentConnectionScopeError() error {
	return &domain.ValidationError{
		Field:   "scope",
		Message: "unsupported comment connection scope",
	}
}

func (l *Loaders) batchTopLevelCounts(ctx context.Context, keys []uuid.UUID) []*dataloader.Result[int] {
	results := make([]*dataloader.Result[int], len(keys))
	postIDs := make([]domain.PostID, 0, len(keys))
	for _, key := range keys {
		postIDs = append(postIDs, domain.NewPostID(key))
	}

	counts, err := l.service.BatchCountTopLevelComments(ctx, postIDs)
	if err != nil {
		for idx := range keys {
			results[idx] = &dataloader.Result[int]{Error: err}
		}
		return results
	}

	for idx, key := range keys {
		results[idx] = &dataloader.Result[int]{
			Data: counts[domain.NewPostID(key)],
		}
	}

	return results
}

func (l *Loaders) batchReplyCounts(ctx context.Context, keys []uuid.UUID) []*dataloader.Result[int] {
	results := make([]*dataloader.Result[int], len(keys))
	parentIDs := make([]domain.CommentID, 0, len(keys))
	for _, key := range keys {
		parentIDs = append(parentIDs, domain.NewCommentID(key))
	}

	counts, err := l.service.BatchCountReplies(ctx, parentIDs)
	if err != nil {
		for idx := range keys {
			results[idx] = &dataloader.Result[int]{Error: err}
		}
		return results
	}

	for idx, key := range keys {
		results[idx] = &dataloader.Result[int]{
			Data: counts[domain.NewCommentID(key)],
		}
	}

	return results
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}
