package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
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
	owner        = "railwayapp"
	repo         = "railpack"
	cacheFile    = "railpack_releases.json"
	dockerDir    = "dockerfiles"
	tmplFile     = "dockerfile.tmpl"
	buildYmlFile = ".github/workflows/build.yml"
)

func main() {
	// コマンドライン引数の設定
	// 特定バージョンのみ生成する場合に使用: -single-version 0.23.0
	singleVersion := flag.String("single-version", "", "特定のバージョンのみ Dockerfile を生成します")
	flag.Parse()

	var allReleases []Release
	var err error

	// 1. 最新リリースの取得 (GitHub API)
	fmt.Println("GitHub APIから最新リリースを取得中...")
	allReleases = fetchAllReleases()
	if len(allReleases) > 0 {
		saveCache(allReleases)
	} else {
		// APIエラーなどの場合はキャッシュから読み込む
		fmt.Println("APIから取得できなかったため、キャッシュを読み込みます...")
		allReleases, _ = readCache()
	}

	// 全バージョンのリストを作成 (v を除く)
	var versions []string
	for _, r := range allReleases {
		versions = append(versions, strings.TrimPrefix(r.TagName, "v"))
	}

	// 2. テンプレートの読み込み
	tmpl, err := template.ParseFiles(tmplFile)
	if err != nil {
		fmt.Printf("テンプレートファイルの読み込み失敗: %v\n", err)
		os.Exit(1)
	}

	// 3. Dockerfile の生成処理
	os.MkdirAll(dockerDir, 0755)

	if *singleVersion != "" {
		// 特定バージョンのみ生成
		fmt.Printf("バージョン %s の Dockerfile を生成します...\n", *singleVersion)
		generateOne(*singleVersion, tmpl)
	} else {
		// 全バージョンを生成
		fmt.Println("全バージョンの Dockerfile を生成します...")
		for _, v := range versions {
			generateOne(v, tmpl)
		}
	}

	fmt.Println("\n処理が完了しました。")
}

// 1つの Dockerfile をテンプレートから生成
func generateOne(version string, tmpl *template.Template) {
	filePath := fmt.Sprintf("%s/Dockerfile.%s", dockerDir, version)
	f, err := os.Create(filePath)
	if err != nil {
		fmt.Printf("ファイル作成失敗 %s: %v\n", filePath, err)
		return
	}
	defer f.Close()

	data := TemplateData{Version: version}
	err = tmpl.Execute(f, data)
	if err != nil {
		fmt.Printf("テンプレート実行失敗 %s: %v\n", filePath, err)
	}
}

func generateOne(version string, tmpl *template.Template) {

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
