# Railpack Container Registry

このリポジトリは [railwayapp/railpack](https://github.com/railwayapp/railpack) の各バージョンを Docker イメージとしてビルドし、GHCR に提供するためのものです。

<!-- VERSIONS_START -->
### 🚀 ビルド済みバージョン (GHCR)
- `0.23.0`

### 📋 利用可能な最新バージョン (GitHub)
- `0.23.0`
- `0.22.2`
- `0.22.1`
- `0.22.0`
- `0.21.0`
- `0.20.0`
- `0.19.0`
- `0.18.0`
- `0.17.2`
- `0.17.1`
- `0.17.0`
- `0.16.0`
- `0.15.4`
- `0.15.3`
- `0.15.2`
- `0.15.1`
- `0.15.0`
- `0.14.0`
- `0.13.0`
- `0.12.0`

*全リリースは `railpack_releases.json` を参照してください。*
<!-- VERSIONS_END -->

## 使い方

```bash
docker pull ghcr.io/${{ github.repository }}:<VERSION>
```

## 自動更新について
この README は GitHub Actions によって自動的に更新されます。
- `Update Railpack Version List` を実行すると、最新のリリース一覧が取得されます。
- `Build and Push Railpack Image` を実行すると、指定したバージョンがビルドされ、ビルド済みリストに追加されます。
