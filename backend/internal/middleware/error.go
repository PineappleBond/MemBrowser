package middleware

import (
	"net/http"

	"github.com/PineappleBond/MemBrowser/backend/pkg/errors"
	"github.com/PineappleBond/MemBrowser/backend/pkg/response"
	"github.com/gin-gonic/gin"
)

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			if appErr, ok := err.(*errors.AppError); ok {
				httpCode := httpStatusCodeForErrorCode(appErr.Code)
				c.JSON(httpCode, response.Fail(appErr))
				return
			}
			c.JSON(http.StatusInternalServerError, response.Fail(
				errors.New(errors.ErrCodeInternal, err.Error()),
			))
		}
	}
}

func httpStatusCodeForErrorCode(code errors.ErrorCode) int {
	switch code {
	case errors.ErrCodeUnauthorized:
		return http.StatusUnauthorized
	case errors.ErrCodeNotFound:
		return http.StatusNotFound
	case errors.ErrCodeBadRequest:
		return http.StatusBadRequest
	case errors.ErrCodeConflict:
		return http.StatusConflict
	case errors.ErrCodeTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusOK
	}
}
