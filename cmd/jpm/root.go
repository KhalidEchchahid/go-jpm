package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version information (set at build time via ldflags)
var (
	Version   = "0.2.0"
	BuildDate = "unknown"
	GitCommit = "unknown"
)

// root.go defines the top-level JPM command and global flags that apply to all
// subcommands. The Cobra root command is intentionally lightweight—its primary
// job is to register shared flags and delegate real work to child commands.

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "jpm",
	Short: "JPM - Java Project Manager",
	Long: `JPM (Java Project Manager) is an open-source CLI tool that simplifies
managing Java projects across Maven and Gradle build systems.

It provides commands to manage dependencies, inspect projects, and
initialize new projects with common frameworks.`,
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of JPM",
	Long:  `Display the version, build date, and git commit of JPM.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("JPM (Java Project Manager)\n")
		fmt.Printf("  Version:    %s\n", Version)
		fmt.Printf("  Build Date: %s\n", BuildDate)
		fmt.Printf("  Git Commit: %s\n", GitCommit)
	},
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

	// Add version command
	rootCmd.AddCommand(versionCmd)

	// Also support --version flag on root
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate("JPM version {{.Version}}\n")
}
