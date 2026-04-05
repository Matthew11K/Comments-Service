package memory

import (
	"context"
	"sync"

	"github.com/Matthew11K/Comments-Service/internal/application"
	"github.com/Matthew11K/Comments-Service/internal/domain"
)

const defaultBufferSize = 16

type subscriber struct {
	ch chan application.CommentAddedEvent
}

type Bus struct {
	mu          sync.RWMutex
	subscribers map[domain.PostID]map[uint64]*subscriber
	nextID      uint64
	bufferSize  int
	closed      bool
}

func NewBus(bufferSize int) *Bus {
	if bufferSize <= 0 {
		bufferSize = defaultBufferSize
	}

	return &Bus{
		subscribers: make(map[domain.PostID]map[uint64]*subscriber),
		bufferSize:  bufferSize,
	}
}

func (b *Bus) PublishCommentAdded(_ context.Context, event application.CommentAddedEvent) {
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
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}

	for postID, postSubscribers := range b.subscribers {
		for subscriberID, subscriber := range postSubscribers {
			close(subscriber.ch)
			delete(postSubscribers, subscriberID)
		}

		delete(b.subscribers, postID)
	}

	b.closed = true
	return nil
}
