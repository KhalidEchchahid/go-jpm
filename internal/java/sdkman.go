package java

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// SDKManInstaller handles SDKMAN JDK installation on Linux/macOS.
type SDKManInstaller struct {
	version string
	verbose bool
}

// NewSDKManInstaller creates a new SDKMAN installer.
func NewSDKManInstaller(version string, verbose bool) *SDKManInstaller {
	return &SDKManInstaller{
		version: version,
		verbose: verbose,
	}
}

// IsAvailable checks if SDKMAN is installed or available to install.
func (s *SDKManInstaller) IsAvailable() bool {
	// SDKMAN requires bash
	_, err := exec.LookPath("bash")
	return err == nil
}

// IsPlatformSupported checks if the current OS supports SDKMAN.
func (s *SDKManInstaller) IsPlatformSupported() bool {
	return runtime.GOOS == "linux" || runtime.GOOS == "darwin"
}

// IsInstalled checks if SDKMAN is already installed.
func (s *SDKManInstaller) IsInstalled() bool {
	sdkmanDir := os.ExpandEnv("$HOME/.sdkman")
	_, err := os.Stat(sdkmanDir)
	return err == nil
}

// PromptInstall asks the user if they want to install Java via SDKMAN.
// Returns (proceed bool, installSDKMan bool).
func (s *SDKManInstaller) PromptInstall() (bool, bool) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n• Java not found on PATH.")
	fmt.Println("\nWould you like JPM to help install JDK " + s.version + "?")
	fmt.Println("  1) Yes, using SDKMAN (macOS/Linux)")
	fmt.Println("  2) Yes, using Homebrew (macOS only)")
	fmt.Println("  3) I'll install manually and re-run JPM")
	fmt.Println("  4) Skip for now")
	fmt.Print("> ")

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	switch choice {
	case "1":
		if !s.IsPlatformSupported() {
			fmt.Println("! SDKMAN is only available on macOS and Linux")
			return false, false
		}
		return true, true // Proceed with SDKMAN
	case "2":
		return true, false // Proceed without SDKMAN (user will use Homebrew)
	case "3", "4":
		return false, false // Skip
	default:
		return false, false
	}
}

// Install attempts to install Java via SDKMAN.
// Returns (success bool, error string).
func (s *SDKManInstaller) Install() (bool, string) {
	if !s.IsPlatformSupported() {
		return false, "SDKMAN is not available on this platform"
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return false, fmt.Sprintf("failed to get home directory: %v", err)
	}

	// Step 1: Install SDKMAN if not present
	if !s.IsInstalled() {
		fmt.Println("\nInstalling SDKMAN...")
		cmd := exec.Command("bash", "-c", "curl -s https://get.sdkman.io | bash")
		if s.verbose {
			fmt.Printf("Running: %v\n", cmd)
		}
		if err := cmd.Run(); err != nil {
			return false, fmt.Sprintf("SDKMAN installation failed: %v", err)
		}
		fmt.Println("✔ SDKMAN installed")
	}

	// Step 2: Install Java via SDKMAN
	fmt.Printf("\nInstalling Java %s via SDKMAN...\n", s.version)
	sdkmanScript := fmt.Sprintf(`
source %s/.sdkman/bin/sdkman-init.sh
sdk install java %s -y
`, home, s.sdkmanJavaVersion())

	cmd := exec.Command("bash", "-c", sdkmanScript)
	if s.verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return false, fmt.Sprintf("Java installation failed: %v", err)
	}

	fmt.Println("✔ Java " + s.version + " installed")

	// Step 3: Verify installation
	result := NewDetector(s.verbose).Detect()
	if !result.Found {
		return false, "Java installation succeeded but verification failed"
	}

	return true, ""
}

// sdkmanJavaVersion returns the SDKMAN identifier for the requested Java version.
// Examples: "21.0.1-tem" for Temurin, "17.0.8-tem", etc.
func (s *SDKManInstaller) sdkmanJavaVersion() string {
	// Use Temurin (Eclipse) distribution for stability
	// Latest patches can be found at: https://api.sdkman.io/2/candidates/java/versions
	versions := map[string]string{
		"17": "17.0.8-tem",
		"21": "21.0.1-tem",
		"23": "23.0.1-tem",
	}

	if ver, ok := versions[s.version]; ok {
		return ver
	}

	// Fallback: use version as-is
	return s.version + "-tem"
}

// GetInstallationGuide returns a human-readable guide for manual installation.
func (s *SDKManInstaller) GetInstallationGuide() string {
	return fmt.Sprintf(`
To install Java %s manually:

Option 1: SDKMAN (Linux/macOS)
  curl -s https://get.sdkman.io | bash
  source "$HOME/.sdkman/bin/sdkman-init.sh"
  sdk install java 21.0.1-tem

Option 2: Homebrew (macOS)
  brew install temurin@%s
  # or
  brew install openjdk@%s

Option 3: Direct download
  Visit: https://adoptium.net/
  Download JDK %s for your platform
  Extract and set JAVA_HOME=/path/to/jdk

After installation, run:
  export JAVA_HOME=/path/to/java/home
  jpm init
`, s.version, s.version, s.version, s.version)
}
