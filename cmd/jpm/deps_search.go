package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

// deps_search.go implements "jpm deps search" to search Maven Central for packages.

var depsSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search Maven Central for packages",
	Long:  `Search Maven Central repository for packages matching the query.`,
	Example: `  jpm deps search guava
  jpm deps search "spring boot"
  jpm deps search org.apache.commons`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDepsSearch,
}

func init() {
	depsCmd.AddCommand(depsSearchCmd)

	depsSearchCmd.Flags().IntP("limit", "n", 10, "Maximum number of results to show")
	depsSearchCmd.Flags().Bool("json", false, "Output in JSON format")
}

// MavenSearchResponse represents the Maven Central search API response
type MavenSearchResponse struct {
	Response struct {
		NumFound int `json:"numFound"`
		Docs     []struct {
			GroupID       string   `json:"g"`
			ArtifactID    string   `json:"a"`
			LatestVersion string   `json:"latestVersion"`
			Description   string   `json:"p"` // packaging type
			Timestamp     int64    `json:"timestamp"`
			VersionCount  int      `json:"versionCount"`
			Tags          []string `json:"tags"`
		} `json:"docs"`
	} `json:"response"`
}

func runDepsSearch(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")
	limit, _ := cmd.Flags().GetInt("limit")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	fmt.Printf(headerStyle("→ searching Maven Central for '%s':\n"), query)

	// Build search URL
	searchURL := fmt.Sprintf(
		"https://search.maven.org/solrsearch/select?q=%s&rows=%d&wt=json",
		url.QueryEscape(query),
		limit,
	)

	// Make request
	resp, err := http.Get(searchURL)
	if err != nil {
		return fmt.Errorf("failed to search Maven Central: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Maven Central returned status %d", resp.StatusCode)
	}

	var searchResp MavenSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if searchResp.Response.NumFound == 0 {
		fmt.Println("  no packages found")
		return nil
	}

	if jsonOutput {
		// JSON output for scripting
		output := make([]map[string]interface{}, 0)
		for _, doc := range searchResp.Response.Docs {
			output = append(output, map[string]interface{}{
				"groupId":       doc.GroupID,
				"artifactId":    doc.ArtifactID,
				"latestVersion": doc.LatestVersion,
				"versionCount":  doc.VersionCount,
			})
		}
		jsonBytes, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(jsonBytes))
		return nil
	}

	// Human-readable output
	fmt.Println()
	for i, doc := range searchResp.Response.Docs {
		coord := fmt.Sprintf("%s:%s", doc.GroupID, doc.ArtifactID)

		// Styling
		fmt.Printf("  %s%s%s\n", "\033[1;36m", coord, "\033[0m")
		fmt.Printf("    Latest: %s", doc.LatestVersion)
		if doc.VersionCount > 1 {
			fmt.Printf(" (%d versions)", doc.VersionCount)
		}
		fmt.Println()

		if i < len(searchResp.Response.Docs)-1 {
			fmt.Println()
		}
	}

	fmt.Printf("\n  found %d packages", searchResp.Response.NumFound)
	if searchResp.Response.NumFound > limit {
		fmt.Printf(" (showing top %d)", limit)
	}
	fmt.Println()

	fmt.Println()
	fmt.Println("  use: jpm deps add <group:artifact>")

	return nil
}
