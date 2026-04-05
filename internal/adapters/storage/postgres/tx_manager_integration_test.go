package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	storagepostgres "github.com/Matthew11K/Comments-Service/internal/adapters/storage/postgres"
	"github.com/Matthew11K/Comments-Service/internal/application/txctx"
	"github.com/Matthew11K/Comments-Service/internal/domain"
)

func TestTxManagerRunsAfterCommitOnlyAfterSuccessfulCommit(t *testing.T) {
	resetStorageDB(t)

	repo := storagepostgres.NewPostRepository(suite.Pool)
	manager := storagepostgres.NewTxManager(suite.Pool)
	post := mustPost(
		t,
		"00000000-0000-0000-0000-000000000010",
		"11111111-1111-1111-1111-111111111111",
		time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC),
	)
	callbackRuns := 0
	callbackSawCommittedPost := false

	err := manager.WithinTx(t.Context(), func(ctx context.Context) error {
		if err := repo.Create(ctx, post); err != nil {
			return err
		}

		return txctx.AfterCommit(ctx, func(ctx context.Context) {
			callbackRuns++

			stored, err := repo.GetByID(ctx, post.ID)
			if err == nil && stored.ID == post.ID {
				callbackSawCommittedPost = true
			}
		})
	})
	require.NoError(t, err)
	require.Equal(t, 1, callbackRuns)
	require.True(t, callbackSawCommittedPost)

	stored, err := repo.GetByID(t.Context(), post.ID)
	require.NoError(t, err)
	require.Equal(t, post.ID, stored.ID)
}

func TestTxManagerSkipsAfterCommitOnRollback(t *testing.T) {
	resetStorageDB(t)

	repo := storagepostgres.NewPostRepository(suite.Pool)
	manager := storagepostgres.NewTxManager(suite.Pool)
	post := mustPost(
		t,
		"00000000-0000-0000-0000-000000000010",
		"11111111-1111-1111-1111-111111111111",
		time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC),
	)
	callbackRuns := 0

	err := manager.WithinTx(t.Context(), func(ctx context.Context) error {
		if err := repo.Create(ctx, post); err != nil {
			return err
		}

		if err := txctx.AfterCommit(ctx, func(context.Context) {
			callbackRuns++
		}); err != nil {
			return err
		}

		return &domain.ConflictError{
			Resource: "transaction",
			Message:  "rollback",
		}
	})
	require.Error(t, err)
	require.Zero(t, callbackRuns)

	_, err = repo.GetByID(t.Context(), post.ID)
	require.Error(t, err)

	var notFoundErr *domain.NotFoundError
	require.ErrorAs(t, err, &notFoundErr)
}
