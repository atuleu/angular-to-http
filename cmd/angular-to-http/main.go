package main

import (
	"fmt"
	"os"

	"github.com/atuleu/angular-to-http/internal/ath"
)

func main() {
	if err := ath.Execute(); err != nil {
		fmt.Printf("%s", err)
		os.Exit(1)
	}
}
