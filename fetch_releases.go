package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type Release struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
}

func main() {
	owner := "railwayapp"
	repo := "railpack"
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", owner, repo)

	var allReleases []Release

	for url != "" {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			fmt.Printf("Error creating request: %v\n", err)
			os.Exit(1)
		}

		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "Go-GitHub-Release-Fetcher")
		
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Error making request: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("Error: API returned status %d. Body: %s\n", resp.StatusCode, string(body))
			os.Exit(1)
		}

		var releases []Release
		if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
			fmt.Printf("Error decoding JSON: %v\n", err)
			os.Exit(1)
		}

		allReleases = append(allReleases, releases...)

		url = getNextLink(resp.Header.Get("Link"))
	}

	fmt.Printf("Found %d releases for %s/%s:\n", len(allReleases), owner, repo)
	for _, release := range allReleases {
		fmt.Printf("- %s (%s)\n", release.TagName, release.Name)
	}
}

func getNextLink(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}
	
	links := strings.Split(linkHeader, ",")
	for _, link := range links {
		if strings.Contains(link, `rel="next"`) {
			start := strings.Index(link, "<")
			end := strings.Index(link, ">")
			if start != -1 && end != -1 {
				return link[start+1 : end]
			}
		}
	}
	return ""
}
