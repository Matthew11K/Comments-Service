package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Matthew11K/Comments-Service/internal/domain"
)

func TestCursorRoundTrip(t *testing.T) {
	t.Parallel()

	cursor := domain.NewCursor(time.Date(2026, 4, 5, 10, 30, 0, 0, time.UTC), uuid.NewString())

	encoded, err := domain.EncodeCursor(cursor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded, err := domain.DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !decoded.CreatedAt.Equal(cursor.CreatedAt) {
		t.Fatalf("unexpected createdAt: got %s want %s", decoded.CreatedAt, cursor.CreatedAt)
	}

	if decoded.ID != cursor.ID {
		t.Fatalf("unexpected id: got %s want %s", decoded.ID, cursor.ID)
	}
}

func TestPageInputRejectsInvalidFirst(t *testing.T) {
	t.Parallel()

	_, err := domain.NewPageInput(0, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCursorMatchesAfterUsesDescendingKeysetOrder(t *testing.T) {
	t.Parallel()

	after := domain.NewCursor(time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC), uuid.NewString())
	olderTime := after.CreatedAt.Add(-time.Second)

	if !domain.CursorMatchesAfter(olderTime, uuid.NewString(), &after) {
		t.Fatal("expected older record to match")
	}

	if domain.CursorMatchesAfter(after.CreatedAt, after.ID, &after) {
		t.Fatal("expected exact cursor to be excluded")
	}
}
