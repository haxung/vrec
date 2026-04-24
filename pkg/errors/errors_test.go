package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppError_Error(t *testing.T) {
	e := &AppError{Code: ErrCodeInternalError, Msg: "test error"}
	assert.Equal(t, "test error", e.Error())
}

func TestAppError_ErrorWithUnderlyingError(t *testing.T) {
	underlying := errors.New("underlying error")
	e := &AppError{Code: ErrCodeInternalError, Msg: "test error", Err: underlying}
	assert.Contains(t, e.Error(), "test error")
	assert.Contains(t, e.Error(), "underlying error")
}

func TestIs_SameCode(t *testing.T) {
	err1 := &AppError{Code: ErrCodeInternalError, Msg: "error1"}
	err2 := &AppError{Code: ErrCodeInternalError, Msg: "error1"}

	assert.True(t, Is(err1, err2))
}

func TestIs_DifferentCode(t *testing.T) {
	err1 := &AppError{Code: ErrCodeInternalError, Msg: "error1"}
	err3 := &AppError{Code: ErrCodeRechargeNotFound, Msg: "error2"}

	assert.False(t, Is(err1, err3))
}

func TestIs_AppErrorToErrInternalError(t *testing.T) {
	// err1 has same code (ErrCodeInternalError) as ErrInternalError
	err1 := &AppError{Code: ErrCodeInternalError, Msg: "error1"}

	// Should be true because same code
	assert.True(t, Is(err1, ErrInternalError))
}

func TestIs_NilComparison(t *testing.T) {
	// Standard error comparison should return false
	assert.False(t, Is(ErrInternalError, errors.New("other")))
}

func TestWrap(t *testing.T) {
	original := errors.New("original error")
	wrapped := Wrap(original, ErrCodeRechargeNotFound, "wrapped error")

	assert.Equal(t, ErrCodeRechargeNotFound, wrapped.Code)
	assert.Equal(t, "wrapped error", wrapped.Msg)
	assert.Equal(t, original, wrapped.Unwrap())
}

func TestNew(t *testing.T) {
	e := New(ErrCodeOrderNotFound, "order not found")

	assert.Equal(t, ErrCodeOrderNotFound, e.Code)
	assert.Equal(t, "order not found", e.Msg)
	assert.Nil(t, e.Err)
}

func TestErrorCode_String(t *testing.T) {
	tests := []struct {
		code ErrorCode
		want string
	}{
		{ErrCodeInternalError, "internal error"},
		{ErrCodeInvalidParams, "invalid parameters"},
		{ErrCodeUserNotFound, "user not found"},
		{ErrorCode(9999), "unknown error code: 9999"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.code.String())
		})
	}
}
