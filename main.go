package main

import (
	"fmt"
	"os"

	"github.com/atuleu/angular-to-http/src/app"
)

func main() {
	if err := app.Execute(); err != nil {
		fmt.Printf("%s", err)
		os.Exit(1)
	}
}
