package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/template"
)

type Release struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"` // リリースノート本文（mise バージョン抽出に使用）
}

type TemplateData struct {
	Version     string
	MiseVersion string // railpack が実行時に要求する mise バージョン
}

const (
	owner      = "railwayapp"
	repo       = "railpack"
	cacheFile  = "railpack_releases.json"
	dockerDir  = "dockerfiles"
	tmplFile   = "dockerfile.tmpl"
	readmeFile = "README.md"
)

// リリースノートから mise バージョンを抽出する
// 例: "Mise: Updated mise version from v2026.3.13 to v2026.3.15"
//     "Mise: Updated mise version from v2026.3.12 to v2026.3.13"
// または単純に "mise-2026.3.17" のような文字列
func extractMiseVersion(body string) string {
	// "to v2026.3.17" 形式
	re := regexp.MustCompile(`to v(\d{4}\.\d+\.\d+)`)
	if m := re.FindStringSubmatch(body); len(m) > 1 {
		return m[1]
	}
	// "mise-2026.3.17" 形式
	re2 := regexp.MustCompile(`mise-(\d{4}\.\d+\.\d+)`)
	if m := re2.FindStringSubmatch(body); len(m) > 1 {
		return m[1]
	}
	return ""
}

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
		// 対象バージョンのリリース情報から mise バージョンを取得
		miseVersion := fetchMiseVersionForRelease(*singleVersion, allReleases)
		if miseVersion == "" {
			fmt.Printf("警告: バージョン %s の mise バージョンを特定できませんでした。\n", *singleVersion)
			fmt.Println("railpack のリリースノートを確認して MISE_VERSION を手動で指定してください。")
			os.Exit(1)
		}
		fmt.Printf("  → mise バージョン: %s\n", miseVersion)
		generateOne(*singleVersion, miseVersion, tmpl)
	}

	updateReadme(allReleases)
	fmt.Println("\n処理が完了しました。")
}

// 対象バージョンのリリースから mise バージョンを特定する
func fetchMiseVersionForRelease(version string, releases []Release) string {
	tag := "v" + version
	for _, r := range releases {
		if r.TagName == tag {
			if mv := extractMiseVersion(r.Body); mv != "" {
				return mv
			}
		}
	}
	// キャッシュに Body がない場合は API から直接取得
	return fetchMiseVersionFromAPI(version)
}

// GitHub API で特定バージョンのリリース詳細を取得して mise バージョンを抽出
func fetchMiseVersionFromAPI(version string) string {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/v%s", owner, repo, version)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Go-Railpack-Generator")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return ""
	}
	return extractMiseVersion(release.Body)
}

func generateOne(version, miseVersion string, tmpl *template.Template) {
	filePath := fmt.Sprintf("%s/Dockerfile.%s", dockerDir, version)
	f, err := os.Create(filePath)
	if err != nil {
		fmt.Printf("ファイル作成失敗: %v\n", err)
		return
	}
	defer f.Close()
	tmpl.Execute(f, TemplateData{Version: version, MiseVersion: miseVersion})
	fmt.Printf("  → 生成: %s\n", filePath)
}

func updateReadme(releases []Release) {
	content, err := os.ReadFile(readmeFile)
	if err != nil {
		content = []byte("# Railpack Container Registry\n\n<!-- VERSIONS_START --><!-- VERSIONS_END -->")
	}

	files, _ := os.ReadDir(dockerDir)
	var builtVersions []string
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "Dockerfile.") {
			builtVersions = append(builtVersions, strings.TrimPrefix(f.Name(), "Dockerfile."))
		}
	}
	sort.Strings(builtVersions)

	var allVers []string
	for i, r := range releases {
		if i >= 20 {
			break
		}
		allVers = append(allVers, strings.TrimPrefix(r.TagName, "v"))
	}

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
		if err != nil {
			break
		}
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
	if err != nil {
		return nil, err
	}
	var releases []Release
	err = json.Unmarshal(data, &releases)
	return releases, err
}

func saveCache(releases []Release) error {
	data, err := json.MarshalIndent(releases, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cacheFile, data, 0644)
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
