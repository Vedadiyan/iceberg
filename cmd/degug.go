//go:build debug

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/vedadiyan/iceberg/internal/logger"
)

type (
	DebugLogger struct{}
)

const (
	RED    = 31
	YELLOW = 93
	CYAN   = 36
)

func init() {
	config, err := os.ReadFile("sample.yml")
	if err != nil {
		log.Fatalln(err)
	}
	os.Setenv("ICEBERG_CONFIG", string(config))
	logger.AddLogger(&DebugLogger{})
}

func Color(message string, color int) string {
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", color, message)
}

func (*DebugLogger) Info(message string, params ...any) {
	payload := []any{Color("INFO", CYAN), message}
	payload = append(payload, params...)
	log.Println(payload...)
}

func (*DebugLogger) Warning(message string, params ...any) {
	payload := []any{Color("WARNING", YELLOW), message}
	payload = append(payload, params...)
	log.Println(payload...)
}

func (*DebugLogger) Error(err error, message string, params ...any) {
	payload := []any{Color("ERROR", RED), err, message}
	payload = append(payload, params...)
	log.Println(payload...)
}
