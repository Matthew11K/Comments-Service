package memory

import (
	"sync"

	"github.com/Matthew11K/Comments-Service/internal/domain"
)

type Store struct {
	mu       sync.RWMutex
	posts    map[domain.PostID]domain.Post
	comments map[domain.CommentID]domain.Comment
}

func NewStore() *Store {
	return &Store{
		posts:    make(map[domain.PostID]domain.Post),
		comments: make(map[domain.CommentID]domain.Comment),
	}
}
