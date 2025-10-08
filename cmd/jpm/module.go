package main

import (
	"github.com/spf13/cobra"
)

// moduleCmd represents the module command
var moduleCmd = &cobra.Command{
	Use:   "module",
	Short: "Module operations",
	Long:  `Perform operations on project modules`,
}

func init() {
	rootCmd.AddCommand(moduleCmd)
}