package response

import "github.com/PineappleBond/MemBrowser/backend/pkg/errors"

// Response 统一响应信封
type Response struct {
	Success   bool            `json:"success"`
	ErrorCode errors.ErrorCode `json:"errorCode,omitempty"`
	Error     string          `json:"error,omitempty"`
	Details   string          `json:"details,omitempty"`
	Data      any             `json:"data,omitempty"`
}

// OK 成功响应
func OK(data any) Response {
	return Response{Success: true, Data: data}
}

// Fail 失败响应
func Fail(err *errors.AppError) Response {
	return Response{
		Success:   false,
		ErrorCode: err.Code,
		Error:     err.Message,
		Details:   err.Details,
	}
}
