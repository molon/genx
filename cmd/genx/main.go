package main

import (
	"log"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "genx",
	Short: "GenX CLI",
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Command failed: %v", err)
	}
}
