package main

import (
	"context"
	"fmt"
	"os"

	"github.com/channelwill/learning-os/internal/app"
)

func main() {
	if err := app.Execute(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "learn:", err)
		os.Exit(1)
	}
}
