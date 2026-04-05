package postgres_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	storagepostgres "github.com/Matthew11K/Comments-Service/internal/adapters/storage/postgres"
	"github.com/Matthew11K/Comments-Service/internal/domain"
)

func TestPostRepositoryCreateGetUpdateCount(t *testing.T) {
	resetStorageDB(t)

	repo := storagepostgres.NewPostRepository(suite.Pool)
	post := mustPost(
		t,
		"00000000-0000-0000-0000-000000000010",
		"11111111-1111-1111-1111-111111111111",
		time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC),
	)

	require.NoError(t, repo.Create(t.Context(), post))

	stored, err := repo.GetByID(t.Context(), post.ID)
	require.NoError(t, err)
	require.Equal(t, post.ID, stored.ID)
	require.Equal(t, post.AuthorID, stored.AuthorID)
	require.True(t, stored.CommentsEnabled)

	count, err := repo.Count(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, count)

	require.NoError(t, post.SetCommentsEnabled(post.AuthorID, false))
	require.NoError(t, repo.Update(t.Context(), post))

	updated, err := repo.GetByID(t.Context(), post.ID)
	require.NoError(t, err)
	require.False(t, updated.CommentsEnabled)
}

func TestPostRepositoryListUsesStableKeysetPagination(t *testing.T) {
	resetStorageDB(t)

	repo := storagepostgres.NewPostRepository(suite.Pool)
	createdAt := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	authorID := domain.NewUserID(uuid.MustParse("11111111-1111-1111-1111-111111111111"))

	posts := []domain.Post{
		mustPostWithAuthor(t, "00000000-0000-0000-0000-000000000001", authorID, createdAt),
		mustPostWithAuthor(t, "00000000-0000-0000-0000-000000000003", authorID, createdAt),
		mustPostWithAuthor(t, "00000000-0000-0000-0000-000000000002", authorID, createdAt),
	}
	for _, post := range posts {
		require.NoError(t, repo.Create(t.Context(), post))
	}

	firstPage, err := repo.List(t.Context(), domain.PageInput{First: 2})
	require.NoError(t, err)
	require.Len(t, firstPage.Items, 2)
	require.True(t, firstPage.HasNextPage)
	require.Equal(t, posts[1].ID, firstPage.Items[0].ID)
	require.Equal(t, posts[2].ID, firstPage.Items[1].ID)

	endCursor, err := domain.EncodeCursor(firstPage.Items[1].Cursor())
	require.NoError(t, err)

	secondPageInput, err := domain.NewPageInput(2, &endCursor)
	require.NoError(t, err)

	secondPage, err := repo.List(t.Context(), secondPageInput)
	require.NoError(t, err)
	require.Len(t, secondPage.Items, 1)
	require.False(t, secondPage.HasNextPage)
	require.Equal(t, posts[0].ID, secondPage.Items[0].ID)
}

func TestCommentRepositoryCreateGetListAndCount(t *testing.T) {
	resetStorageDB(t)

	postRepo := storagepostgres.NewPostRepository(suite.Pool)
	commentRepo := storagepostgres.NewCommentRepository(suite.Pool)
	authorID := domain.NewUserID(uuid.MustParse("11111111-1111-1111-1111-111111111111"))
	postID := domain.NewPostID(uuid.MustParse("00000000-0000-0000-0000-000000000100"))
	otherPostID := domain.NewPostID(uuid.MustParse("00000000-0000-0000-0000-000000000200"))
	createdAt := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)

	require.NoError(t, postRepo.Create(t.Context(), mustPostWithAuthor(t, postID.String(), authorID, createdAt)))
	require.NoError(t, postRepo.Create(t.Context(), mustPostWithAuthor(t, otherPostID.String(), authorID, createdAt.Add(-time.Minute))))

	topLevelFirst := mustComment(
		t,
		"00000000-0000-0000-0000-000000000001",
		postID,
		nil,
		authorID,
		"first",
		createdAt,
	)
	topLevelSecond := mustComment(
		t,
		"00000000-0000-0000-0000-000000000002",
		postID,
		nil,
		authorID,
		"second",
		createdAt,
	)
	reply := mustComment(
		t,
		"00000000-0000-0000-0000-000000000003",
		postID,
		&topLevelSecond.ID,
		authorID,
		"reply",
		createdAt.Add(-time.Second),
	)
	otherPostComment := mustComment(
		t,
		"00000000-0000-0000-0000-000000000004",
		otherPostID,
		nil,
		authorID,
		"other-post",
		createdAt.Add(-2*time.Second),
	)

	for _, comment := range []domain.Comment{topLevelFirst, topLevelSecond, reply, otherPostComment} {
		require.NoError(t, commentRepo.Create(t.Context(), comment))
	}

	storedTopLevel, err := commentRepo.GetByID(t.Context(), topLevelFirst.ID)
	require.NoError(t, err)
	require.Nil(t, storedTopLevel.ParentID)

	storedReply, err := commentRepo.GetByID(t.Context(), reply.ID)
	require.NoError(t, err)
	require.NotNil(t, storedReply.ParentID)
	require.Equal(t, topLevelSecond.ID, *storedReply.ParentID)

	firstPage, err := commentRepo.ListTopLevel(t.Context(), postID, domain.PageInput{First: 1})
	require.NoError(t, err)
	require.Len(t, firstPage.Items, 1)
	require.True(t, firstPage.HasNextPage)
	require.Equal(t, topLevelSecond.ID, firstPage.Items[0].ID)

	endCursor, err := domain.EncodeCursor(firstPage.Items[0].Cursor())
	require.NoError(t, err)

	secondPageInput, err := domain.NewPageInput(1, &endCursor)
	require.NoError(t, err)

	secondPage, err := commentRepo.ListTopLevel(t.Context(), postID, secondPageInput)
	require.NoError(t, err)
	require.Len(t, secondPage.Items, 1)
	require.False(t, secondPage.HasNextPage)
	require.Equal(t, topLevelFirst.ID, secondPage.Items[0].ID)

	replies, err := commentRepo.ListReplies(t.Context(), topLevelSecond.ID, domain.PageInput{First: 10})
	require.NoError(t, err)
	require.Len(t, replies.Items, 1)
	require.Equal(t, reply.ID, replies.Items[0].ID)

	topLevelCount, err := commentRepo.CountTopLevel(t.Context(), postID)
	require.NoError(t, err)
	require.Equal(t, 2, topLevelCount)

	replyCount, err := commentRepo.CountReplies(t.Context(), topLevelSecond.ID)
	require.NoError(t, err)
	require.Equal(t, 1, replyCount)
}

