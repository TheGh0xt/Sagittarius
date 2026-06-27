package shared

import "fmt"

// ErrInvalidInput is returned when the caller provides invalid data.
type (
	ErrInvalidInput struct {
		Field   string
		Message string
	}

	ErrInternalServerError struct {
		Message string
	}
)

func (e ErrInvalidInput) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("invalid input for field '%s': %s", e.Field, e.Message)
	}
	return fmt.Sprintf("invalid input: %s", e.Message)
}

func (e ErrInternalServerError) Error() string {
	return fmt.Sprintf("internal server error: %s", e.Message)
}
