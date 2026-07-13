package main

import (
	"fmt"
	"os"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		// Robot I/O contract (bt-ah53): stderr lines start with "Error:" so
		// agents can distinguish them from leaked log/info output. Errors
		// carrying a *RobotError (bt-s5zgk.3) render as a single-line JSON
		// envelope instead of plain prose; see renderCLIError.
		fmt.Fprintln(os.Stderr, renderCLIError(err))
		os.Exit(1)
	}
}
