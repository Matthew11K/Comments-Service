package txctx

import "context"

type AfterCommitAction func(context.Context)

type MissingRegistryError struct{}

func (e *MissingRegistryError) Error() string {
	return "after-commit registry is missing from context"
}

type Registry struct {
	actions []AfterCommitAction
}

type registryContextKey struct{}

func WithRegistry(ctx context.Context) (context.Context, *Registry) {
	registry := &Registry{}
	return context.WithValue(ctx, registryContextKey{}, registry), registry
}

func FromContext(ctx context.Context) (*Registry, bool) {
	registry, ok := ctx.Value(registryContextKey{}).(*Registry)
	return registry, ok
}

func AfterCommit(ctx context.Context, action AfterCommitAction) error {
	registry, ok := FromContext(ctx)
	if !ok {
		return &MissingRegistryError{}
	}

	registry.actions = append(registry.actions, action)
	return nil
}

func (r *Registry) Run(ctx context.Context) {
	for _, action := range r.actions {
		action(ctx)
	}
}
