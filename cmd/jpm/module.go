package main

import (
	"github.com/spf13/cobra"
)

// module.go declares the parent "module" command. It groups module-related
// subcommands so that the CLI can expose operations like "jpm module find" in a
// discoverable hierarchy.

// moduleCmd represents the module command
var moduleCmd = &cobra.Command{
	Use:   "module",
	Short: "Module operations",
	Long:  `Perform operations on project modules`,
}

func init() {
	rootCmd.AddCommand(moduleCmd)
}
