package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		// Robot I/O contract (bt-ah53): stderr lines start with "Error:" so
		// agents can distinguish them from leaked log/info output.
		msg := err.Error()
		if !strings.HasPrefix(msg, "Error:") {
			msg = "Error: " + msg
		}
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}
