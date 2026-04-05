package application

import (
	"context"

	"github.com/Matthew11K/Comments-Service/internal/domain"
)

type CreatePostInput struct {
	AuthorID domain.UserID
	Title    string
	Content  string
}

func (s *PostsService) CreatePost(ctx context.Context, input CreatePostInput) (domain.Post, error) {
	var created domain.Post

	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		post, err := domain.NewPost(
			domain.NewPostID(s.newID()),
			input.AuthorID,
			input.Title,
			input.Content,
			s.now(),
		)
		if err != nil {
			return err
		}

		if err := s.postRepo.Create(ctx, post); err != nil {
			return err
		}

		created = post
		return nil
	})
	if err != nil {
		return domain.Post{}, err
	}

	return created, nil
}

type SetCommentsEnabledInput struct {
	PostID  domain.PostID
	ActorID domain.UserID
	Enabled bool
}

func (s *PostsService) SetCommentsEnabled(ctx context.Context, input SetCommentsEnabledInput) (domain.Post, error) {
	var updated domain.Post

	err := s.txManager.WithinTx(ctx, func(ctx context.Context) error {
		post, err := s.postRepo.GetByID(ctx, input.PostID)
		if err != nil {
			return err
		}

		if err := post.SetCommentsEnabled(input.ActorID, input.Enabled); err != nil {
			return err
		}

		if err := s.postRepo.Update(ctx, post); err != nil {
			return err
		}

		updated = post
		return nil
	})
	if err != nil {
		return domain.Post{}, err
	}

	return updated, nil
}

func (s *PostsService) ListPosts(ctx context.Context, page domain.PageInput) (domain.Page[domain.Post], error) {
	return s.postRepo.List(ctx, page)
}

func (s *PostsService) CountPosts(ctx context.Context) (int, error) {
	return s.postRepo.Count(ctx)
}

func (s *PostsService) GetPost(ctx context.Context, postID domain.PostID) (domain.Post, error) {
	return s.postRepo.GetByID(ctx, postID)
}
