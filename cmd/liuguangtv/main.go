package main

import (
	"os"

	"github.com/tangqiu0205/LiuguangTV/internal/app"
)

func main() {
	if err := app.Run(version); err != nil {
		reportStartupError(err)
		os.Exit(1)
	}
}
