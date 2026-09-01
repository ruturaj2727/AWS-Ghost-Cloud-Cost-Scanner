package main

import (
	"fmt"
	"os"

	"https://github.com/ruturaj2727/AWS-Ghost-Cloud-Cost-Scanner/tree/main/cmd/aws-ghost/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
