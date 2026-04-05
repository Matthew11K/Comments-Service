package application

import (
	"context"

	"github.com/Matthew11K/Comments-Service/internal/application/txctx"
	"github.com/Matthew11K/Comments-Service/internal/domain"
)

type CreateCommentInput struct {
	PostID   domain.PostID
	ParentID *domain.CommentID
	AuthorID domain.UserID
	Body     string
}

func (s *CommentsService) CreateComment(ctx context.Context, input CreateCommentInput) (domain.Comment, error) {
	var created domain.Comment

	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		post, err := s.postRepo.GetByID(ctx, input.PostID)
		if err != nil {
			return err
		}

		if !post.CommentsEnabled {
			return &domain.ConflictError{
				Resource: "post",
				Message:  "comments are disabled",
			}
		}

		if input.ParentID != nil {
			parentComment, err := s.commentRepo.GetByID(ctx, *input.ParentID)
			if err != nil {
				return err
			}

			if parentComment.PostID != input.PostID {
				return &domain.ConflictError{
					Resource: "comment",
					Message:  "parent comment belongs to a different post",
				}
			}
		}

		comment, err := domain.NewComment(
			domain.NewCommentID(s.newID()),
			input.PostID,
			input.ParentID,
			input.AuthorID,
			input.Body,
			s.now(),
		)
		if err != nil {
			return err
		}

		if err := s.commentRepo.Create(ctx, comment); err != nil {
			return err
		}

		if s.events != nil {
			event := CommentAddedEvent{
				PostID:    comment.PostID,
				CommentID: comment.ID,
				AuthorID:  comment.AuthorID,
			}

			if err := txctx.AfterCommit(ctx, func(ctx context.Context) {
				s.events.PublishCommentAdded(ctx, event)
			}); err != nil {
				return err
			}
		}

		created = comment
		return nil
	})
	if err != nil {
		return domain.Comment{}, err
	}

	return created, nil
}

func (s *CommentsService) GetComment(ctx context.Context, commentID domain.CommentID) (domain.Comment, error) {
	return s.commentRepo.GetByID(ctx, commentID)
}

func (s *CommentsService) ListTopLevelComments(
	ctx context.Context,
	postID domain.PostID,
	page domain.PageInput,
) (domain.Page[domain.Comment], error) {
	return s.commentRepo.ListTopLevel(ctx, postID, page)
}

func (s *CommentsService) ListReplies(
	ctx context.Context,
	parentID domain.CommentID,
	page domain.PageInput,
) (domain.Page[domain.Comment], error) {
	return s.commentRepo.ListReplies(ctx, parentID, page)
}

func (s *CommentsService) CountTopLevelComments(ctx context.Context, postID domain.PostID) (int, error) {
	return s.commentRepo.CountTopLevel(ctx, postID)
}

func (s *CommentsService) CountReplies(ctx context.Context, parentID domain.CommentID) (int, error) {
	return s.commentRepo.CountReplies(ctx, parentID)
}

func (s *CommentsService) BatchListTopLevelComments(
	ctx context.Context,
	postIDs []domain.PostID,
	page domain.PageInput,
) (map[domain.PostID]domain.Page[domain.Comment], error) {
	return s.commentRepo.BatchListTopLevel(ctx, postIDs, page)
}

func (s *CommentsService) BatchListReplies(
	ctx context.Context,
	parentIDs []domain.CommentID,
	page domain.PageInput,
) (map[domain.CommentID]domain.Page[domain.Comment], error) {
	return s.commentRepo.BatchListReplies(ctx, parentIDs, page)
}

func (s *CommentsService) BatchCountTopLevelComments(
	ctx context.Context,
	postIDs []domain.PostID,
) (map[domain.PostID]int, error) {
	return s.commentRepo.BatchCountTopLevel(ctx, postIDs)
}

func (s *CommentsService) BatchCountReplies(
	ctx context.Context,
	parentIDs []domain.CommentID,
) (map[domain.CommentID]int, error) {
	return s.commentRepo.BatchCountReplies(ctx, parentIDs)
}
