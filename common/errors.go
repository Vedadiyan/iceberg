package common

import "fmt"

type (
	HandlerErrorClass int
	HandlerError      struct {
		Class      HandlerErrorClass
		StatusCode int
		Message    string
	}
)

const (
	HANDLER_ERROR_INTERNAL HandlerErrorClass = 0
	HANDLER_ERROR_FILTER   HandlerErrorClass = 1
	HANDLER_ERROR_PROXY    HandlerErrorClass = 2
)

func (handlerError HandlerError) Error() string {
	return fmt.Sprintf("%d: %s", handlerError.Class, handlerError.Message)
}

func NewHandlerError(class HandlerErrorClass, statusCode int, message string) error {
	handlerError := HandlerError{
		Class:   class,
		Message: message,
	}
	return handlerError
}
