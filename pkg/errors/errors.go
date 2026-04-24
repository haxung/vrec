package errors

import (
	"fmt"
)

// 错误码枚举
type ErrorCode int

const (
	// 通用错误 (1xxx)
	ErrCodeInternalError ErrorCode = 1001

	// 参数错误 (1xxx)
	ErrCodeInvalidParams   ErrorCode = 1002
	ErrCodeAudioTooLarge   ErrorCode = 1003
	ErrCodeAudioTooLong    ErrorCode = 1004
	ErrCodeRateLimit       ErrorCode = 1006

	// 认证错误 (2xxx)
	ErrCodeAuthMissingHeader ErrorCode = 2001
	ErrCodeAuthInvalidToken  ErrorCode = 2002
	ErrCodeAuthTokenExpired  ErrorCode = 2003
	ErrCodeAuthInvalidUser   ErrorCode = 2004

	// 用户错误 (3xxx)
	ErrCodeUserNotFound      ErrorCode = 3001
	ErrCodeUserAlreadyExists ErrorCode = 3002
	ErrCodeInvalidPassword   ErrorCode = 3003

	// 余额错误 (4xxx)
	ErrCodeInsufficientBalance ErrorCode = 4001

	// 订单/业务错误 (5xxx)
	ErrCodeOrderNotFound       ErrorCode = 5001
	ErrCodeOrderStatusInvalid  ErrorCode = 5002
	ErrCodeResultNotFound     ErrorCode = 5003
	ErrCodeSubtitleNotFound   ErrorCode = 5004
	ErrCodeMeetingNoteNotFound ErrorCode = 5005
	ErrCodeLLMNotEnabled      ErrorCode = 5006

	// 充值错误 (6xxx)
	ErrCodeRechargeNotFound    ErrorCode = 6001
	ErrCodeRechargeExpired     ErrorCode = 6002
	ErrCodeRechargeAlreadyPaid ErrorCode = 6003
	ErrCodePayChannelInvalid   ErrorCode = 6004
)

func (c ErrorCode) String() string {
	switch c {
	case ErrCodeInternalError:
		return "internal error"
	case ErrCodeInvalidParams:
		return "invalid parameters"
	case ErrCodeAudioTooLarge:
		return "audio file exceeds 1GB limit"
	case ErrCodeAudioTooLong:
		return "audio duration exceeds 6 hours limit"
	case ErrCodeRateLimit:
		return "rate limit exceeded"
	case ErrCodeAuthMissingHeader:
		return "missing authorization header"
	case ErrCodeAuthInvalidToken:
		return "invalid token"
	case ErrCodeAuthTokenExpired:
		return "token expired"
	case ErrCodeAuthInvalidUser:
		return "invalid username or password"
	case ErrCodeUserNotFound:
		return "user not found"
	case ErrCodeUserAlreadyExists:
		return "user already exists"
	case ErrCodeInvalidPassword:
		return "invalid password"
	case ErrCodeInsufficientBalance:
		return "insufficient balance"
	case ErrCodeOrderNotFound:
		return "order not found"
	case ErrCodeOrderStatusInvalid:
		return "invalid order status"
	case ErrCodeResultNotFound:
		return "result not found"
	case ErrCodeSubtitleNotFound:
		return "subtitle not found"
	case ErrCodeMeetingNoteNotFound:
		return "meeting note not found"
	case ErrCodeLLMNotEnabled:
		return "llm not enabled"
	case ErrCodeRechargeNotFound:
		return "recharge order not found"
	case ErrCodeRechargeExpired:
		return "recharge order expired"
	case ErrCodeRechargeAlreadyPaid:
		return "recharge already paid"
	case ErrCodePayChannelInvalid:
		return "invalid pay channel"
	default:
		return fmt.Sprintf("unknown error code: %d", c)
	}
}

