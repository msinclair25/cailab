// Command learningcheck validates the versioned data-only learning catalog.
package main

import (
	"fmt"
	"os"

	"github.com/msinclair25/cailab/internal/learning"
)

func main() {
	root := "."
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./internal/tools/learningcheck [root]")
		os.Exit(2)
	}
	if len(os.Args) == 2 {
		root = os.Args[1]
	}
	if err := learning.ValidateRepository(root); err != nil {
		fmt.Fprintf(os.Stderr, "learningcheck: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("learning catalog checks passed")
}
