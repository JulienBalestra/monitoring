package main

import (
	"context"

	"github.com/JulienBalestra/dry/pkg/exit"
	root "github.com/JulienBalestra/monitoring/cmd"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	rootCommand := root.NewRootCommand(ctx)
	err := rootCommand.Execute()
	cancel()
	exit.Exit(err)
}
