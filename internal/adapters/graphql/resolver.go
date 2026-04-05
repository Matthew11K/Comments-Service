package graphql

import (
	"log/slog"

	"github.com/Matthew11K/Comments-Service/internal/application"
)

type Resolver struct {
	posts    *application.PostsService
	comments *application.CommentsService
	events   application.CommentEventSubscriber
	logger   *slog.Logger
}

func NewResolver(
	posts *application.PostsService,
	comments *application.CommentsService,
	events application.CommentEventSubscriber,
	logger *slog.Logger,
) *Resolver {
	if logger == nil {
		logger = slog.Default()
	}

	return &Resolver{
		posts:    posts,
		comments: comments,
		events:   events,
		logger:   logger,
	}
}
