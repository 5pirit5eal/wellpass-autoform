package main

import (
	"bufio"
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	receipts "github.com/wellpass-autoform/receipts-function"
)

func loadEnvFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			if os.Getenv(key) == "" {
				_ = os.Setenv(key, val)
			}
		}
	}
}

func main() {
	envFile := flag.String("env", ".env", "Path to .env file")
	flag.Parse()

	if *envFile != "" {
		loadEnvFile(*envFile)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctx := context.Background()
	if err := funcframework.RegisterCloudEventFunctionContext(ctx, "/", receipts.ProcessReceipt); err != nil {
		log.Fatalf("funcframework.RegisterCloudEventFunctionContext: %v", err)
	}

	log.Printf("Starting receipts-function CloudEvent server on port %s...", port)
	if err := funcframework.Start(port); err != nil {
		log.Fatalf("funcframework.Start: %v", err)
	}
}
