package main

import "github.com/spf13/cobra"

// deps.go contains the "deps" command group. It aggregates dependency-focused
// subcommands such as "jpm deps show" so that future operations (add/remove,
// etc.) have a consistent entry point.
var depsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Dependency operations",
	Long:  "Inspect and manage project dependencies",
}

func init() {
	rootCmd.AddCommand(depsCmd)
}
