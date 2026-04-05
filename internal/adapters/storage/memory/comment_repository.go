package memory

import (
	"context"
	"sort"

	"github.com/Matthew11K/Comments-Service/internal/domain"
)

type CommentRepository struct {
	store *Store
}

func NewCommentRepository(store *Store) *CommentRepository {
	return &CommentRepository{store: store}
}

func (r *CommentRepository) Create(_ context.Context, comment domain.Comment) error {
	r.store.mu.Lock()
	r.store.comments[comment.ID] = comment
	r.store.mu.Unlock()
	return nil
}

func (r *CommentRepository) GetByID(_ context.Context, id domain.CommentID) (domain.Comment, error) {
	r.store.mu.RLock()
	comment, exists := r.store.comments[id]
	r.store.mu.RUnlock()
	if !exists {
		return domain.Comment{}, &domain.NotFoundError{
			Resource: "comment",
			ID:       id.String(),
		}
	}

	return comment, nil
}

func (r *CommentRepository) ListTopLevel(
	_ context.Context,
	postID domain.PostID,
	page domain.PageInput,
) (domain.Page[domain.Comment], error) {
	comments := r.commentsByPredicate(func(comment domain.Comment) bool {
		return comment.PostID == postID && comment.ParentID == nil
	})

	return paginateComments(comments, page), nil
}

func (r *CommentRepository) ListReplies(
	_ context.Context,
	parentID domain.CommentID,
	page domain.PageInput,
) (domain.Page[domain.Comment], error) {
	comments := r.commentsByPredicate(func(comment domain.Comment) bool {
		return comment.ParentID != nil && *comment.ParentID == parentID
	})

	return paginateComments(comments, page), nil
}

func (r *CommentRepository) CountTopLevel(_ context.Context, postID domain.PostID) (int, error) {
	return len(r.commentsByPredicate(func(comment domain.Comment) bool {
		return comment.PostID == postID && comment.ParentID == nil
	})), nil
}

func (r *CommentRepository) CountReplies(_ context.Context, parentID domain.CommentID) (int, error) {
	return len(r.commentsByPredicate(func(comment domain.Comment) bool {
		return comment.ParentID != nil && *comment.ParentID == parentID
	})), nil
}

func (r *CommentRepository) BatchListTopLevel(
	_ context.Context,
	postIDs []domain.PostID,
	page domain.PageInput,
) (map[domain.PostID]domain.Page[domain.Comment], error) {
	grouped := make(map[domain.PostID][]domain.Comment, len(postIDs))
	idSet := make(map[domain.PostID]struct{}, len(postIDs))
	for _, id := range postIDs {
		idSet[id] = struct{}{}
		grouped[id] = nil
	}

	for _, comment := range r.commentsByPredicate(func(comment domain.Comment) bool {
		_, ok := idSet[comment.PostID]
		return ok && comment.ParentID == nil
	}) {
		grouped[comment.PostID] = append(grouped[comment.PostID], comment)
	}

	result := make(map[domain.PostID]domain.Page[domain.Comment], len(grouped))
	for postID, comments := range grouped {
		result[postID] = paginateComments(comments, page)
	}

	return result, nil
}

func (r *CommentRepository) BatchListReplies(
	_ context.Context,
	parentIDs []domain.CommentID,
	page domain.PageInput,
) (map[domain.CommentID]domain.Page[domain.Comment], error) {
	grouped := make(map[domain.CommentID][]domain.Comment, len(parentIDs))
	idSet := make(map[domain.CommentID]struct{}, len(parentIDs))
	for _, id := range parentIDs {
		idSet[id] = struct{}{}
		grouped[id] = nil
	}

	for _, comment := range r.commentsByPredicate(func(comment domain.Comment) bool {
		if comment.ParentID == nil {
			return false
		}

		_, ok := idSet[*comment.ParentID]
		return ok
	}) {
		grouped[*comment.ParentID] = append(grouped[*comment.ParentID], comment)
	}

	result := make(map[domain.CommentID]domain.Page[domain.Comment], len(grouped))
	for parentID, comments := range grouped {
		result[parentID] = paginateComments(comments, page)
	}

	return result, nil
}

func (r *CommentRepository) BatchCountTopLevel(
	_ context.Context,
	postIDs []domain.PostID,
) (map[domain.PostID]int, error) {
	counts := make(map[domain.PostID]int, len(postIDs))
	idSet := make(map[domain.PostID]struct{}, len(postIDs))
	for _, id := range postIDs {
		idSet[id] = struct{}{}
		counts[id] = 0
	}

	for _, comment := range r.commentsByPredicate(func(comment domain.Comment) bool {
		_, ok := idSet[comment.PostID]
		return ok && comment.ParentID == nil
	}) {
		counts[comment.PostID]++
	}

	return counts, nil
}

func (r *CommentRepository) BatchCountReplies(
	_ context.Context,
	parentIDs []domain.CommentID,
) (map[domain.CommentID]int, error) {
	counts := make(map[domain.CommentID]int, len(parentIDs))
	idSet := make(map[domain.CommentID]struct{}, len(parentIDs))
	for _, id := range parentIDs {
		idSet[id] = struct{}{}
		counts[id] = 0
	}

	for _, comment := range r.commentsByPredicate(func(comment domain.Comment) bool {
		if comment.ParentID == nil {
			return false
		}

		_, ok := idSet[*comment.ParentID]
		return ok
	}) {
		counts[*comment.ParentID]++
	}

	return counts, nil
}

func (r *CommentRepository) commentsByPredicate(predicate func(comment domain.Comment) bool) []domain.Comment {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()

	return filterComments(r.store.comments, predicate)
}

func filterComments(
	comments map[domain.CommentID]domain.Comment,
	predicate func(comment domain.Comment) bool,
) []domain.Comment {
	result := make([]domain.Comment, 0)
	for _, comment := range comments {
		if predicate(comment) {
			result = append(result, comment)
		}
	}

	return result
}

func paginateComments(comments []domain.Comment, page domain.PageInput) domain.Page[domain.Comment] {
	sortComments(comments)

	items := make([]domain.Comment, 0, page.LimitPlusOne())
	for _, comment := range comments {
		if !domain.CursorMatchesAfter(comment.CreatedAt, comment.ID.String(), page.After) {
			continue
		}

		items = append(items, comment)
		if len(items) == page.LimitPlusOne() {
			break
		}
	}

	hasNextPage := len(items) > page.First
	if hasNextPage {
		items = items[:page.First]
	}

	return domain.Page[domain.Comment]{
		Items:       items,
		HasNextPage: hasNextPage,
	}
}

func sortComments(comments []domain.Comment) {
	sort.Slice(comments, func(i int, j int) bool {
		if comments[i].CreatedAt.Equal(comments[j].CreatedAt) {
			return comments[i].ID.String() > comments[j].ID.String()
		}

		return comments[i].CreatedAt.After(comments[j].CreatedAt)
	})
}