func TestCommentRepositoryBatchOperations(t *testing.T) {
	resetStorageDB(t)

	postRepo := storagepostgres.NewPostRepository(suite.Pool)
	commentRepo := storagepostgres.NewCommentRepository(suite.Pool)
	authorID := domain.NewUserID(uuid.MustParse("11111111-1111-1111-1111-111111111111"))
	createdAt := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	postIDOne := domain.NewPostID(uuid.MustParse("00000000-0000-0000-0000-000000000100"))
	postIDTwo := domain.NewPostID(uuid.MustParse("00000000-0000-0000-0000-000000000200"))

	require.NoError(t, postRepo.Create(t.Context(), mustPostWithAuthor(t, postIDOne.String(), authorID, createdAt)))
	require.NoError(t, postRepo.Create(t.Context(), mustPostWithAuthor(t, postIDTwo.String(), authorID, createdAt)))

	topLevelOne := mustComment(
		t,
		"00000000-0000-0000-0000-000000000001",
		postIDOne,
		nil,
		authorID,
		"one",
		createdAt,
	)
	topLevelTwo := mustComment(
		t,
		"00000000-0000-0000-0000-000000000002",
		postIDOne,
		nil,
		authorID,
		"two",
		createdAt.Add(-time.Second),
	)
	topLevelThree := mustComment(
		t,
		"00000000-0000-0000-0000-000000000003",
		postIDTwo,
		nil,
		authorID,
		"three",
		createdAt.Add(-2*time.Second),
	)
	replyOne := mustComment(
		t,
		"00000000-0000-0000-0000-000000000004",
		postIDOne,
		&topLevelOne.ID,
		authorID,
		"reply-one",
		createdAt.Add(-3*time.Second),
	)
	replyTwo := mustComment(
		t,
		"00000000-0000-0000-0000-000000000005",
		postIDOne,
		&topLevelOne.ID,
		authorID,
		"reply-two",
		createdAt.Add(-4*time.Second),
	)
	replyThree := mustComment(
		t,
		"00000000-0000-0000-0000-000000000006",
		postIDTwo,
		&topLevelThree.ID,
		authorID,
		"reply-three",
		createdAt.Add(-5*time.Second),
	)

	for _, comment := range []domain.Comment{
		topLevelOne,
		topLevelTwo,
		topLevelThree,
		replyOne,
		replyTwo,
		replyThree,
	} {
		require.NoError(t, commentRepo.Create(t.Context(), comment))
	}

	topLevelPages, err := commentRepo.BatchListTopLevel(
		t.Context(),
		[]domain.PostID{postIDOne, postIDTwo},
		domain.PageInput{First: 1},
	)
	require.NoError(t, err)
	require.Len(t, topLevelPages[postIDOne].Items, 1)
	require.True(t, topLevelPages[postIDOne].HasNextPage)
	require.Equal(t, topLevelOne.ID, topLevelPages[postIDOne].Items[0].ID)
	require.Len(t, topLevelPages[postIDTwo].Items, 1)
	require.False(t, topLevelPages[postIDTwo].HasNextPage)
	require.Equal(t, topLevelThree.ID, topLevelPages[postIDTwo].Items[0].ID)

	topLevelCounts, err := commentRepo.BatchCountTopLevel(t.Context(), []domain.PostID{postIDOne, postIDTwo})
	require.NoError(t, err)
	require.Equal(t, 2, topLevelCounts[postIDOne])
	require.Equal(t, 1, topLevelCounts[postIDTwo])

	replyPages, err := commentRepo.BatchListReplies(
		t.Context(),
		[]domain.CommentID{topLevelOne.ID, topLevelThree.ID},
		domain.PageInput{First: 1},
	)
	require.NoError(t, err)
	require.Len(t, replyPages[topLevelOne.ID].Items, 1)
	require.True(t, replyPages[topLevelOne.ID].HasNextPage)
	require.Equal(t, replyOne.ID, replyPages[topLevelOne.ID].Items[0].ID)
	require.Len(t, replyPages[topLevelThree.ID].Items, 1)
	require.False(t, replyPages[topLevelThree.ID].HasNextPage)
	require.Equal(t, replyThree.ID, replyPages[topLevelThree.ID].Items[0].ID)

	replyCounts, err := commentRepo.BatchCountReplies(t.Context(), []domain.CommentID{topLevelOne.ID, topLevelThree.ID})
	require.NoError(t, err)
	require.Equal(t, 2, replyCounts[topLevelOne.ID])
	require.Equal(t, 1, replyCounts[topLevelThree.ID])
}

