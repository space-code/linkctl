package main

import (
	"os"

	"github.com/space-code/linkctl/internal/cmd"
)

func main() {
	code := cmd.Main()
	os.Exit(int(code))
}
