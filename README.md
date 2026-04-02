# Railpack Container Registry

このリポジトリは [railwayapp/railpack](https://github.com/railwayapp/railpack) の各バージョンを Docker イメージとしてビルドし、GHCR に提供するためのものです。

<!-- VERSIONS_START -->
<!-- VERSIONS_END -->

## 使い方

```bash
docker pull ghcr.io/${{ github.repository }}:<VERSION>
```

## 自動更新について
この README は GitHub Actions によって自動的に更新されます。
- `Update Railpack Version List` を実行すると、最新のリリース一覧が取得されます。
- `Build and Push Railpack Image` を実行すると、指定したバージョンがビルドされ、ビルド済みリストに追加されます。
