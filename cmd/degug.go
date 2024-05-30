//go:build debug

package main

import (
	"log"
	"os"
)

func init() {
	config, err := os.ReadFile("sample.yml")
	if err != nil {
		log.Fatalln(err)
	}
	os.Setenv("ICEBERG_CONFIG", string(config))
}
