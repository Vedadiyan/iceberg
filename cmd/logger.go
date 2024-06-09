package main

import (
	"fmt"
	"log"

	"github.com/vedadiyan/iceberg/internal/logger"
)

type (
	DefaultLogger struct{}
)

const (
	RED    = 31
	YELLOW = 93
	CYAN   = 36
)

func init() {
	logger.AddLogger(new(DefaultLogger))
}

func Color(message string, color int) string {
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", color, message)
}

func (*DefaultLogger) Info(message string, params ...any) {
	payload := []any{Color("[INFO]", CYAN), message}
	payload = append(payload, params...)
	log.Println(payload...)
}

func (*DefaultLogger) Warning(message string, params ...any) {
	payload := []any{Color("[WARNING]", YELLOW), message}
	payload = append(payload, params...)
	log.Println(payload...)
}

func (*DefaultLogger) Error(err error, message string, params ...any) {
	payload := []any{Color("[ERROR]", RED), err, message}
	payload = append(payload, params...)
	log.Println(payload...)
}
