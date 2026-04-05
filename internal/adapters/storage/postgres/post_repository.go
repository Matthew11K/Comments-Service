package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Matthew11K/Comments-Service/internal/domain"
)

type PostRepository struct {
	pool *pgxpool.Pool
}

func NewPostRepository(pool *pgxpool.Pool) *PostRepository {
	return &PostRepository{pool: pool}
}

func (r *PostRepository) Create(ctx context.Context, post domain.Post) error {
	_, err := dbFromContext(ctx, r.pool).Exec(
		ctx,
		`insert into posts (id, author_id, title, content, comments_enabled, created_at)
		 values ($1, $2, $3, $4, $5, $6)`,
		post.ID.UUID(),
		post.AuthorID.UUID(),
		post.Title,
		post.Content,
		post.CommentsEnabled,
		post.CreatedAt,
	)
	if err != nil {
		return &domain.OperationError{
			Op:  "insert post",
			Err: err,
		}
	}

	return nil
}

func (r *PostRepository) GetByID(ctx context.Context, id domain.PostID) (domain.Post, error) {
	row := dbFromContext(ctx, r.pool).QueryRow(
		ctx,
		`select id, author_id, title, content, comments_enabled, created_at
		 from posts
		 where id = $1`,
		id.UUID(),
	)

	post, found, err := scanPost(row)
	if err != nil {
		return domain.Post{}, err
	}

	if !found {
		return domain.Post{}, &domain.NotFoundError{
			Resource: "post",
			ID:       id.String(),
		}
	}

	return post, nil
}

func (r *PostRepository) List(ctx context.Context, page domain.PageInput) (domain.Page[domain.Post], error) {
	query := `
select id, author_id, title, content, comments_enabled, created_at
from posts
%s
order by created_at desc, id desc
limit $%d`

	args := make([]any, 0, 3)
	where := ""
	limitIndex := 1
	if page.After != nil {
		where = "where (created_at, id) < ($1, $2)"
		args = append(args, page.After.CreatedAt, page.After.ID)
		limitIndex = 3
	}
	args = append(args, page.LimitPlusOne())

	rows, err := dbFromContext(ctx, r.pool).Query(ctx, formatSQL(query, where, limitIndex), args...)
	if err != nil {
		return domain.Page[domain.Post]{}, &domain.OperationError{
			Op:  "list posts",
			Err: err,
		}
	}
	defer rows.Close()

	items := make([]domain.Post, 0, page.LimitPlusOne())
	for rows.Next() {
		post, _, scanErr := scanPost(rows)
		if scanErr != nil {
			return domain.Page[domain.Post]{}, scanErr
		}

		items = append(items, post)
	}

	if err := rows.Err(); err != nil {
		return domain.Page[domain.Post]{}, &domain.OperationError{
			Op:  "iterate posts",
			Err: err,
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

func (r *PostRepository) Count(ctx context.Context) (int, error) {
	row := dbFromContext(ctx, r.pool).QueryRow(ctx, `select count(*) from posts`)

	var count int
	if err := row.Scan(&count); err != nil {
		return 0, &domain.OperationError{
			Op:  "count posts",
			Err: err,
		}
	}

	return count, nil
}

func (r *PostRepository) Update(ctx context.Context, post domain.Post) error {
	commandTag, err := dbFromContext(ctx, r.pool).Exec(
		ctx,
		`update posts
		 set author_id = $2,
		     title = $3,
		     content = $4,
		     comments_enabled = $5,
		     created_at = $6
		 where id = $1`,
		post.ID.UUID(),
		post.AuthorID.UUID(),
		post.Title,
		post.Content,
		post.CommentsEnabled,
		post.CreatedAt,
	)
	if err != nil {
		return &domain.OperationError{
			Op:  "update post",
			Err: err,
		}
	}

	if commandTag.RowsAffected() == 0 {
		return &domain.NotFoundError{
			Resource: "post",
			ID:       post.ID.String(),
		}
	}

	return nil
}

func scanPost(scanner interface{ Scan(dest ...any) error }) (domain.Post, bool, error) {
	var (
		id              uuid.UUID
		authorID        uuid.UUID
		title           string
		content         string
		commentsEnabled bool
		createdAt       time.Time
	)

	err := scanner.Scan(&id, &authorID, &title, &content, &commentsEnabled, &createdAt)
	if err != nil {
		if isNoRows(err) {
			return domain.Post{}, false, nil
		}

		return domain.Post{}, false, &domain.OperationError{
			Op:  "scan post",
			Err: err,
		}
	}

	post, buildErr := domain.NewPost(domain.NewPostID(id), domain.NewUserID(authorID), title, content, createdAt)
	if buildErr != nil {
		return domain.Post{}, false, buildErr
	}

	post.CommentsEnabled = commentsEnabled
	return post, true, nil
}
