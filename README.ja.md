# Proxmox Backup Client：Proxmox Backup Server 用の Windows・Linux クライアント

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)
[![Ko-fi](https://img.shields.io/badge/Ko--fi-buy%20me%20a%20coffee-FF5E5B?logo=ko-fi&logoColor=white)](https://ko-fi.com/mickbeeby)

> **他の方々の成果の上に成り立っています。** このフォークは、Tiziano Bacocco (tizbac) によるオリジナルの [proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) と、RDEM Systems の Nimbus Backup GUI クライアントを基にしています。後者はオリジナルに統合され、本アプリの GUI はそこから発展しました。
>
> ☕ このクライアントは無料で、今後も無料です。時間の節約に役立ったら、[Ko-fi でコーヒーをおごっていただけます](https://ko-fi.com/mickbeeby)。完全に任意であり、サポートや機能の約束を買うものではありません。

**Proxmox Backup Client は、Proxmox Backup Server (PBS) 向けのオープンソース (GPL-3.0) バックアップクライアントで、Windows と Linux で動作します。** フォルダーは閲覧できるファイルアーカイブとして、マシン全体は起動可能なディスクイメージとしてバックアップし、スケジュールでも必要なときにも実行できます。復元は 1 つのファイルからマシン全体まで、同じコンピューター、交換用のマシン、Proxmox VE の仮想マシンのいずれにも戻せます。

<p align="center">
  <a href="docs/screenshots/overview-how-it-works.png"><img src="docs/screenshots/overview-how-it-works.png" width="400" alt="How it works: Windows and Linux machines back up through the client to Proxmox Backup Server, then restore as files, onto bare metal, or into a Proxmox VE VM"></a>
  <a href="docs/screenshots/overview-screenshots.png"><img src="docs/screenshots/overview-screenshots.png" width="400" alt="Screenshots of the app: Backup Sets, Reports, schedules, a whole PC as a VM, backup progress and restore"></a>
</p>

私はこれを自分のホームラボのために作っています。Windows と Linux のサーバー、デスクトップ、ノート PC が混在する環境で、Backup for Workgroups、rsync によるミラー、Clonezilla/RescueZilla のイメージに代えて、PBS を唯一のバックアップシステムにするためです。2026 年 9 月に [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) のフォークとして始まり、今では本家を大きく超えています。それ以降のコミットは 350 件を超え、18 件の修正と機能を本家プロジェクトに還元しました（[下の一覧](#-本家プロジェクトへの貢献)）。

**そのままの状態でご自由にお使いください。** 正式には**アルファ版ソフトウェア**であり、使用はすべて自己責任で、いかなる種類の保証も一切ありません。各リリースは私自身がテストし、私のマシンで日常的に使っていますが、頼りにする前に[状況、バグ報告と機能要望](#-状況バグ報告と機能要望)を読んでください。

構成：

- **Proxmox Backup Client GUI**：Windows と Linux 用のアプリ。それぞれ複数のスケジュールを持てる Backup Sets（バックアップセット）、一貫性のあるスナップショット（Windows は VSS、Linux は elastio-snap/dattobd）によるフォルダーとディスク全体のバックアップ、フォルダーバックアップとマシンバックアップからのファイル復元、Undelete と Roll back、複数の PBS サーバー、クライアント側暗号化、そして誰もサインインしていなくてもスケジュールを動かすバックグラウンドサービス。
- **`proxmoxbackup-directory`**：重複排除と変更検出付きの、コマンドラインのフォルダー (pxar) バックアップ。
- **`proxmoxbackup-machine`**：Windows と Linux マシンのディスク全体をオンラインでバックアップ (fidx) するコマンドラインツール。復元後、イメージはそのまま起動できます。
- **`proxmoxbackup-nbd`**：ディスクバックアップを NBD デバイスとして接続します (Linux)。
- Clonezilla Live をベースにした**ベアメタル復元 ISO**。電源投入から再起動まで、案内に沿って復元できます。

> キーワード：Proxmox バックアップクライアント Windows · PBS クライアント · Windows VSS バックアップ · 物理から仮想へ (P2V) · ベアメタル復元 · Proxmox Backup Server の GUI。

## ⭐ できることと実測結果

| | 結果 |
|---|---|
| **増分フォルダーバックアップ** | 761,866 ファイル、480.9 GB のファイルサーバー共有：初回 4 時間 17 分、2 回目以降は毎回 10〜15 分。変更のないファイルは開きもしません（公式クライアントの Metadata 変更検出）。 |
| **高速なマシンバックアップ** | 空き領域、ページファイル、休止状態ファイルは読まず、空白のディスク領域はハッシュしません。373 GB 使用中の 1 TB ノート PC ディスク：以前は 1 時間 23 分〜1 時間 44 分、今は 38 分。50 GB のテストディスク：3 分 36 秒が 1 分 15 秒に。 |
| **マシンバックアップからファイルを取り出し** | ディスクイメージをフォルダーのように閲覧し、1 つのファイルから復元できます。NTFS、FAT、exFAT、ext2/3/4、XFS に対応。43 万ファイルある 127 GB のノート PC の C: ドライブが 4〜15 秒で一覧表示されます。事前のマウントやダウンロードは不要です。 |
| **物理マシンを仮想マシンに** | PC やサーバーをバックアップし、Proxmox VE の VM として起動できます。Proxmox VE 自体で復元しても、復元 ISO を使っても構いません。[下記参照](#-物理マシンを仮想マシンに-p2v)。 |
| **ベアメタル復元** | ISO を起動し（USB メモリ、CD、PXE）、PBS の情報を入力してバックアップを選ぶだけ。32 GB の Windows マシンが、電源投入から 8 分後にログイン画面に戻りました。 |
| **Undelete** | 761,000 ファイルの共有から 39 GB を削除した後、7 日間の検索で約 1 分のうちにそのファイルだけを正確に見つけ、元に戻しました。 |
| **Roll back** | セットのフォルダーを、選んだバックアップ時点の状態に戻します。事前にプレビューがあり、後から Undo できます。 |
| **スケジュール** | Backup Set ごとに複数設定でき、数分ごとから年 1 回まで対応します。「毎日 02:00」に加えて「毎月末日」や「第 2 火曜日」など、それぞれわかりやすい言葉で表示され、次の 3 回の実行予定も示されます。 |
| **復元の忠実度** | Windows では NTFS のアクセス権、属性、代替データストリームと拡張属性、作成日時とフォルダーの日時。Linux ではモード、所有者、ACL、拡張属性、シンボリックリンク。任意の検証パスで、復元したものを読み直してバックアップと比較できます。 |
| **Proxmox 純正ツールと連携** | ここで作ったバックアップは公式 `proxmox-backup-client` でバイト単位まで同一に復元でき、その逆もできます。[詳細](#-公式-proxmox-クライアントと双方向に互換)。 |

## 🧩 本家プロジェクトに対して追加したもの

[本家プロジェクトにマージされた 18 件の変更](#-本家プロジェクトへの貢献)に加えて、このクライアントは次のものを追加しています：

**バックアップ**
- 公式クライアントの変更検出（Data モードと Metadata モード、分割 pxar v2 アーカイブ）による増分フォルダーバックアップ。N 回ごとの全読み込みも可能。
- 空き領域、ページファイル、休止状態ファイルを読まないマシンバックアップ（Windows の NTFS、Linux の ext2/3/4 と XFS）。読めないセクターはゼロとして書いて処理を続け、実行中にブロック、ディスク、パーティション、チャンクの数を表示します。
- フォルダーバックアップとマシンバックアップの Pause、Resume、Stop。バックグラウンドサービスが実行するバックアップにも対応。
- Windows のフォルダーバックアップで、代替データストリーム、NTFS 拡張属性、作成日時を保持。

**スケジュールと Backup Sets**
- Backup Set ごとに複数のスケジュール。数分ごとから年 1 回まで（第 n 曜日、月末日など）、それぞれわかりやすい言葉で表示し、次の 3 回の実行予定も表示。
- Windows だけでなく Linux にもバックグラウンドサービス。Linux でも Restart as administrator が使えます。
- 高速または全読み込みの Run Now、VSS / Snapshot バッジ、SHARED ID 警告、セット名を PBS のスナップショットコメントとして記録、メール通知、バックアップ後のアクション、設定のエクスポートとインポート。

**復元**
- マシンバックアップから個別のファイルを取り出し（NTFS、FAT、exFAT、ext2/3/4、XFS）。イメージのマウントやダウンロードは不要。
- フォルダーとマシンの Backup Sets で Undelete と Roll back。GUI からもコマンドラインからも使えます。
- 復元後の任意の検証パス、Windows の作成日時、そしてフォルダーを 1 つずつ読み込むスナップショットツリー。非常に大きなバックアップもすぐに開けます。
- 空の VM への復元にも使える、案内付きのベアメタル復元 ISO と、テスト済みの物理から仮想への手順（[P2V](#-物理マシンを仮想マシンに-p2v)）。
- 公式 `proxmox-backup-client` と双方向にバイト単位で互換。その Data と Metadata（分割アーカイブ）のスナップショットも含みます。

**アプリ**
- 作り直したインターフェース：メニューバー、サイドメニュー、バックアップと復元の全履歴を持つ Reports、Message Log、5 種類のカラーテーマとカスタム色、トレイ通知とトースト通知。
- 18 の表示言語。すべて見直し済みです（本家は 5 言語）。
- 設定フォルダーは管理者 (Windows) または所有者 (Linux) のみに制限し、サービス API のトークンは起動のたびに更新。
- MSI、`.deb`、`.rpm` のパッケージと、GUI、コマンドライン、ベアメタル復元のユーザーマニュアル。

## 🔄 物理マシンを仮想マシンに (P2V)

物理 PC やサーバーのマシンバックアップは、2 つの方法で Proxmox VE の仮想マシンにできます：

**1. Proxmox VE で復元する。** Backup Set（または単発バックアップ）を **Backup as: Proxmox VE virtual machine** に設定します。バックアップは小さな VM 構成付きの `vm/<number>` として保存されるので、Proxmox VE はほかの VM バックアップと一緒に一覧表示します。Proxmox VE で PBS ストレージを開き、バックアップを選んで **Restore** を押し、VM ID とストレージを選びます。VM には SATA ディスク 1 台と、ディスクに合ったファームウェア（UEFI/GPT なら OVMF、BIOS/MBR なら SeaBIOS）が設定されます。起動する前にネットワークカードを追加し、UEFI の VM が起動しない場合は EFI ディスク（Pre-Enroll keys のチェックを外す）を追加してください。

**2. ベアメタル復元 ISO で空の VM に復元する。** 元のマシンと同じファームウェア（UEFI/GPT なら EFI ディスク付きの OVMF、BIOS/MBR なら SeaBIOS）、元のディスク以上の容量の SATA ディスク、Intel E1000 ネットワークカードを持つ VM を作り、ISO から起動して復元を実行します。`host` バックアップでも `vm` バックアップでも同じように使え、同じバックアップを引き続きベアメタルにも復元できます。

テスト済みの手順：

| 元 | 先 | 手順 | 結果 |
|---|---|---|---|
| Windows 10 ノート PC、UEFI/GPT | Proxmox VE の VM | 復元 ISO | ユーザーのデスクトップまで起動 |
| Windows Server、UEFI/GPT | Proxmox VE の VM | Proxmox VE での復元 | 変更なしで起動 |
| Xubuntu デスクトップ、UEFI/GPT | Proxmox VE の VM | Proxmox VE での復元、および復元 ISO | デスクトップまで起動 |
| Linux、BIOS/MBR | Proxmox VE の VM | 復元 ISO、および Proxmox VE での復元 | ログインプロンプトまで起動 |
| Linux Mint、暗号化バックアップ | Proxmox VE の VM | 復元 ISO、鍵は USB メモリから | 起動 |
| Linux PC | 新しいディスクに換えた同じ PC | 復元 ISO | 起動 |

Windows は初回起動時に新しい仮想ハードウェアをセットアップします（「デバイスを準備しています」、英語版では「Getting devices ready」）。ファームウェアの対応表、ドライバー、手順の詳細は[ベアメタル復元マニュアル](docs/manual/BMR.md#7-restoring-onto-different-hardware-p2v-and-replacement-machines)にあります。

## 📖 ユーザーマニュアル

| マニュアル | 内容 |
|---|---|
| **[GUI マニュアル](docs/manual/GUI.md)** | インストール、PBS サーバー、Backup Sets とスケジュール、増分バックアップ、進捗、復元、Undelete、Roll back、Reports、ログ、Preferences、バックグラウンドサービス、トラブルシューティング |
| **[コマンドラインマニュアル](docs/manual/CLI.md)** | `proxmoxbackup-directory`、`proxmoxbackup-machine`、`proxmoxbackup-nbd`：全フラグ、設定ファイル、終了コード、スケジュール実行、コマンドラインからの Undelete と Roll back |
| **[ベアメタル復元マニュアル](docs/manual/BMR.md)** | 復元 ISO の手順、`vm` バックアップを Proxmox VE へ、物理から仮想へ、異なるハードウェアへの復元 |
| [復元シナリオ](docs/RESTORE_GUIDE.md) | どの場面でどの復元を使うか |
| [マニュアル索引](docs/manual/README.md) | 3 つのマニュアルの概要 |

## 🆕 v0.10.0 の新機能：ネットワークを実際に流れる量、手元に残せる鍵、アプリ内のマニュアル

- **バックアップが実際に送る量。** 進捗カード、Reports、Message Log に **New/changed data**、**Data transferred**（ネットワーク上で実際に使った量、圧縮後）、**Compression ratio** が表示されます。変更データが 365 MB ある 128 GB ディスクの増分バックアップでは、送信量は 53 MB でした。
- **PBS との接続が切れるとマシンバックアップをやり直します**。2、5、10、15 分後に再開し、カードにカウントダウンを表示します。
- **暗号化鍵のコピーを保管**：**Save a copy...**、QR コード付きの **Print a paper key...**、そしてファイル、カメラ、写真、入力したテキストから鍵を取り戻す **Import...**。保存する前にフィンガープリントで照合します。
- **アプリ内のユーザーマニュアル**：GUI、コマンドライン、ベアメタル復元の各マニュアルを収めた **Help** ページ（F1）。オフラインで読め、バージョンに合った内容です。
- **トレイから Pause**（Windows）。アプリまたはバックグラウンドサービスが実行するバックアップが対象です。
- マシンの Backup Sets 向けの **Forensic copy**：空き領域も読み取るため、削除されたファイルをイメージから復元できます。
- **Windows では `config.json` 内のパスワードを暗号化**（DPAPI、このコンピューター用）。その後 0.9.1 以前に戻すと、パスワードを入力し直す必要があります。
- **MSI で多数のコンピューターをセットアップ**：`msiexec /i ... CONFIGFILE=<file>` でエクスポートした設定ファイルを取り込み、Backup ID は各コンピューター自身の名前から作られます。
- **各 Backup Set のカードに前回の実行からの経過時間を表示**。また、バックアップが意図的に除外するリンク、ソケット、システムファイルは警告として表示されなくなりました。
- **修正**：VSS がビジーのとき、バックアップが止まったままになり、コンピューター上のシャドウコピーをすべて削除することがありました。VSS スナップショットがハンドルをリークしていました。サービスに渡した Run Now が、実行開始の時点でバックアップ後のアクションを実行していました（コンピューターの電源が切れることもありました）。ベアメタル復元 ISO は、パスワードをプロセス一覧に出さず、復元した Linux システムの machine ID を保持し、完了後は USB メモリから復元をやり直す代わりにマシンの電源を切ります。

以前のリリース：**v0.9** は Backup Set ごとに複数のスケジュール、Windows と Linux のインストーラー、Linux のバックグラウンドサービス、一般的なすべてのファイルシステムのマシンバックアップからのファイル取り出し。**v0.8** はより速いマシンバックアップ（空き領域のスキップ、不良セクターでも停止しない）、マシンバックアップからのファイル取り出し、Pause と Resume。**v0.7** は Undelete と Roll back。**v0.6** は増分フォルダーバックアップ。すべての変更は [CHANGELOG](CHANGELOG.md) にあります。

## 🤝 公式 Proxmox クライアントと双方向に互換

`proxmox-backup-client` 3.4.9 と PBS 4.2 で確認済み：

- **このクライアントで作った Legacy、Data、Metadata スナップショットは、公式クライアントでバイト単位まで同一に復元できます**。GUI とコマンドラインのどちらで作っても、Windows と Linux のどちらで作っても同じで、PBS 4.2 のファイルブラウザーでも開けます。Linux では、復元したファイルのモード、ナノ秒単位の更新日時、シンボリックリンク先も正確に保たれます。
- **公式クライアントで作った Legacy、Data、Metadata スナップショットは、このクライアントでバイト単位まで同一に復元できます**（全体でも一部でも）。
- 同じ分割アーカイブ形式 (pxar version 2)、同じ変更検出モード、同じ暗号鍵ファイル形式（`proxmox-backup-client key create` の鍵はここで使え、ここで作った鍵はあちらで使えます）、同じ `PBS_REPOSITORY` 形式の環境変数。

つまり、バックアップがこのクライアントに縛られることはありません。このクライアントが作るスナップショットはすべて Proxmox 純正のツールで復元でき、公式クライアントが作るスナップショットはすべてこのクライアントで復元できます。唯一の追加物は、フォルダースナップショットのルートに置かれる小さなファイル `.proxmox_backup_client_meta.json`（元のパス）で、公式クライアントは普通のファイルとして復元します。

> ⚠️ **免責事項：** このプロジェクトは **Proxmox Server Solutions GmbH** とは**一切関係ありません**。「Proxmox」、Proxmox のロゴおよび関連する名称はそれぞれの権利者に帰属し、ここでは互換性を示す目的に**限って**使用しています。同社の製品は [proxmox.com](https://www.proxmox.com/) をご覧ください。

## 📦 ダウンロードとインストール

👉 **[最新リリースをダウンロード](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| 環境 | 使うもの |
|---|---|
| Windows | **MSI インストーラー**（`ProxmoxBackupClient-<version>-x64.msi`）がアプリとバックグラウンドサービスを Program Files に入れ、スタートメニューに登録します。または `ProxmoxBackupClient-v<version>-windows-amd64.zip` を展開します（アプリと `ProxmoxBackupClientSVC.exe`。同じフォルダーに置いてください） |
| Debian、Ubuntu、Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| その他の Linux | `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` を展開（WebKitGTK 4.1 ランタイムが必要） |
| コマンドラインのみ | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` または `-linux-amd64.tar.gz` |
| 復元するマシン | `ProxmoxBackupClient-GO-BMR-Restore-v<version>.iso`（ベアメタル復元 ISO） |

MSI、`.deb`、`.rpm` は v0.9.0 で新しく加わったもので、v0.8.1 までのリリースには zip と tar.gz のファイルしかありません。自分でビルドする方法は [BUILD.md](BUILD.md) にあります。

Linux パッケージは 2 つのメニュー項目を追加します。**Proxmox Backup Client** はアプリを自分のユーザーとして実行し（自分のファイルのフォルダーバックアップ用）、**Proxmox Backup Client (administrator)** は polkit でパスワードを確認して root で実行します（マシンバックアップとブロックスナップショット用）。バックグラウンドサービスはインストールされますが、オンにはなっていません。[packaging/debian/README.md](packaging/debian/README.md) を参照してください。

> ⚠️ **Windows で「ウイルスが検出されました」（例：`Trojan:Win32/Sabsik.FL.A!ml`）や SmartScreen の警告が出ますか？**
> これは Go/Wails アプリで**よく知られた誤検知**で、ウイルス*ではありません*。`!ml` は機械学習モデルによる判定を示し、*署名がなく珍しい*実行ファイルに付きます。ダウンロードしたファイルを `SHA256SUMS.txt`（下記）と照合してください。

### 🔎 ダウンロードの検証

各リリースには `SHA256SUMS.txt` が付いています：

```powershell
Get-FileHash .\ProxmoxBackupClient-0.9.0-x64.msi -Algorithm SHA256   # compare with SHA256SUMS.txt
```

Linux では `sha256sum -c SHA256SUMS.txt --ignore-missing`。

> ℹ️ **コード署名、CI 署名付きの証明、VirusTotal スキャンはまだありません。** リリースは手作業でビルド・アップロードしており、検証可能なビルド来歴の証明を作れる CI パイプラインはまだ使っていません。整備を手伝っていただける方は [TODO.md](TODO.md) をご覧ください。

## ✨ 機能

### GUI
- **🌍 多言語**：18 の表示言語：フランス語、英語、イタリア語、ドイツ語、ポーランド語、スペイン語、ブルガリア語、チェコ語、ギリシャ語、ハンガリー語、ラトビア語、リトアニア語、オランダ語、ポルトガル語、ルーマニア語、スロバキア語、トルコ語、ウクライナ語
- ネイティブのメニューバーとサイドメニュー（Backup、Restore、Undelete、Roll back、Reports、Message Log、Preferences、About）
- **Backup Sets**：名前付きで再利用できるジョブ。**それぞれ複数のスケジュール**（数分ごとから年 1 回まで、または手動）、ディレクトリかマシンかの種類バッジ、VSS / Snapshot バッジ、「Run Now」（Metadata セットでは高速読み込みか全読み込みを選択）、2 つのセットが同じバックアップグループに書き込む場合の **SHARED ID** 警告
- フォルダーまたはディスク全体の**単発バックアップ**
- バックアップと復元の全履歴を表示する **Reports** ページと、診断用の上限付き **Message Log**
- 5 つのカラーテーマ（Amber、Blue、Green、Red、Dark）とカスタム色を選べる **Preferences** ダイアログ
- **増分フォルダーバックアップ**（変更検出、Metadata モード）：公式クライアントと同じく、変更のないファイルは読まずに再利用。N 回ごとの全読み込みも設定可能
- Windows と Linux の**マシン（ディスク全体）バックアップ**：空き領域をスキップし、不良セクターでも止まらず、`host` バックアップまたは Proxmox VE の `vm` バックアップとして保存
- **物理から仮想へ**：マシンバックアップを Proxmox VE の VM に復元。Proxmox VE 経由でも復元 ISO でも可能
- **復元**：スナップショットの閲覧、ファイル検索（ワイルドカード、正規表現、フルパス）、選択的復元。NTFS/POSIX ACL、代替データストリーム、フォルダーのタイムスタンプ、Windows の作成日時を復元し、検証パスも選択可能
- **マシンバックアップからファイルを取り出し**：ディスクイメージ内の NTFS、FAT、exFAT、ext2/3/4、XFS ボリュームを閲覧してファイルを復元
- **Undelete** と **Roll back**：Backup Set のバックアップにはあるがディスクにはないファイルを見つけて戻す、またはセットのフォルダーを選んだバックアップ時点の状態に戻す（事前プレビュー、事後の Undo 付き）。フォルダーとマシンのセットに対応。[GUI マニュアル](docs/manual/GUI.md#10-undelete)を参照
- 実行中のバックアップの **Pause、Resume、Stop**
- **Run as Service**：Preferences > Advanced からバックグラウンドサービスのインストール・開始・停止・削除ができ、誰もサインインしていなくてもスケジュールが動きます（Windows と Linux）
- ライブ進捗カード：開始時刻と終了予想、処理済み/全体のファイル数、処理速度（毎秒のデータ量とファイル数）と実際のアップロード速度、現在のファイル、全ファイルを読んでいる理由
- Windows では VSS（ボリュームシャドウコピー）、Linux では elastio-snap/dattobd のブロックスナップショット
- メール通知とバックアップ後のアクション、Backup Set の複製、設定のエクスポートとインポート
- 複数の PBS サーバー、証明書フィンガープリントの固定（初回接続時に信頼）、API トークンまたはユーザー/パスワードでのログイン
- **クライアント側暗号化**：サーバーごとの PBS 鍵ファイル（AES-256-GCM、マニフェスト署名）、パスフレーズ保護は任意。[クライアント側暗号化](#-クライアント側暗号化)を参照
- Backup Sets の変更を管理者に限る保護と Restart as administrator
- 診断用のデバッグログ

### コマンドラインツール
- `proxmoxbackup-directory`：重複排除と `-change-detection-mode legacy|data|metadata` 付きのフォルダー (pxar) バックアップ
- `proxmoxbackup-machine`：Windows (VSS) と Linux (elastio-snap/dattobd) マシンのディスク全体のオンラインバックアップ（fidx、増分、`-readfreespace` を付けない限り空き領域をスキップ、`-type vm` で Proxmox VE バックアップ）
- `proxmoxbackup-nbd`：ディスクバックアップ復元用の NBD サーバー (Linux)
- `ProxmoxBackupClientSVC` (Windows) と `proxmoxbackupclient`（Linux パッケージ）：GUI で定義した Backup Sets に対する `-list-sets`、`-undelete`、`-rollback`

最初の 3 つは、暗号化スナップショット用に `-keyfile` と `-keyfile-passphrase` を受け付けます。完全なリファレンスは[コマンドラインマニュアル](docs/manual/CLI.md)にあります。

### 📸 スクリーンショット

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Backup Sets**：次に実行されるスケジュールのバッジ（DAILY、WEEKLY、MONTHLY、それ以外は +N）とディレクトリ/マシンの種類バッジ付きの名前付きジョブ。それぞれに Run Now

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**バックアップのライブ進捗**：実行中のスループット、残り時間、新規チャンクと再利用チャンクの数

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**復元**：PBS サーバーと Backup ID を選び、スナップショットを一覧表示、またはファイル名で横断検索

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-schedule-tab.png" width="280">](docs/screenshots/gui-schedule-tab.png)
**スケジュール**：セットごとに複数、数分ごとから年 1 回まで（「毎月末日」「第 2 火曜日」）、次の 3 回の実行予定付き

</td>
<td width="33%">

[<img src="docs/screenshots/backup-set-as-vm.png" width="280">](docs/screenshots/backup-set-as-vm.png)
**Proxmox VE の VM としてバックアップ**：仮想マシンの種類と数値 ID を選ぶと、入力に合わせて PBS の Backup ID が表示されます

</td>
<td width="33%">

[<img src="docs/screenshots/backup-sets-vm-badge.png" width="280">](docs/screenshots/backup-sets-vm-badge.png)
**Proxmox VE の VM としての Backup Sets**：`vm` バックアップとして保存するセットには `MACHINE (VM)` バッジ

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/oneoff-as-vm.png" width="280">](docs/screenshots/oneoff-as-vm.png)
**単発バックアップ**：単発バックアップのフォームにも同じオプション

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore-progress.png" width="280">](docs/screenshots/gui-restore-progress.png)
**選択的復元**：スナップショットのツリーから個別のファイル/フォルダーを選び、元の場所か別の場所へ復元

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Reports**：詳細パネル付きの全実行履歴（所要時間、チャンク、バックアップしたフォルダー）

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-message-log.png" width="280">](docs/screenshots/gui-message-log.png)
**Message Log**：すべてのバックアップと復元にわたる、上限付きでタイムスタンプ付きの診断ログ

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-servers.png" width="280">](docs/screenshots/gui-preferences-servers.png)
**マルチ PBS**：複数の PBS サーバーを管理し、接続をテストし、既定を選択

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-edit-server.png" width="280">](docs/screenshots/gui-preferences-edit-server.png)
**サーバーの追加/編集**：URL、ユーザー/パスワードまたは API トークン、データストア、名前空間

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-theme-amber.png" width="280">](docs/screenshots/gui-theme-amber.png)
**テーマ**：5 つの組み込みカラーパレットとカスタム色。アプリ全体の色がその場で変わります（下の一覧を参照）

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-advanced.png" width="280">](docs/screenshots/gui-preferences-advanced.png)
**バックアップオプション**：正直に説明しています。並列での復元展開は実験的機能で、私たち自身のテスト機では*遅く*なると計測されました

</td>
</tr>
</table>

**🎨 カラーテーマ**：見本だけでなく、Preferences ダイアログ自体が選ぶたびに色を変えます：

<table>
<tr>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-amber.png" width="180">](docs/screenshots/gui-theme-amber.png)
Amber

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-blue.png" width="180">](docs/screenshots/gui-theme-blue.png)
Classic Blue

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-green.png" width="180">](docs/screenshots/gui-theme-green.png)
Green

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-red.png" width="180">](docs/screenshots/gui-theme-red.png)
Red

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-dark.png" width="180">](docs/screenshots/gui-theme-dark.png)
Dark

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-custom.png" width="180">](docs/screenshots/gui-theme-custom.png)
Custom

</td>
</tr>
</table>

**🌍 多言語**：フランス語、イタリア語、スペイン語の Reports ページ（ほかに英語、ドイツ語、ポーランド語など 15 言語）：

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-lang-french.png" width="280">](docs/screenshots/gui-lang-french.png)
Français

</td>
<td width="33%">

[<img src="docs/screenshots/gui-lang-italian.png" width="280">](docs/screenshots/gui-lang-italian.png)
Italiano

</td>
<td width="33%">

[<img src="docs/screenshots/gui-lang-spanish.png" width="280">](docs/screenshots/gui-lang-spanish.png)
Español

</td>
</tr>
</table>

### システムファイルの自動除外（フォルダーバックアップ）
フォルダーバックアップでは、次のものを自動的に除外します：

**システムフォルダー：** 場所を問わず `System Volume Information`（VSS の保存領域で 100 GB を超えることもある）と `$RECYCLE.BIN`。`Recovery` はドライブのルートにある場合のみ。`.pbs-rollback`（Roll back の退避フォルダー）。
**システムファイル：** `pagefile.sys`、`hiberfil.sys`、`swapfile.sys`、`DumpStack.log.tmp`。

**なぜ重要か：** ドライブの使用量が 1.03 TB と表示されていても、実際のファイルは約 141 GB ということがあります。除外しないと VSS スナップショットまでバックアップされ（容量と時間の無駄）、除外すればサイズが実データと一致します。

**推奨：** ファイル単位のバックアップにはこれらの除外が効く**フォルダー**バックアップを、ベアメタル復元と P2V には別の Backup Set で**マシン**バックアップを使ってください（ディスク全体のイメージで、空き領域とページファイル・休止状態ファイルはゼロとして入ります）。

### セキュリティと品質
- 全チャンクのクライアント側暗号化（任意）と署名付きマニフェスト
- 設定フォルダーは Windows では管理者のみ、Linux では所有者のみがアクセス可能
- ローカルのサービス API は 127.0.0.1 のみで待ち受け、トークンが必要で、トークンはサービス起動のたびに更新
- 入力検証と資格情報のサニタイズ
- パストラバーサル対策
- 指数バックオフ付きの再試行ロジック
- 全モジュールのエラー処理と単体テスト、さらに任意のライブテストと実際の PBS に対するエンドツーエンドテスト

## 🔐 クライアント側暗号化

スナップショットは、マシンを出る前に Proxmox Backup Server の鍵ファイルで暗号化できます（`proxmox-backup-client key create` が書く形式なので、鍵は公式クライアントと相互に使えます）。各チャンクは AES-256-GCM で暗号化され、マニフェストには署名が付きます。PBS サーバーが鍵を目にすることはありません。

**GUI：** **Preferences > Account Information** を開き、サーバーを編集してその **Encryption** タブを使います。手元にある鍵を **Import...** で取り込む（鍵ファイルのコピー、またはカメラ、スキャン、写真から読み取るか手入力したペーパーキー）か、**Generate...** で新しく作ります。タブには鍵のフィンガープリントが表示されます。Reports には各バックアップが暗号化されたかどうかが表示されます。**鍵ファイルがなければスナップショットは復元できません。このマシン以外の安全な場所にコピーを保管してください。**

**パスフレーズで保護された鍵（scrypt または PBKDF2）：** アプリが鍵をどう解錠するかを選びます：
- **Ask me**：パスフレーズはアプリのセッションごとに 1 回尋ね、メモリにだけ保持します。スケジュール実行と起動時のバックアップは入力を求めません。そのセッションでパスフレーズが入力されるまで、分かりやすいメッセージを出してすぐに失敗します。Ask の鍵では、バックグラウンドサービス経由のバックアップもすぐに失敗します。
- **Remember on this computer**：パスフレーズは PBS トークンと同じく設定に保存されるので、`config.json` を読める人なら誰でも鍵を解錠できます。無人のスケジュールジョブとバックグラウンドサービスにはこちらを選びます。

暗号化されたスナップショットを選ぶと、復元時にパスフレーズを尋ねます。

**コマンドライン：**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
`-keyfile-passphrase` は保護された鍵にだけ必要で、省略するとプロンプトで尋ねます。`proxmoxbackup-machine` と `proxmoxbackup-nbd` も同じ 2 つのフラグを受け付けます。`proxmoxbackup-nbd` は暗号化ディスクイメージの接続にこれらが必要で、NBD デバイスに触れる前に最初のチャンクを確認するため、鍵が間違っていればすぐに失敗します。

**ベアメタル復元：** 復元 ISO が鍵（なし、USB メモリ上の鍵ファイル、パスの入力）と、保護された鍵のパスフレーズを尋ねます。鍵が ISO に書き込まれることはありません。[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md) を参照してください。

**既知の制限**
- GUI が生成した鍵は保護された ACL（現在のユーザー、SYSTEM、Administrators のみ）で書き込まれます。ほかの場所に置いた鍵ファイルのアクセス権は、そのフォルダーの設定に従います。
- `config.json` には PBS トークン、パスワード、記憶したパスフレーズが平文で入っています。v0.9.0 からは、そのフォルダーは管理者のみ (Windows) または所有者のみ (Linux) に開放されています。v0.8.2 までは Windows のローカル Users が読めたため、共用マシンでは自分でアクセスを制限してください。
- 暗号化されたスナップショットの復元には同じ鍵ファイルが必要です。失った場合の回復手段はありません。
- Ask の鍵の流れとサービスは別のプロセスです。サービス経由で動くものには Remember を使ってください。

## より速いフォルダーバックアップ：変更検出

既定の Legacy モードでは、フォルダーバックアップのたびにすべてのファイルを読むため、何も変わっていなくてもフォルダー全体を読むのと同じ時間がかかります。Backup Sets（とコマンドライン）では、代わりに公式 `proxmox-backup-client` の変更検出モードを使えます。Metadata モードは 761,866 ファイル、480.9 GB のサーバーで毎日使われており、変更の少ない実行は 4 時間以上かかっていたものが 10〜15 分で終わります。

| モード | 読み込み | 保存形式 |
|---|---|---|
| **Legacy**（既定） | 全ファイル | フォルダーごとに `<name>.pxar.didx` 1 つ（従来どおり） |
| **Data** | 全ファイル | 分割アーカイブ：`<name>.mpxar.didx`（メタデータ）+ `<name>.ppxar.didx`（ファイル内容） |
| **Metadata** | 新規・変更ファイルのみ | 同じ分割アーカイブ。サイズ、更新日時（ナノ秒単位）、モード、所有者が前回のバックアップと一致するファイルは、読まずに前回から引き継ぐ |

**Metadata** モードの初回はすべてを読み（まだ比較対象がないため）、以降は変わった分だけを読みます。サイズと更新日時が変わらない形で変更されたファイルには気付かないため、セットでは **N 回ごとに全ファイルを読む**（Data での実行）設定もできます。その実行でそうした変更を拾い、再利用によってアーカイブに残った詰め物も取り除きます。完全に読めなかったファイル（読み取りエラーの後にゼロで埋めたもの）は、次の実行で必ず読み直します。再利用は、同じ方式・同じ鍵で暗号化された前回のバックアップからのみ行われます。

変更のないデータは読みも送りもしないため、リモートやオフサイトの PBS への通信量も、実際に変わった分とほぼ同じで済みます。

**GUI：** Backup Set を編集し、**Destination** タブの **Change detection**。フォルダーのセット専用です。マシン（ディスク全体）のバックアップはディスクイメージなので影響を受けず、ベアメタル復元や Proxmox VE での復元は従来とまったく同じです。単発バックアップは Legacy を使います。次の全読み込みまでの回数には、正常に完了した実行だけが数えられます。Backup Sets の一覧には「Next full read in N runs」と表示されます。進捗カードには、ある実行がいつ、なぜ全ファイルを読んでいるのかが表示されます。**Reports** には各実行のモードと、再利用した変更のないファイルの数が表示されます。

**セットごとに Backup ID を 1 つ。** Metadata モードはバックアップグループ内の最新スナップショットと比べます。2 つのセット（または 2 台のマシン）が同じサーバーで同じ Backup ID を使うと、各実行が相手のスナップショットと比べることになり、すべてを読んでしまいます。Backup Set のエディターはこれを警告します。

**コマンドライン：** `-change-detection-mode legacy|data|metadata`、または JSON 設定の `"change-detection-mode"`。コマンドラインには「N 回ごと」のフラグはないので、代わりに `-change-detection-mode data` での実行をときどきスケジュールしてください。

**セットを Data か Metadata に切り替える前に、** 復元に使うすべてのマシンを v0.6.0 以降に更新してください。分割アーカイブ対応前のビルドは分割アーカイブを読めません（フォルダーは見えても中のファイルが表示されません）。公式 `proxmox-backup-client` はバイト単位まで同一に復元でき（3.4.9 でテスト済み）、PBS 4.2 のファイルブラウザーでも開けます。あるフォルダーの最初の分割バックアップは、以前の Legacy バックアップとほとんどデータを共有しないため、古いスナップショットが削除 (prune) されるまでは、データストア上でそのフォルダーのフルコピーがおよそ 1 つ分増えます。

## 🚀 クイックスタート (GUI)

1. インストール：Windows は MSI、Linux は `.deb` か `.rpm`、または[リリース](https://github.com/mjb-is/proxmoxbackupclient_go/releases)の zip か tar.gz を展開（[ダウンロードとインストール](#-ダウンロードとインストール)を参照）
2. 起動する。Windows では VSS、マシンバックアップ、Backup Sets の変更のために **Run as administrator**（管理者として実行）します。Linux では自分のファイルのフォルダーバックアップは自分のユーザーで動きます。マシンバックアップとブロックスナップショットには **Proxmox Backup Client (administrator)** を使います（または root で実行）
3. **Preferences > Account Information** で PBS サーバーを追加し、**Test** する
4. Backup Set（または単発バックアップ）を作り、バックアップするフォルダーかディスクを選ぶ
5. 実行する。誰もサインインしていなくてもスケジュールを動かすには **Preferences > Advanced > Run as Service** をオンにする

各手順は [GUI マニュアル](docs/manual/GUI.md)で詳しく説明しています。

## マシン全体のライブバックアップ

`proxmoxbackup-machine`（と GUI のマシンバックアップ）は、Windows 10/11 や Windows Server のシステム全体、または Linux マシンを、停止させずにバックアップします。

コマンドの書き方はフォルダーバックアップとほぼ同じで、`-backupdir` の代わりに `-backupdev` を使います（複数ディスクなら繰り返し指定）。
たとえば次のように実行します：

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

上のコマンドはディスク 0 を調べ、マウント中のパーティションをすべて検出して VSS スナップショットを取り、ディスク全体の起動可能なバックアップイメージを fidx として作ります。空き領域とページファイル・休止状態ファイルは読まずにゼロとしてイメージに入ります（Linux では ext2/3/4 と XFS の空き領域も同様）。`-readfreespace` を付けると、フォレンジック用のコピーとしてすべてを読みます。読めないセクターはゼロとしてイメージに入り、実行結果にその場所が記録されます。`-type vm` を付けると、バックアップを Proxmox VE の VM バックアップとして保存します（[P2V](#-物理マシンを仮想マシンに-p2v)）。

次回のバックアップは増分になります。ハッシュ計算は並列化されているため、1 GB/秒の速度も容易に出ます。

### Linux マシンバックアップの前提条件（VSS 相当）

Windows のマシンバックアップは VSS を使います（上で「管理者として実行」が必要なのはそのためです）。Linux には VSS がないため、このプロジェクトは同じ役割に `elastio-snap` カーネルモジュール（`dattobd` のフォーク）を使い、書き込みが続いているライブのブロックデバイスから、一貫したある時点のスナップショットを取ります。元の `elastio/elastio-snap` リポジトリはアーカイブ済みで、メンテナンスされている版は [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap) です。インストールするのはこちらです。`dattobd` がすでに読み込まれていれば、それも使えます。

次の 2 つはどちらも、それぞれ独立に必須です。欠けると Linux のマシンバックアップは実行を拒否します：

1. **カーネルモジュールがインストールされ、読み込まれていること。** `elastio-snap` の文書にあるリポジトリパッケージでのインストール（[その INSTALL.md](https://github.com/Axcient/elastio-snap/blob/develop/INSTALL.md) を参照）は、現在少なくとも Ubuntu 22.04 では移動済みまたは壊れた URL を指しています（2026-09-24 確認：リダイレクトの後にリポジトリパッケージが 404 になる）。代わりにソースからビルドすれば確実に動きます：
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
   これは今回の起動中だけ有効な形でビルド・読み込みします。起動のたびに自動で読み込みたい場合は、ディストリビューションのドキュメントを参照してください（例：`/etc/modules-load.d/` への登録）。

2. **バックアップ処理自体が root (EUID 0) で動くこと。** モジュールが読み込まれていても、`CreateVSSSnapshot`（名前は Windows 風ですが Linux のスナップショット処理です）は root 以外のユーザーでの実行をはっきり拒否します。一貫したディスク全体のスナップショットにはモジュールの root 専用の制御インターフェースが必要なため、これは絶対条件で、`disk` グループへの所属などで代わりになるものではありません。マシンバックアップには **Proxmox Backup Client (administrator)** メニュー項目、バックグラウンドサービス、または `sudo`（か root）で GUI や `proxmoxbackup-machine` を実行してください。通常のファイル/ディレクトリのバックアップにはこの要件はありません。

マシンバックアップが `backup device /dev/sdX: open /dev/sdX: permission denied` で失敗する場合、たいていは基本的な読み取りのためにユーザーを `disk` グループに入れれば済みます（`sudo usermod -aG disk $USER` の後、ログアウトしてログインし直す）。ただし、それだけでは本当のスナップショットには**足りません**。その直後に上の root 要件に引っかかります。

### ディスクバックアップからのファイル復元

GUI の Restore ページは、マシンバックアップをフォルダーバックアップと同じように開きます。イメージ内の各ボリューム（NTFS、FAT、exFAT、ext2/3/4、XFS）が、閲覧して復元できるフォルダーになります。マウントは何もしません。

Linux では、nbd ツールでディスクバックアップをブロックデバイスとして接続することもできます。

nbd を使うには、まず `modprobe nbd max_part=0` を実行してください。

理由は不明ですが、`max_part != 0` を使うとパーティション検出が無限ループします。

NBD ツールは、VM バックアップでも host バックアップでも、固定ディスクのバックアップなら何でも接続できます（つまり PVE のバックアップにも使えます）。

使うには、次のようなコマンドを実行します：
`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

`-path` を省くと、fidx ファイルを選ぶためのターミナル UI が表示されます。

> 重要な処理を動かしているマシンでは使わないでください。壊れたファイルシステムが OS をクラッシュさせることがあり、Proxmox VE がこの用途に QEMU インスタンスを使うのはそのためです。

`proxmoxbackup-nbd` を止める前に、nbd ディスク上のものはすべてアンマウントしてください。さもないと、パーティションが使用中のまま残り、アンマウントできなくなる可能性が高くなります。

`Device or resource busy` エラーが出たら、`nbd-client -d /dev/nbd0` で強制切断するか、単に再起動してください。

### 物理マシンまたは VM への復元

ベアメタル復元 ISO には、完全自動の **「PBS Bare Metal Restore」** 起動メニュー項目があります。下の[ベアメタル復元 ISO](#-ベアメタル復元-iso) を参照してください。メニューの先頭にあり、タイムアウトで既定として起動し、Clonezilla 自体の言語とキーボードの質問を飛ばします。PBS の接続情報を入力し、スナップショットを選び、自動検出された復元先ディスクを確認すれば、ネットワーク設定、NBD 接続、復元そのもの、自動再起動まで、追加の入力なしで進みます。同じ ISO で空の仮想マシンにも復元でき、[P2V](#-物理マシンを仮想マシンに-p2v) に使えます。

以前からある半手動の方法も同じ ISO に残っており、Clonezilla の通常のモード選択メニューに独立した項目（「pbs-nbd」）として入っています。PBS の接続情報を案内に沿って入力すると、選んだスナップショットをバックグラウンドで `/dev/nbd0` に接続し、あとは Clonezilla 標準のディスク復元ウィザードで実際の復元を手動で進めます。

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
  -no-mtime                  Record 1970-01-01 as every file and folder time (advanced; see the CLI manual)
  -backupstream string       Back up standard input as <name>.didx instead of a directory
  -pxarout string            Also write the archive to a local file (debugging)
  -mail-host, -mail-port, -mail-username, -mail-password, -mail-insecure,
  -mail-from, -mail-to, -mail-subject-template, -mail-body-template
                             Email report (optional)
  -config string             JSON config file; flags given on the command line override it
```

公式クライアントの `PBS_REPOSITORY`、`PBS_SERVER`、`PBS_PORT`、`PBS_DATASTORE`、`PBS_AUTH_ID`、`PBS_PASSWORD`、`PBS_FINGERPRINT` 環境変数も読み込みます。

JSON 設定用に JSON の例が用意されているので、必要な項目だけ埋めてください。

メールのテンプレートについて：
メールの件名と本文には [Go のテンプレートエンジン](https://pkg.go.dev/text/template)を使います。書き方はそのドキュメントを参照してください。
テンプレートで使える変数は次のとおりです：

- `.NewChunks`：新たに作られたチャンクの数
- `.ReusedChunks`：再利用されたチャンクの数
- `.Datastore`：データストア名
- `.Error`：エラー（ある場合）（`.ErrorStr` はテキストとして返し、エラーがなければ空）
- `.ReadErrors`：読めなかったファイルやフォルダー。`.ReadErrorCount` はその数
- `.Hostname`：マシンのホスト名
- `.StartTime`：バックアップの開始時刻
- `.EndTime`：バックアップの終了時刻
- `.Duration`：バックアップの所要時間
- `.FromattedDuration`：整形されたバックアップの所要時間（コード上この綴りです）
- `.Success`：エラーも読めないものもなくバックアップが完了したとき true
- `.Partial`：スナップショットは保存されたが、一部のファイルが読めなかったとき true
- `.Status`：`Success`、`Partial`、`Failed` のいずれか

## ストリームバックアップ

pxar の代わりにストリームをバックアップでき、可能性は無限に広がります。たとえば次のように実行できます：

```
mysqldump yourdatabase | ./proxmoxbackup-directory -backupstream yourdatabase.sql [other options]
```

これにより、たとえば tar を使う場合や SQL ダンプそのものでも、buzhash による重複排除を活用できます。

## 既知の問題

Windows Defender のマルウェア対策が有効だと、バックアップは本来出せる速度より最大 25% 遅くなります。

Windows のコマンドラインツールはプロセスロックを取ります。実行中に 2 つ目のバックアップを始めると「Another backup job is running」と表示して終了します。Linux にはこのロックがないので、2 つのコマンドライン実行を同時刻にスケジュールしないでください。

## 📋 動作要件

- Windows 10/11 または Windows Server（64 ビット）。VSS とマシンバックアップには管理者権限
- または Linux（パッケージは Debian、Ubuntu、Mint、Fedora 向け）。GUI には WebKitGTK ランタイム、一貫したマシンバックアップには root と elastio-snap または dattobd カーネルモジュール
- Proxmox Backup Server へのネットワーク接続
- P2V の場合：PBS データストアをストレージとして追加した Proxmox VE、または復元 ISO を起動できる任意のハイパーバイザー

## 🔨 ソースからのビルド

GUI とバックグラウンドサービス、コマンドラインツール、MSI、Linux パッケージの完全な手順は **[BUILD.md](BUILD.md)** にあります。概要としては、Go 1.25 以降と Wails CLI v2.13 で：

```bash
cd gui
wails build                                    # the app, in gui/build/bin
go build -tags service -o build/bin/ProxmoxBackupClientSVC.exe   # the background service
```

```bash
packaging/debian/build-deb.sh     # .deb
packaging/fedora/build-rpm.sh     # .rpm (in Docker)
```

```powershell
installer\wix\build-msi.ps1       # MSI (WiX 3), after building the app and the service
```

ビルド済みのフロントエンド (`gui/frontend/dist`) はリポジトリに含まれているので、Node.js が必要なのはフロントエンドを変更するときだけです（`gui/frontend` で `npm install` と `npm run build`、またはホットリロード用に `wails dev`）。テスト：`make test`。

## 🔧 高度な使い方とガイド

### マルチ PBS（複数の PBS サーバー）

複数の PBS サーバーを設定し、Backup Set や単発バックアップごとに送り先を選べます（例：`C:\Users` は毎日高速な SSD の PBS へ、`D:\` は毎週大容量の PBS へ、さらに DR 用サーバー）。

- **[ユーザーガイド](MULTI_PBS_USER_GUIDE.md)**：サーバーの追加とテスト、既定サーバー、バックアップごとのサーバー選択、FAQ、トラブルシューティング。
- **[実装ガイド](MULTI_PBS_GUIDE.md)**：データモデル、単一 PBS 構成からの自動移行、バックエンドのメソッド。

従来の単一 PBS 構成は、初回読み込み時に自動で `default` サーバーへ移行されます。

### 💿 ベアメタル復元 ISO

復元 ISO は、標準の Clonezilla Live ISO に `pbsnbd` / `machinebackup` バイナリと 2 つのメニュー項目を加えるパッチを当てたものです。CD、USB メモリ（`dd` で書き込むか、Ventoy の USB メモリから）、PXE から起動でき、UEFI と BIOS のどちらのマシンにも対応します：

- **`ocs-pbs-bare-metal-restore`**：メニューの先頭にあり、タイムアウトで既定として起動する **「Proxmox Backup Client Go - PBS Bare Metal Restore」**。Clonezilla の言語とキーボードの質問、汎用ウィザードを飛ばし、PBS の資格情報の入力から、復元完了と自動再起動まで一気に進みます。まず DHCP を試し（失敗したときだけ固定 IP を尋ねる）、NTP 同期を行い、バックアップが暗号化されていれば USB メモリから暗号鍵を読み、起動に使った USB メモリを決して候補にしない復元先ディスク検出を行います。成功すると完了メッセージを表示して再起動します。Windows と Linux のどちらのバックアップでも同じように復元でき（復元はブロック単位）、ベアメタルにも空の VM にも復元できます。
- **`ocs-pbs-nbd`**：Clonezilla 自体のモード選択メニューにある **「pbs-nbd」** 項目。PBS の接続情報と復元するスナップショットを尋ね、それをバックグラウンドで `/dev/nbd0` に接続してから、Clonezilla 通常のディスク復元ウィザードに引き渡し、そこからは手動で操作します。

復元側ではほかにも次の問題を修正しました。マシン全体の復元を止めていた PBS チャンク取得処理の `zstd.NewReader` デッドロック（[PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85) として本家に貢献）、Clonezilla 自体の `is_disk_without_part_and_fs()` が udev と競合して起きる「初回の復元が 'no partition' で失敗し、再実行すると成功する」バグ、そして BIOS/MBR マシンが復元後に起動しなかった問題（MBR と最初のパーティションの間のデータも復元するようにしました）です。

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-37-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

詳細（その場での置き換えではなく ISO 全体を作り直す理由、前提条件、メニューの流れ、検証）は **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)** に、使い方は **[ベアメタル復元マニュアル](docs/manual/BMR.md)** にあります。

### Windows GUI のビルド

**ネイティブ Windows**：ツールチェーン（Go、MinGW、Wails）と、アプリ・サービス・MSI のビルドコマンドは **[BUILD.md](BUILD.md)** を参照してください。

**Docker（Linux でビルドする場合）。** `./build_gui_windows_docker.sh` が、使い捨ての `golang` コンテナーで、WebView2 に正しく対応した Windows アプリを `ProxmoxBackupClientGO.exe` としてビルドします（mingw と Wails をインストールし、フロントエンドをビルドし、`wails build` を実行）。バックグラウンドサービスと MSI はビルドしません。

### 機能の状況、変更履歴、内部ドキュメント

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)**：完了・一部完了・未完了の機能と、どのバージョンからか。
- **[CHANGELOG.md](CHANGELOG.md)**：バージョンごとの変更履歴。master にある内容は [Unreleased]。
- **[TODO.md](TODO.md)**：未着手のロードマップとアイデア。
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)**：各リリースの内容。
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**：MSI のアンインストールダイアログ（設定を残す/削除する）とそのテスト計画。
- **[AGENTS.md](AGENTS.md)**：コントリビューターとコーディングエージェント向けの、リポジトリ構成と規約。

## 🔁 本家プロジェクトへの貢献

汎用的に役立つ修正と機能は、[tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) にプルリクエストとして提案しています。これまでに 18 件がマージされました：

  - [PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85): Fix zstd decoder goroutine deadlock in `GetChunkData` (hung full-machine restores)
  - [PR #86](https://github.com/tizbac/proxmoxbackupclient_go/pull/86): Fully automated PBS Bare Metal Restore boot entry for the Clonezilla ISO
  - [PR #87](https://github.com/tizbac/proxmoxbackupclient_go/pull/87): Linux snapshots: drive elastio-snap/dattobd through ioctl instead of `elioctl`/`dbdctl`
  - [PR #88](https://github.com/tizbac/proxmoxbackupclient_go/pull/88): Restore: re-apply captured NTFS ACLs (Windows) and POSIX ACLs and xattrs (Linux)
  - [PR #90](https://github.com/tizbac/proxmoxbackupclient_go/pull/90): `machinebackuplib`: make `vm` backups restorable and bootable on Proxmox VE
  - [PR #91](https://github.com/tizbac/proxmoxbackupclient_go/pull/91): GUI: back machines up as `host`, not `vm`, by default
  - [PR #92](https://github.com/tizbac/proxmoxbackupclient_go/pull/92): `patch-clonezilla`: skip patches whose change is already in the base ISO
  - [PR #93](https://github.com/tizbac/proxmoxbackupclient_go/pull/93): `patch-clonezilla`: add the PBS bare-metal restore entry to `isolinux.cfg`
  - [PR #94](https://github.com/tizbac/proxmoxbackupclient_go/pull/94): Restore the data between the MBR and the first partition in the BMR flow (BIOS/MBR machines boot after restore)
  - [PR #95](https://github.com/tizbac/proxmoxbackupclient_go/pull/95): Fix reading passphrase-protected key files made by `proxmox-backup-client`
  - [PR #96](https://github.com/tizbac/proxmoxbackupclient_go/pull/96): `directorybackup`: back up several directories in one run
  - [PR #97](https://github.com/tizbac/proxmoxbackupclient_go/pull/97): Docs: point elastio-snap at the maintained Axcient fork
  - [PR #98](https://github.com/tizbac/proxmoxbackupclient_go/pull/98): `pbscommon`: skip the key file permission check on Windows
  - [PR #99](https://github.com/tizbac/proxmoxbackupclient_go/pull/99): Restore encrypted backups from the Clonezilla BMR ISO
  - [PR #102](https://github.com/tizbac/proxmoxbackupclient_go/pull/102): `snapshot`: delete our VSS shadow copies after every backup, through the VSS API
  - [PR #103](https://github.com/tizbac/proxmoxbackupclient_go/pull/103): `machinebackup`: skip hashing blank (all-zero) chunks
  - [PR #104](https://github.com/tizbac/proxmoxbackupclient_go/pull/104): CLIs: report errors on stderr, never in a message box
  - [PR #105](https://github.com/tizbac/proxmoxbackupclient_go/pull/105): Restore: put back every file time, also 1970 and earlier, and folder times

## 🧪 状況、バグ報告と機能要望

**これはアルファ版ソフトウェアであり、現状のまま提供され、いかなる種類の保証も一切なく、使用はすべて自己責任です。** ソースファイルを含め、データの損失や損傷について私は一切の責任を負いません。

これは私自身のホームラボのために作っています。各リリースは公開前に私がテストし、そこで日常的に使っています。Windows と Linux のマシンに対するスケジュールされたフォルダーバックアップとマシンバックアップを行い、その過程で復元、Undelete、Roll back、ベアメタル復元、P2V 復元もテストしてきました。実際に復元してみるまで、どのバックアップも実証されたとは言えません。頼りにする前に、ベアメタル復元も含めて、ご自身の復元を必ずテストしてください。

- **バグ報告は歓迎します**：[issue](https://github.com/mjb-is/proxmoxbackupclient_go/issues) を開き、何をしたか、何が起きたか、ログ（Message Log、または設定フォルダー内のファイル）を添えてください。
- **機能要望**：私は自分の用途に役立つ機能を追加しているので、ほかの方のための要望は受け付けていません。本家プロジェクト [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go/issues) に送ってください。バグを修正するプルリクエストや、明らかに私の用途にも役立つプルリクエストは歓迎します。

最も役に立つのはバグ報告と修正です。別の形でお礼をしたい場合は、[Ko-fi でコーヒーをおごっていただけます](https://ko-fi.com/mickbeeby)。

## 🙏 クレジットと経緯

元のコマンドラインクライアントは、2023 年から Tiziano Bacocco (tizbac) が書いたものです。後に RDEM Systems のグラフィカルクライアントがそこに統合され、それとともに最初の GUI、マルチ PBS 対応、Windows サービスが加わりました。このフォークは 2026 年 9 月に、PBS を私の唯一のバックアップシステムにするうえで足りなかった部分を埋めるために始まりました。リリースは v0.3.0 から始まり、汎用的な修正は本家プロジェクトに還元しています。

- 元のクライアント [proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go)。作者は Tiziano Bacocco (tizbac) とそのコントリビューター。
- RDEM Systems の Nimbus Backup GUI クライアント。本アプリの GUI の出発点です。
- Backup for Workgroups (BFW) のワークフローと使い心地。Backup Sets、Undelete、Roll back はこれにならっています。
- [Clonezilla Live](https://clonezilla.org/)。ベアメタル復元 ISO のベースです。

## 📄 ライセンス

GPLv3：[LICENSE](LICENSE) ファイルを参照してください。

---

**フォーク以降の変更 © 2026 mjb-is。Tiziano Bacocco (tizbac)、Proxmox Backup Client GO のコントリビューター、RDEM Systems の成果 (© 2024-2026) に基づいています。**

☕ このクライアントが役に立ったら、[Ko-fi でコーヒーをおごっていただけます](https://ko-fi.com/mickbeeby)。ありがとうございます。
