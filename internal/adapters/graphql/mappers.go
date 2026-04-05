package graphql

import (
	"github.com/google/uuid"

	"github.com/Matthew11K/Comments-Service/internal/adapters/graphql/graph/model"
	"github.com/Matthew11K/Comments-Service/internal/domain"
)

func newPostModel(post domain.Post) *model.Post {
	return &model.Post{
		ID:              post.ID.UUID(),
		AuthorID:        post.AuthorID.UUID(),
		Title:           post.Title,
		Content:         post.Content,
		CommentsEnabled: post.CommentsEnabled,
		CreatedAt:       post.CreatedAt,
	}
}

func newCommentModel(comment domain.Comment) *model.Comment {
	var parentID *uuid.UUID
	if comment.ParentID != nil {
		value := comment.ParentID.UUID()
		parentID = &value
	}

	return &model.Comment{
		ID:        comment.ID.UUID(),
		PostID:    comment.PostID.UUID(),
		ParentID:  parentID,
		AuthorID:  comment.AuthorID.UUID(),
		Body:      comment.Body,
		CreatedAt: comment.CreatedAt,
	}
}

func newPostConnection(page domain.Page[domain.Post]) (*model.PostConnection, error) {
	edges := make([]*model.PostEdge, 0, len(page.Items))
	var endCursor *string

	for _, post := range page.Items {
		cursor, err := domain.EncodeCursor(post.Cursor())
		if err != nil {
			return nil, err
		}

		edges = append(edges, &model.PostEdge{
			Cursor: cursor,
			Node:   newPostModel(post),
		})
		endCursor = &cursor
	}

	return &model.PostConnection{
		Edges: edges,
		PageInfo: &model.PageInfo{
			HasNextPage: page.HasNextPage,
			EndCursor:   endCursor,
		},
	}, nil
}

func newCommentConnection(page domain.Page[domain.Comment]) (*model.CommentConnection, error) {
	edges := make([]*model.CommentEdge, 0, len(page.Items))
	var endCursor *string

	for _, comment := range page.Items {
		cursor, err := domain.EncodeCursor(comment.Cursor())
		if err != nil {
			return nil, err
		}

		edges = append(edges, &model.CommentEdge{
			Cursor: cursor,
			Node:   newCommentModel(comment),
		})
		endCursor = &cursor
	}

	return &model.CommentConnection{
		Edges: edges,
		PageInfo: &model.PageInfo{
			HasNextPage: page.HasNextPage,
			EndCursor:   endCursor,
		},
	}, nil
}
