package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kelos-dev/kelos/internal/capture"
	"github.com/kelos-dev/kelos/internal/observability"
)

func main() {
	os.Exit(run())
}

func run() int {
	shutdown, err := observability.Init(context.Background(), "cody-runtime")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: OpenTelemetry initialization failed: %v\n", err)
		return capture.Run()
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "warning: OpenTelemetry shutdown failed: %v\n", err)
		}
	}()

	return capture.Run()
}
