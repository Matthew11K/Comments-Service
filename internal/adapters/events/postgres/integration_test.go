package postgres_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	eventspostgres "github.com/Matthew11K/Comments-Service/internal/adapters/events/postgres"
	storagepostgres "github.com/Matthew11K/Comments-Service/internal/adapters/storage/postgres"
	"github.com/Matthew11K/Comments-Service/internal/application"
	"github.com/Matthew11K/Comments-Service/internal/application/txctx"
	"github.com/Matthew11K/Comments-Service/internal/domain"
)

func TestBusPublishSubscribeAndUnsubscribe(t *testing.T) {
	resetEventsDB(t)

	bus := newBus(t)
	defer closeBus(t, bus)

	postID := domain.NewPostID(uuid.MustParse("00000000-0000-0000-0000-000000000100"))
	event := application.CommentAddedEvent{
		PostID:    postID,
		CommentID: domain.NewCommentID(uuid.MustParse("00000000-0000-0000-0000-000000000200")),
		AuthorID:  domain.NewUserID(uuid.MustParse("11111111-1111-1111-1111-111111111111")),
	}

	events, unsubscribe, err := bus.Subscribe(t.Context(), postID)
	require.NoError(t, err)

	bus.PublishCommentAdded(t.Context(), event)

	received := receiveEvent(t, events)
	require.Equal(t, event, received)

	unsubscribe()
	assertChannelClosed(t, events)
}

func TestBusCloseClosesSubscriptionsAndRejectsNewSubscribers(t *testing.T) {
	resetEventsDB(t)

	bus := newBus(t)
	postID := domain.NewPostID(uuid.MustParse("00000000-0000-0000-0000-000000000100"))

	events, _, err := bus.Subscribe(t.Context(), postID)
	require.NoError(t, err)

	require.NoError(t, bus.Close())
	assertChannelClosed(t, events)

	_, _, err = bus.Subscribe(t.Context(), postID)
	require.Error(t, err)

	var conflictErr *domain.ConflictError
	require.ErrorAs(t, err, &conflictErr)
}

func TestBusPublishesOnlyAfterCommit(t *testing.T) {
	resetEventsDB(t)

	bus := newBus(t)
	defer closeBus(t, bus)

	manager := storagepostgres.NewTxManager(suite.Pool)
	postID := domain.NewPostID(uuid.MustParse("00000000-0000-0000-0000-000000000100"))
	event := application.CommentAddedEvent{
		PostID:    postID,
		CommentID: domain.NewCommentID(uuid.MustParse("00000000-0000-0000-0000-000000000200")),
		AuthorID:  domain.NewUserID(uuid.MustParse("11111111-1111-1111-1111-111111111111")),
	}

	events, unsubscribe, err := bus.Subscribe(t.Context(), postID)
	require.NoError(t, err)
	defer unsubscribe()

	err = manager.WithinTx(t.Context(), func(ctx context.Context) error {
		if err := txctx.AfterCommit(ctx, func(ctx context.Context) {
			bus.PublishCommentAdded(ctx, event)
		}); err != nil {
			return err
		}

		assertNoEvent(t, events)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, event, receiveEvent(t, events))
}

func TestBusRollbackSkipsAfterCommitPublication(t *testing.T) {
	resetEventsDB(t)

	bus := newBus(t)
	defer closeBus(t, bus)

	manager := storagepostgres.NewTxManager(suite.Pool)
	postID := domain.NewPostID(uuid.MustParse("00000000-0000-0000-0000-000000000100"))
	event := application.CommentAddedEvent{
		PostID:    postID,
		CommentID: domain.NewCommentID(uuid.MustParse("00000000-0000-0000-0000-000000000200")),
		AuthorID:  domain.NewUserID(uuid.MustParse("11111111-1111-1111-1111-111111111111")),
	}

	events, unsubscribe, err := bus.Subscribe(t.Context(), postID)
	require.NoError(t, err)
	defer unsubscribe()

	err = manager.WithinTx(t.Context(), func(ctx context.Context) error {
		if err := txctx.AfterCommit(ctx, func(ctx context.Context) {
			bus.PublishCommentAdded(ctx, event)
		}); err != nil {
			return err
		}

		return &domain.ConflictError{
			Resource: "transaction",
			Message:  "rollback",
		}
	})
	require.Error(t, err)
	assertNoEvent(t, events)
}

func newBus(t *testing.T) *eventspostgres.Bus {
	t.Helper()

	bus, err := eventspostgres.New(
		t.Context(),
		suite.DSN,
		suite.Pool,
		"comment_events",
		8,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	require.NoError(t, err)

	return bus
}

func closeBus(t *testing.T, bus *eventspostgres.Bus) {
	t.Helper()
	require.NoError(t, bus.Close())
}

func receiveEvent(t *testing.T, events <-chan application.CommentAddedEvent) application.CommentAddedEvent {
	t.Helper()

	select {
	case event, ok := <-events:
		require.True(t, ok)
		return event
	case <-time.After(5 * time.Second):
		t.Fatal("expected postgres event")
		return application.CommentAddedEvent{}
	}
}

func assertNoEvent(t *testing.T, events <-chan application.CommentAddedEvent) {
	t.Helper()

	select {
	case event, ok := <-events:
		if ok {
			t.Fatalf("unexpected postgres event before commit: %+v", event)
		}
		t.Fatal("subscription channel closed unexpectedly")
	case <-time.After(300 * time.Millisecond):
	}
}

func assertChannelClosed(t *testing.T, events <-chan application.CommentAddedEvent) {
	t.Helper()

	select {
	case _, ok := <-events:
		require.False(t, ok)
	case <-time.After(5 * time.Second):
		t.Fatal("expected subscription channel to close")
	}
}

func resetEventsDB(t *testing.T) {
	t.Helper()
	require.NotNil(t, suite)
	require.NoError(t, suite.Reset(t.Context()))
}
