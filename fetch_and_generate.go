package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"text/template"
)

type Release struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
}

type TemplateData struct {
	Version string
}

const (
	owner      = "railwayapp"
	repo       = "railpack"
	cacheFile  = "railpack_releases.json"
	dockerDir  = "dockerfiles"
	tmplFile   = "dockerfile.tmpl"
	readmeFile = "README.md"
)

func main() {
	singleVersion := flag.String("single-version", "", "特定のバージョンのみ Dockerfile を生成します")
	flag.Parse()

	fmt.Println("GitHub API からリリース一覧を取得中...")
	releases := fetchAllReleases()
	if len(releases) > 0 {
		saveCache(releases)
	} else {
		releases, _ = readCache()
	}

	if *singleVersion != "" {
		tmpl, err := template.ParseFiles(tmplFile)
		if err != nil {
			fmt.Printf("テンプレート読み込み失敗: %v\n", err)
			os.Exit(1)
		}
		os.MkdirAll(dockerDir, 0755)
		generateOne(*singleVersion, tmpl)
	}

	updateReadme(releases)
	fmt.Println("完了")
}

func generateOne(version string, tmpl *template.Template) {
	path := fmt.Sprintf("%s/Dockerfile.%s", dockerDir, version)
	f, err := os.Create(path)
	if err != nil {
		fmt.Printf("ファイル作成失敗: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()
	tmpl.Execute(f, TemplateData{Version: version})
	fmt.Printf("生成: %s\n", path)
}

func updateReadme(releases []Release) {
	content, err := os.ReadFile(readmeFile)
	if err != nil {
		content = []byte("# Railpack Container Registry\n\n<!-- VERSIONS_START --><!-- VERSIONS_END -->")
	}

	// dockerfiles/ 配下のビルド済みバージョンを収集
	files, _ := os.ReadDir(dockerDir)
	var built []string
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "Dockerfile.") {
			built = append(built, strings.TrimPrefix(f.Name(), "Dockerfile."))
		}
	}
	sort.Strings(built)

	// 最新 20 件
	var available []string
	for i, r := range releases {
		if i >= 20 {
			break
		}
		available = append(available, strings.TrimPrefix(r.TagName, "v"))
	}

	var sb strings.Builder
	sb.WriteString("<!-- VERSIONS_START -->\n")
	sb.WriteString("### 🚀 ビルド済みバージョン (GHCR)\n")
	if len(built) > 0 {
		for _, v := range built {
			sb.WriteString(fmt.Sprintf("- `%s`\n", v))
		}
	} else {
		sb.WriteString("なし\n")
	}
	sb.WriteString("\n### 📋 利用可能な最新バージョン (GitHub)\n")
	for _, v := range available {
		sb.WriteString(fmt.Sprintf("- `%s`\n", v))
	}
	sb.WriteString("\n*全リリースは `railpack_releases.json` を参照してください。*\n")
	sb.WriteString("<!-- VERSIONS_END -->")

	s := string(content)
	start := strings.Index(s, "<!-- VERSIONS_START -->")
	end := strings.Index(s, "<!-- VERSIONS_END -->")
	if start != -1 && end != -1 {
		os.WriteFile(readmeFile, []byte(s[:start]+sb.String()+s[end+len("<!-- VERSIONS_END -->"):]), 0644)
		fmt.Println("README.md を更新しました")
	}
}

func fetchAllReleases() []Release {
	var releases []Release
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=100", owner, repo)
	for url != "" {
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "Go-Railpack-Generator")
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := (&http.Client{}).Do(req)
		if err != nil {
			break
		}
		defer resp.Body.Close()
		var page []Release
		json.NewDecoder(resp.Body).Decode(&page)
		releases = append(releases, page...)
		url = nextLink(resp.Header.Get("Link"))
	}
	return releases
}

func readCache() ([]Release, error) {
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, err
	}
	var releases []Release
	return releases, json.Unmarshal(data, &releases)
}

func saveCache(releases []Release) {
	data, _ := json.MarshalIndent(releases, "", "  ")
	os.WriteFile(cacheFile, data, 0644)
}

func nextLink(header string) string {
	for _, link := range strings.Split(header, ",") {
		if strings.Contains(link, `rel="next"`) {
			s, e := strings.Index(link, "<"), strings.Index(link, ">")
			if s != -1 && e != -1 {
				return link[s+1 : e]
			}
		}
	}
	return ""
}
