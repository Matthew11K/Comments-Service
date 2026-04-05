package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Matthew11K/Comments-Service/internal/domain"
)

func TestNewCommentRejectsTooLongBody(t *testing.T) {
	t.Parallel()

	_, err := domain.NewComment(
		domain.NewCommentID(uuid.New()),
		domain.NewPostID(uuid.New()),
		nil,
		domain.NewUserID(uuid.New()),
		strings.Repeat("a", domain.MaxCommentBodyLength+1),
		time.Now(),
	)
	if err == nil {
		t.Fatal("expected validation error")
	}

	if _, ok := errors.AsType[*domain.ValidationError](err); !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestNewCommentAcceptsNestedComment(t *testing.T) {
	t.Parallel()

	parentID := domain.NewCommentID(uuid.New())
	comment, err := domain.NewComment(
		domain.NewCommentID(uuid.New()),
		domain.NewPostID(uuid.New()),
		&parentID,
		domain.NewUserID(uuid.New()),
		"reply",
		time.Now(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if comment.ParentID == nil || *comment.ParentID != parentID {
		t.Fatal("expected parent id to be preserved")
	}
}
