package application

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Matthew11K/Comments-Service/internal/domain"
)

type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type PostRepository interface {
	Create(ctx context.Context, post domain.Post) error
	GetByID(ctx context.Context, id domain.PostID) (domain.Post, error)
	List(ctx context.Context, page domain.PageInput) (domain.Page[domain.Post], error)
	Count(ctx context.Context) (int, error)
	Update(ctx context.Context, post domain.Post) error
}

type CommentRepository interface {
	Create(ctx context.Context, comment domain.Comment) error
	GetByID(ctx context.Context, id domain.CommentID) (domain.Comment, error)
	ListTopLevel(ctx context.Context, postID domain.PostID, page domain.PageInput) (domain.Page[domain.Comment], error)
	ListReplies(ctx context.Context, parentID domain.CommentID, page domain.PageInput) (domain.Page[domain.Comment], error)
	CountTopLevel(ctx context.Context, postID domain.PostID) (int, error)
	CountReplies(ctx context.Context, parentID domain.CommentID) (int, error)
	BatchListTopLevel(
		ctx context.Context,
		postIDs []domain.PostID,
		page domain.PageInput,
	) (map[domain.PostID]domain.Page[domain.Comment], error)
	BatchListReplies(
		ctx context.Context,
		parentIDs []domain.CommentID,
		page domain.PageInput,
	) (map[domain.CommentID]domain.Page[domain.Comment], error)
	BatchCountTopLevel(ctx context.Context, postIDs []domain.PostID) (map[domain.PostID]int, error)
	BatchCountReplies(ctx context.Context, parentIDs []domain.CommentID) (map[domain.CommentID]int, error)
}

type CommentAddedEvent struct {
	PostID    domain.PostID
	CommentID domain.CommentID
	AuthorID  domain.UserID
}

type CommentEventPublisher interface {
	PublishCommentAdded(ctx context.Context, event CommentAddedEvent)
}

type CommentEventSubscriber interface {
	Subscribe(ctx context.Context, postID domain.PostID) (<-chan CommentAddedEvent, func(), error)
}

type CommentEvents interface {
	CommentEventPublisher
	CommentEventSubscriber
	Close() error
}

type PostsService struct {
	postRepo  PostRepository
	txManager TxManager
	newID     func() uuid.UUID
	now       func() time.Time
}

func NewPostsService(
	postRepo PostRepository,
	txManager TxManager,
	newID func() uuid.UUID,
	now func() time.Time,
) *PostsService {
	return &PostsService{
		postRepo:  postRepo,
		txManager: txManager,
		newID:     withDefaultID(newID),
		now:       withDefaultClock(now),
	}
}

type CommentsService struct {
	postRepo    PostRepository
	commentRepo CommentRepository
	txManager   TxManager
	newID       func() uuid.UUID
	now         func() time.Time
	events      CommentEventPublisher
}

func NewCommentsService(
	postRepo PostRepository,
	commentRepo CommentRepository,
	txManager TxManager,
	newID func() uuid.UUID,
	now func() time.Time,
	events CommentEventPublisher,
) *CommentsService {
	return &CommentsService{
		postRepo:    postRepo,
		commentRepo: commentRepo,
		txManager:   txManager,
		newID:       withDefaultID(newID),
		now:         withDefaultClock(now),
		events:      events,
	}
}

func withDefaultID(newID func() uuid.UUID) func() uuid.UUID {
	if newID != nil {
		return newID
	}

	return uuid.New
}

func withDefaultClock(now func() time.Time) func() time.Time {
	if now != nil {
		return now
	}

	return func() time.Time {
		return time.Now().UTC()
	}
}
