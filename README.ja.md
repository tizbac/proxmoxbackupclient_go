# Proxmox Backup Client — Proxmox Backup Server 用 Windows クライアント

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/tizbac/proxmoxbackupclient_go)](https://github.com/tizbac/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-github-orange)](https://github.com/tizbac/proxmoxbackupclient_go)

**Proxmox Backup Client は、Proxmox Backup Server (PBS) 向けのオープンソース (GPL-3.0) バックアップクライアントで、Windows と Linux で動作します。**
PBS へのバックアップを行うための**ツール群（スイート）**です：

- **Proxmox Backup Client GUI**（RDEM Systems の Nimbus Backup GUI ベース）— Windows サーバー・ワークステーションを PBS にバックアップするための最新グラフィカルインターフェース：一貫性のある VSS スナップショット、スケジュールジョブ、ファイル・ディスクモード、スナップショット閲覧・復元、マルチ PBS 対応、Windows サービスモード。
- **`proxmoxbackup-directory`** — 重複排除付きのディレクトリ (PXAR) バックアップ用コマンドラインツール。
- **`proxmoxbackup-machine`** — Windows システムの完全オンラインバックアップ（FIDX、VSS、増分）用コマンドラインツール。
- **`proxmoxbackup-nbd`** — ディスクバックアップの復元用 NBD サーバー（Linux）。

> キーワード：proxmox backup client windows · PBS クライアント · Windows VSS バックアップ · イミュータブルなオフサイトバックアップ · Proxmox Backup Server インターフェース。

> ⚠️ **免責事項：** このプロジェクトは **Proxmox Server Solutions GmbH** とは**一切関係ありません**。「Proxmox」、Proxmox ロゴ、および関連する名称はそれぞれの権利者の所有物であり、ここでは**互換性を示すためだけに**使用しています。同社の製品については [proxmox.com](https://www.proxmox.com/) をご覧ください。

> 🤖 **この翻訳は AI によって生成されたもので、小さな誤りが含まれる可能性があります。改善のためのご貢献を歓迎します。**

## 📦 ダウンロード

👉 **[最新リリースをダウンロード](https://github.com/tizbac/proxmoxbackupclient_go/releases)**

> ⚠️ **Windows が「ウイルスが検出されました」（例：`Trojan:Win32/Sabsik.FL.A!ml`）や SmartScreen の警告を表示しますか？**
> これは Go/Wails アプリの**既知の誤検知**です — ウイルス*ではありません*。`!ml` の接尾辞は、*署名されておらず珍しい*実行ファイルを検出する機械学習モデルによる判定であることを示します。
> [なぜこうなるのか、ダウンロードをどう検証するか](https://github.com/tizbac/proxmoxbackupclient_go)をご覧ください。

### 🔎 ダウンロードの検証

各リリースでは SHA-256 チェックサムと**署名付き生成元証明**（このリポジトリの CI が特定のコミットからビルドしたという暗号学的証明）を提供しています：

```powershell
Get-FileHash .\ProxmoxBackupClient.exe -Algorithm SHA256   # SHA256SUMS.txt と比較
gh attestation verify .\ProxmoxBackupClient.exe --repo tizbac/proxmoxbackupclient_go
```

**VirusTotal — 0 件の検出。** 最近の MSI インストーラーの独立した複数エンジンによるレポート：
[0.2.108](https://www.virustotal.com/gui/file/6e8fb7ce9af740d470e947addb8daba4331c0b88e8bfdec9e0697ea8f7f29e9e/detection) ·
[0.2.107](https://www.virustotal.com/gui/file/6fd6c6fa77e0305c129ef882a3745100aa6033187a6d52a4af94149ab6b666d2/detection) ·
[0.2.106](https://www.virustotal.com/gui/file/ad6e56700ed9df8e088906e38cee2e2882fc7045f4e39269de0e379a01784ad7/detection)

> ℹ️ **コード署名：** Windows バイナリは**まだ Authenticode 署名されていません**（[SignPath Foundation](https://signpath.org) 経由の OSS 証明書を申請中）。それまでは、上記の証明とチェックサムによって生成元を確認できます。

## 📚 ドキュメント

- **Proxmox バックアップ完全ガイド** — PBS デプロイのベストプラクティス
- **Proxmox Backup Server で Windows をバックアップする** — Windows 向けデプロイガイド
- **PBS と Veeam の比較** — Proxmox バックアップの比較

## ✨ 機能

### Proxmox Backup Client GUI（推奨）
- **🌍 多言語対応** — 日本語、英語などに対応したインターフェース
- 接続テスト付きの使いやすい設定
- 速度と残り時間を表示するリアルタイムのバックアップ進捗
- 一貫性のあるバックアップのための VSS（Volume Shadow Copy）対応
- 複数フォルダーのバックアップ、ファイル・ディスクモード
- スナップショットの閲覧、ファイル検索（ワイルドカード）、復元
- 証明書フィンガープリントの固定（TOFU）によるマルチ PBS サーバー対応
- Windows サービスモード＋スケジュールバックアップ
- 診断用デバッグログ

### 📸 スクリーンショット

![サーバー設定](docs/screenshots/nimbus-gui-liste-servers.png)
*ステータスインジケーター付きのマルチ PBS サーバー管理*

![サーバー追加フォーム](docs/screenshots/nimbus-gui-add-server-form.png)
*接続テスト付きのシンプルなサーバー設定*

![ワンショットバックアップ](docs/screenshots/nimbus-gui-one-shot-backup.png)
*ETA とスループット表示のリアルタイムバックアップ進捗*

### スマートシステム除外（ファイルモード）
ドライブ全体（例：`D:\`）をバックアップする際、Proxmox Backup Client は自動的に除外します：

**システムフォルダー：** `System Volume Information`（VSS ストレージ、100+ GB に達することがある）、`$RECYCLE.BIN`、`Recovery`。
**システムファイル:** `pagefile.sys`、`hiberfil.sys`、`swapfile.sys`。

**重要な理由：** ドライブの使用量が 1.03 TB と表示されていても、実際のファイルは約 141 GB の場合があります。除外しないとバックアップに VSS スナップショットが含まれます（容量と時間の無駄）。除外すればサイズが実データと一致します。

**推奨事項：** ファイルレベルのバックアップには自動除外付きの**ファイルモード**（既定）を使用し、ベアメタル復元用（すべてを含む）には別ジョブで**ディスクモード**を使用してください。

### セキュリティと品質
- 入力検証と資格情報のサニタイズ
- パストラバーサル対策
- 指数バックオフ付き再試行ロジック
- 包括的なエラー処理とテスト、lint 100% 準拠

## 🚀 クイックスタート

1. リリースから `ProxmoxBackupClient.exe`（または `.msi`）をダウンロード
2. 管理者権限で起動（VSS に必要）
3. PBS 接続を設定してテスト
4. バックアップするフォルダーを選択
5. バックアップを開始

## 📋 前提条件

- Windows 10/11（64 ビット）
- 管理者権限（VSS スナップショット用）
- Proxmox Backup Server へのネットワークアクセス

## 🔨 ソースからのビルド

### 前提条件
- Go 1.22 以降
- Node.js 20 以降
- Wails CLI：`go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### ビルド
```bash
cd gui
npm install --prefix frontend
wails build      # または：wails dev（ホットリロード）
```

## 🔧 高度な使い方とガイド

### マルチ PBS（複数 PBS サーバー）

複数の PBS サーバーを設定し、バックアップごとにターゲットを選択します（例：`C:\Users` → 高速 SSD PBS、毎日；`C:\` → 大容量データ PBS、毎週；さらに DR サーバー）。

- **[ユーザーガイド](MULTI_PBS_USER_GUIDE.md)** — サーバーの追加・テスト、既定サーバー、FAQ、トラブルシューティング。
- **[実装ガイド](MULTI_PBS_GUIDE.md)** — データモデル、単一 PBS 構成からの自動移行、バックエンド API メソッド。

既存の単一 PBS 構成は、初回読み込み時に自動的に `default` サーバーへ移行されます。

### Clonezilla ISO（ベアメタル復元）

レスキューワークフローは、Clonezilla Live ISO に `pbsnbd` / `machinebackup` バイナリと **pbs-nbd** エントリーを Clonezilla のメインメニューに追加する形でパッチを当てて作成します（CD ブート、USB は `dd`、UEFI 対応）：

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd
```

詳細（全面リビルドする理由、前提条件、メニューの流れ、検証）は **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)** をご覧ください。

### Windows GUI のビルド

**Docker（特に Linux でのビルドに推奨）。** ワンコマンドスクリプトは、使い捨ての `golang` コンテナー（mingw + Wails のインストール、フロントエンドビルド、`wails build` の実行）を使い、WebView2 を適切にサポートした `ProxmoxBackupClientGO.exe` を生成します：

```bash
./build_gui_windows_docker.sh
```

**ネイティブ Windows（Chocolatey）。** 完全な Windows ツールチェーン設定は **[BUILD.md](BUILD.md)** を参照：

```powershell
choco install go
choco install mingw
# その後、管理者権限のないシェルで：
build.bat          # GUI
build_cli.bat      # CLI
```

### 機能ステータス、変更履歴、社内ドキュメント

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)** — 機能別ステータスマトリクス（実装済み / テスト済み / ロードマップ）。
- **[CHANGELOG.md](CHANGELOG.md)** — バージョン別の変更履歴。
- **[TODO.md](TODO.md)** — 未決のロードマップとアイデア。
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)** — 安定した製品状態と利用可能なビルド。
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)** — MSI アンインストールダイアログ（設定を保持/削除）とそのテスト計画。
- **[FIXES_SUMMARY.md](FIXES_SUMMARY.md)** — GUI 修正メモ（ディレクトリモードとマシンモードの切り替え）。

## 🖥️ GUI の帰属

**Proxmox Backup Client GUI** は、**[RDEM Systems](https://www.rdem-systems.com/)** が開発・保守する **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)** をベースにしています。

この GUI（もともとはこのプロジェクトのフォーク）はこのリポジトリに統合されました：GUI とその全機能を含むコード全体は、GPLv3 ライセンスのもとでオープンソースのままです。RDEM Systems は GUI の開発をスポンサーし、商用サポートを提供しています。

**オリジナル CLI の作者：** Tiziano Bacocco (tizbac) · **ライセンス：** GPLv3

## ⚠️ 警告

このソフトウェアは「現状のまま」提供されます。私たちは信頼性を目指していますが、データの損失や損傷については一切の責任を負いません。本番環境でバックアップに依存する前に、必ずバックアップをテストし、復元を確認してください。

## 📄 ライセンス

GPLv3 — [LICENSE](LICENSE) ファイルを参照。

## 🏷️ ブランディング

機能や修正を追加する**コミットを 5 件以上**貢献したすべてのコントリビューターは、商業用途のブランディングデータを追加してもらう権利があります。

唯一の条件は、ブランディングが指す企業が以下のいずれの活動も行っていないことです：

- マルウェアキャンペーン
- 戦争を助長する事業（西側諸国を含むすべての国に適用されます）
- 詐欺
- データ窃取
- 人身売買／児童売買
- 暴力
- 差別
- 麻薬
- 一般に違法と認められる活動

いずれかのコントリビューターに対して苦情が寄せられた場合、私たちは連絡を試みます。妥当な説明がなければ、その特典を**直ちに終了**します。

**GPLv3 ライセンスは引き続き有効**であり、プロジェクトをフォークして独自の実行ファイルをビルドすることは今後も自由です。

## Proxmox Backup Client GO のコントリビューターについて

Proxmox Backup Client GO のコントリビューターがこのプロジェクトを開発・保守しています。このソフトウェアは NTP/NTS インフラストラクチャと、コミュニティリファレンスに記載された [11 の公開 NTS サーバー](https://github.com/jauderho/nts-servers) に依存しています。

## 🤝 貢献

GUI は完全に実装されましたが、貢献は引き続き歓迎します。特に：

1. 暗号化サポート（まだ未実装）
2. 物理から仮想（P2V）への移行、ベアメタルバックアップの仮想マシンへの復元（まだ不完全）
3. チャンクの非同期アップロード／マルチコアアップロード（マルチコア圧縮は既に machine backup で実装済み）
4. pxar 形式に Windows セキュリティ記述子を持つ新しいエントリー種別を追加する Proxmox 側パッチ
5. Windows シンボリックリンクのサポート
6. 気になる興味深いものなら何でも :)

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**