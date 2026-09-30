package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hedykan/learning-system/internal/agent"
	"github.com/hedykan/learning-system/internal/app"
)

func main() {
	root := app.New().RootCommand()
	// The agent package drives the CLI's command tree, so the app package cannot
	// register this command itself; main wires it.
	root.AddCommand(agent.ServeCommand())
	if err := root.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "learn:", err)
		os.Exit(1)
	}
}
