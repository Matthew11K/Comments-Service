package memory

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Matthew11K/Comments-Service/internal/application"
	"github.com/Matthew11K/Comments-Service/internal/domain"
)

func TestBusSubscribePublishAndContextCleanup(t *testing.T) {
	t.Parallel()

	bus := NewBus(1)
	postID := domain.NewPostID(uuid.New())
	ctx, cancel := context.WithCancel(t.Context())

	events, _, err := bus.Subscribe(ctx, postID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := application.CommentAddedEvent{
		PostID:    postID,
		CommentID: domain.NewCommentID(uuid.New()),
		AuthorID:  domain.NewUserID(uuid.New()),
	}

	bus.PublishCommentAdded(t.Context(), expected)

	select {
	case event := <-events:
		if event.CommentID != expected.CommentID {
			t.Fatalf("unexpected comment id: got %s want %s", event.CommentID, expected.CommentID)
		}
	case <-time.After(time.Second):
		t.Fatal("expected event")
	}

	cancel()

	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("expected channel to be closed")
		}
	case <-time.After(time.Second):
		t.Fatal("expected subscription cleanup")
	}

	bus.mu.RLock()
	defer bus.mu.RUnlock()

	if len(bus.subscribers[postID]) != 0 {
		t.Fatal("expected subscribers to be cleaned up")
	}
}
