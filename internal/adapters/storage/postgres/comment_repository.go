package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Matthew11K/Comments-Service/internal/domain"
)

const keysetAfterPredicate = "and (created_at, id) < ($2, $3)"

type CommentRepository struct {
	pool *pgxpool.Pool
}

func NewCommentRepository(pool *pgxpool.Pool) *CommentRepository {
	return &CommentRepository{pool: pool}
}

func (r *CommentRepository) Create(ctx context.Context, comment domain.Comment) error {
	var parentID *uuid.UUID
	if comment.ParentID != nil {
		value := comment.ParentID.UUID()
		parentID = &value
	}

	_, err := dbFromContext(ctx, r.pool).Exec(
		ctx,
		`insert into comments (id, post_id, parent_id, author_id, body, created_at)
		 values ($1, $2, $3, $4, $5, $6)`,
		comment.ID.UUID(),
		comment.PostID.UUID(),
		parentID,
		comment.AuthorID.UUID(),
		comment.Body,
		comment.CreatedAt,
	)
	if err != nil {
		return &domain.OperationError{
			Op:  "insert comment",
			Err: err,
		}
	}

	return nil
}

func (r *CommentRepository) GetByID(ctx context.Context, id domain.CommentID) (domain.Comment, error) {
	row := dbFromContext(ctx, r.pool).QueryRow(
		ctx,
		`select id, post_id, parent_id, author_id, body, created_at
		 from comments
		 where id = $1`,
		id.UUID(),
	)

	comment, found, err := scanComment(row)
	if err != nil {
		return domain.Comment{}, err
	}

	if !found {
		return domain.Comment{}, &domain.NotFoundError{
			Resource: "comment",
			ID:       id.String(),
		}
	}

	return comment, nil
}

func (r *CommentRepository) ListTopLevel(
	ctx context.Context,
	postID domain.PostID,
	page domain.PageInput,
) (domain.Page[domain.Comment], error) {
	query := `
select id, post_id, parent_id, author_id, body, created_at
from comments
where post_id = $1 and parent_id is null %s
order by created_at desc, id desc
limit $%d`

	args := make([]any, 0, 4)
	args = append(args, postID.UUID())
	filter := ""
	limitIndex := 2
	if page.After != nil {
		filter = keysetAfterPredicate
		args = append(args, page.After.CreatedAt, page.After.ID)
		limitIndex = 4
	}
	args = append(args, page.LimitPlusOne())

	return r.listWithQuery(ctx, formatSQL(query, filter, limitIndex), args, page)
}

func (r *CommentRepository) ListReplies(
	ctx context.Context,
	parentID domain.CommentID,
	page domain.PageInput,
) (domain.Page[domain.Comment], error) {
	query := `
select id, post_id, parent_id, author_id, body, created_at
from comments
where parent_id = $1 %s
order by created_at desc, id desc
limit $%d`

	args := make([]any, 0, 4)
	args = append(args, parentID.UUID())
	filter := ""
	limitIndex := 2
	if page.After != nil {
		filter = keysetAfterPredicate
		args = append(args, page.After.CreatedAt, page.After.ID)
		limitIndex = 4
	}
	args = append(args, page.LimitPlusOne())

	return r.listWithQuery(ctx, formatSQL(query, filter, limitIndex), args, page)
}

func (r *CommentRepository) CountTopLevel(ctx context.Context, postID domain.PostID) (int, error) {
	return r.countQuery(ctx, `select count(*) from comments where post_id = $1 and parent_id is null`, postID.UUID())
}

func (r *CommentRepository) CountReplies(ctx context.Context, parentID domain.CommentID) (int, error) {
	return r.countQuery(ctx, `select count(*) from comments where parent_id = $1`, parentID.UUID())
}

func (r *CommentRepository) BatchListTopLevel(
	ctx context.Context,
	postIDs []domain.PostID,
	page domain.PageInput,
) (map[domain.PostID]domain.Page[domain.Comment], error) {
	args := []any{toUUIDs(postIDs)}
	filter := ""
	if page.After != nil {
		filter = keysetAfterPredicate
		args = append(args, page.After.CreatedAt, page.After.ID)
	}
	args = append(args, page.LimitPlusOne())

	query := `
with ranked as (
	select id, post_id, parent_id, author_id, body, created_at,
	       row_number() over (partition by post_id order by created_at desc, id desc) as rn
	from comments
	where post_id = any($1) and parent_id is null %s
)
select id, post_id, parent_id, author_id, body, created_at, rn
from ranked
where rn <= $%d
order by post_id, created_at desc, id desc`

	return r.batchListByPostID(ctx, formatSQL(query, filter, len(args)), args, postIDs, page)
}

