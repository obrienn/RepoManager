package main

import (
	"log"
	"path/filepath"

	"repomanager/internal/config"
	"repomanager/internal/server"
)

func main() {
	cfg, err := config.Load(filepath.Join(".", ".env"))
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := server.Run(cfg); err != nil {
		log.Fatalf("server: %v", err)
	}
}
