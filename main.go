// Application entrypoint
package main

import (
	"context"

	"github.com/charmbracelet/fang"
)

func main() {
	rootCmd := rootCmd()
	fang.Execute(context.Background(), rootCmd)
}
