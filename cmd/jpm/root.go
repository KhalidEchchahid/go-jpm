package main

import (
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "jpm",
	Short: "JPM - Java Project Manager",
	Long: `JPM (Java Project Manager) is an open-source CLI tool that simplifies
managing Java projects across Maven and Gradle build systems.

It provides commands to manage dependencies, inspect projects, and
initialize new projects with common frameworks.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Show verbose output")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug mode")
}