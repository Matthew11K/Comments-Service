package di

type Error struct {
	Field   string
	Message string
}

func (e *Error) Error() string {
	if e.Field == "" {
		return e.Message
	}

	return e.Field + ": " + e.Message
}

type MissingDependencyError struct {
	Dependency string
}

func (e *MissingDependencyError) Error() string {
	return "missing dependency: " + e.Dependency
}

type InitError struct {
	Component string
	Err       error
}

func (e *InitError) Error() string {
	return "init " + e.Component + ": " + e.Err.Error()
}

func (e *InitError) Unwrap() error {
	return e.Err
}

type BuildError struct {
	Component string
	Err       error
}

func (e *BuildError) Error() string {
	return "build " + e.Component + ": " + e.Err.Error()
}

func (e *BuildError) Unwrap() error {
	return e.Err
}

type LifecycleError struct {
	Operation string
	Errors    []error
}

func (e *LifecycleError) Error() string {
	if len(e.Errors) == 0 {
		return e.Operation + " failed"
	}

	return e.Operation + " failed: " + e.Errors[0].Error()
}

func (e *LifecycleError) Unwrap() []error {
	return e.Errors
}
