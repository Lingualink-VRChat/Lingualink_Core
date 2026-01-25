package errors

// ErrorCode is a stable identifier for a category of application errors.
type ErrorCode string

const (
	// ErrCodeValidation indicates a request validation error.
	ErrCodeValidation ErrorCode = "VALIDATION_ERROR"
	// ErrCodeAuth indicates an authentication or authorization error.
	ErrCodeAuth ErrorCode = "AUTH_ERROR"
	// ErrCodeLLM indicates an upstream LLM backend error.
	ErrCodeLLM ErrorCode = "LLM_ERROR"
	// ErrCodeParsing indicates an LLM response parsing error.
	ErrCodeParsing ErrorCode = "PARSING_ERROR"
	// ErrCodeInternal indicates an internal server error.
	ErrCodeInternal ErrorCode = "INTERNAL_ERROR"
)

// AppError is the standard structured error type used across the service.
type AppError struct {
	Code    ErrorCode
	Message string
	Cause   error
	Details map[string]interface{}
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return string(e.Code)
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// NewValidationError creates an AppError with ErrCodeValidation.
func NewValidationError(msg string, cause error) *AppError {
	return &AppError{Code: ErrCodeValidation, Message: msg, Cause: cause}
}

// NewAuthError creates an AppError with ErrCodeAuth.
func NewAuthError(msg string, cause error) *AppError {
	return &AppError{Code: ErrCodeAuth, Message: msg, Cause: cause}
}

// NewLLMError creates an AppError with ErrCodeLLM.
func NewLLMError(msg string, cause error) *AppError {
	return &AppError{Code: ErrCodeLLM, Message: msg, Cause: cause}
}

// NewParsingError creates an AppError with ErrCodeParsing.
func NewParsingError(msg string, cause error) *AppError {
	return &AppError{Code: ErrCodeParsing, Message: msg, Cause: cause}
}

// NewInternalError creates an AppError with ErrCodeInternal.
func NewInternalError(msg string, cause error) *AppError {
	return &AppError{Code: ErrCodeInternal, Message: msg, Cause: cause}
}
