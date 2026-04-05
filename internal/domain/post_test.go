package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Matthew11K/Comments-Service/internal/domain"
)

func TestNewPostValidatesRequiredFields(t *testing.T) {
	t.Parallel()

	_, err := domain.NewPost(
		domain.PostID{},
		domain.NewUserID(uuid.New()),
		"title",
		"content",
		time.Now(),
	)
	if err == nil {
		t.Fatal("expected validation error")
	}

	if _, ok := errors.AsType[*domain.ValidationError](err); !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestPostSetCommentsEnabledAllowsAuthor(t *testing.T) {
	t.Parallel()

	authorID := domain.NewUserID(uuid.New())
	post, err := domain.NewPost(
		domain.NewPostID(uuid.New()),
		authorID,
		"title",
		"content",
		time.Now(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := post.SetCommentsEnabled(authorID, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if post.CommentsEnabled {
		t.Fatal("expected comments to be disabled")
	}
}

func TestPostSetCommentsEnabledRejectsNonAuthor(t *testing.T) {
	t.Parallel()

	post, err := domain.NewPost(
		domain.NewPostID(uuid.New()),
		domain.NewUserID(uuid.New()),
		"title",
		"content",
		time.Now(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = post.SetCommentsEnabled(domain.NewUserID(uuid.New()), false)
	if err == nil {
		t.Fatal("expected forbidden error")
	}

	if _, ok := errors.AsType[*domain.ForbiddenError](err); !ok {
		t.Fatalf("expected ForbiddenError, got %T", err)
	}
}
