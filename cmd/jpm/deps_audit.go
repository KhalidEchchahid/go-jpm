package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/KhalidEchchahid/go-jpm/internal/core"
	"github.com/KhalidEchchahid/go-jpm/internal/engine/native"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// deps_audit.go implements "jpm deps audit" to check for security vulnerabilities.

var depsAuditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Check dependencies for security vulnerabilities",
	Long: `Scan project dependencies against the OSV (Open Source Vulnerabilities) database
to identify known security vulnerabilities.

The audit checks both direct and transitive dependencies.`,
	RunE: runDepsAudit,
}

func init() {
	depsCmd.AddCommand(depsAuditCmd)

	depsAuditCmd.Flags().StringP("path", "p", ".", "Project directory")
	depsAuditCmd.Flags().Bool("json", false, "Output in JSON format")
}

// OSVQuery represents a query to the OSV API
type OSVQuery struct {
	Package struct {
		Name      string `json:"name"`
		Ecosystem string `json:"ecosystem"`
	} `json:"package"`
	Version string `json:"version"`
}

// OSVResponse represents the response from OSV API
type OSVResponse struct {
	Vulns []OSVVulnerability `json:"vulns"`
}

// OSVVulnerability represents a single vulnerability
type OSVVulnerability struct {
	ID       string `json:"id"`
	Summary  string `json:"summary"`
	Details  string `json:"details"`
	Severity []struct {
		Type  string `json:"type"`
		Score string `json:"score"`
	} `json:"severity"`
	References []struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"references"`
}

// VulnerabilityReport holds audit results for a dependency
type VulnerabilityReport struct {
	GroupID    string
	ArtifactID string
	Version    string
	Vulns      []OSVVulnerability
	Severity   string // CRITICAL, HIGH, MEDIUM, LOW
}

func runDepsAudit(cmd *cobra.Command, args []string) error {
	projectPath, _ := cmd.Flags().GetString("path")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	projectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return err
	}

	// Load manifest
	manifestPath := filepath.Join(projectPath, "jpm.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("no jpm.yaml found (run 'jpm init')")
	}

	var manifest core.Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse jpm.yaml: %w", err)
	}

	if len(manifest.Dependencies) == 0 {
		fmt.Println("  no dependencies to audit")
		return nil
	}

	fmt.Println(headerStyle("→ auditing dependencies:"))

	// Resolve all dependencies including transitives
	cacheDir := filepath.Join(projectPath, ".jpm", "cache")
	cache := native.NewCache(cacheDir)
	downloader := native.NewDownloader("", cache)
	resolver := native.NewResolver(downloader, cache)

	coreDeps := make([]core.Dependency, len(manifest.Dependencies))
	for i, d := range manifest.Dependencies {
		coreDeps[i] = core.Dependency{
			GroupID:    d.GroupID,
			ArtifactID: d.ArtifactID,
			Version:    d.Version,
			Scope:      d.Scope,
		}
	}

	resolved, err := resolver.Resolve(coreDeps)
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	fmt.Printf("  scanning %d packages against OSV database...\n\n", len(resolved))

	// Check each dependency
	reports := make([]VulnerabilityReport, 0)
	criticalCount := 0
	highCount := 0
	mediumCount := 0
	lowCount := 0

	for _, dep := range resolved {
		vulns, err := checkVulnerabilities(dep.Artifact.GroupID, dep.Artifact.ArtifactID, dep.Artifact.Version)
		if err != nil {
			// Continue on error, just skip this dependency
			continue
		}

		if len(vulns) > 0 {
			severity := determineSeverity(vulns)
			report := VulnerabilityReport{
				GroupID:    dep.Artifact.GroupID,
				ArtifactID: dep.Artifact.ArtifactID,
				Version:    dep.Artifact.Version,
				Vulns:      vulns,
				Severity:   severity,
			}
			reports = append(reports, report)

			switch severity {
			case "CRITICAL":
				criticalCount++
			case "HIGH":
				highCount++
			case "MEDIUM":
				mediumCount++
			case "LOW":
				lowCount++
			}
		}
	}

	if jsonOutput {
		output := map[string]interface{}{
			"total":    len(resolved),
			"critical": criticalCount,
			"high":     highCount,
			"medium":   mediumCount,
			"low":      lowCount,
			"reports":  reports,
		}
		jsonBytes, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(jsonBytes))
		return nil
	}

	// Human-readable output
	if len(reports) == 0 {
		fmt.Println("  " + "\033[32m✓ no vulnerabilities found\033[0m")
		fmt.Printf("\n  scanned %d packages\n", len(resolved))
		return nil
	}

	for _, report := range reports {
		coord := fmt.Sprintf("%s:%s:%s", report.GroupID, report.ArtifactID, report.Version)

		var severityColor string
		switch report.Severity {
		case "CRITICAL":
			severityColor = "\033[1;31m" // Bold red
		case "HIGH":
			severityColor = "\033[31m" // Red
		case "MEDIUM":
			severityColor = "\033[33m" // Yellow
		case "LOW":
			severityColor = "\033[36m" // Cyan
		}

		fmt.Printf("  %s%s%s: %s\n", severityColor, report.Severity, "\033[0m", coord)

		for _, vuln := range report.Vulns {
			summary := vuln.Summary
			if len(summary) > 60 {
				summary = summary[:57] + "..."
			}
			fmt.Printf("    • %s: %s\n", vuln.ID, summary)
		}
		fmt.Println()
	}

	// Summary
	fmt.Println("  " + headerStyle("audit summary:"))
	if criticalCount > 0 {
		fmt.Printf("    \033[1;31m%d critical\033[0m\n", criticalCount)
	}
	if highCount > 0 {
		fmt.Printf("    \033[31m%d high\033[0m\n", highCount)
	}
	if mediumCount > 0 {
		fmt.Printf("    \033[33m%d medium\033[0m\n", mediumCount)
	}
	if lowCount > 0 {
		fmt.Printf("    \033[36m%d low\033[0m\n", lowCount)
	}

	if criticalCount > 0 || highCount > 0 {
		return fmt.Errorf("found %d critical and %d high severity vulnerabilities", criticalCount, highCount)
	}

	return nil
}

func checkVulnerabilities(groupID, artifactID, version string) ([]OSVVulnerability, error) {
	// Query OSV API
	query := struct {
		Package struct {
			Name      string `json:"name"`
			Ecosystem string `json:"ecosystem"`
		} `json:"package"`
		Version string `json:"version"`
	}{
		Version: version,
	}
	query.Package.Name = fmt.Sprintf("%s:%s", groupID, artifactID)
	query.Package.Ecosystem = "Maven"

	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(
		"https://api.osv.dev/v1/query",
		"application/json",
		strings.NewReader(string(queryBytes)),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OSV API returned status %d", resp.StatusCode)
	}

	var osvResp OSVResponse
	if err := json.NewDecoder(resp.Body).Decode(&osvResp); err != nil {
		return nil, err
	}

	return osvResp.Vulns, nil
}

func determineSeverity(vulns []OSVVulnerability) string {
	highestSeverity := "LOW"

	for _, vuln := range vulns {
		for _, sev := range vuln.Severity {
			if sev.Type == "CVSS_V3" {
				score := sev.Score
				if strings.HasPrefix(score, "9") || score == "10" {
					return "CRITICAL"
				} else if strings.HasPrefix(score, "7") || strings.HasPrefix(score, "8") {
					if highestSeverity != "CRITICAL" {
						highestSeverity = "HIGH"
					}
				} else if strings.HasPrefix(score, "4") || strings.HasPrefix(score, "5") || strings.HasPrefix(score, "6") {
					if highestSeverity != "CRITICAL" && highestSeverity != "HIGH" {
						highestSeverity = "MEDIUM"
					}
				}
			}
		}

		// Also check by ID prefix for common patterns
		if strings.Contains(vuln.ID, "CVE") {
			if strings.Contains(strings.ToLower(vuln.Summary), "remote code execution") ||
				strings.Contains(strings.ToLower(vuln.Summary), "rce") {
				return "CRITICAL"
			}
		}
	}

	return highestSeverity
}
