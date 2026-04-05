package memory

import (
	"context"
	"sort"

	"github.com/Matthew11K/Comments-Service/internal/domain"
)

type PostRepository struct {
	store *Store
}

func NewPostRepository(store *Store) *PostRepository {
	return &PostRepository{store: store}
}

func (r *PostRepository) Create(_ context.Context, post domain.Post) error {
	r.store.mu.Lock()
	r.store.posts[post.ID] = post
	r.store.mu.Unlock()
	return nil
}

func (r *PostRepository) GetByID(_ context.Context, id domain.PostID) (domain.Post, error) {
	r.store.mu.RLock()
	post, exists := r.store.posts[id]
	r.store.mu.RUnlock()
	if !exists {
		return domain.Post{}, &domain.NotFoundError{
			Resource: "post",
			ID:       id.String(),
		}
	}

	return post, nil
}

func (r *PostRepository) List(_ context.Context, page domain.PageInput) (domain.Page[domain.Post], error) {
	posts := r.listPosts()
	sortPosts(posts)

	items := make([]domain.Post, 0, page.LimitPlusOne())
	for _, post := range posts {
		if !domain.CursorMatchesAfter(post.CreatedAt, post.ID.String(), page.After) {
			continue
		}

		items = append(items, post)
		if len(items) == page.LimitPlusOne() {
			break
		}
	}

	hasNextPage := len(items) > page.First
	if hasNextPage {
		items = items[:page.First]
	}

	return domain.Page[domain.Post]{
		Items:       items,
		HasNextPage: hasNextPage,
	}, nil
}

func (r *PostRepository) Count(_ context.Context) (int, error) {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()

	return len(r.store.posts), nil
}

func (r *PostRepository) Update(_ context.Context, post domain.Post) error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	if _, ok := r.store.posts[post.ID]; !ok {
		return &domain.NotFoundError{
			Resource: "post",
			ID:       post.ID.String(),
		}
	}

	r.store.posts[post.ID] = post
	return nil
}

func (r *PostRepository) listPosts() []domain.Post {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()

	return snapshotPosts(r.store.posts)
}

func snapshotPosts(posts map[domain.PostID]domain.Post) []domain.Post {
	result := make([]domain.Post, 0, len(posts))
	for _, post := range posts {
		result = append(result, post)
	}

	return result
}

func sortPosts(posts []domain.Post) {
	sort.Slice(posts, func(i int, j int) bool {
		if posts[i].CreatedAt.Equal(posts[j].CreatedAt) {
			return posts[i].ID.String() > posts[j].ID.String()
		}

		return posts[i].CreatedAt.After(posts[j].CreatedAt)
	})
}
