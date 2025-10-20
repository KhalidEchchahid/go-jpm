package main

import (
	"fmt"

	"github.com/fatih/color"
)

var (
	headerStyle      = color.New(color.FgGreen, color.Bold).SprintFunc()
	subduedStyle     = color.New(color.FgHiBlack).SprintFunc()
	indexStyle       = color.New(color.FgCyan, color.Bold).SprintFunc()
	primaryTextStyle = color.New(color.FgHiWhite).SprintFunc()
	warningIconStyle = color.New(color.FgHiYellow, color.Bold).SprintFunc()
	warningTextStyle = color.New(color.FgYellow).SprintFunc()
)

// formatIndex wraps the numerical index in a consistent color treatment so
// ordered lists across commands share the same look and feel.
func formatIndex(i int) string {
	return indexStyle(fmt.Sprintf("%d)", i))
}