func (r *CommentRepository) BatchListReplies(
	ctx context.Context,
	parentIDs []domain.CommentID,
	page domain.PageInput,
) (map[domain.CommentID]domain.Page[domain.Comment], error) {
	args := []any{toUUIDsFromComments(parentIDs)}
	filter := ""
	if page.After != nil {
		filter = keysetAfterPredicate
		args = append(args, page.After.CreatedAt, page.After.ID)
	}
	args = append(args, page.LimitPlusOne())

	query := `
with ranked as (
	select id, post_id, parent_id, author_id, body, created_at,
	       row_number() over (partition by parent_id order by created_at desc, id desc) as rn
	from comments
	where parent_id = any($1) %s
)
select id, post_id, parent_id, author_id, body, created_at, rn
from ranked
where rn <= $%d
order by parent_id, created_at desc, id desc`

	return r.batchListByParentID(ctx, formatSQL(query, filter, len(args)), args, parentIDs, page)
}

func (r *CommentRepository) BatchCountTopLevel(
	ctx context.Context,
	postIDs []domain.PostID,
) (map[domain.PostID]int, error) {
	rows, err := dbFromContext(ctx, r.pool).Query(
		ctx,
		`select post_id, count(*)
		 from comments
		 where post_id = any($1) and parent_id is null
		 group by post_id`,
		toUUIDs(postIDs),
	)
	if err != nil {
		return nil, &domain.OperationError{
			Op:  "batch count top-level comments",
			Err: err,
		}
	}
	defer rows.Close()

	counts := make(map[domain.PostID]int, len(postIDs))
	for _, postID := range postIDs {
		counts[postID] = 0
	}

	for rows.Next() {
		var (
			postID uuid.UUID
			count  int
		)
		if err := rows.Scan(&postID, &count); err != nil {
			return nil, &domain.OperationError{
				Op:  "scan top-level comment counts",
				Err: err,
			}
		}

		counts[domain.NewPostID(postID)] = count
	}

	if err := rows.Err(); err != nil {
		return nil, &domain.OperationError{
			Op:  "iterate top-level comment counts",
			Err: err,
		}
	}

	return counts, nil
}

func (r *CommentRepository) BatchCountReplies(
	ctx context.Context,
	parentIDs []domain.CommentID,
) (map[domain.CommentID]int, error) {
	rows, err := dbFromContext(ctx, r.pool).Query(
		ctx,
		`select parent_id, count(*)
		 from comments
		 where parent_id = any($1)
		 group by parent_id`,
		toUUIDsFromComments(parentIDs),
	)
	if err != nil {
		return nil, &domain.OperationError{
			Op:  "batch count replies",
			Err: err,
		}
	}
	defer rows.Close()

	counts := make(map[domain.CommentID]int, len(parentIDs))
	for _, parentID := range parentIDs {
		counts[parentID] = 0
	}

	for rows.Next() {
		var (
			parentID uuid.UUID
			count    int
		)
		if err := rows.Scan(&parentID, &count); err != nil {
			return nil, &domain.OperationError{
				Op:  "scan reply counts",
				Err: err,
			}
		}

		counts[domain.NewCommentID(parentID)] = count
	}

	if err := rows.Err(); err != nil {
		return nil, &domain.OperationError{
			Op:  "iterate reply counts",
			Err: err,
		}
	}

	return counts, nil
}

func (r *CommentRepository) listWithQuery(
	ctx context.Context,
	query string,
	args []any,
	page domain.PageInput,
) (domain.Page[domain.Comment], error) {
	rows, err := dbFromContext(ctx, r.pool).Query(ctx, query, args...)
	if err != nil {
		return domain.Page[domain.Comment]{}, &domain.OperationError{
			Op:  "query comments",
			Err: err,
		}
	}
	defer rows.Close()

	items := make([]domain.Comment, 0, page.LimitPlusOne())
	for rows.Next() {
		comment, _, scanErr := scanComment(rows)
		if scanErr != nil {
			return domain.Page[domain.Comment]{}, scanErr
		}

		items = append(items, comment)
	}

	if err := rows.Err(); err != nil {
		return domain.Page[domain.Comment]{}, &domain.OperationError{
			Op:  "iterate comments",
			Err: err,
		}
	}

	hasNextPage := len(items) > page.First
	if hasNextPage {
		items = items[:page.First]
	}

	return domain.Page[domain.Comment]{
		Items:       items,
		HasNextPage: hasNextPage,
	}, nil
}

