package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Matthew11K/Comments-Service/internal/application"
	"github.com/Matthew11K/Comments-Service/internal/domain"
)

const defaultBufferSize = 16

type payload struct {
	Type      string `json:"type"`
	PostID    string `json:"postId"`
	CommentID string `json:"commentId"`
	AuthorID  string `json:"authorId"`
}

type subscriber struct {
	ch chan application.CommentAddedEvent
}

type Bus struct {
	logger      *slog.Logger
	publisher   *pgxpool.Pool
	listener    *pgx.Conn
	channelName string
	bufferSize  int

	mu          sync.RWMutex
	subscribers map[domain.PostID]map[uint64]*subscriber
	nextID      uint64
	closed      bool

	cancel context.CancelFunc
	done   chan struct{}
}

func New(
	ctx context.Context,
	dsn string,
	publisher *pgxpool.Pool,
	channelName string,
	bufferSize int,
	logger *slog.Logger,
) (*Bus, error) {
	if bufferSize <= 0 {
		bufferSize = defaultBufferSize
	}

	if logger == nil {
		logger = slog.Default()
	}

	listener, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, &domain.OperationError{
			Op:  "connect postgres event listener",
			Err: err,
		}
	}

	if _, err := listener.Exec(ctx, "listen "+pgx.Identifier{channelName}.Sanitize()); err != nil {
		_ = listener.Close(ctx)
		return nil, &domain.OperationError{
			Op:  "listen for postgres comment events",
			Err: err,
		}
	}

	loopCtx, cancel := context.WithCancel(context.Background())
	bus := &Bus{
		logger:      logger,
		publisher:   publisher,
		listener:    listener,
		channelName: channelName,
		bufferSize:  bufferSize,
		subscribers: make(map[domain.PostID]map[uint64]*subscriber),
		cancel:      cancel,
		done:        make(chan struct{}),
	}

	go bus.listen(loopCtx)
	return bus, nil
}

func (b *Bus) PublishCommentAdded(ctx context.Context, event application.CommentAddedEvent) {
	body, err := json.Marshal(payload{
		Type:      "comment_added",
		PostID:    event.PostID.String(),
		CommentID: event.CommentID.String(),
		AuthorID:  event.AuthorID.String(),
	})
	if err != nil {
		b.logger.Error("marshal postgres comment event", "error", err)
		return
	}

	if _, err := b.publisher.Exec(ctx, `select pg_notify($1, $2)`, b.channelName, string(body)); err != nil {
		b.logger.Error("publish postgres comment event", "error", err, "channel", b.channelName)
	}
}

func (b *Bus) Subscribe(
	ctx context.Context,
	postID domain.PostID,
) (<-chan application.CommentAddedEvent, func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, nil, &domain.ConflictError{
			Resource: "event bus",
			Message:  "closed",
		}
	}

	b.nextID++
	subscriberID := b.nextID
	channel := make(chan application.CommentAddedEvent, b.bufferSize)

	if _, ok := b.subscribers[postID]; !ok {
		b.subscribers[postID] = make(map[uint64]*subscriber)
	}

	b.subscribers[postID][subscriberID] = &subscriber{ch: channel}

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()

			postSubscribers, ok := b.subscribers[postID]
			if !ok {
				return
			}

			subscriber, ok := postSubscribers[subscriberID]
			if !ok {
				return
			}

			delete(postSubscribers, subscriberID)
			close(subscriber.ch)
			if len(postSubscribers) == 0 {
				delete(b.subscribers, postID)
			}
		})
	}

	go func() {
		<-ctx.Done()
		unsubscribe()
	}()

	return channel, unsubscribe, nil
}

func (b *Bus) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.mu.Unlock()

	b.cancel()
	<-b.done

	closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := b.listener.Close(closeCtx); err != nil && !errors.Is(err, context.Canceled) {
		return &domain.OperationError{
			Op:  "close postgres event listener",
			Err: err,
		}
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for postID, postSubscribers := range b.subscribers {
		for subscriberID, subscriber := range postSubscribers {
			close(subscriber.ch)
			delete(postSubscribers, subscriberID)
		}

		delete(b.subscribers, postID)
	}

	return nil
}

func (b *Bus) listen(ctx context.Context) {
	defer close(b.done)

	for {
		notification, err := b.listener.WaitForNotification(ctx)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				return
			}

			b.logger.Error("wait for postgres notification", "error", err)
			continue
		}

		event, err := decodePayload(notification.Payload)
		if err != nil {
			b.logger.Error("decode postgres notification", "error", err)
			continue
		}

		b.dispatch(event)
	}
}

func (b *Bus) dispatch(event application.CommentAddedEvent) {
	b.mu.RLock()
	targets := make([]*subscriber, 0)
	for _, subscriber := range b.subscribers[event.PostID] {
		targets = append(targets, subscriber)
	}
	b.mu.RUnlock()

	for _, subscriber := range targets {
		select {
		case subscriber.ch <- event:
		default:
		}
	}
}

func decodePayload(raw string) (application.CommentAddedEvent, error) {
	var body payload
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return application.CommentAddedEvent{}, &domain.OperationError{
			Op:  "decode postgres comment event payload",
			Err: err,
		}
	}

	postID, err := domain.ParsePostID(body.PostID)
	if err != nil {
		return application.CommentAddedEvent{}, err
	}

	commentID, err := domain.ParseCommentID(body.CommentID)
	if err != nil {
		return application.CommentAddedEvent{}, err
	}

	authorID, err := domain.ParseUserID(body.AuthorID)
	if err != nil {
		return application.CommentAddedEvent{}, err
	}

	return application.CommentAddedEvent{
		PostID:    postID,
		CommentID: commentID,
		AuthorID:  authorID,
	}, nil
}
