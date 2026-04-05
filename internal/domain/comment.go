package domain

import (
	"strings"
	"time"
	"unicode/utf8"
)

const MaxCommentBodyLength = 2000

type Comment struct {
	ID        CommentID
	PostID    PostID
	ParentID  *CommentID
	AuthorID  UserID
	Body      string
	CreatedAt time.Time
}

func NewComment(
	id CommentID,
	postID PostID,
	parentID *CommentID,
	authorID UserID,
	body string,
	createdAt time.Time,
) (Comment, error) {
	if id.IsZero() {
		return Comment{}, &ValidationError{
			Field:   "id",
			Message: "must not be empty",
		}
	}

	if postID.IsZero() {
		return Comment{}, &ValidationError{
			Field:   "postId",
			Message: "must not be empty",
		}
	}

	if authorID.IsZero() {
		return Comment{}, &ValidationError{
			Field:   "authorId",
			Message: "must not be empty",
		}
	}

	if parentID != nil && parentID.IsZero() {
		return Comment{}, &ValidationError{
			Field:   "parentId",
			Message: "must not be empty",
		}
	}

	if strings.TrimSpace(body) == "" {
		return Comment{}, &ValidationError{
			Field:   "body",
			Message: "must not be empty",
		}
	}

	if utf8.RuneCountInString(body) > MaxCommentBodyLength {
		return Comment{}, &ValidationError{
			Field:   "body",
			Message: "must be at most 2000 characters",
		}
	}

	if createdAt.IsZero() {
		return Comment{}, &ValidationError{
			Field:   "createdAt",
			Message: "must not be empty",
		}
	}

	return Comment{
		ID:        id,
		PostID:    postID,
		ParentID:  parentID,
		AuthorID:  authorID,
		Body:      body,
		CreatedAt: createdAt.UTC(),
	}, nil
}

func (c Comment) IsTopLevel() bool {
	return c.ParentID == nil
}

func (c Comment) Cursor() Cursor {
	return NewCursor(c.CreatedAt, c.ID.String())
}
