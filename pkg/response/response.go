package response

import (
	"net/http"

	"vrec/pkg/errors"

	"github.com/gin-gonic/gin"
)

const sidContextKey = "sid"

type Response struct {
	Code errors.ErrorCode `json:"code"`
	Msg  string           `json:"msg"`
	Sid  string           `json:"sid,omitempty"`
	Data any              `json:"data,omitempty"`
}

func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Code: 0,
		Msg:  "success",
		Sid:  c.GetString(sidContextKey),
		Data: data,
	})
}

func Error(c *gin.Context, code errors.ErrorCode, msg string) {
	c.JSON(http.StatusOK, Response{
		Code: code,
		Msg:  msg,
		Sid:  c.GetString(sidContextKey),
	})
}

func ErrorWithStatus(c *gin.Context, httpStatus int, code errors.ErrorCode, msg string) {
	c.JSON(httpStatus, Response{
		Code: code,
		Msg:  msg,
		Sid:  c.GetString(sidContextKey),
	})
}

func ErrorWithData(c *gin.Context, code errors.ErrorCode, msg string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: code,
		Msg:  msg,
		Sid:  c.GetString(sidContextKey),
		Data: data,
	})
}

func GetHTTPStatus(code errors.ErrorCode) int {
	switch code {
	case errors.ErrCodeInternalError:
		return http.StatusInternalServerError
	case errors.ErrCodeAuthMissingHeader, errors.ErrCodeAuthInvalidToken, errors.ErrCodeAuthTokenExpired, errors.ErrCodeAuthInvalidUser:
		return http.StatusUnauthorized
	case errors.ErrCodeUserNotFound, errors.ErrCodeOrderNotFound, errors.ErrCodeRechargeNotFound, errors.ErrCodeResultNotFound:
		return http.StatusNotFound
	case errors.ErrCodeInsufficientBalance:
		return http.StatusPaymentRequired
	case errors.ErrCodeRateLimit:
		return http.StatusTooManyRequests
	default:
		return http.StatusOK
	}
}
