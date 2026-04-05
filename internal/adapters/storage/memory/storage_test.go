package memory

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Matthew11K/Comments-Service/internal/application/txctx"
	"github.com/Matthew11K/Comments-Service/internal/domain"
)

func TestPostRepositoryListUsesStableKeysetPagination(t *testing.T) {
	t.Parallel()

	store := NewStore()
	repo := NewPostRepository(store)
	baseTime := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	authorID := domain.NewUserID(uuid.New())

	posts := []domain.Post{
		mustPost(t, "00000000-0000-0000-0000-000000000001", authorID, baseTime),
		mustPost(t, "00000000-0000-0000-0000-000000000003", authorID, baseTime),
		mustPost(t, "00000000-0000-0000-0000-000000000002", authorID, baseTime),
	}
	for _, post := range posts {
		if err := repo.Create(context.Background(), post); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	firstPage, err := repo.List(context.Background(), domain.PageInput{First: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(firstPage.Items) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(firstPage.Items))
	}

	if firstPage.Items[0].ID.String() != "00000000-0000-0000-0000-000000000003" {
		t.Fatalf("unexpected first post order: %s", firstPage.Items[0].ID)
	}

	endCursor, err := domain.EncodeCursor(firstPage.Items[1].Cursor())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	secondPageInput, err := domain.NewPageInput(2, &endCursor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	secondPage, err := repo.List(context.Background(), secondPageInput)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(secondPage.Items) != 1 {
		t.Fatalf("expected 1 post, got %d", len(secondPage.Items))
	}

	if secondPage.Items[0].ID.String() != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("unexpected second page post: %s", secondPage.Items[0].ID)
	}
}

func TestCommentRepositoryBatchListTopLevelGroupsByPost(t *testing.T) {
	t.Parallel()

	store := NewStore()
	repo := NewCommentRepository(store)
	postID1 := domain.NewPostID(uuid.New())
	postID2 := domain.NewPostID(uuid.New())
	parentID := domain.NewCommentID(uuid.New())
	authorID := domain.NewUserID(uuid.New())
	baseTime := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	comments := []domain.Comment{
		mustComment(t, uuid.NewString(), postID1, nil, authorID, "one", baseTime),
		mustComment(t, uuid.NewString(), postID2, nil, authorID, "two", baseTime.Add(-time.Second)),
		mustComment(t, parentID.String(), postID1, nil, authorID, "three", baseTime.Add(-2*time.Second)),
		mustComment(t, uuid.NewString(), postID1, &parentID, authorID, "reply", baseTime.Add(-3*time.Second)),
	}
	for _, comment := range comments {
		if err := repo.Create(context.Background(), comment); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	pages, err := repo.BatchListTopLevel(context.Background(), []domain.PostID{postID1, postID2}, domain.PageInput{First: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pages[postID1].Items) != 2 {
		t.Fatalf("expected 2 top-level comments for post1, got %d", len(pages[postID1].Items))
	}

	if len(pages[postID2].Items) != 1 {
		t.Fatalf("expected 1 top-level comment for post2, got %d", len(pages[postID2].Items))
	}
}

func TestTxManagerRunsAfterCommitOnlyOnSuccess(t *testing.T) {
	t.Parallel()

	manager := NewTxManager()
	ctx := context.Background()
	calls := 0

	err := manager.WithinTx(ctx, func(ctx context.Context) error {
		return txctx.AfterCommit(ctx, func(context.Context) {
			calls++
		})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls != 1 {
		t.Fatalf("expected 1 after-commit call, got %d", calls)
	}

	err = manager.WithinTx(ctx, func(ctx context.Context) error {
		if err := txctx.AfterCommit(ctx, func(context.Context) {
			calls++
		}); err != nil {
			return err
		}

		return &domain.ConflictError{
			Resource: "transaction",
			Message:  "rollback",
		}
	})
	if err == nil {
		t.Fatal("expected rollback error")
	}

	if calls != 1 {
		t.Fatalf("expected callbacks to be skipped on rollback, got %d total calls", calls)
	}
}

func mustPost(t *testing.T, id string, authorID domain.UserID, createdAt time.Time) domain.Post {
	t.Helper()

	post, err := domain.NewPost(
		domain.NewPostID(uuid.MustParse(id)),
		authorID,
		"title",
		"content",
		createdAt,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return post
}

func mustComment(
	t *testing.T,
	id string,
	postID domain.PostID,
	parentID *domain.CommentID,
	authorID domain.UserID,
	body string,
	createdAt time.Time,
) domain.Comment {
	t.Helper()

	commentID := domain.NewCommentID(uuid.MustParse(id))
	comment, err := domain.NewComment(commentID, postID, parentID, authorID, body, createdAt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return comment
}
