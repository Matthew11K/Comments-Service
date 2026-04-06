package graphql

import (
	"context"
	"errors"
	"log/slog"

	gqlgraphql "github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/Matthew11K/Comments-Service/internal/domain"
)

func newErrorPresenter(logger *slog.Logger) gqlgraphql.ErrorPresenterFunc {
	return func(ctx context.Context, err error) *gqlerror.Error {
		presented := gqlgraphql.DefaultErrorPresenter(ctx, err)
		if presented.Extensions != nil {
			if _, ok := presented.Extensions["code"]; ok {
				return presented
			}
		}

		if validationErr, ok := errors.AsType[*domain.ValidationError](err); ok {
			presented.Message = validationErr.Message
			presented.Extensions = map[string]any{"code": validationErr.Code()}
			if validationErr.Field != "" {
				presented.Extensions["field"] = validationErr.Field
			}
			return presented
		}

		if notFoundErr, ok := errors.AsType[*domain.NotFoundError](err); ok {
			presented.Message = notFoundErr.Error()
			presented.Extensions = map[string]any{"code": notFoundErr.Code()}
			return presented
		}

		if forbiddenErr, ok := errors.AsType[*domain.ForbiddenError](err); ok {
			presented.Message = forbiddenErr.Error()
			presented.Extensions = map[string]any{"code": forbiddenErr.Code()}
			return presented
		}

		if conflictErr, ok := errors.AsType[*domain.ConflictError](err); ok {
			presented.Message = conflictErr.Error()
			presented.Extensions = map[string]any{"code": conflictErr.Code()}
			return presented
		}

		if _, ok := errors.AsType[*gqlerror.Error](err); ok {
			if presented.Extensions == nil {
				presented.Extensions = map[string]any{}
			}
			presented.Extensions["code"] = "GRAPHQL_ERROR"
			return presented
		}

		logger.ErrorContext(ctx, "graphql internal error", "error", err)
		presented.Message = "internal server error"
		presented.Extensions = map[string]any{"code": "INTERNAL"}
		return presented
	}
}

func newRecoverFunc(logger *slog.Logger) gqlgraphql.RecoverFunc {
	return func(ctx context.Context, recovered any) error {
		logger.ErrorContext(ctx, "graphql panic recovered", "panic", recovered)
		return &gqlerror.Error{
			Message:    "internal server error",
			Extensions: map[string]any{"code": "INTERNAL"},
		}
	}
}
