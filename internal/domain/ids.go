package domain

import "github.com/google/uuid"

type PostID uuid.UUID

func NewPostID(id uuid.UUID) PostID {
	return PostID(id)
}

func ParsePostID(raw string) (PostID, error) {
	parsed, err := parseUUID("postId", raw)
	if err != nil {
		return PostID{}, err
	}

	return NewPostID(parsed), nil
}

func (id PostID) UUID() uuid.UUID {
	return uuid.UUID(id)
}

func (id PostID) String() string {
	return uuid.UUID(id).String()
}

func (id PostID) IsZero() bool {
	return uuid.UUID(id) == uuid.Nil
}

type CommentID uuid.UUID

func NewCommentID(id uuid.UUID) CommentID {
	return CommentID(id)
}

func ParseCommentID(raw string) (CommentID, error) {
	parsed, err := parseUUID("commentId", raw)
	if err != nil {
		return CommentID{}, err
	}

	return NewCommentID(parsed), nil
}

func (id CommentID) UUID() uuid.UUID {
	return uuid.UUID(id)
}

func (id CommentID) String() string {
	return uuid.UUID(id).String()
}

func (id CommentID) IsZero() bool {
	return uuid.UUID(id) == uuid.Nil
}

type UserID uuid.UUID

func NewUserID(id uuid.UUID) UserID {
	return UserID(id)
}

func ParseUserID(raw string) (UserID, error) {
	parsed, err := parseUUID("authorId", raw)
	if err != nil {
		return UserID{}, err
	}

	return NewUserID(parsed), nil
}

func (id UserID) UUID() uuid.UUID {
	return uuid.UUID(id)
}

func (id UserID) String() string {
	return uuid.UUID(id).String()
}

func (id UserID) IsZero() bool {
	return uuid.UUID(id) == uuid.Nil
}

func parseUUID(field string, raw string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, &ValidationError{
			Field:   field,
			Message: "must be a valid UUID",
		}
	}

	return parsed, nil
}
