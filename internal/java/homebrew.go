package java

import (
	"fmt"
)

// HomebrewInstaller provides hints for Homebrew Java installation on macOS.
type HomebrewInstaller struct {
	version string
	verbose bool
}

// NewHomebrewInstaller creates a new Homebrew hint provider.
func NewHomebrewInstaller(version string, verbose bool) *HomebrewInstaller {
	return &HomebrewInstaller{
		version: version,
		verbose: verbose,
	}
}

// GetInstallCommands returns suggested Homebrew install commands.
func (h *HomebrewInstaller) GetInstallCommands() []string {
	return []string{
		fmt.Sprintf("brew install temurin@%s", h.version),
		fmt.Sprintf("brew install openjdk@%s", h.version),
	}
}

// PrintGuide prints installation guide for the user.
func (h *HomebrewInstaller) PrintGuide() {
	fmt.Println("\n• To install Java " + h.version + " on macOS:")
	fmt.Println("\n  Option 1 (Temurin - recommended):")
	fmt.Println("    brew install temurin@" + h.version)
	fmt.Println("\n  Option 2 (OpenJDK):")
	fmt.Println("    brew install openjdk@" + h.version)
	fmt.Println("\n  After installation, set JAVA_HOME:")
	fmt.Println("    export JAVA_HOME=$(/usr/libexec/java_home -v " + h.version + ")")
	fmt.Println("\n  Then run:")
	fmt.Println("    jpm init")
}
