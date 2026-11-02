# Proxmox Backup Client：Proxmox Backup Server 用の Windows・Linux クライアント

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

> ℹ️ これは[英語版 README](README.md) の翻訳です。内容が異なる場合は、英語版が最新です。

> ⚠️ **商標および提携関係に関するお知らせ。** 本プロジェクトは**独立したコミュニティプロジェクト**であり、Proxmox の公式製品ではありません。**Proxmox Server Solutions GmbH との提携関係はなく、同社による承認、推奨、後援も受けていません。** **Proxmox®** は Proxmox Server Solutions GmbH の EU、米国およびその他の国における登録商標であり、本書では互換性を説明する目的に**限って**使用しています。Proxmox 自身の商標ガイドライン（登録商標は製品名やアプリ名に使用してはならず、たとえば「Proxmox XY App」ではなく「XY App for Proxmox Virtual Environment」とする）に従い、**ブランドなしクライアントの名称変更を進めています**。現在 *Proxmox Backup Client* と呼ばれている製品は、Proxmox の商標を含まない名称に移行します。正確なルールについては [Proxmox メディアキット](https://www.proxmox.com/en/about/company-details/media-kit)を参照してください。

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/tizbac/proxmoxbackupclient_go)](https://github.com/tizbac/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Proxmox Backup Client は、Proxmox Backup Server (PBS) 用のオープンソース (GPL-3.0) バックアップクライアントで、Windows と Linux に対応しています。** フォルダーは閲覧可能なファイルアーカイブとして、マシン全体は起動可能なディスクイメージとして、スケジュールに従って、またはオンデマンドでバックアップします。単一のファイルからマシン全体まで、何でも復元できます。復元先は同じコンピューター、交換用のマシン、または Proxmox VE の仮想マシンです。

これは**ツール群**です。

- **Proxmox Backup Client GUI**（RDEM Systems が Nimbus Backup GUI として開発し、mjb-is が改良・拡張したもの）：Windows と Linux 用のアプリです。それぞれ複数のスケジュールを持てる **Backup Sets**（バックアップセット）、一貫性のあるスナップショット（Windows では VSS、Linux では elastio-snap/dattobd）を使ったフォルダーおよびディスク全体のバックアップ、フォルダーバックアップとマシンバックアップからのファイル復元、**Undelete**（削除の取り消し）と **Roll back**（ロールバック）、複数の PBS サーバー、クライアント側暗号化、そして誰もサインインしていなくてもスケジュールを実行するバックグラウンドサービスを備えています。
- **`proxmoxbackup-directory`**：重複排除と変更検出を備えた、コマンドラインのフォルダー (pxar) バックアップ。
- **`proxmoxbackup-machine`**：Windows および Linux マシンの、コマンドラインによるディスク全体のライブバックアップ (fidx)。イメージは復元後に起動できます。
- **`proxmoxbackup-nbd`**：ディスクバックアップを NBD ブロックデバイスとして接続します (Linux)。
- **ベアメタル復元 ISO**：Clonezilla Live をベースにしており、電源投入から再起動までガイドに従って復元できます。

> キーワード：proxmox backup client windows · PBS クライアント · Windows VSS バックアップ · イミュータブルなオフサイトバックアップ · 物理から仮想へ (P2V) · ベアメタル復元 · Proxmox Backup Server インターフェース。

## 🆕 2.0 の新機能

バージョン 2.0 では、GUI、バックグラウンドサービス、バックアップエンジンに関する大量の作業がプロジェクトに統合されました（[CHANGELOG](CHANGELOG.md) を参照）。1.2 と比べた変更点は次のとおりです。

- **アプリは管理者権限でバックグラウンドサービスに接続します。** サービスがインストールされていると、コンピューターの PBS サーバーと **Backup Sets**（バックアップセット）はサービスに属し、サービスの設定フォルダーを読めるのはサービスだけになります。アプリは接続のために一度だけ管理者権限を求めます（Windows では UAC、Linux では pkexec。標準アカウントでは管理者の名前とパスワード）。昇格するのは小さなヘルパーだけで、アプリはサービスのトークンをメモリ上にのみ保持します。このプロンプトに応答できないユーザーは **Use my own settings**（自分の設定を使う）を選び、自分のサーバーと Backup Sets を使い続けられます。これらはアプリが開いている間に実行されます。後で **Connect**（接続）を使うと、それらをコンピューターの設定にコピーするかどうかを尋ねます。アプリ自体は管理者権限なしで動作し、操作に必要な場合は **Restart as administrator**（管理者として再起動）を提示します。[GUI マニュアル](docs/manual/GUI.md#connecting-to-the-background-service)の第 15 節を参照してください。
- **アプリは通信相手が本当にサービスであることを確認します。** サービスのローカル API に何かを送る前に、アプリは応答している相手を確認します。Windows ではサービス自身のプロセス、Linux では root が所有するソケットです。サービスが停止している間にそのポートを奪ったプログラムは、何も受け取れません。
- **それぞれ複数のスケジュールを持てる Backup Sets**：数分ごとから年 1 回まで（月の最終日、第 2 火曜日など）設定でき、各スケジュールは平易な言葉で、次の 3 回の実行予定とともに表示されます。
- **増分フォルダーバックアップ**：公式クライアントの変更検出（Data モードと Metadata モード）を使い、変更されていないファイルは再び読み込みません。
- **より高速なマシンバックアップ**：空き領域、ページングファイル、休止状態ファイルをスキップし、読み取れないセクターは失敗させずにゼロとして書き込みます。
- **複数のバックアップを同時に実行**（**Maximum Concurrent Backups**（最大同時バックアップ数）、1 から 8、既定値 3）。バックグラウンドサービスでも同様で、それぞれに独自のスナップショット、ログ、**Pause**（一時停止）、**Stop**（停止）があります。
- **速度制限**：PBS へのアップロード、PBS からのダウンロード、ディスク読み取りのそれぞれに設定でき、各制限は同時に実行中のすべての処理で共有されます。
- **Undelete**（削除の取り消し）と **Roll back**（ロールバック）：フォルダーとマシンの Backup Sets で、GUI からもコマンドラインからも使えます。
- **マシンバックアップからのファイル復元**：ディスクイメージ内の NTFS、FAT、exFAT、ext2/3/4、XFS ボリュームを閲覧し、個別のファイルを復元できます。
- **保管できる暗号化キー**：コピーを保存し、QR コード付きの紙のキーを印刷し、ファイル、カメラ、写真、入力したテキストからキーをインポートできます。
- **マシンの Backup Sets はディスクを識別情報で固定します**。ハードウェアの変更後にディスク番号が変わっても同じディスクがバックアップされ、ディスクが見つからない場合は別のディスクをバックアップせず、その実行が失敗します。
- **マシンバックアップからパーティションを除外できます**：各ディスクの下で、バックアップするパーティションにチェックを入れます。除外したパーティションは読み取りもスナップショットもされず、イメージにはゼロとして格納されます。そのため復元すると、同じ位置に同じサイズで、空の未フォーマット状態で戻ります。ディスク選択画面では、復元したディスクの起動に必要なパーティションが示されます。
- **elastio-snap または dattobd による Linux マシンバックアップ。** Linux のマシンセットはスナップショットを無効にしてディスクをライブで読み取ることもできますが、所有者が「バックアップに不整合が生じる可能性を受け入れる」にチェックを入れた場合に限ります。
- **ユーザーごとにカラーテーマを選択可能**（ブランドなしのアプリ）。ブランド版アプリはそのブランドの色を保ちます。
- **Backup Set の有効化と無効化**をメニューだけでなくエディターからも行えます。また、保存した内容のままではセットが実行されない場合（無効、有効なスケジュールなし、スナップショットの問題、ディスクの欠落）には、保存時に警告します。
- **Linux パッケージ**（`.deb`、`.rpm`、Arch）。パッケージ名とサービス名は `proxmoxbackupclient` で、バックグラウンドサービスが付属します。
- **ガイド付きのベアメタル復元 ISO**：空の仮想マシンへの復元 (P2V) にも対応します。Proxmox VE の仮想マシンとしてバックアップしたマシンは、Proxmox VE 自身がバックアップした VM と同じように、Proxmox VE で PBS から直接復元することもできます。
- **ブランド版ビルド**：同じバイナリが Proxmox Backup Client、Nimbus Backup、Acme Backup、Etitech Backup の各インストーラーとして提供されます。
- **18 のインターフェース言語。**

## ⭐ 機能と実測結果

| | 結果 |
|---|---|
| **増分フォルダーバックアップ** | 761,866 ファイル、480.9 GB のファイル共有：初回は 4 時間 17 分、以降は毎回 10 から 15 分。変更されていないファイルは開かれることさえありません（公式クライアントと同じ Metadata 変更検出）。 |
| **高速なマシンバックアップ** | 空き領域、ページングファイル、休止状態ファイルは読み込まれず、空白のディスク領域はハッシュ計算されません。使用量 373 GB の 1 TB ノート PC ディスクでは、1 時間 23 分から 1 時間 44 分かかっていたものが 38 分に短縮されました。 |
| **低速回線と従量制回線** | 新しいデータだけを圧縮して送信し、実際にネットワークを通過した量をアプリが表示します。4G 回線（上り 4.8 Mbit/s）の遠隔拠点にある小型 Linux PC：128 GB のディスクイメージ全体とホームフォルダーを 11 分でバックアップし、転送量は約 172 MiB でした。 |
| **マシンバックアップからのファイル復元** | 430,000 ファイルを含むノート PC の 127 GB の C: ドライブが、4 から 15 秒で一覧表示されます。事前のマウントやダウンロードは一切不要です。 |
| **ベアメタル復元** | ISO を起動し（USB メモリ、CD、ネットワーク）、PBS の情報を入力してバックアップを選ぶだけ：32 GB の Windows マシンが、電源投入から 8 分後にログイン画面まで戻りました。 |
| **Undelete**（削除の取り消し） | 761,000 ファイルの共有から 39 GB が削除された後、過去 7 日間のバックアップを対象にした Undelete 検索で、約 1 分でまさにそのファイルを見つけ出し、元に戻しました。 |
| **復元の忠実度** | Windows では NTFS のアクセス許可、属性、代替データストリーム、拡張属性、作成日時とフォルダーの日時。Linux ではモード、所有者、ACL、拡張属性、シンボリックリンク。 |

## 📖 ユーザーマニュアル

| マニュアル | 内容 |
|---|---|
| **[GUI マニュアル](docs/manual/GUI.md)** | インストール、PBS サーバー、**Backup Sets**（バックアップセット）とスケジュール、増分バックアップ、進行状況、復元、**Undelete**（削除の取り消し）、**Roll back**（ロールバック）、**Reports**（レポート）、ログ、**Preferences**（設定）、バックグラウンドサービス、トラブルシューティング |
| **[コマンドラインマニュアル](docs/manual/CLI.md)** | `proxmoxbackup-directory`、`proxmoxbackup-machine`、`proxmoxbackup-nbd`：すべてのフラグ、設定ファイル、終了コード、スケジュール設定。コマンドラインからの Undelete と Roll back |
| **[ベアメタル復元マニュアル](docs/manual/BMR.md)** | 復元 ISO を手順ごとに解説。Proxmox VE への `vm` バックアップ、物理から仮想へ、異なるハードウェアへの復元 |
| [復元シナリオ](docs/RESTORE_GUIDE.md) | どの場面でどの復元を使うか |
| [マニュアル索引](docs/manual/README.md) | マニュアルの概要 |

同じマニュアルがアプリに組み込まれているため（**Help**（ヘルプ）、F1）、オフラインでも使え、インストールされているバージョンと内容が一致します。

## 📦 ダウンロード

👉 **[最新リリースをダウンロード](https://github.com/tizbac/proxmoxbackupclient_go/releases)**

| お使いの環境 | 使うもの |
|---|---|
| Windows | **MSI インストーラー** `ProxmoxBackupClient-<version>.msi`。アプリとバックグラウンドサービスを Program Files にインストールし、サービスを開始して、スタートメニューに項目を追加します。ブランド版インストーラー（`NimbusBackup-<version>.msi`、`AcmeBackup-<version>.msi`、`EtitechBackup-<version>.msi`）は、同じビルドを別のブランドにしたものです |
| Debian、Ubuntu、Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-*.rpm` |
| Arch | [packaging/arch/PKGBUILD](packaging/arch/PKGBUILD) からビルドしたパッケージ（`make pkgarch`） |
| コマンドラインのみ | プラットフォーム別のアーカイブ（`proxmoxbackup-cli-*` と `proxmoxbackupclient_go_*`） |
| 復元するマシン | ベアメタル復元 ISO（[ベアメタル復元マニュアル](docs/manual/BMR.md#3-get-the-iso-and-make-the-boot-media)）、または自分でビルド（[後述](#-bare-metal-restore-iso)） |

リリースに Linux パッケージが含まれていない場合は、`make deb`、`make pkgfedora`、`make pkgarch` でビルドしてください（[BUILD.md](BUILD.md)、[packaging/debian/README.md](packaging/debian/README.md)）。

Linux パッケージは **Proxmox Backup Client** というメニュー項目を 1 つインストールし、アプリを現在のユーザーとして起動します。新規インストールではバックグラウンドサービス（`proxmoxbackupclient.service`）が有効になり、アップグレードでは以前の選択が維持されます。パッケージには polkit (pkexec) が必要で、アプリはこれを使ってサービスに接続します。

> ⚠️ **Windows で「ウイルスが検出されました」（例：`Trojan:Win32/Sabsik.FL.A!ml`）や SmartScreen の警告が表示されますか？**
> これは Go/Wails アプリケーションで**既知の誤検知**であり、ウイルス*ではありません*。接尾辞 `!ml` は機械学習モデルによる検出を示しており、*署名がなく一般的でない*実行ファイルにフラグを立てます。以下の方法でダウンロードを検証してください。

### 🔎 ダウンロードの検証

各リリースには SHA-256 チェックサム（`SHA256SUMS.txt`。リリースページにも記載）と**署名付きの来歴証明 (provenance attestation)** が付いています。これは、ファイルがこのリポジトリの CI によって特定のコミットから生成されたことを示す暗号学的な証明です。

```powershell
Get-FileHash .\ProxmoxBackupClient-<version>.msi -Algorithm SHA256   # compare with SHA256SUMS.txt
gh attestation verify .\ProxmoxBackupClient-<version>.msi --repo tizbac/proxmoxbackupclient_go
```

Linux では：`sha256sum -c SHA256SUMS.txt --ignore-missing`。

リリースのインストーラーが VirusTotal で問題なしと判定された場合は、リリースページにそのレポートへのリンクが掲載されます。

> ℹ️ **コード署名：** Windows バイナリは**まだ Authenticode 署名されていません**（[SignPath Foundation](https://signpath.org) による OSS 証明書を申請中です）。それまでの間、来歴は上記の証明とチェックサムによって確認できます。

## 🚀 クイックスタート (GUI)

1. インストールします：Windows では MSI、Linux では `.deb`、`.rpm`、または Arch パッケージ（[ダウンロード](#-download)）。
2. **Proxmox Backup Client** を起動します。インストーラーがバックグラウンドサービスを設定するため、アプリはサービスに接続するために一度だけ管理者権限を求めます（Windows では UAC、Linux では pkexec によるパスワード入力）。権限を与えられない場合は **Use my own settings**（自分の設定を使う）を選んでください。
3. **Preferences > Account Information**（設定 > アカウント情報）で PBS サーバーを追加し、**Test**（テスト）で確認します。
4. **Backup Set**（バックアップセット）を作成し（または単発のバックアップを作成し）、バックアップするフォルダーまたはディスクを選択します。
5. 実行します。バックグラウンドサービスがあれば、誰かがサインインしているかどうかに関係なくスケジュールが実行されます。

すべての手順は [GUI マニュアル](docs/manual/GUI.md)で説明しています。

### 🪟 無人／サイレントインストール (MSI)

MSI は Windows Installer の標準のサイレントスイッチに加え、すべての既定値に対応するパブリックプロパティを受け付けます。

```powershell
# Silent install with the defaults (service: Automatic + started)
msiexec /i ProxmoxBackupClient-<version>.msi /quiet

# Silent install that installs the service but does NOT enable/autostart it
# (created as Manual and left stopped)
msiexec /i ProxmoxBackupClient-<version>.msi /quiet SERVICEAUTOSTART=0

# Silent install that also applies a settings file exported from another computer
msiexec /i ProxmoxBackupClient-<version>.msi /quiet CONFIGFILE="C:\Deploy\settings.json"

# Custom directory + verbose install log
msiexec /i ProxmoxBackupClient-<version>.msi /quiet INSTALLFOLDER="D:\Apps\ProxmoxBackupClient" /L*v "C:\Temp\install.log"

# Silent uninstall (KEEP_CONFIG=0 also purges %ProgramData%\<name>)
msiexec /x ProxmoxBackupClient-<version>.msi /quiet KEEP_CONFIG=0
```

| プロパティ | 既定値 | 意味 |
| --- | --- | --- |
| `SERVICEAUTOSTART` | `1` | `0` = サービスを **Manual**（手動）としてインストールし、**stopped**（停止）のままにします：インストールはされますが、有効化も自動起動もされません |
| `AUTOSTART` | `1` | `0` = 各ユーザーのサインイン時にアプリをトレイで起動しません |
| `DESKTOPSHORTCUTS` | `1` | `0` = デスクトップショートカットを作成しません（スタートメニュー項目は常に追加されます） |
| `CONFIGFILE` | | **Preferences > Export Settings**（設定 > 設定のエクスポート）で保存したファイルのフルパス：そのファイルの PBS サーバーと Backup Sets が、サービスの開始前にマージされます。エクスポート元のコンピューター名から作られたバックアップ ID は、各コンピューター自身の名前から作られます。[installer/DEPLOYMENT.md](installer/DEPLOYMENT.md) を参照 |
| `KEEP_CONFIG` | `1` | アンインストール時のみ：`0` = 状態ディレクトリ `%ProgramData%\<name>` も削除します |
| `INSTALLFOLDER` | `Program Files` | インストール先ディレクトリ（UI でも変更可能） |

管理者権限で実行してください。終了コード `0` = 成功、`3010` = 成功（再起動が必要）。ブランド版 MSI もまったく同じスイッチを受け付けます。

## ✨ 機能

### GUI
主な機能は [2.0 の新機能](#-new-in-20)に記載しています。そのほかに次の機能があります。
- 18 のインターフェース言語：フランス語、英語、イタリア語、ドイツ語、ポーランド語、スペイン語、ブルガリア語、チェコ語、ギリシャ語、ハンガリー語、ラトビア語、リトアニア語、オランダ語、ポルトガル語、ルーマニア語、スロバキア語、トルコ語、ウクライナ語
- フォルダーまたはディスク全体の**単発バックアップ**。マシンバックアップは `host` として、または Proxmox VE の `vm` バックアップとして保存でき、空き領域も読み取る**フォレンジックコピー**のオプションがあります
- **Restore**（復元）：ファイル検索（ワイルドカード、正規表現、フルパス）、選択的な復元、NTFS/POSIX ACL、代替データストリーム、Windows の作成日時、オプションの検証パスに対応
- 実行中のバックアップの **Pause**（一時停止）、**Resume**（再開）、**Stop**（停止）。Windows ではトレイからも操作でき、バックグラウンドサービスによる実行にも使えます
- ライブ進行状況カード（終了予測、処理済みファイル数、アップロード速度、転送データ量、圧縮率）、**Reports**（レポート）、**Message Log**（メッセージログ）、**View Full Logs**（完全なログを表示）
- メール通知、バックアップ後のアクション、**Backup Set**（バックアップセット）の複製、設定のエクスポートとインポート
- 複数の PBS サーバー、証明書フィンガープリントの固定（初回使用時に信頼）、API トークンまたはユーザー名とパスワードによるログイン
- 5 つのカラーテーマとカスタムカラー。ユーザーごとに選択できます（ブランドなしのアプリの場合。ブランド版アプリはそのブランドの色を保ちます）

### CLI ツール
- `proxmoxbackup-directory`：重複排除と `-change-detection-mode legacy|data|metadata` を備えたフォルダー (pxar) バックアップ
- `proxmoxbackup-machine`：Windows (VSS) および Linux (elastio-snap/dattobd) マシンのディスク全体のライブバックアップ（fidx、増分、`-readfreespace` を指定しない限り空き領域はスキップ、Proxmox VE バックアップ用の `-type vm`、パーティションを除外する `-skippartition <disk>:<number>`）
- `proxmoxbackup-nbd`：ディスクバックアップを復元するための NBD サーバー (Linux)
- `ProxmoxBackupClientSVC` (Windows) と `proxmoxbackupclient`（Linux パッケージ）：GUI で定義した **Backup Sets**（バックアップセット）用の `-list-sets`、`-undelete`、`-rollback`

最初の 3 つは、暗号化スナップショット用に `-keyfile` と `-keyfile-passphrase` を受け付けます。完全なリファレンス：[コマンドラインマニュアル](docs/manual/CLI.md)。

### 📸 スクリーンショット

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Backup Sets**（バックアップセット）：スケジュールと種類のバッジが付いた名前付きのジョブ。それぞれに **Run Now**（今すぐ実行）

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**バックアップの進行状況**：スループット、終了予測、新規チャンクと再利用チャンク

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Restore**（復元）：PBS サーバーとバックアップ ID を選び、スナップショットを一覧表示するか、ファイル名で検索します

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-schedule-tab.png" width="280">](docs/screenshots/gui-schedule-tab.png)
**Schedules**（スケジュール）：セットごとに複数設定でき、次の 3 回の実行予定を表示

</td>
<td width="33%">

[<img src="docs/screenshots/backup-set-as-vm.png" width="280">](docs/screenshots/backup-set-as-vm.png)
**Proxmox VE の VM としてバックアップ**：入力に合わせて PBS のバックアップ ID が表示されます

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Reports**（レポート）：詳細パネル付きの完全な実行履歴

</td>
</tr>
</table>

### スマートなシステム除外（フォルダーバックアップ）
フォルダーバックアップでは、以下が自動的に除外されます。

**システムフォルダー：** `System Volume Information`（VSS の保存領域。100 GB を超えることもあります）と `$RECYCLE.BIN` は場所を問わず除外。`Recovery` はドライブのルートにある場合のみ。`.pbs-rollback`（**Roll back**（ロールバック）の安全用フォルダー）。
**システムファイル：** `pagefile.sys`、`hiberfil.sys`、`swapfile.sys`、`DumpStack.log.tmp`。

**なぜ重要か：** ドライブの使用量が 1.03 TB と表示されていても、実際のファイルは約 141 GB ということがあります。除外しなければバックアップに VSS スナップショットが含まれ（容量と時間の無駄）、除外すればサイズが実データと一致します。

**推奨：** ファイル単位のバックアップには**フォルダー**バックアップを使い、ベアメタル復元と P2V には、別の **Backup Set**（バックアップセット）で**マシン**バックアップを使ってください（ディスク全体のイメージで、空き領域、ページングファイル、休止状態ファイルはゼロとして格納されます）。

### セキュリティと品質
- サービスの設定フォルダーにアクセスできるのは SYSTEM と Administrators のみ（Windows）、または root のみ（Linux）です。ローカル API は 127.0.0.1 で待ち受け、トークンが必要です。トークンは起動のたびと 24 時間ごとに更新されます
- アプリは、サービス自身が応答していること（Windows ではサービスのプロセス、Linux では root 所有のソケット）を確認するまで、その API に何も送信しません
- Windows では、`config.json` 内の PBS トークンとパスワードはそのコンピューター用に暗号化されます (DPAPI)
- 入力検証、認証情報のサニタイズ、パストラバーサルの防止、指数バックオフ付きの再試行
- すべてのモジュールにユニットテストがあり、さらに実際の PBS に対するオプトインのライブテストとエンドツーエンドテストがあります

## 🔐 クライアント側暗号化

スナップショットは、マシンから送信される前に Proxmox Backup Server のキーファイルを使って暗号化できます（`proxmox-backup-client key create` が書き出す形式なので、公式クライアントとキーを相互に使えます）。すべてのチャンクは AES-256-GCM で暗号化され、マニフェストには署名が付きます。PBS サーバーがキーを知ることはありません。

**GUI：** **Preferences > Account Information**（設定 > アカウント情報）を開き、サーバーを編集して **Encryption**（暗号化）タブを使います。既存のキー（キーファイルのコピー、またはカメラ、スキャン、写真から読み取った紙のキー、あるいは手入力したキー）を **Import...**（インポート）するか、新しいキーを **Generate...**（生成）します。タブにはキーのフィンガープリントが表示されます。**Save a copy...**（コピーを保存）と **Print a paper key...**（紙のキーを印刷、QR コード付き）で、コンピューターから離れた場所にコピーを保管できます。**Reports**（レポート）には、各バックアップが暗号化されたかどうかが表示されます。**キーファイルがなければスナップショットは復元できません。このマシンから離れた安全な場所にコピーを保管してください。**

**パスフレーズで保護されたキー（scrypt または PBKDF2）：** **Ask me**（毎回確認する）は、アプリの 1 セッションの間だけパスフレーズをメモリに保持します。**Remember on this computer**（このコンピューターに記憶する）は PBS トークンと同様に保存するもので、スケジュールとバックグラウンドサービスにはこちらを選びます。

**CLI：**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
`-keyfile-passphrase`（または `PBS_ENCRYPTION_PASSWORD`）は保護されたキーの場合にのみ必要で、省略すると入力を求められます。`proxmoxbackup-machine` と `proxmoxbackup-nbd` も同じフラグを受け付けます。`proxmoxbackup-nbd` は NBD デバイスに触れる前にキーをスナップショットと照合するため、誤ったキーはすぐに失敗します。

**ベアメタル復元：** 復元 ISO はキー（なし、USB メモリ上のキーファイル、または入力したパス）と、保護されたキーのパスフレーズを尋ねます。キーが ISO に書き込まれることはありません。

暗号化されたスナップショットの復元には同じキーファイルが必要です。紛失した場合、復旧する手段はありません。

## フォルダーバックアップの高速化：変更検出

既定の Legacy モードでは、フォルダーバックアップのたびにすべてのファイルを読み込むため、何も変更されていなくても、フォルダー全体を読み込むのと同じだけ時間がかかります。**Backup Sets**（バックアップセット）と `proxmoxbackup-directory` では、代わりに公式 `proxmox-backup-client` の変更検出モードを使えます。

| モード | 読み込むもの | 保存形式 |
|---|---|---|
| **Legacy**（既定） | すべてのファイル | フォルダーごとに 1 つの `<name>.pxar.didx` |
| **Data** | すべてのファイル | 分割アーカイブ：`<name>.mpxar.didx`（メタデータ）+ `<name>.ppxar.didx`（ファイルの内容） |
| **Metadata** | 新規および変更されたファイルのみ | 同じ分割アーカイブ。サイズ、更新日時（ナノ秒単位）、モード、所有者が前回のバックアップと一致するファイルは、読み込まずに前回のバックアップから取り込みます |

**Metadata** モードでは、初回はすべてを読み込み、以降は変更されたものだけを読み込みます。サイズと更新日時が変わらない形で変更されたファイルは検出されません。そのため、セットでは **N 回ごとにすべてのファイルを読み込む**ように設定することもできます。各 Backup Set には固有の **Backup ID**（バックアップ ID）を付けてください。Metadata モードはバックアップグループ内の最新のスナップショットと比較するためです。コマンドラインでは `-change-detection-mode` を使います。

**セットを Data または Metadata に切り替える前に、** 復元に使うすべてのマシンを 2.0 に更新してください。それより前のバージョンでは、分割アーカイブのフォルダーがファイルなしで表示されます。[GUI マニュアルの第 7 節](docs/manual/GUI.md#7-incremental-folder-backups-change-detection)を参照してください。

## 🔄 物理マシンから仮想マシンへ (P2V)

物理 PC やサーバーのマシンバックアップは、次の 2 つの方法で Proxmox VE の仮想マシンにできます。

1. **Proxmox VE で復元する。** **Backup Set**（バックアップセット）（または単発のバックアップ）を **Backup as: Proxmox VE virtual machine**（バックアップ形式：Proxmox VE 仮想マシン）に設定します。バックアップは `vm/<number>` として保存されるため、Proxmox VE は他の VM バックアップと一緒に一覧表示し、VM として復元します（SATA ディスク 1 台。起動前にネットワークカードを追加してください）。
2. **復元 ISO を使って空の VM に復元する。** VM は元のマシンと同じファームウェア、元のディスク以上の容量の SATA ディスク、Intel E1000 ネットワークカードで構成します。これは `host` バックアップでも `vm` バックアップでも同様に機能し、同じバックアップをベアメタルに復元することも引き続きできます。

テスト済みの構成には、Windows 10 と Windows Server (UEFI/GPT)、Linux デスクトップ (UEFI/GPT と BIOS/MBR)、キーを USB メモリに入れた暗号化 Linux バックアップが含まれます。完全な一覧表、ファームウェアの選択、ドライバー、手順については [ベアメタル復元マニュアル](docs/manual/BMR.md#7-restoring-onto-different-hardware-p2v-and-replacement-machines)を参照してください。

## 🤝 公式 Proxmox クライアントと双方向に互換

`proxmox-backup-client` 3.4.9 と PBS 4.2 で検証済みです。

- **このクライアントで作成した Legacy、Data、Metadata のスナップショットは、公式クライアントでバイト単位まで同一に復元できます。** GUI とコマンドラインのどちらで作成したものでも、Windows と Linux のどちらで作成したものでも同様で、PBS 4.2 のファイルブラウザーでも開けます。
- **公式クライアントで作成した Legacy、Data、Metadata のスナップショットは、このクライアントでバイト単位まで同一に復元できます。** 全体の復元も選択的な復元も可能です。
- 同じ分割アーカイブ形式（pxar バージョン 2）、同じ変更検出モード、同じ暗号化キーファイル形式、同じ `PBS_REPOSITORY` 形式の環境変数。

バックアップがこのクライアントに縛られることはありません。唯一追加されるのは、フォルダースナップショットのルートにある小さなファイル `.proxmox_backup_client_meta.json`（元のパス）だけで、公式クライアントはこれを通常のファイルとして復元します。

## マシン全体のライブバックアップ

`proxmoxbackup-machine`（および GUI のマシンバックアップ）は、Windows 10/11 または Windows Server のシステム全体、あるいは Linux マシンを、ダウンタイムなしでバックアップします。

コマンドの構文はフォルダーバックアップとほぼ同じですが、`-backupdir` の代わりに `-backupdev` を使います（複数のディスクには繰り返し指定します）。例：

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

上記のコマンドはディスク 0 を調べ、マウントされているすべてのパーティションを検出して VSS スナップショットを作成し、ディスク全体の起動可能なバックアップイメージを fidx として作成します。空き領域、ページングファイル、休止状態ファイルは読み込まれずにゼロとしてイメージに格納されます（Linux では ext2/3/4 と XFS の空き領域も同様）。`-readfreespace` を指定すると、フォレンジックコピーとしてすべてを読み込みます。読み取れないセクターはゼロとしてイメージに格納され、その場所が実行結果に示されます。`-type vm`（数値の `-backup-id` と併用）を追加すると、バックアップを Proxmox VE の VM バックアップとして保存します（[P2V](#-physical-machines-into-virtual-machines-p2v)）。

次回のバックアップは増分になります。ハッシュ計算は並列化されているため、1 GB/s の速度に達することもあります。

パーティションを除外するには（たとえばゲームライブラリや、すでにフォルダーバックアップの対象になっているデータパーティション）、`-skippartition <disk>:<number>` を追加します（複数ある場合は繰り返し指定）。例：`-skippartition \\.\PhysicalDrive0:4` や `-skippartition /dev/sda:3`。そのパーティションは読み取りもスナップショットもされず、ゼロとしてイメージに格納されます。パーティションテーブルは保持されるため、復元すると同じ位置に同じサイズで、空の未フォーマット状態で戻ります。GUI では、**Backup Set**（バックアップセット）内のディスクの下にある、そのパーティションのチェックを外します。

### Linux マシンバックアップの前提条件（VSS 相当）

Windows のマシンバックアップは VSS を使います。Linux には VSS がないため、稼働中の Linux ホストのマシンバックアップには、書き込み中のブロックデバイスのある時点のスナップショットを取得する `elastio-snap` カーネルモジュール（`dattobd` のフォーク）が必要です。`dattobd` がすでに読み込まれていれば、それも使えます。

元の `elastio/elastio-snap` リポジトリはアーカイブ済みで、現在のカーネルではビルドできません。メンテナンスされているフォーク [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap)（ブランチ `develop`）を使ってください。このフォークのバージョン 0.12.11 は、カーネル 6.12 の Debian 13 でビルドと動作が確認されています。`.deb` は `elastio-snap-dkms` パッケージを推奨 (suggests) しています。

次の 2 点が必要です。満たされていない場合、Linux のマシンバックアップは実行を拒否します。

1. **カーネルモジュールがインストールされ、読み込まれている必要があります。** ソースからビルドする場合：
   ```bash
   sudo apt-get install linux-headers-$(uname -r) build-essential   # Debian/Ubuntu
   git clone --depth 1 https://github.com/Axcient/elastio-snap.git
   cd elastio-snap
   sudo make
   sudo make install
   sudo depmod -a      # `make install` does not run depmod, and modprobe fails without it
   sudo modprobe elastio-snap
   lsmod | grep elastio_snap
   ```
   これで読み込まれるのは現在の起動中だけです。起動のたびに読み込むには、`/etc/modules-load.d/` にエントリを追加してください。パッケージによるインストールについては、モジュール自身の [INSTALL.md](https://github.com/Axcient/elastio-snap/blob/develop/INSTALL.md) を参照してください。

2. **バックアップは root で実行する必要があります。** モジュールの制御インターフェースは root 専用で、一般ユーザーではスナップショットが拒否されます。バックグラウンドサービスは root で動作します。サービスを使わない場合は、アプリで **Restart as administrator**（管理者として再起動）を使うか、`proxmoxbackup-machine` を `sudo` で実行してください。通常のフォルダーバックアップには root は不要です。

GUI では、Linux マシンの **Backup Set**（バックアップセット）で **Use snapshots**（スナップショットを使用）のチェックを外し、ディスクをライブで読み取ることもできますが、2 つ目のチェックボックス（「Run without a snapshot: I accept the backup may be inconsistent」（スナップショットなしで実行：バックアップに不整合が生じる可能性を受け入れる））にチェックを入れた場合に限ります。チェックがないと、毎回の実行が停止し、その理由が表示されます。マウントされていないパーティションは直接読み込まれます。

### ディスクバックアップからのファイル復元

GUI の **Restore**（復元）ページでは、マシンバックアップをフォルダーバックアップと同じように開けます。イメージ内の各ボリューム（NTFS、FAT、exFAT、ext2/3/4、XFS）が、閲覧して復元できるフォルダーになります。何もマウントされません。

Linux では、nbd ツールを使ってディスクバックアップをブロックデバイスとして接続することもできます。まず `modprobe nbd max_part=0` を実行してください。`max_part != 0` の場合、カーネルがパーティションの検出で無限ループに陥ることがあります。

NBD ツールは、VM バックアップでも host バックアップでも、あらゆる固定ディスクバックアップに接続できます（そのため Proxmox VE のバックアップにも使えます）。例：

`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

`-path` を省略すると、ターミナル UI で fidx ファイルを選択できます。`-list` を指定すると、代わりにすべてのイメージを一覧表示します。

> 重要な処理を実行しているマシンではこれを使わないでください。破損したファイルシステムによって OS がクラッシュする可能性があります。Proxmox VE がこの用途に QEMU インスタンスを使うのはそのためです。

`proxmoxbackup-nbd` を停止する前に、nbd ディスク上のものをすべてアンマウントしてください。そうしないと、パーティションがビジー状態のまま残る可能性が高くなります。`Device or resource busy` エラーが出た場合は、`nbd-client -d /dev/nbd0` で強制的に切断するか、再起動してください。

### 物理マシンまたは VM への復元

[ベアメタル復元 ISO](#-bare-metal-restore-iso) を使って、ベアメタルまたは空の仮想マシンに復元します。手順は [ベアメタル復元マニュアル](docs/manual/BMR.md)で説明しています。

## 使い方：ディレクトリバックアップ

Windows と Linux の例、終了コード、スケジュール設定を含む完全なリファレンスは **[コマンドラインマニュアル](docs/manual/CLI.md)** にあります。典型的なコマンドは次のとおりです。

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
  -backupdir string          Source directory, must not be a symlink; repeat for several
  -change-detection-mode     legacy (default), data or metadata
  -novss                     Do not snapshot the source (VSS on Windows, elastio-snap/dattobd on Linux)
  -keyfile string            PBS encryption key file
  -keyfile-passphrase string Passphrase for a protected key (asked for when omitted)
  -no-mtime                  Record 1970-01-01 as every file and folder time (see below)
  -backupstream string       Back up standard input as <name>.didx instead of a directory
  -pxarout string            Also write the archive to a local file (debugging)
  -mail-host, -mail-port, -mail-username, -mail-password, -mail-insecure,
  -mail-from, -mail-to, -mail-subject-template, -mail-body-template
                             Email report (optional)
  -config string             JSON config file; flags given on the command line override it
```

公式クライアントの環境変数 `PBS_REPOSITORY`、`PBS_SERVER`、`PBS_PORT`、`PBS_DATASTORE`、`PBS_AUTH_ID`、`PBS_PASSWORD`、`PBS_FINGERPRINT` も読み込まれます。

JSON による設定については [config.json.example](config.json.example) を参照してください。必要なフィールドだけを記入すれば十分です。

### 1 回の実行で複数のディレクトリ

`-backupdir` を繰り返し指定すると、1 つのコマンドで複数のディレクトリをバックアップできます。

```shell
proxmoxbackup-directory.exe -baseurl "https://yourpbshost:8007" -certfingerprint pbsfingerprint -authid "user@realm!apiid" -secret "apisecret" -datastore "datastorename" -backupdir "C:\Data" -backupdir "D:\Photos"
```

各ディレクトリは独立したバックアップグループとしてバックアップされ、バックアップ ID は `<backup-id>_<path>` になります（`-backup-id` の既定値はホスト名）。そのため、保持、プルーニング、復元では、各ディレクトリが独立した系列として扱われます。1 つのディレクトリが失敗しても他のディレクトリは止まらず、失敗は終了コードで報告されます。`-pxarout` は複数のディレクトリと組み合わせることはできず、同じバックアップ ID になってしまう 2 つのディレクトリは、何かをアップロードする前に拒否されます。

JSON 設定ファイルでは `"backupdirs": ["C:\\Data", "D:\\Photos"]` を使います。単一の `"backupdir"` フィールドも引き続き使え、`"backupdirs"` と組み合わせることもできます。暗号化は単一ディレクトリの場合と同様に機能します。キーは一度だけ読み込まれ、すべてのディレクトリのバックアップグループがそのキーで暗号化されます。

### アーカイブに日時を含めない

すべてのファイルとフォルダーの更新日時はアーカイブの一部です。そのため日時が変わるとアーカイブのバイト列が変わり、そのエントリ周辺のチャンクは、中身が何も変わっていなくても再びアップロードされます。ほとんどのデータでは、これが妥当なトレードオフです。しかし、ファイルは同じままなのにフォルダーに触れ続けるツリーもあります。データをハッシュ名のフォルダー（`aa/bb/<rest of the hash>`）の下に格納するストアでは、それだけの理由で、毎回アーカイブの大部分を再アップロードしてしまうことがあります。

`-no-mtime`（JSON 設定ファイルでは `"no-mtime": true`）を指定すると、代わりにすべてのファイルとフォルダーの日時として 1970-01-01 を記録するため、新しいチャンクが生じるのは実際の内容が変わったときだけになります。実際の日時はどこにも保持されません。復元すると、すべての項目の日付が 1970-01-01 になります。`-change-detection-mode metadata` と併用すると、`data` モードと同様にすべてのファイルが読み込まれます。このオプションは既定でオフです。日時が重要でないソースに使ってください。

### メールのテンプレート

ログイン不要のリレー（多くはポート 25）も使えます。その場合はユーザー名とパスワードを省略してください。送信者には名前を含めることができます（`Backups <backup@example.com>`）。

メールの件名と本文には [Go のテンプレートエンジン](https://pkg.go.dev/text/template)を使います。構文についてはそのドキュメントを参照してください。使用できる変数は次のとおりです。

- `.NewChunks`：作成された新しいチャンクの数
- `.ReusedChunks`：再利用されたチャンクの数
- `.Datastore`：データストア名
- `.Error`：エラー（ある場合）（`.ErrorStr` はそれをテキストで返し、エラーがなければ空）
- `.ReadErrors`：読み取れなかったファイルまたはフォルダー。`.ReadErrorCount` はその数
- `.Hostname`：マシンのホスト名
- `.StartTime`：バックアップの開始時刻
- `.EndTime`：バックアップの終了時刻
- `.Duration`：バックアップの所要時間
- `.FromattedDuration`：書式化されたバックアップの所要時間（コード内でこの綴りになっています）
- `.Success`：エラーも読み取れないものもなくバックアップが完了した場合に true
- `.Partial`：スナップショットは保存されたが、一部のファイルを読み取れなかった場合に true
- `.Status`：`Success`、`Partial`、`Failed` のいずれか

## ストリームバックアップ

pxar アーカイブの代わりにストリームをバックアップします。例：

```
mysqldump yourdatabase | ./proxmoxbackup-directory -backupstream yourdatabase.sql [other options]
```

ストリーム自体（たとえば tar アーカイブや SQL ダンプ）の重複排除に、buzhash チャンキングを使います。

## 既知の問題

Windows Defender のマルウェア対策が有効になっていると、バックアップ速度が達成可能な速度の約 25% まで低下することがあります。

Windows では、コマンドラインツールはプロセスロックを取得します。実行中に 2 つ目のバックアップを開始すると「Another backup job is running」と表示され、コード 2 で終了します。Linux にはこのようなロックがないため、コマンドラインの実行を 2 つ同時にスケジュールしないでください。（GUI とバックグラウンドサービスは複数のバックアップを安全に同時実行できます。[複数のバックアップの同時実行](docs/manual/GUI.md#67-several-backups-at-once)を参照してください。）

## 📋 前提条件

- Windows 10/11 または Windows Server（64 ビット）。インストールと、アプリをバックグラウンドサービスに接続するには管理者権限が必要です
- または Linux：パッケージは Debian、Ubuntu、Mint、Fedora、Arch 用です。GUI には WebKitGTK ランタイムと polkit が必要です。一貫性のあるマシンバックアップには、elastio-snap または dattobd カーネルモジュールと root 権限が必要です
- Proxmox Backup Server へのネットワークアクセス
- P2V の場合：PBS データストアをストレージとして追加した Proxmox VE、または復元 ISO を起動できる任意のハイパーバイザー

## 🔨 ソースからのビルド

**[BUILD.md](BUILD.md)** に、GUI とバックグラウンドサービス、コマンドラインツール、MSI、Linux パッケージの完全な手順があります。要約すると、Go 1.25 以降と Wails CLI v2.13 を使って次のようにします。

```bash
cd gui
wails build                                                       # the app, in gui/build/bin
go build -tags service -o build/bin/ProxmoxBackupClientSVC.exe    # the background service
```

```bash
make deb          # .deb
make pkgfedora    # .rpm (in Docker)
make pkgarch      # Arch package (on Arch)
make msi          # Windows MSIs on Linux (Docker + Wine + WiX), one per brand
make clonezilla   # the bare-metal restore ISO
```

ビルド済みのフロントエンド（`gui/frontend/dist`）はコミットされているため、Node.js が必要なのはフロントエンドを変更する場合だけです（`gui/frontend` で `npm install` と `npm run build`、またはホットリロード用に `wails dev`）。テスト：`make test`。

## 🔧 高度な使い方とガイド

### マルチ PBS（複数の PBS サーバー）

複数の PBS サーバーを設定し、**Backup Set**（バックアップセット）または単発のバックアップごとにバックアップ先を選べます（例：`C:\Users` を高速 SSD の PBS へ毎日、`D:\` を大容量データ用の PBS へ毎週、さらに DR サーバーへ）。

- **[ユーザーガイド](MULTI_PBS_USER_GUIDE.md)**：サーバーの追加とテスト、既定のサーバー、バックアップごとのサーバー選択、FAQ とトラブルシューティング。
- **[実装ガイド](MULTI_PBS_GUIDE.md)**：データモデル、単一 PBS 設定からの自動移行、バックエンドのメソッド。

従来の単一 PBS 設定は、初回読み込み時に自動的に `default` サーバーへ移行されます。

### 💿 ベアメタル復元 ISO

復元 ISO は、標準の Clonezilla Live ISO に `pbsnbd` / `machinebackup` バイナリと 2 つのメニュー項目を追加したものです。CD や USB メモリ（`dd` で書き込むか、Ventoy の USB メモリから）から起動でき、UEFI マシンでも BIOS マシンでも動作します。

- **`ocs-pbs-bare-metal-restore`**：最初の既定のブートメニュー項目 **PBS Bare Metal Restore**（PBS ベアメタル復元）です。Clonezilla の言語とキーボードの質問を省略し、PBS の認証情報の入力から復元の完了までまっすぐ進みます。まず DHCP を試し（失敗した場合にのみ静的 IP を尋ねます）、バックアップが暗号化されていれば USB メモリから暗号化キーを読み込み、ターゲットディスクの検出では起動元の USB メモリを候補に出しません。Windows と Linux のバックアップをどちらも復元でき（復元はブロック単位）、ベアメタルにも空の VM にも復元できます。
- **`ocs-pbs-nbd`**：Clonezilla 自身のモード選択メニュー内の **pbs-nbd** 項目です。PBS の接続情報とスナップショットを尋ね、それを `/dev/nbd0` に接続した後、Clonezilla の通常のディスク復元ウィザードに引き渡し、手動で操作できるようにします。

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-37-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

`make clonezilla` は標準の ISO をダウンロードして同じ処理を行います。詳細（その場での差し替えではなく ISO 全体を再構築する理由、前提条件、メニューの流れ、検証）は **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)** に、使い方は **[ベアメタル復元マニュアル](docs/manual/BMR.md)** にあります。

### Windows GUI のビルド

**ネイティブ Windows**：ツールチェーン（Chocolatey からの Go と MinGW、Wails）と、アプリ、サービス、MSI のビルドコマンドについては **[BUILD.md](BUILD.md)** を参照してください。

**Docker（Linux でビルドする場合）。** `./build_gui_windows_docker.sh` は、使い捨ての `golang` コンテナ内で、WebView2 に正しく対応した Windows アプリを `ProxmoxBackupClientGO.exe` としてビルドします。バックグラウンドサービスや MSI はビルドしません。

### 変更履歴と内部ドキュメント

- **[CHANGELOG.md](CHANGELOG.md)**：バージョンごとの変更履歴。
- **[TODO.md](TODO.md)**：未完了のロードマップとアイデア。
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)**：安定版の製品状態と利用可能なビルド。
- **[installer/DEPLOYMENT.md](installer/DEPLOYMENT.md)**：多数のコンピューターへの MSI の展開。
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**：MSI のアンインストールダイアログ（設定の保持／削除）とそのテスト計画。
- **[AGENTS.md](AGENTS.md)**：リポジトリの構成と規約（コントリビューターとコーディングエージェント向け）。

## 🌐 イタリアでの技術サポート

本プロジェクトとその**すべて**の貢献は今後も永久に公開され、GPLv3 ライセンスの下で提供されることを前提として、本プロジェクトおよび最新のマシンバックアップ機能のメインスポンサーである E.T.I. Srl ( https://etitech.net ) は、必要とする方に、Proxmox の導入全般、特に Windows バックアップツールに関するサポートを提供できます。

「コミュニティ版」と「エンタープライズ版」に分かれることは今後もなく、技術サポートだけが独立したサービスになります。
ご依頼いただくカスタマイズは、開発が有償となる場合でも、プロジェクト全体と同様に GPLv3 で公開されます。

## 🖥️ GUI のクレジット

**Proxmox Backup Client GUI** は、**[RDEM Systems](https://www.rdem-systems.com/)** が **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)** として生み出したものです。同社は元のアプリを設計・開発し、そのブランド版である Nimbus Backup に関する商用サポートを提供しています。その後 **[mjb-is](https://github.com/mjb-is)** が、他のバックアップシステムのワークフローや使用感を参考にして改良・拡張し、その成果はこのリポジトリに統合されました。GUI とそのすべての機能を含むコードベース全体は、引き続き GPLv3 ライセンスの下でオープンソースです。

## ⚠️ 警告

本ソフトウェアは「現状のまま」提供されます。信頼性を目指してはいますが、データの損失や損傷について一切の責任を負いません。本番環境で頼る前に、必ずバックアップをテストし、復元を検証してください。バックアップは、そこから実際に復元できて初めて有効だと証明されます。ベアメタル復元も含め、ご自身で復元をテストしてください。

本ソフトウェアはまだアルファ品質であり、ソースファイルを含め、いかなる種類の損害やデータ損失についても責任を負いません。

バグ報告を歓迎します。行った操作、起きたこと、ログ（**Message Log**（メッセージログ）または **View Full Logs**（完全なログを表示））を添えて [issue](https://github.com/tizbac/proxmoxbackupclient_go/issues) を作成してください。

## 📄 ライセンス

GPLv3：[LICENSE](LICENSE) ファイルを参照してください。

## 🏷️ ブランディング

機能の追加または修正を行う**少なくとも 5 件のコミット**を貢献したコントリビューターは全員、商用利用のために自身のブランディングデータを追加してもらう権利があります。

唯一の条件は、ブランディングが示す企業が次のいずれも行って**いない**ことです。

- マルウェアキャンペーン
- 戦争を助長する事業（西側諸国を含むあらゆる国に適用されます）
- 詐欺
- データ窃盗
- 児童の人身売買
- 暴力
- 差別
- 薬物
- 一般に違法と認められているあらゆる活動

いずれかのコントリビューターに苦情が寄せられた場合、私たちは連絡を取るよう努めます。正当な説明がない場合は、その特典を**直ちに終了**します。

**GPLv3 ライセンスは引き続き有効**であり、プロジェクトをフォークして独自の実行ファイルをビルドする自由は変わりません。

## 🤝 貢献

貢献を歓迎します。特に次のものを歓迎します。

1. バージョン 2.0 向けの README とユーザーマニュアルの翻訳
2. チャンクの非同期／マルチコアアップロード（マルチコア圧縮はマシンバックアップですでに実装済み）
3. Windows のセキュリティ記述子を含む別種のエントリを pxar 形式に追加するための、Proxmox 側のパッチ
4. Windows のシンボリックリンクとジャンクションのサポート（現在はスキップされます）
5. より多くのハードウェア、Linux ディストリビューション、カーネルでのテスト、特にベアメタル復元
6. 思いついた面白いことなら何でも :)

## Proxmox Backup Client GO のコントリビューターについて

Proxmox Backup Client GO のコントリビューターが本プロジェクトを開発・保守しています。本ソフトウェアは NTP/NTS インフラストラクチャと、コミュニティのリファレンスに掲載されている [11 の公開 NTS サーバー](https://github.com/jauderho/nts-servers)に依存しています。

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**
