package main

import (
	"os"

	"github.com/pandalee99/Memory-Sync/go/memorysync"
)

func main() {
	os.Exit(memorysync.RunCLI(os.Args[1:], os.Stdout, os.Stderr))
}
