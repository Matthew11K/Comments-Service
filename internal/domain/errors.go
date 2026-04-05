package domain

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}

	return e.Field + ": " + e.Message
}

func (e *ValidationError) Code() string {
	return "VALIDATION"
}

type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	if e.ID == "" {
		return e.Resource + " not found"
	}

	return e.Resource + " not found: " + e.ID
}

func (e *NotFoundError) Code() string {
	return "NOT_FOUND"
}

type ForbiddenError struct {
	Action   string
	Resource string
}

func (e *ForbiddenError) Error() string {
	return "forbidden: " + e.Action + " on " + e.Resource
}

func (e *ForbiddenError) Code() string {
	return "FORBIDDEN"
}

type ConflictError struct {
	Resource string
	Message  string
}

func (e *ConflictError) Error() string {
	if e.Resource == "" {
		return e.Message
	}

	return e.Resource + ": " + e.Message
}

func (e *ConflictError) Code() string {
	return "CONFLICT"
}

type OperationError struct {
	Op  string
	Err error
}

func (e *OperationError) Error() string {
	if e.Err == nil {
		return e.Op
	}

	return e.Op + ": " + e.Err.Error()
}

func (e *OperationError) Unwrap() error {
	return e.Err
}
