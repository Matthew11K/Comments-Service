package graphql

import (
	"context"
	"strconv"

	gqlgraphql "github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func guardMiddleware(maxDepth int, maxPageSize int) gqlgraphql.OperationMiddleware {
	return func(ctx context.Context, next gqlgraphql.OperationHandler) gqlgraphql.ResponseHandler {
		opCtx := gqlgraphql.GetOperationContext(ctx)
		if opCtx == nil {
			return next(ctx)
		}

		if err := validateDepthLimit(opCtx, maxDepth); err != nil {
			return errorResponse(err)
		}

		if err := validateFirstArguments(opCtx.Operation.SelectionSet, opCtx.Doc.Fragments, opCtx.Variables, maxPageSize); err != nil {
			return errorResponse(err)
		}

		return next(ctx)
	}
}

func errorResponse(err *gqlerror.Error) gqlgraphql.ResponseHandler {
	return func(context.Context) *gqlgraphql.Response {
		return &gqlgraphql.Response{Errors: gqlerror.List{err}}
	}
}

func selectionDepth(
	selectionSet ast.SelectionSet,
	fragments ast.FragmentDefinitionList,
	currentDepth int,
	visited map[string]struct{},
) int {
	maxDepth := currentDepth
	for _, selection := range selectionSet {
		switch value := selection.(type) {
		case *ast.Field:
			childDepth := currentDepth
			if len(value.SelectionSet) > 0 {
				childDepth = selectionDepth(value.SelectionSet, fragments, currentDepth+1, visited)
			}
			if childDepth > maxDepth {
				maxDepth = childDepth
			}
		case *ast.InlineFragment:
			childDepth := selectionDepth(value.SelectionSet, fragments, currentDepth+1, visited)
			if childDepth > maxDepth {
				maxDepth = childDepth
			}
		case *ast.FragmentSpread:
			if _, ok := visited[value.Name]; ok {
				continue
			}

			fragment := fragments.ForName(value.Name)
			if fragment == nil {
				continue
			}

			visited[value.Name] = struct{}{}
			childDepth := selectionDepth(fragment.SelectionSet, fragments, currentDepth+1, visited)
			delete(visited, value.Name)
			if childDepth > maxDepth {
				maxDepth = childDepth
			}
		}
	}

	return maxDepth
}

func validateDepthLimit(opCtx *gqlgraphql.OperationContext, maxDepth int) *gqlerror.Error {
	if maxDepth <= 0 {
		return nil
	}

	depth := selectionDepth(opCtx.Operation.SelectionSet, opCtx.Doc.Fragments, 1, map[string]struct{}{})
	if depth <= maxDepth {
		return nil
	}

	return &gqlerror.Error{
		Message:    "query depth exceeds the configured limit",
		Extensions: map[string]any{"code": "GRAPHQL_DEPTH_LIMIT"},
	}
}

func validateFirstArguments(
	selectionSet ast.SelectionSet,
	fragments ast.FragmentDefinitionList,
	variables map[string]any,
	maxPageSize int,
) *gqlerror.Error {
	if maxPageSize <= 0 {
		return nil
	}

	return validateSelectionSetFirstArguments(selectionSet, fragments, variables, maxPageSize, map[string]struct{}{})
}

func validateSelectionSetFirstArguments(
	selectionSet ast.SelectionSet,
	fragments ast.FragmentDefinitionList,
	variables map[string]any,
	maxPageSize int,
	visited map[string]struct{},
) *gqlerror.Error {
	for _, selection := range selectionSet {
		if err := validateSelectionFirstArgument(selection, fragments, variables, maxPageSize, visited); err != nil {
			return err
		}
	}

	return nil
}

func validateSelectionFirstArgument(
	selection ast.Selection,
	fragments ast.FragmentDefinitionList,
	variables map[string]any,
	maxPageSize int,
	visited map[string]struct{},
) *gqlerror.Error {
	switch value := selection.(type) {
	case *ast.Field:
		if err := validateFieldFirstArgument(value, variables, maxPageSize); err != nil {
			return err
		}

		return validateSelectionSetFirstArguments(value.SelectionSet, fragments, variables, maxPageSize, visited)
	case *ast.InlineFragment:
		return validateSelectionSetFirstArguments(value.SelectionSet, fragments, variables, maxPageSize, visited)
	case *ast.FragmentSpread:
		return validateFragmentFirstArgument(value, fragments, variables, maxPageSize, visited)
	default:
		return nil
	}
}

func validateFieldFirstArgument(field *ast.Field, variables map[string]any, maxPageSize int) *gqlerror.Error {
	argument := field.Arguments.ForName("first")
	if argument == nil {
		return nil
	}

	rawValue, err := argument.Value.Value(variables)
	if err != nil {
		return invalidFirstArgumentError("invalid pagination argument", "GRAPHQL_PAGINATION_INVALID")
	}

	first, ok := intValue(rawValue)
	if !ok || first <= 0 {
		return invalidFirstArgumentError("first must be greater than zero", "GRAPHQL_PAGINATION_INVALID")
	}

	if first > maxPageSize {
		return invalidFirstArgumentError("first exceeds the configured maximum", "GRAPHQL_PAGINATION_LIMIT")
	}

	return nil
}

func validateFragmentFirstArgument(
	fragmentSpread *ast.FragmentSpread,
	fragments ast.FragmentDefinitionList,
	variables map[string]any,
	maxPageSize int,
	visited map[string]struct{},
) *gqlerror.Error {
	if _, ok := visited[fragmentSpread.Name]; ok {
		return nil
	}

	fragment := fragments.ForName(fragmentSpread.Name)
	if fragment == nil {
		return nil
	}

	visited[fragmentSpread.Name] = struct{}{}
	defer delete(visited, fragmentSpread.Name)

	return validateSelectionSetFirstArguments(fragment.SelectionSet, fragments, variables, maxPageSize, visited)
}

func invalidFirstArgumentError(message string, code string) *gqlerror.Error {
	return &gqlerror.Error{
		Message:    message,
		Extensions: map[string]any{"code": code, "field": "first"},
	}
}

func intValue(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case string:
		parsed, err := strconv.Atoi(typed)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}
