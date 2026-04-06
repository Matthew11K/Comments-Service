package graphql

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/Matthew11K/Comments-Service/internal/domain"
)

func TestErrorPresenterPrefersDomainConflictCodeOverGenericGraphQLError(t *testing.T) {
	t.Parallel()

	conflictErr := &domain.ConflictError{
		Resource: "comment",
		Message:  "parent comment belongs to a different post",
	}
	wrapped := &gqlerror.Error{
		Err: conflictErr,
	}

	presented := newErrorPresenter(slog.New(slog.NewTextHandler(io.Discard, nil)))(context.Background(), wrapped)
	if presented.Extensions["code"] != "CONFLICT" {
		t.Fatalf("unexpected code: %v", presented.Extensions["code"])
	}
	if presented.Message != conflictErr.Error() {
		t.Fatalf("unexpected message: %s", presented.Message)
	}
}
