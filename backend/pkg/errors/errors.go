package errors

import "fmt"

// ErrorCode 错误码类型
type ErrorCode string

const (
	// 通用错误码
	ErrCodeBadRequest   ErrorCode = "BAD_REQUEST"
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"
	ErrCodeNotFound     ErrorCode = "NOT_FOUND"
	ErrCodeConflict     ErrorCode = "CONFLICT"
	ErrCodeTimeout      ErrorCode = "TIMEOUT"
	ErrCodeInternal     ErrorCode = "INTERNAL_ERROR"

	// MemBrowser 特有错误码
	ErrCodeAgentFailed     ErrorCode = "AGENT_FAILED"
	ErrCodeWSDisconnected  ErrorCode = "WS_DISCONNECTED"
	ErrCodeDOMEmpty        ErrorCode = "DOM_EMPTY"
	ErrCodeElementNotFound ErrorCode = "ELEMENT_NOT_FOUND"
	ErrCodeTeachTimeout    ErrorCode = "TEACH_TIMEOUT"
	ErrCodeTabClosed       ErrorCode = "TAB_CLOSED"
	ErrCodeAIThrottled     ErrorCode = "AI_THROTTLED"
	ErrCodeTokenOverflow   ErrorCode = "TOKEN_OVERFLOW"
)

// AppError 应用错误
type AppError struct {
	Code    ErrorCode `json:"errorCode"`
	Message string    `json:"error"`
	Details string    `json:"details,omitempty"`
}

func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// New 创建应用错误
func New(code ErrorCode, msg string) *AppError {
	return &AppError{
		Code:    code,
		Message: msg,
	}
}

// Newf 创建格式化的应用错误
func Newf(code ErrorCode, format string, args ...any) *AppError {
	return &AppError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}
