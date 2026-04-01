package main

import (
	"github.com/lucasvavon/gtc/cmd"

	// Providers register themselves via init().
	_ "github.com/lucasvavon/gtc/internal/provider/github"
)

func main() {
	cmd.Execute()
}
