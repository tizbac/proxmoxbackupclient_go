# Proxmox Backup Client：Proxmox Backup Server 用の Windows・Linux クライアント

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Proxmox Backup Client は、Proxmox Backup Server (PBS) 向けのオープンソース (GPL-3.0) バックアップクライアントで、Windows と Linux で動作します。**

PBS へバックアップするための**ツール群**です：

- **Proxmox Backup Client GUI**（RDEM Systems の Nimbus Backup GUI がベース）：Windows と Linux のサーバー・ワークステーションを PBS にバックアップするグラフィカルクライアント。一貫性のあるスナップショット（Windows は VSS、Linux は elastio-snap/dattobd）、スケジュール付きの Backup Sets、フォルダーバックアップとディスク全体のバックアップ、復元、Undelete と Roll back、複数の PBS サーバー、Windows と Linux のバックグラウンドサービスに対応します。
- **`proxmoxbackup-directory`**：重複排除と変更検出付きのフォルダー (pxar) バックアップ用コマンドラインツール。
- **`proxmoxbackup-machine`**：Windows (VSS) と Linux (elastio-snap/dattobd) マシンのディスク全体をオンラインでバックアップ (fidx) するコマンドラインツール。ベアメタル復元後、イメージはそのまま起動できます。
- **`proxmoxbackup-nbd`**：ディスクバックアップを NBD デバイスとして接続し、復元に使います (Linux)。
- Clonezilla Live をベースにした**ベアメタル復元 ISO**。

