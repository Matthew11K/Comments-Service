package postgres

import (
	"errors"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Matthew11K/Comments-Service/internal/domain"
)

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func formatSQL(template string, replacement string, position int) string {
	result := strings.Replace(template, "%s", replacement, 1)
	return strings.Replace(result, "%d", strconv.Itoa(position), 1)
}

func toUUIDs(ids []domain.PostID) []uuid.UUID {
	result := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		result = append(result, id.UUID())
	}

	return result
}

func toUUIDsFromComments(ids []domain.CommentID) []uuid.UUID {
	result := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		result = append(result, id.UUID())
	}

	return result
}
