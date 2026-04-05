package generated

import (
	"context"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/uuid"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/Matthew11K/Comments-Service/internal/domain"
)

func (ec *executionContext) unmarshalInputTime(_ context.Context, value any) (time.Time, error) {
	return graphql.UnmarshalTime(value)
}

func (ec *executionContext) _Time(_ context.Context, _ ast.SelectionSet, value *time.Time) graphql.Marshaler {
	if value == nil {
		return graphql.Null
	}

	return graphql.MarshalTime(*value)
}

func (ec *executionContext) unmarshalInputUUID(_ context.Context, value any) (uuid.UUID, error) {
	raw, err := graphql.UnmarshalString(value)
	if err != nil {
		return uuid.UUID{}, err
	}

	parsed, err := uuid.Parse(raw)
	if err != nil {
		return uuid.UUID{}, &domain.ValidationError{
			Field:   "UUID",
			Message: "must be a valid UUID",
		}
	}

	return parsed, nil
}

func (ec *executionContext) _UUID(_ context.Context, _ ast.SelectionSet, value *uuid.UUID) graphql.Marshaler {
	if value == nil {
		return graphql.Null
	}

	return graphql.MarshalString(value.String())
}