> ⚠️ **免責事項：** このプロジェクトは **Proxmox Server Solutions GmbH** とは**一切関係ありません**。「Proxmox」、Proxmox のロゴおよび関連する名称はそれぞれの権利者に帰属し、ここでは互換性を示す目的に**限って**使用しています。同社の製品は [proxmox.com](https://www.proxmox.com/) をご覧ください。

> ℹ️ **このページについて：** これは英語版 README の日本語要約です。**[英語版 README.md](README.md) が正式版**で、すべての詳細が載っています。ユーザーマニュアルは英語のみです（下の表を参照）。アプリの画面は現在日本語に対応していないため、メニュー・タブ・ボタンの名前はアプリ上の英語表記のまま記載しています。

## 📖 ユーザーマニュアル

| マニュアル | 内容 |
|---|---|
| **[GUI マニュアル](docs/manual/GUI.md)** | インストール、PBS サーバー、Backup Sets とスケジュール、増分バックアップ、進捗、復元、Undelete、Roll back、Reports、ログ、Preferences、バックグラウンドサービス、トラブルシューティング |
| **[コマンドラインマニュアル](docs/manual/CLI.md)** | `proxmoxbackup-directory`、`proxmoxbackup-machine`、`proxmoxbackup-nbd`：全フラグ、設定ファイル、終了コード、スケジュール実行、コマンドラインからの Undelete と Roll back |
| **[ベアメタル復元マニュアル](docs/manual/BMR.md)** | 復元 ISO の手順、`vm` スナップショットを Proxmox VE へ、異なるハードウェアへの復元 |
| [復元シナリオ](docs/RESTORE_GUIDE.md) | どの場面でどの復元を使うか |
| [マニュアル索引](docs/manual/README.md) | 3 つのマニュアルの概要 |

## 🆕 v0.9.0 の新機能：Backup Set ごとに複数のスケジュール、インストーラー、Linux サービス

- **Backup Set ごとに複数のスケジュール**：数分ごとから年 1 回まで設定でき、「毎日 02:00」に加えて「毎月末日」や「第 2 火曜日」も組み合わせられます。各スケジュールはわかりやすい言葉で表示され、次の 3 回の実行予定も示されます。既存のセットはスケジュールをそのまま引き継ぎます。
- **フォルダーバックアップが代替データストリームと NTFS 拡張属性を保持**（Windows）。「インターネットからダウンロードした」印なども含め、復元時に元に戻します。既知の制限は現在ありません。
- **インストーラー**：**Windows MSI**（アプリとバックグラウンドサービスを Program Files に配置、スタートメニュー登録、上書きアップグレード、削除時に設定を残すか消すかを選択）と、**Linux パッケージ** `.deb`（Debian、Ubuntu、Mint）および `.rpm`（Fedora）。アプリ、3 つのコマンドラインツール、マニュアル、メニュー項目を含みます。
- **Linux のバックグラウンドサービス**（`proxmoxbackupclient.service`）：スケジュールされた Backup Sets を root で実行し、誰もログインしていなくても動きます。Windows と同じく **Preferences > Advanced > Run as Service** でオンにします。**Restart as administrator** も Linux で使えるようになりました（pkexec 経由）。
- サービスが実行するバックアップも **Pause、Resume、Stop** でき、開いているアプリに進捗カードが表示されます。
- 2 つの Backup Sets が同じバックアップグループ（同じサーバー、種類、Backup ID）を共有すると**警告**し、両方のカードに **SHARED ID** バッジを表示します。
- **設定の保護**：Windows では `C:\ProgramData\ProxmoxBackupClient` を管理者のみに制限し、Linux では設定フォルダーを所有者専用にしました。
- **一般的なすべてのファイルシステムのマシンバックアップからファイルを取り出し可能に**：NTFS に加えて FAT、exFAT、ext2/3/4、XFS。Windows イメージの復元ではアクセス権と代替データストリームも戻します。
- **Linux のマシンバックアップも空き領域をスキップ**（ext2/3/4、XFS）。Windows では v0.8.0 から対応済みです。
- Linux マシンの Backup Sets で **Undelete と Roll back** が使えるようになり、任意の Backup Set に対して**コマンドラインからも**実行できます：`-list-sets`、`-undelete`、`-rollback`（[コマンドラインマニュアル](docs/manual/CLI.md)の 12 章）。
- コマンドラインツールはメッセージボックスを出さずエラーを stderr に出力し、`proxmoxbackup-machine` は通知メールを送信します。
- **ベアメタル復元 ISO** は、起動に使った USB メモリを復元先ディスクとして表示しません。物理マシンを VM にする際にファームウェア（UEFI または BIOS）を合わせる方法もマニュアルで説明しています。
- **すべての言語を見直しました**（17 言語、約 1,850 文字列）。日付と数値はアプリの言語に合わせて表示されます。
- v0.8.2 の空白領域の高速化（タグのみで単独では公開されていません）もこのリリースに含まれます。

全項目は [CHANGELOG](CHANGELOG.md) を参照してください。

## 💽 v0.8.0 の新機能：より速いマシンバックアップ、そこからのファイル復元、一時停止/再開

- Windows の**マシンバックアップは空き領域**（とページファイル・休止状態ファイル）**をスキップ**するため、半分空いたディスクは約半分の時間で読み終わります。**不良セクター**があっても止まらず、その部分はゼロとしてイメージに入り、実行結果に場所が記録されます。
- マシンバックアップの進捗カードには、全体に対するブロック数、ディスクとパーティション、New / Reused / 空白チャンク数、アップロード速度が表示されます。
- **マシンバックアップからファイルを復元**：イメージ内の各 NTFS ボリュームが、閲覧・検索・チェックできるフォルダーになります。事前のマウントやダウンロードは不要で、読むのはファイルテーブルと選んだファイルのデータだけです（43 万ファイルある 127 GB のノート PC の C: が 4〜15 秒で一覧表示されます）。ノート PC 1 台につきマシンバックアップ 1 つで、ベアメタル復元にも「あのファイルを戻したい」にも対応できます。
- **Undelete と Roll back** がマシンの Backup Sets (Windows) でも使えます。対象はそのセットで選んだフォルダー（既定は Desktop、Documents、Pictures、Music、Videos、Downloads）です。
- 実行中のフォルダー・マシンバックアップの **Pause と Resume**、各セットカードの **VSS / Snapshot ON-OFF バッジ**、Windows の**作成日時**のバックアップと復元。
- **v0.8.2：** 空白のディスク領域をブロックごとにハッシュしなくなり、ほぼ空のディスクは約 3 分の 1 の時間で済みます（50 GB のテストディスク：3 分 36 秒から 1 分 15 秒に）。

詳細は [CHANGELOG](CHANGELOG.md) を参照してください。

## 🗑️ v0.7.0 の新機能：Undelete と Roll back

フォルダーの Backup Sets 用に、GUI に 2 つのページが加わりました：

- **Undelete** は、セットのバックアップにはあるがディスクからは消えたファイルを探します（最新のバックアップ、または過去 7 日・30 日のすべてのバックアップが対象）。フォルダーツリーで各ファイルがいつ消えたかを表示し（同じファイルが別の場所にあれば「probably moved to ...」と表示）、チェックしたファイルを元の場所か別のフォルダーに復元します。76.1 万ファイルの共有から 39 GB を削除した後、7 日間の検索で約 1 分のうちにそのファイルを正確に見つけました。
- **Roll back** は、セットのフォルダーを選んだバックアップ時点の状態に戻します。まず変更内容（消えたもの、変わったもの、その後に増えたもの）をプレビューしてから復元します。置き換え・削除されるファイルは事前に同じドライブ上の別の場所へ移されるので、**Undo** ですべて元に戻せます。

読むのはバックアップのファイル一覧とフォルダーの一覧だけで、復元するまでファイルは開きません。ほかにも、アプリ内の確認ダイアログ、各 Backup Set カードの次回実行時刻、セットごとの Backup ID の提案（`deepthought-data`）、セット名の重複防止、Windows 版ダウンロードへのバックグラウンドサービス同梱（Preferences > Advanced > Run as Service）が追加されました。[GUI マニュアル](docs/manual/GUI.md#10-undelete)と [CHANGELOG](CHANGELOG.md) を参照してください。

## ⚡ v0.6.0 の新機能：増分フォルダーバックアップ

フォルダーバックアップで、変更のないファイルをスキップできるようになりました。公式の `proxmox-backup-client` と同じ**変更検出**を使います。**Metadata** モードでは、各ファイルのサイズ、更新日時（ナノ秒単位）、モード、所有者を前回のスナップショットと比べます。変更のないファイルは開きもせず、前回のスナップショットのチャンクをそのまま再利用し、新規・変更ファイルだけを読み込んでアップロードします。

実際のファイルサーバー（480.9 GB、USB ディスク上の 761,866 ファイル）での計測：

| 実行 | 時間 | 読み込み | アップロードした新規チャンク |
|---|---|---|---|
| 最初の Metadata 実行（比較対象なし） | 4 時間 17 分 | 全ファイル | 135,549 |
| 2 回目以降 | 約 10〜15 分（前回から数分しか経っていなければさらに短い） | 変更ファイルのみ。変更のない 761,866 ファイルはすべて再利用 | 4〜40 |

変更のないデータは読みも送りもしないため、リモートやオフサイトの PBS への通信量も、実際に変わった分とほぼ同じで済みます。

使い方：フォルダーの **Backup Set** を編集し、**Destination** タブで **Change detection: Metadata** を選びます。**Read every file every N runs** を設定すると N 回に 1 回は全ファイルを読み、サイズも日時も変わらない珍しい変更を拾います（例：4 時間ごとに動くセットなら 42 で、ほぼ週 1 回）。セットごとに**別の Backup ID** を使ってください。Metadata モードはグループ内の最新スナップショットと比べるので、2 つのセットが ID を共有すると互いに全読み込みになってしまいます。コマンドラインでは `-change-detection-mode metadata`。既定は引き続き Legacy です。詳細は下の「より速いフォルダーバックアップ」と [GUI マニュアル](docs/manual/GUI.md#7-incremental-folder-backups-change-detection)を参照してください。

## 🤝 公式 Proxmox クライアントと双方向に互換

`proxmox-backup-client` 3.4.9 と PBS 4.2 で確認済み：

- **このクライアントで作った Legacy、Data、Metadata スナップショットは、公式クライアントでバイト単位まで同一に復元できます**。GUI とコマンドラインのどちらで作っても、Windows・Linux のどちらで作っても同じで、PBS 4.2 のファイルブラウザーでも開けます。Linux では、復元したファイルのモード、ナノ秒単位の更新日時、シンボリックリンク先も正確に保たれます。
- **公式クライアントで作った Legacy、Data、Metadata スナップショットは、このクライアントでバイト単位まで同一に復元できます**（全体でも一部でも）。
- 同じ分割アーカイブ形式 (pxar version 2)、同じ変更検出モード、同じ暗号鍵ファイル形式（`proxmox-backup-client key create` の鍵はここで使え、ここで作った鍵はあちらで使えます）、同じ `PBS_REPOSITORY` 形式の環境変数。

つまり、バックアップがこのクライアントに縛られることはありません。唯一の追加物は、フォルダースナップショットのルートに置かれる小さなファイル `.proxmox_backup_client_meta.json`（元のパス）で、公式クライアントは普通のファイルとして復元します。

## 🔧 このフォークについて

これは上流プロジェクトの **mjb-is フォーク**です。リポジトリ：**https://github.com/mjb-is/proxmoxbackupclient_go**。目標は、ホームラボで現在混在しているバックアップ手段をまとめ、PBS を唯一のサーバー型バックアップシステムにすることです。あわせて Windows/Linux の GUI クライアントを強化し、Clonezilla 上に完全自動のベアメタル復元経路を作っています。上流に対する主な改良は、作り直した GUI（メニューバー、サイドメニュー、Backup Sets、Reports、Message Log、5 種類のカラーテーマとカスタム色、18 の表示言語）、PBS/NBD のチャンク取得で起きていた `zstd.NewReader` のデッドロック修正（[PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85)）、完全自動の「PBS Bare Metal Restore」ウィザード、NTFS ACL・POSIX ACL・拡張属性の復元と任意の復元後検証、Backup Set の操作性向上、マシンバックアップを Proxmox VE の `vm` スナップショットとして保存する機能などです。

汎用的な改良は上流へ PR として送っています。これまでにこのフォークから 17 件の PR が本家にマージされ、1 件（[#105](https://github.com/tizbac/proxmoxbackupclient_go/pull/105)）がオープン中です。一覧は[英語版 README](README.md#-about-this-fork) にあります。

## 📦 ダウンロードとインストール

👉 **[最新リリースをダウンロード](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| 環境 | 使うもの |
|---|---|
| Windows | **MSI インストーラー**（`ProxmoxBackupClient-<version>-x64.msi`）がアプリとバックグラウンドサービスを Program Files に入れ、スタートメニューに登録します。または `ProxmoxBackupClient-v<version>-windows-amd64.zip` を展開（アプリと `ProxmoxBackupClientSVC.exe`。同じフォルダーに置いてください） |
| Debian、Ubuntu、Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| その他の Linux | `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` を展開（WebKitGTK ランタイムが必要） |
| コマンドラインのみ | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` または `-linux-amd64.tar.gz` |

MSI、`.deb`、`.rpm` は master で新しく加わったもので、v0.8.1 までのリリースには zip と tar.gz しかありません。インストーラー付きのリリースが出るまでは、[BUILD.md](BUILD.md) の手順でビルドしてください。

Linux パッケージは 2 つのメニュー項目を追加します。**Proxmox Backup Client** は自分のユーザーとして実行し（自分のファイルのフォルダーバックアップ用）、**Proxmox Backup Client (administrator)** は polkit でパスワードを確認して root で実行します（マシンバックアップとブロックスナップショット用）。バックグラウンドサービスはインストールされますが、オンにはなっていません。[packaging/debian/README.md](packaging/debian/README.md) を参照してください。

> ⚠️ **Windows で「ウイルスが検出されました」（例：`Trojan:Win32/Sabsik.FL.A!ml`）や SmartScreen の警告が出ますか？**
> これは Go/Wails アプリで**よく知られた誤検知**で、ウイルス*ではありません*。`!ml` は機械学習モデルによる判定を示し、*署名がなく珍しい*実行ファイルに付きます。詳しくは上流の説明を参照してください：[原因とダウンロードの検証方法](https://github.com/tizbac/proxmoxbackupclient_go)。

### 🔎 ダウンロードの検証

このフォークの各リリースには `SHA256SUMS.txt` が付いています：

```powershell
Get-FileHash .\ProxmoxBackupClient-v0.8.1-windows-amd64.zip -Algorithm SHA256   # compare with SHA256SUMS.txt
```

Linux では `sha256sum -c SHA256SUMS.txt --ignore-missing`。

> ℹ️ このフォーク独自のビルドには、**コード署名、CI 署名付きの証明、VirusTotal スキャンはまだありません**。手作業でビルド・アップロードしています。整備を手伝っていただける方は [TODO.md](TODO.md) をご覧ください。

## ✨ 機能

### GUI：Proxmox Backup Client GUI（推奨）
- **🌍 多言語**：18 の表示言語（フランス語、英語、イタリア語、ドイツ語、ポーランド語、スペイン語、ブルガリア語、チェコ語、ギリシャ語、ハンガリー語、ラトビア語、リトアニア語、オランダ語、ポルトガル語、ルーマニア語、スロバキア語、トルコ語、ウクライナ語）
- ネイティブのメニューバーとサイドメニュー（Backup、Restore、Undelete、Roll back、Reports、Message Log、Preferences、About）
- **Backup Sets**：名前付きで再利用できるジョブ。毎日 / 間隔 / 手動のスケジュール、ディレクトリかマシンかの種類バッジ、VSS / Snapshot バッジ、「Run Now」（Metadata セットでは高速読み込みか全読み込みを選択）、2 つのセットが同じバックアップグループに書き込む場合の **SHARED ID** 警告
- フォルダーまたはディスク全体の**単発バックアップ**
- バックアップと復元の全履歴を表示する **Reports** ページと、診断用の **Message Log**（上限付き）
- 5 つのカラーテーマ（Amber、Blue、Green、Red、Dark）とカスタム色を選べる **Preferences** ダイアログ
- **増分フォルダーバックアップ**（変更検出、Metadata モード）：公式クライアントと同じく、変更のないファイルは読まずに再利用。N 回ごとの全読み込みも設定可能
- Windows と Linux の**マシン（ディスク全体）バックアップ**：空き領域をスキップし、不良セクターでも止まらず、`host` または Proxmox VE の `vm` スナップショットとして保存
- **復元**：スナップショットの閲覧、ファイル検索（ワイルドカード、正規表現、フルパス）、選択的復元。NTFS/POSIX ACL、フォルダーのタイムスタンプ、Windows の作成日時を復元し、検証パスも選択可能
- **マシンバックアップからファイルを取り出し**：ディスクイメージ内の NTFS ボリュームを閲覧して復元（master では FAT、exFAT、ext2/3/4、XFS も）
- **Undelete** と **Roll back**（v0.7.0 から）：バックアップにはあるがディスクにないファイルを見つけて戻す、またはセットのフォルダーを選んだバックアップ時点に戻す（事前プレビュー、事後の Undo 付き）。フォルダーとマシンのセットに対応。[GUI マニュアル](docs/manual/GUI.md#10-undelete)を参照
- 実行中のバックアップの **Pause、Resume、Stop**
- **Run as Service**：Preferences > Advanced からバックグラウンドサービスのインストール・開始・停止・削除ができ、誰もサインインしていなくてもスケジュールが動きます（Windows は v0.7.0 から、Linux は master で）
- ライブ進捗カード：開始時刻と終了予想、処理済み/全体のファイル数、処理速度と実際のアップロード速度、現在のファイル、全ファイルを読んでいる理由
- Windows では VSS（ボリュームシャドウコピー）、Linux では elastio-snap/dattobd のブロックスナップショット
- メール通知とバックアップ後のアクション、Backup Set の複製、設定のエクスポートとインポート
- 複数の PBS サーバー、証明書フィンガープリントの固定（初回接続時に信頼）、API トークンまたはユーザー/パスワードでのログイン
- **クライアント側暗号化**：サーバーごとの PBS 鍵ファイル（AES-256-GCM、マニフェスト署名）、パスフレーズ保護は任意
- Backup Sets の変更に管理者権限が必要 (Windows)、Restart as administrator 付き
- 診断用のデバッグログ

### コマンドラインツール
- `proxmoxbackup-directory`：重複排除と `-change-detection-mode legacy|data|metadata` 付きのフォルダー (pxar) バックアップ
- `proxmoxbackup-machine`：Windows (VSS) と Linux (elastio-snap/dattobd) のディスク全体のオンラインバックアップ（fidx、増分、`-readfreespace` を付けない限り空き領域をスキップ）
- `proxmoxbackup-nbd`：ディスクバックアップ復元用の NBD サーバー (Linux)
- `ProxmoxBackupClientSVC` (Windows) と `proxmoxbackupclient`（Linux パッケージ）：GUI で定義した Backup Sets に対する `-list-sets`、`-undelete`、`-rollback`（master）

最初の 3 つは、暗号化スナップショット用に `-keyfile` と `-keyfile-passphrase` を受け付けます。完全なリファレンスは[コマンドラインマニュアル](docs/manual/CLI.md)にあります。

### 📸 スクリーンショット

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Backup Sets**：繰り返し/手動のトリガーバッジとディレクトリ/マシンの種類バッジ付きの名前付きジョブ。それぞれに Run Now

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**ライブ進捗**：スループット、残り時間、新規チャンクと再利用チャンクの数

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**復元**：PBS サーバーと Backup ID を選び、スナップショットを一覧表示、またはファイル名で横断検索

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/backup-sets-vm-badge.png" width="280">](docs/screenshots/backup-sets-vm-badge.png)
**Proxmox VE の VM としての Backup Sets**：`vm` スナップショットとして保存するセットには `MACHINE (VM)` バッジ

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore-progress.png" width="280">](docs/screenshots/gui-restore-progress.png)
**選択的復元**：スナップショットのツリーからファイル/フォルダーを選び、元の場所か別の場所へ復元

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Reports**：詳細パネル付きの全実行履歴（所要時間、チャンク、バックアップしたフォルダー）

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-servers.png" width="280">](docs/screenshots/gui-preferences-servers.png)
**マルチ PBS**：複数の PBS サーバーを管理し、接続をテストし、既定を選択

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-edit-server.png" width="280">](docs/screenshots/gui-preferences-edit-server.png)
**サーバーの追加/編集**：URL、ユーザー/パスワードまたは API トークン、データストア、名前空間

</td>
<td width="33%">

[<img src="docs/screenshots/gui-theme-amber.png" width="280">](docs/screenshots/gui-theme-amber.png)
**テーマ**：5 つの組み込みカラーとカスタム色。アプリ全体の色がその場で変わります

</td>
</tr>
</table>

ほかのスクリーンショット（Message Log、詳細オプション、全テーマ、各言語の画面）は[英語版 README](README.md#-screenshots) にあります。

### システムファイルの自動除外（フォルダーバックアップ）
フォルダーバックアップでは、次のものを自動的に除外します：

**システムフォルダー：** 場所を問わず `System Volume Information`（VSS の保存領域で 100 GB を超えることもある）と `$RECYCLE.BIN`。`Recovery` はドライブのルートにある場合のみ（master。以前のバージョンはどの階層の同名フォルダーもスキップ）。`.pbs-rollback`（Roll back の退避フォルダー）。
**システムファイル：** `pagefile.sys`、`hiberfil.sys`、`swapfile.sys`、`DumpStack.log.tmp`。

**なぜ重要か：** ドライブの使用量が 1.03 TB と表示されていても、実際のファイルは約 141 GB ということがあります。除外しないと VSS スナップショットまでバックアップされ（容量と時間の無駄）、除外すればサイズが実データと一致します。

**推奨：** ファイル単位のバックアップにはこれらの除外が効く**フォルダー**バックアップを、ベアメタル復元には別の Backup Set で**マシン**バックアップを使ってください（ディスク全体のイメージで、空き領域とページファイル・休止状態ファイルはゼロとして入ります）。

### セキュリティと品質
- 全チャンクのクライアント側暗号化（任意）と署名付きマニフェスト
- 設定フォルダーは Windows では管理者のみ、Linux では所有者のみがアクセス可能（master）
- ローカルのサービス API は 127.0.0.1 のみで待ち受け、トークンが必要で、サービス起動のたびに更新
- 入力検証と資格情報のサニタイズ
- パストラバーサル対策
- 指数バックオフ付きの再試行
- 全モジュールのエラー処理と単体テスト、さらに任意のライブテストと実際の PBS に対するエンドツーエンドテスト

## 🔐 クライアント側暗号化

スナップショットは、マシンを出る前に Proxmox Backup Server の鍵ファイルで暗号化できます（`proxmox-backup-client key create` が書く形式なので、鍵は公式クライアントと相互に使えます）。各チャンクは AES-256-GCM で暗号化され、マニフェストには署名が付きます。PBS サーバーが鍵を目にすることはありません。

**GUI：** **Preferences > Account Information** を開き、サーバーを編集して **Encryption** タブを使います。既存の鍵ファイルを選ぶか、**Generate...** で新しく作ります（詳細オプションでパスフレーズ保護）。Reports には各バックアップが暗号化されたかどうかが表示されます。**鍵ファイルがなければスナップショットは復元できません。このマシン以外の安全な場所にコピーを保管してください。**

**パスフレーズ付きの鍵：** **Ask me** はアプリのセッションごとに 1 回尋ねてメモリにだけ保持し、スケジュール実行やサービス経由のバックアップは分かりやすいメッセージを出してすぐ失敗します。**Remember on this computer** は PBS トークンと同じく設定に保存するので、無人のスケジュールジョブやバックグラウンドサービス向けです。

**コマンドライン：**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```

**ベアメタル復元：** Clonezilla ISO が鍵（なし、USB メモリ上の鍵ファイル、パスの入力）とパスフレーズを尋ねます。鍵が ISO に書き込まれることはありません。[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md) を参照してください。注意：`config.json` には PBS トークン、パスワード、記憶したパスフレーズが平文で入っています。既知の制限は[英語版 README](README.md#-client-side-encryption) を参照してください。

## より速いフォルダーバックアップ：変更検出

既定の Legacy モードでは毎回すべてのファイルを読みます。Backup Sets（とコマンドライン）では、公式 `proxmox-backup-client` の変更検出モードを使えます：

| モード | 読み込み | 保存形式 |
|---|---|---|
| **Legacy**（既定） | 全ファイル | フォルダーごとに `<name>.pxar.didx` 1 つ（従来どおり） |
| **Data** | 全ファイル | 分割アーカイブ：`<name>.mpxar.didx`（メタデータ）+ `<name>.ppxar.didx`（ファイル内容） |
| **Metadata** | 新規・変更ファイルのみ | 同じ分割アーカイブ。サイズ、更新日時（ナノ秒）、モード、所有者が前回と一致するファイルは読まずに前回から引き継ぐ |

Metadata モードの初回はすべてを読み、以降は変わった分だけを読みます。サイズと更新日時が変わらない変更には気付かないため、セットでは **N 回ごとに全ファイルを読む**設定もできます。再利用は、同じ方式・同じ鍵で暗号化された前回のバックアップからのみ行われます。

**GUI：** Backup Set を編集し、**Destination** タブの **Change detection**。フォルダーのセット専用で、マシン（ディスク全体）のバックアップには影響しません。単発バックアップは Legacy を使います。**セットごとに Backup ID を 1 つ**にしてください。

**コマンドライン：** `-change-detection-mode legacy|data|metadata`、または JSON 設定の `"change-detection-mode"`。

**セットを Data か Metadata に切り替える前に**、復元に使うすべてのマシンを v0.6.0 以降に更新してください。それより古いビルドは分割アーカイブを読めません。

## 🚀 クイックスタート (GUI)

1. インストール：Windows は MSI、Linux は `.deb` か `.rpm`、または[リリース](https://github.com/mjb-is/proxmoxbackupclient_go/releases)の zip か tar.gz を展開
2. 起動する。Windows では VSS、マシンバックアップ、Backup Sets の変更のために**管理者として実行**します。Linux では自分のファイルのフォルダーバックアップは通常ユーザーで動きます。マシンバックアップとブロックスナップショットには **Proxmox Backup Client (administrator)** を使います（または root で実行）
3. **Preferences > Account Information** で PBS サーバーを追加し、**Test** する
4. Backup Set（または単発バックアップ）を作り、バックアップするフォルダーかディスクを選ぶ
5. 実行する。誰もサインインしていなくてもスケジュールを動かすには **Preferences > Advanced > Run as Service** をオンにする

各手順は [GUI マニュアル](docs/manual/GUI.md)で詳しく説明しています。

## マシン全体のライブバックアップ

`proxmoxbackup-machine`（と GUI のマシンバックアップ）は、Windows 10/11、Windows Server、または Linux マシンのシステム全体を、停止させずにバックアップします。コマンドの書き方はフォルダーバックアップとほぼ同じで、`-backupdir` の代わりに `-backupdev` を使います（複数ディスクなら繰り返し指定）。例：

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

このコマンドはディスク 0 を調べ、マウント中のパーティションをすべて検出して VSS スナップショットを取り、ディスク全体の起動可能なバックアップイメージを fidx として作ります。空き領域とページファイル・休止状態ファイルは読まずにゼロとして入ります（master では Linux の ext2/3/4・XFS の空き領域も同様）。`-readfreespace` を付けると、フォレンジック用にすべてを読みます。次回のバックアップは増分で、ハッシュ計算は並列化されているため 1 GB/秒も容易に出ます。

### Linux マシンバックアップの前提条件（VSS の代わり）

Linux には VSS がないため、このプロジェクトは `elastio-snap` カーネルモジュール（`dattobd` のフォーク）を使い、書き込み中のブロックデバイスから一貫したある時点のスナップショットを取ります。元の `elastio/elastio-snap` リポジトリはアーカイブ済みで、インストールすべきはメンテナンスされている [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap) です。`dattobd` がすでに読み込まれていれば、それも使えます。

次の 2 つはどちらも必須で、欠けると Linux のマシンバックアップは実行を拒否します：

1. **カーネルモジュールがインストールされ、読み込まれていること。** 公式のリポジトリパッケージによる手順（[INSTALL.md](https://github.com/Axcient/elastio-snap/blob/develop/INSTALL.md)）は、少なくとも Ubuntu 22.04 ではリンク切れです（2026-09-24 確認）。ソースからのビルドは確実に動きます：
   ```bash
   sudo apt-get install linux-headers-$(uname -r) build-essential   # Debian/Ubuntu
   git clone --depth 1 https://github.com/Axcient/elastio-snap.git
   cd elastio-snap
   sudo make
   sudo make install
   sudo depmod -a     # required - `make install` only copies the .ko, it never
                       # runs depmod, so a bare `modprobe elastio-snap` right
                       # after a fresh install fails with "not found in
                       # directory" even though the file is right there
                       # (confirmed live 2026-09-24)
   sudo modprobe elastio-snap
   lsmod | grep elastio-snap   # confirms it loaded
   ```
   これは今回の起動中だけ有効です。起動のたびに自動で読み込むには、ディストリビューションのドキュメントを参照してください（例：`/etc/modules-load.d/` への登録）。

2. **バックアップ処理自体が root (EUID 0) で動くこと。** モジュールが読み込まれていても、`CreateVSSSnapshot`（名前は Windows 風ですが Linux のスナップショット処理です）は root 以外では実行を拒否します。`disk` グループへの所属などでは代わりになりません。マシンバックアップには **Proxmox Backup Client (administrator)** メニュー項目、バックグラウンドサービス、または `sudo`（か root）で GUI や `proxmoxbackup-machine` を実行してください。通常のファイル/ディレクトリのバックアップにはこの要件はありません。

マシンバックアップが `backup device /dev/sdX: open /dev/sdX: permission denied` で失敗する場合、たいていはユーザーを `disk` グループに入れれば読み取りはできるようになります（`sudo usermod -aG disk $USER` の後、ログインし直す）。ただしそれだけでは本当のスナップショットには足りず、すぐに上の root 要件に引っかかります。

### ディスクバックアップからのファイル復元

GUI の Restore ページは、マシンバックアップをフォルダーバックアップと同じように開きます。イメージ内の各ボリュームが、閲覧して復元できるフォルダーになります（NTFS は v0.8.0 から、FAT、exFAT、ext2/3/4、XFS は master で）。マウントは不要です。

Linux では、nbd ツールでディスクバックアップをブロックデバイスとして接続することもできます。まず `modprobe nbd max_part=0` を実行してください（理由は不明ですが、`max_part != 0` ではパーティション検出が無限ループします）。NBD ツールは、VM でも host でも、固定ディスクのバックアップなら何でも接続できます（PVE のバックアップにも使えます）。コマンドの例：

`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

`-path` を省くと、fidx ファイルを選ぶためのターミナル UI が表示されます。

> 重要な処理を動かしているマシンでは使わないでください（壊れたファイルシステムが OS をクラッシュさせることがあり、Proxmox VE がこの用途に QEMU インスタンスを使うのはそのためです）。

pbsnbd を止める前に、nbd ディスク上のものは必ずすべてアンマウントしてください。さもないと、使用中でアンマウントできないパーティションが残る可能性が高くなります。`Device or resource busy` エラーが出たら、`nbd-client -d /dev/nbd0` で強制切断するか、再起動してください。

### 物理マシンへの復元

このフォークは、完全自動の **「PBS Bare Metal Restore」** 起動メニュー項目を備えたパッチ済み Clonezilla Live ISO を提供します（下の「Clonezilla Live ISO」を参照）。メニューの先頭にあり、タイムアウトで既定として起動し、Clonezilla の言語/キーボードの質問を飛ばします。PBS の接続情報を入力し、スナップショットを選び、自動検出された復元先ディスクを確認すれば、ネットワーク設定、NBD 接続、復元、自動再起動まで追加の入力なしで進みます。

以前からある半手動の方法も同じ ISO に残っており、Clonezilla の通常のモード選択メニューに「pbs-nbd」項目として入っています。PBS の接続情報を案内に沿って入力すると、選んだスナップショットをバックグラウンドで `/dev/nbd0` に接続し、あとは Clonezilla 標準のディスク復元ウィザードを手動で操作します。

## 使い方：ディレクトリバックアップ

Windows と Linux の例、終了コード、スケジュール実行を含む完全なリファレンスは **[コマンドラインマニュアル](docs/manual/CLI.md)** にあります。典型的なコマンド：

```shell
proxmoxbackup-directory.exe -baseurl "https://yourpbshost:8007" -certfingerprint pbsfingerprint -authid "user@realm!apiid" -secret "apisecret" -backupdir "C:\path\to\backup" -datastore "datastorename"
```

```
proxmoxbackup-directory
  -baseurl string            PBS URL, example: https://192.168.1.10:8007
  -certfingerprint string    Certificate fingerprint (asked for interactively when omitted)
  -authid string             API token ID (user@realm!token)
  -secret string             API token secret
  -pbsusername string        PBS user for ticket login (with -pbspassword; overrides -authid/-secret)
  -pbspassword string        Password for -pbsusername (asked for when omitted)
  -datastore string          Datastore name
  -namespace string          Namespace (optional)
  -backup-id string          Backup ID (default: the hostname)
  -backupdir string          Source directory; repeat for several (each becomes group <id>_<path>)
  -change-detection-mode     legacy (default), data or metadata
  -novss                     Do not snapshot the source (VSS on Windows, elastio-snap/dattobd on Linux)
  -keyfile string            PBS encryption key file
  -keyfile-passphrase string Passphrase for a protected key (asked for when omitted)
  -backupstream string       Back up standard input as <name>.didx instead of a directory
  -pxarout string            Also write the archive to a local file (debugging)
  -mail-host, -mail-port, -mail-username, -mail-password, -mail-insecure,
  -mail-from, -mail-to, -mail-subject-template, -mail-body-template
                             Email report (optional)
  -config string             JSON config file; flags given on the command line override it
```

公式クライアントの `PBS_REPOSITORY`、`PBS_SERVER`、`PBS_PORT`、`PBS_DATASTORE`、`PBS_AUTH_ID`、`PBS_PASSWORD`、`PBS_FINGERPRINT` 環境変数も読み込みます。JSON 設定の例が用意されているので、必要な項目だけ埋めてください。メールの件名と本文には [Go のテンプレートエンジン](https://pkg.go.dev/text/template)を使います。使える変数（`.NewChunks`、`.ReusedChunks`、`.Status` など）は[英語版 README](README.md#usage-directory-backup) を参照してください。

**ストリームバックアップ：** `-backupstream` で pxar の代わりにストリームをバックアップできます。例：`mysqldump yourdatabase | ./proxmoxbackup-directory -backupstream yourdatabase.sql [other options]`。

**既知の問題：** Windows Defender のマルウェア対策が有効だと、バックアップが最大 25% 遅くなります。Windows のコマンドラインツールはプロセスロックを取るため、実行中に 2 つ目のバックアップを始めると「Another backup job is running」と表示して終了します。Linux にはこのロックがないので、2 つのコマンドライン実行を同時刻にスケジュールしないでください。

## 📋 動作要件

- Windows 10/11 または Windows Server（64 ビット）。VSS とマシンバックアップには管理者権限
- または Linux（パッケージは Debian、Ubuntu、Mint、Fedora 向け）。GUI には WebKitGTK ランタイム、一貫したマシンバックアップには root と elastio-snap または dattobd カーネルモジュール
- Proxmox Backup Server へのネットワーク接続

## 🔨 ソースからのビルド

GUI とバックグラウンドサービス、コマンドラインツール、MSI、Linux パッケージの完全な手順は **[BUILD.md](BUILD.md)** にあります。概要としては、Go 1.25 以降と Wails CLI v2.13 で：

```bash
cd gui
wails build                                    # the app, in gui/build/bin
go build -tags service -o build/bin/ProxmoxBackupClientSVC.exe   # the background service
```

ビルド済みのフロントエンド (`gui/frontend/dist`) はリポジトリに含まれているので、Node.js が必要なのはフロントエンドを変更するときだけです。テスト：`make test`。

## 🔧 高度な使い方とガイド

### マルチ PBS（複数の PBS サーバー）

複数の PBS サーバーを設定し、Backup Set や単発バックアップごとに送り先を選べます（例：`C:\Users` は毎日高速な SSD の PBS へ、`D:\` は毎週大容量の PBS へ、さらに DR 用サーバー）。

- **[ユーザーガイド](MULTI_PBS_USER_GUIDE.md)**：サーバーの追加とテスト、既定サーバー、バックアップごとのサーバー選択、FAQ、トラブルシューティング。
- **[実装ガイド](MULTI_PBS_GUIDE.md)**：データモデル、単一 PBS 構成からの自動移行、バックエンドのメソッド。

従来の単一 PBS 構成は、初回読み込み時に自動で `default` サーバーへ移行されます。

### Clonezilla Live ISO（ベアメタル復元）

レスキュー手順は、標準の Clonezilla Live ISO に `pbsnbd` / `machinebackup` バイナリと 2 つのメニュー項目を加えるパッチを当てて作ります（CD 起動、`dd` での USB、UEFI に対応）：

- **`ocs-pbs-nbd`**：Clonezilla のモード選択メニューに追加される **「pbs-nbd」** 項目。スナップショットを接続した後は、Clonezilla 通常のディスク復元ウィザードを手動で操作します。
- **`ocs-pbs-bare-metal-restore`**：先頭にあり既定で起動する新しいメニュー項目 **「Proxmox Backup Client Go - PBS Bare Metal Restore」**。PBS の資格情報の入力から復元完了と自動再起動までを一気に進めます。まず DHCP を試し（失敗したときだけ固定 IP を尋ねる）、既定で NTP 同期を行い、復元先ディスクを自動検出します。Windows と Linux のどちらのバックアップでも同じように動きます。

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

詳細は **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)** にあります。

### Windows GUI のビルド

**ネイティブ Windows：** ツールチェーン（Go、MinGW、Wails）と、アプリ・サービス・MSI のビルドコマンドは **[BUILD.md](BUILD.md)** を参照。**Docker（Linux でビルドする場合）：** `./build_gui_windows_docker.sh` が使い捨ての `golang` コンテナーで `ProxmoxBackupClientGO.exe` をビルドします。バックグラウンドサービスと MSI はビルドしません。

### 機能の状況、変更履歴、内部ドキュメント

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)**：完了・一部完了・未完了の機能と、どのバージョンからか。
- **[CHANGELOG.md](CHANGELOG.md)**：バージョンごとの変更履歴。master の内容は [Unreleased]。
- **[TODO.md](TODO.md)**：未着手のロードマップとアイデア。
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)**：各リリースの内容。
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**：MSI のアンインストールダイアログ（設定を残す/削除する）とそのテスト計画。
- **[AGENTS.md](AGENTS.md)**：コントリビューターとコーディングエージェント向けの、リポジトリ構成と規約。

## 🌐 イタリアでの技術サポート

本プロジェクトとその**すべて**の貢献は今後も公開され、GPLv3 ライセンスのもとにあります。そのうえで、本プロジェクトと最新のマシンバックアップ機能の主要スポンサーである E.T.I. Srl ( https://etitech.net ) が、必要とする方に Proxmox の導入全般、特に Windows バックアップツールのサポートを提供できます。

「コミュニティ版」と「エンタープライズ版」が分かれることは決してなく、技術サポートは独立したサービスです。依頼されたカスタマイズは、開発が有償であっても、プロジェクト全体と同じく GPLv3 で公開されます。

## 🖥️ GUI の出典

**Proxmox Backup Client GUI** は、**[RDEM Systems](https://www.rdem-systems.com/)** が開発・保守する **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)** をベースにしています。

この GUI（もとは本プロジェクトのフォーク）はこのリポジトリに統合されました。GUI とその全機能を含むコード全体は、GPLv3 ライセンスのオープンソースのままです。RDEM Systems が GUI 開発のスポンサーとなり、商用サポートを提供しています。

## ⚠️ 警告

このソフトウェアは「現状のまま」提供されます。信頼性を目指してはいますが、データの損失や損傷について一切の責任を負いません。本番環境で頼る前に、必ずバックアップをテストし、復元を確認してください。

このソフトウェアはまだアルファ品質であり、ソースファイルを含め、いかなる損害やデータ損失についても責任を負いません。

## 📄 ライセンス

GPLv3：[LICENSE](LICENSE) ファイルを参照してください。

## 🏷️ ブランディング

機能追加や修正の**コミットを 5 件以上**貢献したコントリビューターは、商用利用のためのブランディング情報を追加してもらう権利があります。

条件は、ブランディングが示す企業が次のいずれも行って**いない**ことだけです：

- マルウェアキャンペーン
- 戦争を助長する事業（西側諸国を含むすべての国が対象）
- 詐欺
- データ窃取
- 児童の人身売買
- 暴力
- 差別
- 薬物
- 一般に違法と認められるあらゆる活動

いずれかのコントリビューターに苦情が寄せられた場合は連絡を試みます。妥当な説明がなければ、その特典を**直ちに打ち切ります**。

**GPLv3 ライセンスは引き続き有効**であり、プロジェクトをフォークして独自の実行ファイルをビルドするのは自由です。

## 🤝 貢献

GUI は完全に実装されましたが、引き続き貢献を歓迎します。特に：

1. 暗号化：機械翻訳された暗号化関連の文言のネイティブによる見直しと、鍵ファイル以外の鍵管理（例：Windows DPAPI や資格情報マネージャー）
2. 物理から仮想 (P2V) への移行、ベアメタルバックアップの仮想マシンへの復元（`vm` スナップショット形式で基本は動作し、Windows UEFI ディスクで確認済み。NIC、TPM、virtio ドライバー、実機から取った CPU/RAM、SATA 以外のディスクバスはまだ未対応）
3. チャンクの非同期アップロード / マルチコアアップロード（マルチコア圧縮はマシンバックアップで実装済み）
4. pxar 形式に Windows セキュリティ記述子を含む新しい種類のエントリーを加える、Proxmox 側のパッチ
5. Windows のシンボリックリンク対応
6. 思いついた面白いことなら何でも :)

## Proxmox Backup Client GO のコントリビューターについて

Proxmox Backup Client GO のコントリビューターがこのプロジェクトを開発・保守しています。このソフトウェアは NTP/NTS インフラと、コミュニティのリファレンスに掲載された [11 の公開 NTS サーバー](https://github.com/jauderho/nts-servers) を利用しています。

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**