func (r *CommentRepository) batchListByPostID(
	ctx context.Context,
	query string,
	args []any,
	postIDs []domain.PostID,
	page domain.PageInput,
) (map[domain.PostID]domain.Page[domain.Comment], error) {
	rows, err := dbFromContext(ctx, r.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, &domain.OperationError{
			Op:  "batch list top-level comments",
			Err: err,
		}
	}
	defer rows.Close()

	grouped := make(map[domain.PostID][]domain.Comment, len(postIDs))
	for _, postID := range postIDs {
		grouped[postID] = nil
	}

	for rows.Next() {
		comment, scanErr := scanCommentWithRank(rows)
		if scanErr != nil {
			return nil, scanErr
		}

		grouped[comment.PostID] = append(grouped[comment.PostID], comment)
	}

	if err := rows.Err(); err != nil {
		return nil, &domain.OperationError{
			Op:  "iterate batched top-level comments",
			Err: err,
		}
	}

	result := make(map[domain.PostID]domain.Page[domain.Comment], len(grouped))
	for postID, comments := range grouped {
		hasNextPage := len(comments) > page.First
		if hasNextPage {
			comments = comments[:page.First]
		}

		result[postID] = domain.Page[domain.Comment]{
			Items:       comments,
			HasNextPage: hasNextPage,
		}
	}

	return result, nil
}

func (r *CommentRepository) batchListByParentID(
	ctx context.Context,
	query string,
	args []any,
	parentIDs []domain.CommentID,
	page domain.PageInput,
) (map[domain.CommentID]domain.Page[domain.Comment], error) {
	rows, err := dbFromContext(ctx, r.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, &domain.OperationError{
			Op:  "batch list replies",
			Err: err,
		}
	}
	defer rows.Close()

	grouped := make(map[domain.CommentID][]domain.Comment, len(parentIDs))
	for _, parentID := range parentIDs {
		grouped[parentID] = nil
	}

	for rows.Next() {
		comment, scanErr := scanCommentWithRank(rows)
		if scanErr != nil {
			return nil, scanErr
		}

		if comment.ParentID == nil {
			continue
		}

		grouped[*comment.ParentID] = append(grouped[*comment.ParentID], comment)
	}

	if err := rows.Err(); err != nil {
		return nil, &domain.OperationError{
			Op:  "iterate batched replies",
			Err: err,
		}
	}

	result := make(map[domain.CommentID]domain.Page[domain.Comment], len(grouped))
	for parentID, comments := range grouped {
		hasNextPage := len(comments) > page.First
		if hasNextPage {
			comments = comments[:page.First]
		}

		result[parentID] = domain.Page[domain.Comment]{
			Items:       comments,
			HasNextPage: hasNextPage,
		}
	}

	return result, nil
}

func (r *CommentRepository) countQuery(ctx context.Context, query string, arg any) (int, error) {
	row := dbFromContext(ctx, r.pool).QueryRow(ctx, query, arg)

	var count int
	if err := row.Scan(&count); err != nil {
		return 0, &domain.OperationError{
			Op:  "count comments",
			Err: err,
		}
	}

	return count, nil
}

func scanComment(scanner interface{ Scan(dest ...any) error }) (domain.Comment, bool, error) {
	var (
		id        uuid.UUID
		postID    uuid.UUID
		parentID  *uuid.UUID
		authorID  uuid.UUID
		body      string
		createdAt time.Time
	)

	err := scanner.Scan(&id, &postID, &parentID, &authorID, &body, &createdAt)
	if err != nil {
		if isNoRows(err) {
			return domain.Comment{}, false, nil
		}

		return domain.Comment{}, false, &domain.OperationError{
			Op:  "scan comment",
			Err: err,
		}
	}

	comment, buildErr := buildComment(id, postID, parentID, authorID, body, createdAt)
	if buildErr != nil {
		return domain.Comment{}, false, buildErr
	}

	return comment, true, nil
}

func scanCommentWithRank(scanner interface{ Scan(dest ...any) error }) (domain.Comment, error) {
	var (
		id        uuid.UUID
		postID    uuid.UUID
		parentID  *uuid.UUID
		authorID  uuid.UUID
		body      string
		createdAt time.Time
		rank      int
	)

	if err := scanner.Scan(&id, &postID, &parentID, &authorID, &body, &createdAt, &rank); err != nil {
		return domain.Comment{}, &domain.OperationError{
			Op:  "scan ranked comment",
			Err: err,
		}
	}

	comment, err := buildComment(id, postID, parentID, authorID, body, createdAt)
	if err != nil {
		return domain.Comment{}, err
	}

	return comment, nil
}

func buildComment(
	id uuid.UUID,
	postID uuid.UUID,
	parentID *uuid.UUID,
	authorID uuid.UUID,
	body string,
	createdAt time.Time,
) (domain.Comment, error) {
	var commentParentID *domain.CommentID
	if parentID != nil {
		value := domain.NewCommentID(*parentID)
		commentParentID = &value
	}

	return domain.NewComment(
		domain.NewCommentID(id),
		domain.NewPostID(postID),
		commentParentID,
		domain.NewUserID(authorID),
		body,
		createdAt,
	)
}
