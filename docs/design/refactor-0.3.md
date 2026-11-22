# Argo CD MCP Server 0.3 リファクタ実装設計（確定版）

- 基準: `origin/main` / tag `v0.2.0` 以降（argo-cd **v3.6.2**, mcp-go **v1.6.0**）
- 状態: **実装可**。設計のみの議論は完了。
- 保管: 本ファイルが正。チャット履歴に依存しない。

## 0. ロック済み決定

| ID | 決定 |
|----|------|
| D1 | 公開ツール 23 個維持。未公開書き込みはツール化しない |
| D2 | 変更系は常時登録。`ARGOCD_MCP_READ_ONLY=true` のときだけ外す |
| D3 | 対応 Argo CD = **3.6.x のみ**（CI は 3.6.2） |
| D4 | トランスポート = **stdio のみ** |
| D5 | gRPC-Web **残す**。CI で最低 1 経路 |
| D6 | Cluster/Repository は上流 `Sanitized()`。**Helm values / parameters は redact しない**（コンソールと同水準。秘密を values に載せないのが前提） |
| D7 | `delete_application.cascade` 既定 **`false`**（Argo CD CLI 既定 true とは意図的に異なる）。**tool description に挙動を明記** |
| D8 | get / list / sync / delete（+ refresh / manifests）に任意 `app_namespace` |
| D9 | Breaking は **0.3**。0.2 に混ぜない |
| D10 | ツール allowlist なし。RBAC + read-only |
| D11 | （改訂）`refresh` は読み取り扱い: `readOnly=true`, `destructive=false`, `idempotent=true`。READ_ONLY でも登録する（実体は `Get` + refresh。App への refresh 注釈と再 reconcile は起きるが、クラスタ上のリソースは変更しない） |
| D12 | （撤回）Application source 5箇所の values 全 redact は **やらない** |
| D13 | Cluster/Repository の手書き拒否リスト禁止 → `(*Cluster).Sanitized()` / `(*Repository).Sanitized()` |
| D14 | list の project は `ApplicationQuery.Projects` のみ（`Project` と両方セットしない） |
| D15 | App JSON を返す経路は将来まとめてよいが、values redact 用の必須 `render` 層は不要 |
| D16 | `internal/config` と `client.Config` は統合しない。`MaxGRPCMessageSize` は `client.Config` へ |
| D17 | `delete_applicationset`: 引数を `app_namespace` に統一（移行期間は `appsetNamespace` も受理）。README の偽 cascade 記述を削除 |
| D18 | `get_application_logs`: `follow=true` は明示エラー（stdio 非対応）。description に明記 |

## 1. 痛み（根拠）

- 各 tool handler が毎回 `os.Getenv` → `client.New` → `defer Close`（gRPC-Web なら UDS proxy も毎回）
- `internal/argocd/client/interface.go` に未使用 CUD が残存
- `delete_app.go`: cascade 既定 `true`
- `delete_applicationset`: cascade なし・引数 `appsetNamespace`。README は cascade ありと誤記
- Cluster API はサーバ側 `toAPIResponse` → `Sanitized()` 済み（`server/cluster/cluster.go` @ v3.6.2）
- Repository List もサーバ側 `Normalize().Sanitized()`
- manifests の Secret はサーバ側 `diff.HideSecretData` 済み → 追加 redact 不要
- `list_application` の destination server/ns は ApplicationQuery に無い → client-side 必須

## 2. 目標形

```
cmd/argocd-mcp-server/main.go     # wiring only
internal/config/                  # Load() → ReadOnly 等。ToClientConfig()
internal/argocd/client/           # 接続 Config + 縮小 Interface + Sanitized
internal/grpcwebproxy/            # 残す（プロセス寿命の Client に紐付け）
internal/server/                  # version は ldflags から渡す
internal/tools/
  deps.go / register.go           # Deps{Client, ReadOnly}, makeX(d Deps)
  *_*.go                          # dial 削除。xHandler(ctx, client, ...) は残す
```

### 起動

```go
cfg, err := config.Load()
argo, err := client.New(cfg.ToClientConfig())
defer argo.Close()
s := server.New(version)
tools.Register(s, tools.Deps{Client: argo, ReadOnly: cfg.ReadOnly})
mcp_server.ServeStdio(s)
```

