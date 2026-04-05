package domain

import (
	"strings"
	"time"
)

type Post struct {
	ID              PostID
	AuthorID        UserID
	Title           string
	Content         string
	CommentsEnabled bool
	CreatedAt       time.Time
}

func NewPost(id PostID, authorID UserID, title string, content string, createdAt time.Time) (Post, error) {
	if id.IsZero() {
		return Post{}, &ValidationError{
			Field:   "id",
			Message: "must not be empty",
		}
	}

	if authorID.IsZero() {
		return Post{}, &ValidationError{
			Field:   "authorId",
			Message: "must not be empty",
		}
	}

	if strings.TrimSpace(title) == "" {
		return Post{}, &ValidationError{
			Field:   "title",
			Message: "must not be empty",
		}
	}

	if strings.TrimSpace(content) == "" {
		return Post{}, &ValidationError{
			Field:   "content",
			Message: "must not be empty",
		}
	}

	if createdAt.IsZero() {
		return Post{}, &ValidationError{
			Field:   "createdAt",
			Message: "must not be empty",
		}
	}

	return Post{
		ID:              id,
		AuthorID:        authorID,
		Title:           title,
		Content:         content,
		CommentsEnabled: true,
		CreatedAt:       createdAt.UTC(),
	}, nil
}

func (p *Post) SetCommentsEnabled(actorID UserID, enabled bool) error {
	if actorID != p.AuthorID {
		return &ForbiddenError{
			Action:   "set comments availability",
			Resource: "post",
		}
	}

	p.CommentsEnabled = enabled
	return nil
}

func (p Post) Cursor() Cursor {
	return NewCursor(p.CreatedAt, p.ID.String())
}
