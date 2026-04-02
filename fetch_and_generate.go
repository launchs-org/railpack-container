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

// リリースの情報を格納する構造体
type Release struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
}

// テンプレートへ渡すデータ
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

	var allReleases []Release
	fmt.Println("GitHub APIから最新リリースを取得中...")
	allReleases = fetchAllReleases()
	if len(allReleases) > 0 {
		saveCache(allReleases)
	} else {
		allReleases, _ = readCache()
	}

	tmpl, err := template.ParseFiles(tmplFile)
	if err != nil {
		fmt.Printf("テンプレート読み込み失敗: %v\n", err)
		os.Exit(1)
	}

	os.MkdirAll(dockerDir, 0755)

	if *singleVersion != "" {
		fmt.Printf("バージョン %s の Dockerfile を生成します...\n", *singleVersion)
		generateOne(*singleVersion, tmpl)
	}

	// READMEの更新処理
	updateReadme(allReleases)

	fmt.Println("\n処理が完了しました。")
}

func generateOne(version string, tmpl *template.Template) {
	filePath := fmt.Sprintf("%s/Dockerfile.%s", dockerDir, version)
	f, err := os.Create(filePath)
	if err != nil {
		return
	}
	defer f.Close()
	tmpl.Execute(f, TemplateData{Version: version})
}

// README.md を自動更新する
func updateReadme(releases []Release) {
	content, err := os.ReadFile(readmeFile)
	if err != nil {
		content = []byte("# Railpack Container Registry\n\n<!-- VERSIONS_START --><!-- VERSIONS_END -->")
	}

	// ビルド済みバージョンの確認
	files, _ := os.ReadDir(dockerDir)
	var builtVersions []string
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "Dockerfile.") {
			builtVersions = append(builtVersions, strings.TrimPrefix(f.Name(), "Dockerfile."))
		}
	}
	sort.Strings(builtVersions)

	// 最新の全リリース（上位20件を表示）
	var allVers []string
	for i, r := range releases {
		if i >= 20 { break }
		allVers = append(allVers, strings.TrimPrefix(r.TagName, "v"))
	}

	// 書き換え用文字列の作成
	var sb strings.Builder
	sb.WriteString("<!-- VERSIONS_START -->\n")
	sb.WriteString("### 🚀 ビルド済みバージョン (GHCR)\n")
	if len(builtVersions) > 0 {
		for _, v := range builtVersions {
			sb.WriteString(fmt.Sprintf("- `%s`\n", v))
		}
	} else {
		sb.WriteString("なし\n")
	}
	sb.WriteString("\n### 📋 利用可能な最新バージョン (GitHub)\n")
	for _, v := range allVers {
		sb.WriteString(fmt.Sprintf("- `%s`\n", v))
	}
	sb.WriteString("\n*全リリースは `railpack_releases.json` を参照してください。*\n")
	sb.WriteString("<!-- VERSIONS_END -->")

	// 指定したコメントタグの間を置換
	reStart := "<!-- VERSIONS_START -->"
	reEnd := "<!-- VERSIONS_END -->"
	
	s := string(content)
	startIdx := strings.Index(s, reStart)
	endIdx := strings.Index(s, reEnd)

	if startIdx != -1 && endIdx != -1 {
		newContent := s[:startIdx] + sb.String() + s[endIdx+len(reEnd):]
		os.WriteFile(readmeFile, []byte(newContent), 0644)
		fmt.Println("README.md を更新しました。")
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
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil { break }
		defer resp.Body.Close()
		var pageReleases []Release
		json.NewDecoder(resp.Body).Decode(&pageReleases)
		releases = append(releases, pageReleases...)
		url = getNextLink(resp.Header.Get("Link"))
	}
	return releases
}

func readCache() ([]Release, error) {
	data, err := os.ReadFile(cacheFile)
	if err != nil { return nil, err }
	var releases []Release
	err = json.Unmarshal(data, &releases)
	return releases, err
}

func saveCache(releases []Release) error {
	data, err := json.MarshalIndent(releases, "", "  ")
	if err != nil { return err }
	return os.WriteFile(cacheFile, data, 0644)
}

func getNextLink(linkHeader string) string {
	if linkHeader == "" { return "" }
	links := strings.Split(linkHeader, ",")
	for _, link := range links {
		if strings.Contains(link, `rel="next"`) {
			start := strings.Index(link, "<")
			end := strings.Index(link, ">")
			if start != -1 && end != -1 { return link[start+1 : end] }
		}
	}
	return ""
}