func TestCommentRepositoryRejectsMissingParentForeignKey(t *testing.T) {
	resetStorageDB(t)

	postRepo := storagepostgres.NewPostRepository(suite.Pool)
	commentRepo := storagepostgres.NewCommentRepository(suite.Pool)
	authorID := domain.NewUserID(uuid.MustParse("11111111-1111-1111-1111-111111111111"))
	postID := domain.NewPostID(uuid.MustParse("00000000-0000-0000-0000-000000000100"))
	missingParentID := domain.NewCommentID(uuid.MustParse("00000000-0000-0000-0000-000000000999"))

	require.NoError(
		t,
		postRepo.Create(
			t.Context(),
			mustPostWithAuthor(t, postID.String(), authorID, time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)),
		),
	)

	err := commentRepo.Create(
		t.Context(),
		mustComment(
			t,
			"00000000-0000-0000-0000-000000000001",
			postID,
			&missingParentID,
			authorID,
			"reply",
			time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC),
		),
	)
	require.Error(t, err)

	var operationErr *domain.OperationError
	require.ErrorAs(t, err, &operationErr)
}

func TestCommentsTableRejectsBodiesLongerThanMaxLength(t *testing.T) {
	resetStorageDB(t)

	postRepo := storagepostgres.NewPostRepository(suite.Pool)
	authorID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	postID := uuid.MustParse("00000000-0000-0000-0000-000000000100")

	require.NoError(
		t,
		postRepo.Create(
			t.Context(),
			mustPostWithAuthor(t, postID.String(), domain.NewUserID(authorID), time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)),
		),
	)

	_, err := suite.Pool.Exec(
		t.Context(),
		`insert into comments (id, post_id, author_id, body, created_at) values ($1, $2, $3, $4, $5)`,
		uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		postID,
		authorID,
		strings.Repeat("a", domain.MaxCommentBodyLength+1),
		time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC),
	)
	require.Error(t, err)
}

func resetStorageDB(t *testing.T) {
	t.Helper()
	require.NotNil(t, suite)
	require.NoError(t, suite.Reset(t.Context()))
}

func mustPost(t *testing.T, id string, author string, createdAt time.Time) domain.Post {
	t.Helper()

	return mustPostWithAuthor(t, id, domain.NewUserID(uuid.MustParse(author)), createdAt)
}

func mustPostWithAuthor(t *testing.T, id string, authorID domain.UserID, createdAt time.Time) domain.Post {
	t.Helper()

	post, err := domain.NewPost(
		domain.NewPostID(uuid.MustParse(id)),
		authorID,
		"title",
		"content",
		createdAt,
	)
	require.NoError(t, err)
	return post
}

func mustComment(
	t *testing.T,
	id string,
	postID domain.PostID,
	parentID *domain.CommentID,
	authorID domain.UserID,
	body string,
	createdAt time.Time,
) domain.Comment {
	t.Helper()

	comment, err := domain.NewComment(
		domain.NewCommentID(uuid.MustParse(id)),
		postID,
		parentID,
		authorID,
		body,
		createdAt,
	)
	require.NoError(t, err)
	return comment
}
