package errors

type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *AppError) Error() string {
	return e.Message
}

var (
	ErrInvalidParams     = &AppError{Code: 400, Message: "参数错误"}
	ErrUnauthorized      = &AppError{Code: 401, Message: "未授权"}
	ErrForbidden         = &AppError{Code: 403, Message: "无权限"}
	ErrNotFound          = &AppError{Code: 404, Message: "资源不存在"}
	ErrQuotaExceeded     = &AppError{Code: 429, Message: "今日额度已用完"}
	ErrRateLimitExceeded = &AppError{Code: 429, Message: "请求过于频繁，请稍后再试"}
	ErrPlatformNotSupport = &AppError{Code: 400, Message: "暂不支持该平台"}
	ErrParseFailed       = &AppError{Code: 500, Message: "解析失败，请检查链接是否正确"}
	ErrDownloadFailed    = &AppError{Code: 500, Message: "下载失败"}
	ErrInternalServer    = &AppError{Code: 500, Message: "服务器内部错误"}
	ErrUserExists        = &AppError{Code: 409, Message: "用户已存在"}
	ErrInvalidToken      = &AppError{Code: 401, Message: "无效的Token"}
	ErrTokenExpired      = &AppError{Code: 401, Message: "Token已过期"}
)

func New(code int, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}
