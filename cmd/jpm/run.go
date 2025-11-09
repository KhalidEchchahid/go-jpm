package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run [path]",
	Short: "Run the project (JPM engine)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root := "."
		if len(args) > 0 {
			root = args[0]
		}
		abs, err := filepath.Abs(root)
		if err != nil {
			return err
		}
		m, _, err := core.LoadManifest(abs)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("no jpm.yaml found in %s (run 'jpm init')", abs)
			}
			return err
		}
		// Determine jar path
		artifactID := m.Project.ArtifactID
		if artifactID == "" { artifactID = "app" }
		version := m.Project.Version
		if version == "" { version = "0.1.0-SNAPSHOT" }
		jar := filepath.Join(abs, ".jpm", "out", fmt.Sprintf("%s-%s.jar", artifactID, version))
		if _, statErr := os.Stat(jar); statErr != nil {
			// Try building first
			if buildErr := buildCmd.RunE(buildCmd, []string{abs}); buildErr != nil {
				return fmt.Errorf("build failed: %v", buildErr)
			}
		}
		// Ensure main class
		mainClass := strings.TrimSpace(m.App.MainClass)
		if mainClass == "" {
			reader := bufio.NewReader(os.Stdin)
			mainClass = prompt(reader, "Main class (e.g., com.example.Main)", "")
			if mainClass == "" {
				return fmt.Errorf("main class is required to run")
			}
			m.App.MainClass = mainClass
			if _, err := core.SaveManifest(abs, m); err != nil {
				return err
			}
		}
		// Check java
		if _, err := exec.LookPath("java"); err != nil {
			return fmt.Errorf("java not found on PATH; install JDK and retry")
		}
		cmdExec := exec.Command("java", "-cp", jar, mainClass)
		cmdExec.Stdout = os.Stdout
		cmdExec.Stderr = os.Stderr
		cmdExec.Stdin = os.Stdin
		return cmdExec.Run()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
