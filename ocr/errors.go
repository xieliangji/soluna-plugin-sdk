package ocr

import (
	"errors"
	"fmt"
)

const (
	CodeImageInvalid      = "visual.image_invalid"
	CodeROIInvalid        = "visual.roi_invalid"
	CodeModelUnavailable  = "visual.model_unavailable"
	CodeNativeUnavailable = "visual.native_unavailable"
	CodeRecognizerFailed  = "visual.recognizer_failed"
	CodeResponseInvalid   = "visual.response_invalid"
)

// Error 携带稳定的视觉失败分类，但不暴露外部响应正文、模型数据或图像载荷。
type Error struct {
	Code      string
	Operation string
	Retryable bool
	Cause     error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Operation == "" {
		return e.Code
	}
	return fmt.Sprintf("%s: %s", e.Operation, e.Code)
}

func (e *Error) Unwrap() error { return e.Cause }

// Classified 在公共包边界对视觉失败执行一次包装。
func Classified(code, operation string, retryable bool, cause error) error {
	if cause == nil {
		cause = errors.New(operation)
	}
	return &Error{Code: code, Operation: operation, Retryable: retryable, Cause: cause}
}
