package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/Matthew11K/Comments-Service/internal/application"
	"github.com/Matthew11K/Comments-Service/internal/application/txctx"
	"github.com/Matthew11K/Comments-Service/internal/domain"
	applicationmocks "github.com/Matthew11K/Comments-Service/mocks/application"
)

func TestCreateCommentPublishesAfterCommit(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	commentID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	postAuthorID := domain.NewUserID(uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"))
	postID := domain.NewPostID(uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"))
	commentAuthorID := domain.NewUserID(uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"))
	post := mustNewPost(t, postID, postAuthorID, now)

	postRepo := applicationmocks.NewPostRepository(t)
	commentRepo := applicationmocks.NewCommentRepository(t)
	txManager := applicationmocks.NewTxManager(t)
	publisher := applicationmocks.NewCommentEventPublisher(t)

	expectCommittedTx(ctx, txManager)
	postRepo.EXPECT().GetByID(mock.Anything, postID).Return(post, nil).Once()
	commentRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(comment domain.Comment) bool {
			return comment.ID == domain.NewCommentID(commentID) &&
				comment.PostID == postID &&
				comment.ParentID == nil &&
				comment.AuthorID == commentAuthorID &&
				comment.Body == "hello" &&
				comment.CreatedAt.Equal(now)
		})).
		Return(nil).
		Once()
	publisher.EXPECT().
		PublishCommentAdded(ctx, application.CommentAddedEvent{
			PostID:    postID,
			CommentID: domain.NewCommentID(commentID),
			AuthorID:  commentAuthorID,
		}).
		Once()

	service := application.NewCommentsService(
		postRepo,
		commentRepo,
		txManager,
		func() uuid.UUID { return commentID },
		func() time.Time { return now },
		publisher,
	)

	created, err := service.CreateComment(ctx, application.CreateCommentInput{
		PostID:   postID,
		AuthorID: commentAuthorID,
		Body:     "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if created.ID != domain.NewCommentID(commentID) {
		t.Fatalf("unexpected comment id: got %s want %s", created.ID, commentID)
	}
}

func TestCreateCommentDoesNotPublishOnRollback(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	postAuthorID := domain.NewUserID(uuid.New())
	postID := domain.NewPostID(uuid.New())
	post := mustNewPost(t, postID, postAuthorID, now)

	postRepo := applicationmocks.NewPostRepository(t)
	commentRepo := applicationmocks.NewCommentRepository(t)
	txManager := applicationmocks.NewTxManager(t)
	publisher := applicationmocks.NewCommentEventPublisher(t)

	expectRollbackAfterWork(txManager)
	postRepo.EXPECT().GetByID(mock.Anything, postID).Return(post, nil).Once()
	commentRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(comment domain.Comment) bool {
			return comment.PostID == postID
		})).
		Return(nil).
		Once()

	service := application.NewCommentsService(
		postRepo,
		commentRepo,
		txManager,
		func() uuid.UUID { return uuid.MustParse("22222222-2222-2222-2222-222222222222") },
		func() time.Time { return now },
		publisher,
	)

	_, err := service.CreateComment(ctx, application.CreateCommentInput{
		PostID:   postID,
		AuthorID: domain.NewUserID(uuid.New()),
		Body:     "hello",
	})
	if err == nil {
		t.Fatal("expected rollback error")
	}

	if _, ok := errors.AsType[*domain.ConflictError](err); !ok {
		t.Fatalf("expected ConflictError, got %T", err)
	}
}

func TestCreateCommentRejectsDifferentParentPost(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	postID := domain.NewPostID(uuid.New())
	parentPostID := domain.NewPostID(uuid.New())
	post := mustNewPost(t, postID, domain.NewUserID(uuid.New()), now)

	parentID := domain.NewCommentID(uuid.New())
	parentComment := mustNewComment(t, parentID, parentPostID, nil, domain.NewUserID(uuid.New()), "parent", now)

	postRepo := applicationmocks.NewPostRepository(t)
	commentRepo := applicationmocks.NewCommentRepository(t)
	txManager := applicationmocks.NewTxManager(t)

	expectPassthroughTx(txManager)
	postRepo.EXPECT().GetByID(mock.Anything, postID).Return(post, nil).Once()
	commentRepo.EXPECT().GetByID(mock.Anything, parentID).Return(parentComment, nil).Once()

	service := application.NewCommentsService(
		postRepo,
		commentRepo,
		txManager,
		uuid.New,
		func() time.Time { return now },
		nil,
	)

	_, err := service.CreateComment(ctx, application.CreateCommentInput{
		PostID:   postID,
		ParentID: &parentID,
		AuthorID: domain.NewUserID(uuid.New()),
		Body:     "hello",
	})
	if err == nil {
		t.Fatal("expected error")
	}

	if _, ok := errors.AsType[*domain.ConflictError](err); !ok {
		t.Fatalf("expected ConflictError, got %T", err)
	}
}

func TestSetCommentsEnabledRequiresPostAuthor(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	now := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	authorID := domain.NewUserID(uuid.New())
	post := mustNewPost(t, domain.NewPostID(uuid.New()), authorID, now)

	postRepo := applicationmocks.NewPostRepository(t)
	txManager := applicationmocks.NewTxManager(t)

	expectPassthroughTx(txManager)
	postRepo.EXPECT().GetByID(mock.Anything, post.ID).Return(post, nil).Once()

	service := application.NewPostsService(postRepo, txManager, nil, nil)

	_, err := service.SetCommentsEnabled(ctx, application.SetCommentsEnabledInput{
		PostID:  post.ID,
		ActorID: domain.NewUserID(uuid.New()),
		Enabled: false,
	})
	if err == nil {
		t.Fatal("expected forbidden error")
	}

	if _, ok := errors.AsType[*domain.ForbiddenError](err); !ok {
		t.Fatalf("expected ForbiddenError, got %T", err)
	}
}

func expectCommittedTx(commitCtx context.Context, txManager *applicationmocks.TxManager) {
	txManager.EXPECT().
		WithinTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			txCtx, registry := txctx.WithRegistry(ctx)
			if err := fn(txCtx); err != nil {
				return err
			}

			registry.Run(commitCtx)
			return nil
		}).
		Once()
}

func expectRollbackAfterWork(txManager *applicationmocks.TxManager) {
	txManager.EXPECT().
		WithinTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			txCtx, _ := txctx.WithRegistry(ctx)
			if err := fn(txCtx); err != nil {
				return err
			}

			return &domain.ConflictError{
				Resource: "transaction",
				Message:  "rolled back",
			}
		}).
		Once()
}

func expectPassthroughTx(txManager *applicationmocks.TxManager) {
	txManager.EXPECT().
		WithinTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			txCtx, registry := txctx.WithRegistry(ctx)
			if err := fn(txCtx); err != nil {
				return err
			}

			registry.Run(ctx)
			return nil
		}).
		Once()
}

func mustNewPost(t *testing.T, id domain.PostID, authorID domain.UserID, createdAt time.Time) domain.Post {
	t.Helper()

	post, err := domain.NewPost(id, authorID, "title", "content", createdAt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return post
}

func mustNewComment(
	t *testing.T,
	id domain.CommentID,
	postID domain.PostID,
	parentID *domain.CommentID,
	authorID domain.UserID,
	body string,
	createdAt time.Time,
) domain.Comment {
	t.Helper()

	comment, err := domain.NewComment(id, postID, parentID, authorID, body, createdAt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return comment
}