// 错误定义
type AppError struct {
	Code ErrorCode `json:"code"`
	Msg  string    `json:"msg"`
	Err  error     `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Err)
	}
	return e.Msg
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// 预定义错误实例
var (
	ErrInternalError       = &AppError{Code: ErrCodeInternalError, Msg: ErrCodeInternalError.String()}
	ErrInvalidParams       = &AppError{Code: ErrCodeInvalidParams, Msg: ErrCodeInvalidParams.String()}
	ErrAudioTooLarge       = &AppError{Code: ErrCodeAudioTooLarge, Msg: ErrCodeAudioTooLarge.String()}
	ErrAudioTooLong        = &AppError{Code: ErrCodeAudioTooLong, Msg: ErrCodeAudioTooLong.String()}
	ErrRateLimit           = &AppError{Code: ErrCodeRateLimit, Msg: ErrCodeRateLimit.String()}

	ErrAuthMissingHeader   = &AppError{Code: ErrCodeAuthMissingHeader, Msg: ErrCodeAuthMissingHeader.String()}
	ErrAuthInvalidToken    = &AppError{Code: ErrCodeAuthInvalidToken, Msg: ErrCodeAuthInvalidToken.String()}
	ErrAuthTokenExpired    = &AppError{Code: ErrCodeAuthTokenExpired, Msg: ErrCodeAuthTokenExpired.String()}
	ErrAuthInvalidUser     = &AppError{Code: ErrCodeAuthInvalidUser, Msg: ErrCodeAuthInvalidUser.String()}

	ErrUserNotFound        = &AppError{Code: ErrCodeUserNotFound, Msg: ErrCodeUserNotFound.String()}
	ErrUserAlreadyExists    = &AppError{Code: ErrCodeUserAlreadyExists, Msg: ErrCodeUserAlreadyExists.String()}
	ErrInvalidPassword     = &AppError{Code: ErrCodeInvalidPassword, Msg: ErrCodeInvalidPassword.String()}

	ErrInsufficientBalance = &AppError{Code: ErrCodeInsufficientBalance, Msg: ErrCodeInsufficientBalance.String()}

	ErrOrderNotFound       = &AppError{Code: ErrCodeOrderNotFound, Msg: ErrCodeOrderNotFound.String()}
	ErrOrderStatusInvalid  = &AppError{Code: ErrCodeOrderStatusInvalid, Msg: ErrCodeOrderStatusInvalid.String()}
	ErrResultNotFound     = &AppError{Code: ErrCodeResultNotFound, Msg: ErrCodeResultNotFound.String()}
	ErrSubtitleNotFound    = &AppError{Code: ErrCodeSubtitleNotFound, Msg: ErrCodeSubtitleNotFound.String()}
	ErrMeetingNoteNotFound = &AppError{Code: ErrCodeMeetingNoteNotFound, Msg: ErrCodeMeetingNoteNotFound.String()}
	ErrLLMNotEnabled       = &AppError{Code: ErrCodeLLMNotEnabled, Msg: ErrCodeLLMNotEnabled.String()}

	ErrRechargeNotFound    = &AppError{Code: ErrCodeRechargeNotFound, Msg: ErrCodeRechargeNotFound.String()}
	ErrRechargeExpired     = &AppError{Code: ErrCodeRechargeExpired, Msg: ErrCodeRechargeExpired.String()}
	ErrRechargeAlreadyPaid = &AppError{Code: ErrCodeRechargeAlreadyPaid, Msg: ErrCodeRechargeAlreadyPaid.String()}
	ErrPayChannelInvalid   = &AppError{Code: ErrCodePayChannelInvalid, Msg: ErrCodePayChannelInvalid.String()}
)

// New 创建新错误
func New(code ErrorCode, msg string) *AppError {
	return &AppError{Code: code, Msg: msg}
}

// Wrap 包装错误
func Wrap(err error, code ErrorCode, msg string) *AppError {
	return &AppError{Code: code, Msg: msg, Err: err}
}

// Is 判断错误是否相同
func Is(err, target error) bool {
	if e, ok := err.(*AppError); ok {
		if t, ok := target.(*AppError); ok {
			return e.Code == t.Code
		}
	}
	return err == target
}
