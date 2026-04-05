package domain

import (
	"encoding/base64"
	"encoding/json"
	"time"
)

type Cursor struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        string    `json:"id"`
}

func NewCursor(createdAt time.Time, id string) Cursor {
	return Cursor{
		CreatedAt: createdAt.UTC(),
		ID:        id,
	}
}

func EncodeCursor(cursor Cursor) (string, error) {
	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", &OperationError{
			Op:  "marshal cursor",
			Err: err,
		}
	}

	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func DecodeCursor(raw string) (Cursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return Cursor{}, &ValidationError{
			Field:   "after",
			Message: "invalid cursor",
		}
	}

	var cursor Cursor
	if err := json.Unmarshal(decoded, &cursor); err != nil {
		return Cursor{}, &ValidationError{
			Field:   "after",
			Message: "invalid cursor",
		}
	}

	if cursor.CreatedAt.IsZero() || cursor.ID == "" {
		return Cursor{}, &ValidationError{
			Field:   "after",
			Message: "invalid cursor",
		}
	}

	return cursor, nil
}

type PageInput struct {
	First int
	After *Cursor
}

func NewPageInput(first int, after *string) (PageInput, error) {
	if first <= 0 {
		return PageInput{}, &ValidationError{
			Field:   "first",
			Message: "must be greater than zero",
		}
	}

	if after == nil || *after == "" {
		return PageInput{
			First: first,
		}, nil
	}

	cursor, err := DecodeCursor(*after)
	if err != nil {
		return PageInput{}, err
	}

	return PageInput{
		First: first,
		After: &cursor,
	}, nil
}

func (p PageInput) LimitPlusOne() int {
	return p.First + 1
}

func CursorMatchesAfter(createdAt time.Time, id string, after *Cursor) bool {
	if after == nil {
		return true
	}

	if createdAt.Before(after.CreatedAt) {
		return true
	}

	return createdAt.Equal(after.CreatedAt) && id < after.ID
}

type Page[T any] struct {
	Items       []T
	HasNextPage bool
}