- 注入は **クロージャ `makeX(d Deps)`**（グローバル SetClient 禁止）
- Client はプロセスに 1 つ。handler は Close しない
- tool / client 層で接続用 `os.Getenv` ゼロ（`MaxGRPCMessageSize` も Config フィールドへ）

### READ_ONLY で外すツール

`create_application`, `sync_application`, `delete_application`,
`terminate_operation`, `create_project`, `create_applicationset`, `delete_applicationset`

### Interface

削除: `UpdateApplication`, `RollbackApplication`, Cluster/Project/Repo の未使用 CUD。

拡張:

- `GetApplication(ctx, name, appNamespace)`
- `DeleteApplication(ctx, name, cascade, appNamespace)`
- Sync / Refresh / Manifests に `appNamespace`
- `ListApplications(ctx, ListApplicationsOptions{Selector, Projects, AppNamespace})`
- Logs は P5 で `LogsOptions` 化（引数が多いため）
- Get/Delete は位置引数のまま

destination `cluster` / `namespace` は tools 側 client-side filter 継続。

## 3. 秘密情報（D6 確定）

| 対象 | 方針 |
|------|------|
| Cluster / Repository | 応答前に `Sanitized()`（多層防御） |
| Application / ApplicationSet の Helm values 等 | **触らない** |
| manifests 内 Secret | 追加なし |

補足: values に秘密があるならコンソール表示自体がアウト。MCP だけ全 redact するのはやりすぎで、デバッグ価値を落とす。将来必要ならオプトイン env を検討（0.3 では入れない）。

## 4. Schema / Hint

- `delete_application`: cascade 既定 false。description に「省略時は App のみ。リソース削除は `cascade: true`。Argo CD CLI 既定(true)とは異なる」を明記
- 読み取り: `ReadOnlyHint=true`
- `refresh`: readOnly=true, destructive=false, idempotent=true（D11 改訂）
- `list_application.namespace` = destination ns。`app_namespace` = App CR の metadata.namespace（併存・ドキュメント明記）
- `delete_applicationset`: `app_namespace`（旧 `appsetNamespace` も移行受理）。README から cascade 誤記削除
- `get_application_logs`: `follow=true` → ツールエラー

## 5. PR 順（v0.2.0 後）

| PR | 内容 | Breaking? |
|----|------|-----------|
| **P1** | config + 単一 Client + makeX。HandleX の env エラーテストは削除／config へ移設（起動 Fatal に移る例外を AC に書く） | 実質テスト・エラータイミングのみ |
| **P2** | Interface 縮小 + mock 再生成 | 内部のみ |
| **P3** | Cluster/Repo に `Sanitized()` + ドキュメント（App は UI と同水準） | ほぼなし（サーバ側既にマスク） |
| **P4** | `ARGOCD_MCP_READ_ONLY` + Hints（D11） | なし |
| **P5** | `app_namespace` + ListOptions + LogsOptions + D17 | 引数名は移行受理で緩和 |
| **P6** | cascade 既定 false + description（**挙動 Breaking**）。P3 と分けたまま | **Yes** |
| **P7** | gRPC-Web CI 1 本、dead code、D18 follow 拒否、README 修正 | follow は Yes |

0.3.0 タグ時にリリースノートをまとめる。

## 6. 0.3 Breaking 一覧

1. `delete_application` cascade 既定 `true` → `false`
2. （D17）`delete_applicationset` 推奨引数名 `app_namespace`
3. （D18）`follow=true` がエラー

## 7. 非ゴール

HTTP/SSE、ツール allowlist、Argo CD v2、新書き込みツール、tools 大規模ディレクトリ分割、Helm values のキー名選択 redact / 全 redact、長さ制限・ハッシュ。

## 8. 成功定義

- Client 1・tool 内 dial/env 0
- Interface ⊆ 公開ツール
- READ_ONLY で変更系が消える（refresh は読み取り扱いで残る）
- Cluster/Repo に生秘密が載らない（Sanitized）
- cascade 省略 = false かつ description 明記
- `app_namespace` が get/list/sync/delete 系で使える
- `follow=true` がハングしない
- gRPC-Web が CI で 1 回通る
- 対応宣言は 3.6.x のみ

## 9. 実装開始手順

1. `git fetch origin main` し `v0.2.0` / 最新 main からブランチ
2. ブランチ名: `cursor/<descriptive-name>-feb1`
3. **P1 から**着手
4. 各 PR 後に test green → push → ManagePullRequest
