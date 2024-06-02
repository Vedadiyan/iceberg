//go:build debug

package main

import (
	"log"
	"os"

	"github.com/vedadiyan/iceberg/internal/logger"
)

type (
	DebugLogger struct{}
)

func init() {
	os.Setenv("default_nats", "nats://127.0.0.1:4222")
	config, err := os.ReadFile("./../sample.yml")
	if err != nil {
		log.Fatalln(err)
	}
	os.Setenv("ICEBERG_CONFIG", string(config))
	logger.AddLogger(new(DebugLogger))
}

func (*DebugLogger) Info(message string, params ...any) {
	payload := []any{Color("[INFO]", CYAN), message}
	payload = append(payload, params...)
	log.Println(payload...)
}

func (*DebugLogger) Warning(message string, params ...any) {
	payload := []any{Color("[WARNING]", YELLOW), message}
	payload = append(payload, params...)
	log.Println(payload...)
}

func (*DebugLogger) Error(err error, message string, params ...any) {}
