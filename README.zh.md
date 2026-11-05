# Salvanza for Proxmox Backup Server：适用于 Proxmox Backup Server 的 Windows 和 Linux 客户端

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

> ℹ️ 本文是[英文 README](README.md) 的译文。两者内容不一致时，以英文版为准。

> ⚠️ **商标与关联声明。** 本项目是一个**独立的社区项目**，并非 Proxmox 官方产品：它**与 Proxmox Server Solutions GmbH 没有任何关联，也未获得其认可或赞助**。**Proxmox®** 是 Proxmox Server Solutions GmbH 在欧盟、美国及其他国家的注册商标，本文**仅**用它来说明兼容性。按照 Proxmox 自己的商标指引（注册商标不得用于产品或应用的名称，例如应写成“XY App for Proxmox Virtual Environment”，而不是“Proxmox XY App”），**无品牌版客户端已经更名**：原名为 *Proxmox Backup Client* 的产品现在叫 **Salvanza for Proxmox Backup Server**（简称 Salvanza），这个名称不含 Proxmox 商标。具体规则见 [Proxmox 媒体资料包](https://www.proxmox.com/en/about/company-details/media-kit)。

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/salvanza/salvanza_client)](https://github.com/salvanza/salvanza_client/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Salvanza for Proxmox Backup Server（原名 Proxmox Backup Client）是一款开源（GPL-3.0）的 Proxmox Backup Server（PBS）备份客户端，支持 Windows 和 Linux。** 它可以把文件夹备份为可浏览的文件归档，把整台机器备份为可启动的磁盘映像，按计划或随时手动运行。从单个文件到整台机器，都可以恢复：恢复到同一台计算机、恢复到替换机器，或恢复为 Proxmox VE 虚拟机。

项目网站：**[salvanza.org](https://salvanza.org/)** · 源代码、版本发布和问题反馈：**[github.com/salvanza/salvanza_client](https://github.com/salvanza/salvanza_client)**

它是一**套工具**：

- **Salvanza 应用** `salvanza-gui`（最初由 RDEM Systems 开发，名为 Nimbus Backup GUI，后由 mjb-is 重新设计和扩展）：适用于 Windows 和 Linux 的应用程序。每个 Backup Set（备份集）可设置多个计划；文件夹备份和整盘备份都使用一致性快照（Windows 上用 VSS，Linux 上用 elastio-snap/dattobd）；可以从文件夹备份和机器备份中恢复文件；支持 Undelete（找回已删除文件）和 Roll back（回滚）、多台 PBS 服务器、客户端加密，以及后台服务 `salvanza-svc`，让计划任务在无人登录时也能运行。
- **`salvanza-cli`**：一个命令行工具，每项任务对应一个子命令：
  - **`salvanza-cli backup-folder`**：文件夹（pxar）备份，带重复数据删除和变更检测。
  - **`salvanza-cli backup-disk`**：在线整盘备份（fidx），适用于 Windows 和 Linux 机器。恢复后的映像可以直接启动。
  - **`salvanza-cli attach-nbd`**：把磁盘备份挂接为 NBD 块设备（Linux）。
- **裸机恢复 ISO**（Salvanza-BMR）：基于 Clonezilla Live，从开机到重启全程引导恢复。

> 关键词：salvanza · proxmox backup server client windows · PBS 客户端 · Windows VSS 备份 · 不可变异地备份 · 物理机转虚拟机（P2V） · 裸机恢复 · Proxmox Backup Server 界面。

<a id="-new-in-20"></a>

## 💡 为什么使用本客户端

官方的 `proxmox-backup-client` 是一个 Linux 命令行工具。本客户端使用同一台服务器和相同的备份格式，并且增加了：

- **Windows。** Windows 没有官方客户端。这里有命令行工具和应用程序，通过 VSS 快照备份文件夹，并可在线备份整个磁盘。
- **适用于 Windows 和 Linux 的应用程序**：按计划运行的 Backup Set（备份集），由后台服务执行，无需任何人登录；带搜索的恢复、Undelete（找回已删除文件）和 Roll back（回滚）、报告和电子邮件。
- **整台机器**：可启动的磁盘镜像、从镜像中恢复单个文件，以及引导式裸机恢复 ISO，可恢复到同一台计算机、新硬件或 Proxmox VE 虚拟机。
- **保留 Windows 文件信息**：NTFS 权限、属性、备用数据流、扩展属性和创建时间会随每次文件夹备份一起记录，本客户端的恢复会把它们还原。
- **不被绑定**：本客户端备份的内容可以用官方客户端恢复，官方客户端备份的内容也可以用本客户端恢复（见下文）。

## 🆕 2.0 新特性

2.0 版本把大量关于 GUI、后台服务和备份引擎的工作合并进了本项目（见 [CHANGELOG](CHANGELOG.md)）。与 1.2 相比：

- **应用以管理员权限连接后台服务。** 安装后台服务后，这台计算机的 PBS 服务器和 Backup Sets（备份集）都归服务所有，而且只有服务能读取它的设置文件夹。应用连接时会请求一次管理员权限（Windows 上是 UAC，Linux 上是 pkexec；在标准账户中需要输入管理员的用户名和密码）。只有一个小型辅助程序会被提权，应用只在内存中保存服务的令牌。无法通过权限提示的用户可以选择 **Use my own settings**（使用我自己的设置），保留自己的服务器和备份集，这些备份集在应用打开时运行；之后使用 **Connect**（连接）时，应用会提出把它们复制到这台计算机的设置中。应用本身不以管理员权限运行，在某项操作需要时会提供 **Restart as administrator**（以管理员身份重新启动）。见 [GUI 手册](docs/manual/GUI.md#connecting-to-the-background-service)第 15 节。
- **应用会确认对方确实是后台服务。** 在向服务的本地 API 发送任何内容之前，应用会检查是谁在应答：在 Windows 上必须是服务自己的进程，在 Linux 上必须是属于 root 的套接字。如果服务停止时有其他程序占用了该端口，它什么也得不到。
- **每个备份集可有多个计划**，频率从每隔几分钟到每年一次（例如每月最后一天、每月第二个星期二），每个计划都用通俗的语言显示，并列出接下来的 3 次运行时间。
- **增量文件夹备份**，采用官方客户端的变更检测（Data 和 Metadata 模式）：未改变的文件不会再次读取。
- **更快的机器备份**：跳过空闲空间、分页文件和休眠文件；遇到无法读取的扇区时写入零，而不是让备份失败。
- **可同时运行多个备份**（**Maximum Concurrent Backups**（最大并发备份数），1 到 8，默认 3），后台服务中同样如此，每个备份都有自己的快照、日志、Pause（暂停）和 Stop（停止）。
- **速度限制**：分别限制上传到 PBS、从 PBS 下载和磁盘读取的速度，每项限制由同时运行的所有任务共享。
- **Undelete（找回已删除文件）和 Roll back（回滚）**，适用于文件夹和机器备份集，可在 GUI 中使用，也可在命令行中使用。
- **从机器备份中取回文件**：浏览磁盘映像中的 NTFS、FAT、exFAT、ext2/3/4 和 XFS 卷，并恢复单个文件。
- **可以妥善保管的加密密钥**：保存副本，打印带二维码的纸质密钥，并可从文件、摄像头、照片或手工输入的文本导入密钥。
- **机器备份集按磁盘身份锁定磁盘**：硬件变更后磁盘编号改变，备份的仍是原来那块磁盘；磁盘缺失时本次运行会失败，而不会去备份另一块磁盘。
- **可以把分区排除在机器备份之外**：在每块磁盘下勾选要备份的分区。被排除的分区不会被读取，也不会做快照，在映像中以零填充，因此恢复后它会回到原来的位置、保持原来的大小，但内容为空且未格式化。磁盘选择器会标出恢复后的磁盘启动所需的分区。
- **Linux 机器备份使用 elastio-snap 或 dattobd。** Linux 机器备份集可以关闭快照、直接在线读取磁盘，但前提是所有者勾选确认接受备份可能不一致。
- **每个用户可在无品牌版应用中选择自己的配色主题。** 品牌版应用保留其品牌配色。
- 除了在备份集菜单中，还可以在编辑器中**启用和禁用备份集**；如果备份集按保存的设置无法运行（已禁用、没有启用任何计划、快照有问题、磁盘缺失），保存时会给出警告。
- **一个命令行工具 `salvanza-cli`**，用子命令 `backup-folder`、`backup-disk` 和 `attach-nbd` 取代原来的三个独立工具；旧名称仍然可用。
- **Linux 软件包**（`.deb`、`.rpm` 和 Arch），软件包名为 `salvanza`，服务名为 `salvanza.service`，包含后台服务。
- **引导式裸机恢复 ISO**，也可以恢复到空的虚拟机中（P2V）。以 Proxmox VE 虚拟机形式备份的机器，也可以像 Proxmox VE 自己备份的任何虚拟机一样，在 Proxmox VE 中直接从 PBS 恢复。
- **品牌版构建**：同一套二进制文件分别以 Salvanza、Nimbus Backup 和 Etitech Backup 安装程序发布，每个品牌都有自己的程序名称和设置文件夹。
- **18 种界面语言。**

## 🔁 从 Proxmox Backup Client 升级

Salvanza 是 Proxmox Backup Client 的新名称，升级是自动完成的。安装 Salvanza（MSI 或 Linux 软件包）会卸载之前安装的 Proxmox Backup Client（或其他品牌版）及其后台服务。Salvanza 首次启动时，会把之前的设置（PBS 服务器、密钥、备份集和历史记录）从 `C:\ProgramData\ProxmoxBackupClient`、`%APPDATA%\ProxmoxBackupClient`、`~/.proxmox-backup-guardian` 或 `/var/lib/proxmoxbackupclient` 迁移过来，并在新文件夹中的 `pre-salvanza-<date>` 下保留一份带日期的设置文件副本。PBS 上的备份不会受到任何影响。

| 之前 | 现在 |
|---|---|
| `ProxmoxBackupClient.exe`、`ProxmoxBackupClientSVC.exe` | `salvanza-gui.exe`、`salvanza-svc.exe`（Windows 服务 `salvanza-svc`） |
| `proxmoxbackup-directory`、`proxmoxbackup-machine`、`proxmoxbackup-nbd` | `salvanza-cli backup-folder`、`salvanza-cli backup-disk`、`salvanza-cli attach-nbd`（旧名称仍然可用） |
| `C:\Program Files\ProxmoxBackupClient`、`C:\ProgramData\ProxmoxBackupClient` | `C:\Program Files\Salvanza`、`C:\ProgramData\Salvanza` |
| Linux 软件包和服务 `proxmoxbackupclient` | 软件包 `salvanza`，服务 `salvanza.service`，设置位于 `/var/lib/salvanza`，个人设置位于 `~/.salvanza` |

## ⭐ 功能与实测结果

| | 结果 |
|---|---|
| **增量文件夹备份** | 一个 480.9 GB、包含 761,866 个文件的文件共享：首次运行 4 小时 17 分钟，之后每次 10 到 15 分钟。未改变的文件甚至不会被打开（Metadata 变更检测，与官方客户端相同）。 |
| **快速机器备份** | 不读取空闲空间、分页文件和休眠文件，也不对空白磁盘空间计算哈希。一块已用 373 GB 的 1 TB 笔记本磁盘，备份时间从 1 小时 23 分钟到 1 小时 44 分钟缩短到 38 分钟。 |
| **低速和按流量计费的链路** | 只发送新数据，并且经过压缩；应用会显示实际通过网络传输的数据量。一台远程站点的小型 Linux PC，使用 4G 连接（上行 4.8 Mbit/s）：整个 128 GB 磁盘映像加上主文件夹，11 分钟完成，传输约 172 MiB。 |
| **从机器备份中取回文件** | 一台笔记本 127 GB 的 C: 盘，包含 430,000 个文件，列出文件只需 4 到 15 秒。无需事先挂载或下载任何内容。 |
| **裸机恢复** | 启动 ISO（U 盘、光盘或网络启动），输入 PBS 信息，选择备份：一台 32 GB 的 Windows 机器在开机 8 分钟后就回到了登录界面。 |
| **Undelete（找回已删除文件）** | 从一个包含 761,000 个文件的共享中删除 39 GB 后，对最近 7 天的备份进行 Undelete 搜索，约一分钟就准确找到了这些文件，并把它们放回原处。 |
| **恢复保真度** | Windows 上：NTFS 权限、属性、备用数据流、扩展属性、创建时间和文件夹时间；Linux 上：权限模式、所有者、ACL、扩展属性和符号链接。 |

## 📖 用户手册

| 手册 | 内容 |
|---|---|
| **[GUI 手册](docs/manual/GUI.md)** | 安装、PBS 服务器、备份集与计划、增量备份、进度、恢复、Undelete、Roll back、Reports（报告）、日志、Preferences（首选项）、后台服务、故障排除 |
| **[命令行手册](docs/manual/CLI.md)** | `salvanza-cli backup-folder`、`backup-disk`、`attach-nbd`：所有参数、配置文件、退出代码、计划任务 |
| **[裸机恢复手册](docs/manual/BMR.md)** | 恢复 ISO 的分步说明；把 `vm` 备份恢复到 Proxmox VE；物理机转虚拟机；恢复到不同硬件 |
| [恢复场景](docs/RESTORE_GUIDE.md) | 什么情况下使用哪种恢复方式 |
| [手册索引](docs/manual/README.md) | 各手册概览 |

同样的手册也内置在应用中（**Help**（帮助），F1），因此离线也能使用，并且与已安装的版本一致。

<a id="-download"></a>

## 📦 下载

👉 **[下载最新版本](https://github.com/salvanza/salvanza_client/releases)**

| 你的环境 | 使用 |
|---|---|
| Windows | **MSI 安装程序** `Salvanza-<version>-x64.msi`。它把应用、后台服务和 `salvanza-cli` 安装到 `C:\Program Files\Salvanza`，把该文件夹加入 PATH，启动服务，并添加“开始”菜单项。品牌版安装程序（`NimbusBackup-<version>-x64.msi`、`EtitechBackup-<version>-x64.msi`）是同一构建，只是品牌不同 |
| Debian、Ubuntu、Mint | `sudo apt install ./salvanza_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./salvanza-<version>-*.rpm` |
| Arch | 由 [packaging/arch/PKGBUILD](packaging/arch/PKGBUILD) 构建的软件包（`make pkgarch`） |
| 只用命令行 | 各平台的 `salvanza-cli` 压缩包 |
| 需要恢复的机器 | 裸机恢复 ISO（[裸机恢复手册](docs/manual/BMR.md#3-get-the-iso-and-make-the-boot-media)），或自行构建（[见下文](#-bare-metal-restore-iso)） |

如果某个版本没有提供 Linux 软件包，可以用 `make deb`、`make pkgfedora` 或 `make pkgarch` 自行构建（[BUILD.md](BUILD.md)、[packaging/debian/README.md](packaging/debian/README.md)）。

Linux 软件包会安装一个菜单项 **Salvanza for Proxmox Backup Server**，以当前用户身份启动应用。全新安装时会启用后台服务（`salvanza.service`）；升级时保留你之前的选择。`salvanza` 软件包取代之前的 `proxmoxbackupclient` 和 `pbsgo` 软件包。软件包依赖 polkit（pkexec），应用用它来连接服务。

> ⚠️ **Windows 提示“检测到病毒”（例如 `Trojan:Win32/Sabsik.FL.A!ml`）或出现 SmartScreen 警告？**
> 这是 Go/Wails 应用程序**已知的误报**：它*不是*病毒。后缀 `!ml` 表示这是机器学习模型的检测结果，它会标记*未签名且不常见*的可执行文件。请按下文所示验证下载的文件。

### 🔎 验证下载的文件

每个版本都提供 SHA-256 校验和（`SHA256SUMS.txt`，也列在发布页面上）以及一份**已签名的来源证明（provenance attestation）**：它以密码学方式证明该文件是由本仓库的 CI 从某个确切的提交构建出来的。

```powershell
Get-FileHash .\Salvanza-<version>-x64.msi -Algorithm SHA256   # compare with SHA256SUMS.txt
gh attestation verify .\Salvanza-<version>-x64.msi --repo salvanza/salvanza_client
```

在 Linux 上：`sha256sum -c SHA256SUMS.txt --ignore-missing`。

如果某个版本的安装程序在 VirusTotal 上扫描无异常，发布页面会附上扫描报告的链接。

> ℹ️ **代码签名：** Windows 二进制文件**尚未进行 Authenticode 签名**（正在通过 [SignPath Foundation](https://signpath.org) 申请开源证书）。在此期间，文件来源通过上述来源证明和校验和来确认。

## 🚀 快速入门（GUI）

1. 安装：Windows 上用 MSI，Linux 上用 `.deb`、`.rpm` 或 Arch 软件包（[下载](#-download)）。
2. 启动 **Salvanza for Proxmox Backup Server**。安装程序会设置好后台服务，因此应用会请求一次管理员权限来连接服务（Windows 上是 UAC，Linux 上通过 pkexec 输入你的密码）。如果无法提供管理员权限，请选择 **Use my own settings**（使用我自己的设置）。
3. 在 **Preferences > Account Information**（首选项 > 账户信息）中添加你的 PBS 服务器，并点击 **Test**（测试）。
4. 创建一个 Backup Set（备份集）或一次性备份，选择要备份的文件夹或磁盘。
5. 运行它。有了后台服务，无论是否有人登录，计划任务都会运行。

[GUI 手册](docs/manual/GUI.md)详细介绍了每一个步骤。

### 🪟 无人值守 / 静默安装（MSI）

MSI 支持 Windows Installer 的标准静默安装参数，并为每项默认设置提供公共属性：

```powershell
# Silent install with the defaults (service: Automatic + started)
msiexec /i Salvanza-<version>-x64.msi /quiet

# Silent install that installs the service but does NOT enable/autostart it
# (created as Manual and left stopped)
msiexec /i Salvanza-<version>-x64.msi /quiet SERVICEAUTOSTART=0

# Silent install that also applies a settings file exported from another computer
msiexec /i Salvanza-<version>-x64.msi /quiet CONFIGFILE="C:\Deploy\settings.json"

# Custom directory + verbose install log
msiexec /i Salvanza-<version>-x64.msi /quiet INSTALLFOLDER="D:\Apps\Salvanza" /L*v "C:\Temp\install.log"

# Silent uninstall (KEEP_CONFIG=0 also purges %ProgramData%\Salvanza)
msiexec /x Salvanza-<version>-x64.msi /quiet KEEP_CONFIG=0
```

| 属性 | 默认值 | 含义 |
| --- | --- | --- |
| `SERVICEAUTOSTART` | `1` | `0` = 服务以 **Manual**（手动）方式安装并保持**停止**：已安装，但从不启用或自动启动 |
| `AUTOSTART` | `1` | `0` = 每个用户登录时不在托盘中启动应用 |
| `DESKTOPSHORTCUTS` | `1` | `0` = 不创建桌面快捷方式（“开始”菜单项总会添加） |
| `CONFIGFILE` | | 用 **Preferences > Export Settings**（首选项 > 导出设置）保存的文件的完整路径：在服务启动前合并其中的 PBS 服务器和备份集。根据导出计算机名称生成的备份 ID，会改为根据每台计算机自己的名称生成。见 [installer/DEPLOYMENT.md](installer/DEPLOYMENT.md) |
| `KEEP_CONFIG` | `1` | 仅用于卸载：`0` = 同时删除 `%ProgramData%\Salvanza` 状态目录（品牌版构建则删除它自己的目录，例如 `%ProgramData%\NimbusBackup`） |
| `INSTALLFOLDER` | `C:\Program Files\Salvanza` | 安装目录（也可在安装界面中修改） |

请以管理员身份（提权）运行；退出代码 `0` = 成功，`3010` = 成功但需要重启。品牌版 MSI 接受完全相同的参数。

## ✨ 功能

### GUI
主要功能见 [2.0 新特性](#-new-in-20)。此外还有：
- 18 种界面语言：法语、英语、意大利语、德语、波兰语、西班牙语、保加利亚语、捷克语、希腊语、匈牙利语、拉脱维亚语、立陶宛语、荷兰语、葡萄牙语、罗马尼亚语、斯洛伐克语、土耳其语和乌克兰语
- 文件夹或整盘的**一次性备份**；机器备份可存储为 `host` 备份，也可存储为 Proxmox VE `vm` 备份，并提供连空闲空间一起读取的**取证副本**选项
- **恢复**：支持文件搜索（通配符、正则表达式、完整路径）、选择性恢复、NTFS/POSIX ACL、备用数据流、Windows 创建时间，以及可选的校验步骤
- 对正在运行的备份执行 **Pause, Resume and Stop**（暂停、继续和停止），在 Windows 上也可从托盘操作，后台服务执行的备份同样适用
- 实时进度卡片（预计完成时间、已完成文件数、上传速度、已传输数据量、压缩率）、**Reports**（报告）、**Message Log**（消息日志）和 **View Full Logs**（查看完整日志）
- 电子邮件通知、备份后操作、备份集克隆、设置导出和导入
- 多台 PBS 服务器，支持证书指纹锁定（首次使用时信任），可用 API 令牌或用户名/密码登录
- 五种配色主题和一种自定义颜色，由每个用户自行选择（无品牌版应用；品牌版应用保留其品牌配色）

### 命令行：`salvanza-cli`
- `salvanza-cli backup-folder`：文件夹（pxar）备份，带重复数据删除和 `-change-detection-mode legacy|data|metadata`
- `salvanza-cli backup-disk`：Windows（VSS）和 Linux（elastio-snap/dattobd）机器的在线整盘备份（fidx，增量，除非使用 `-readfreespace` 否则跳过空闲空间，`-type vm` 生成 Proxmox VE 备份，`-skippartition <disk>:<number>` 排除某个分区）
- `salvanza-cli attach-nbd`：用于恢复磁盘备份的 NBD 服务器（Linux）
- `salvanza-cli version` 显示版本，`salvanza-cli help` 列出子命令，`salvanza-cli <command> -help` 列出某个子命令的选项
- 服务程序 `salvanza-svc`（Windows 上为 `salvanza-svc.exe`；在 Linux 上也可以用应用 `salvanza-gui`）：对 GUI 中定义的备份集执行 `-list-sets`、`-undelete` 和 `-rollback`

这三个子命令接受 `-keyfile` 和 `-keyfile-passphrase`，用于加密快照。这些工具的旧名称 `proxmoxbackup-directory`、`proxmoxbackup-machine` 和 `proxmoxbackup-nbd`（以及 `pbsdirectorybackup`、`pbsmachinebackup`、`pbsnbd`）仍然以相同的参数运行相同的子命令，因此现有脚本可以继续使用。完整参考：[命令行手册](docs/manual/CLI.md)。

### 📸 截图

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Backup Sets**（备份集）：带计划和类型标记的命名任务，每项都有 Run Now（立即运行）

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Backup progress**（备份进度）：吞吐量、预计完成时间、新数据块和复用的数据块

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Restore**（恢复）：选择 PBS 服务器和备份 ID，列出快照或按文件名搜索

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-schedule-tab.png" width="280">](docs/screenshots/gui-schedule-tab.png)
**Schedules**（计划）：每个备份集可有多个计划，并显示接下来的 3 次运行

</td>
<td width="33%">

[<img src="docs/screenshots/backup-set-as-vm.png" width="280">](docs/screenshots/backup-set-as-vm.png)
**Backup as a Proxmox VE VM**（备份为 Proxmox VE 虚拟机）：输入时即显示 PBS 备份 ID

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Reports**（报告）：完整的运行历史和详情面板

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-machine-partitions.png" width="280">](docs/screenshots/gui-machine-partitions.png)
**机器备份集**：在每块磁盘下勾选要备份的分区

</td>
<td width="33%">

[<img src="docs/screenshots/gui-themes.png" width="280">](docs/screenshots/gui-themes.png)
**Themes**（主题）：六种配色主题，由每个用户各自选择

</td>
<td width="33%">

[<img src="docs/screenshots/gui-languages.png" width="280">](docs/screenshots/gui-languages.png)
**语言**：18 种，这里是法语、德语、意大利语、西班牙语、波兰语和乌克兰语

</td>
</tr>
</table>

### 智能系统排除（文件夹备份）
文件夹备份会自动排除以下内容：

**系统文件夹：** 任何位置的 `System Volume Information`（VSS 存储区，可能超过 100 GB）和 `$RECYCLE.BIN`；仅位于驱动器根目录时的 `Recovery`；`.pbs-rollback`（Roll back 的安全文件夹）。
**系统文件：** `pagefile.sys`、`hiberfil.sys`、`swapfile.sys` 和 `DumpStack.log.tmp`。

**为什么这很重要：** 一个驱动器可能显示已用 1.03 TB，而实际文件只有约 141 GB。如果不排除，备份会包含 VSS 快照（浪费空间和时间）；排除之后，备份大小与真实数据相符。

**建议：** 文件级备份请使用**文件夹**备份；裸机恢复和 P2V 请在单独的备份集中使用**机器**备份（整盘映像，其中空闲空间、分页文件和休眠文件以零写入）。

### 安全与质量
- 服务的设置文件夹只对 SYSTEM 和 Administrators 开放（Windows），或只对 root 开放（Linux）；其本地 API 监听 127.0.0.1，并需要令牌，该令牌在每次启动时以及每 24 小时更换一次
- 在确认是服务本身在应答之前（Windows 上为服务的进程，Linux 上为属于 root 的套接字），应用不会向该 API 发送任何内容
- 在 Windows 上，`config.json` 中的 PBS 令牌和密码针对本机加密（DPAPI）
- 输入验证、凭据清理、防止路径遍历、按指数退避重试
- 每个模块都有单元测试，另有可选的在线测试和针对真实 PBS 的端到端测试

## 🔐 客户端加密

快照可以在离开本机之前加密，使用 Proxmox Backup Server 的密钥文件（即 `proxmox-backup-client key create` 生成的格式，因此密钥可与官方客户端互换使用）。每个数据块都用 AES-256-GCM 加密，清单（manifest）经过签名。PBS 服务器永远看不到密钥。

**GUI：** 打开 **Preferences > Account Information**（首选项 > 账户信息），编辑某台服务器，使用其 **Encryption**（加密）选项卡。用 **Import...**（导入）导入已有的密钥（密钥文件的副本，或用摄像头、扫描件、照片读取的纸质密钥，或手工输入），或用 **Generate...**（生成）新建一个。选项卡会显示密钥指纹。**Save a copy...**（保存副本）和 **Print a paper key...**（打印纸质密钥，带二维码）可在本机之外保留一份副本。Reports（报告）会显示每次备份是否已加密。**没有密钥文件就无法恢复快照：请把副本保存在安全的地方，并远离这台机器。**

**受口令保护的密钥（scrypt 或 PBKDF2）：** **Ask me**（询问我）在一次应用会话期间把口令保存在内存中；**Remember on this computer**（在这台计算机上记住）像 PBS 令牌一样保存口令，适用于计划任务和后台服务。

**命令行：**
```shell
salvanza-cli backup-folder -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
只有受保护的密钥才需要 `-keyfile-passphrase`（或 `PBS_ENCRYPTION_PASSWORD`），省略时会提示输入。`salvanza-cli backup-disk` 和 `salvanza-cli attach-nbd` 接受相同的参数。`attach-nbd` 会在操作 NBD 设备之前先用快照检验密钥，因此密钥错误时会立即失败。

**裸机恢复：** 恢复 ISO 会询问密钥（无密钥、U 盘上的密钥文件，或手工输入的路径），对于受保护的密钥还会询问口令。密钥永远不会写入 ISO。

恢复加密快照需要相同的密钥文件；密钥丢失后无法恢复。

## 更快的文件夹备份：变更检测

在默认的 Legacy 模式下，每次文件夹备份都会读取每个文件，因此即使什么都没变，一次运行的耗时也和读取整个文件夹一样。备份集和 `salvanza-cli backup-folder` 可以改用官方 `proxmox-backup-client` 的变更检测模式：

| 模式 | 读取 | 存储形式 |
|---|---|---|
| **Legacy**（默认） | 每个文件 | 每个文件夹一个 `<name>.pxar.didx` |
| **Data** | 每个文件 | 拆分归档：`<name>.mpxar.didx`（元数据）+ `<name>.ppxar.didx`（文件内容） |
| **Metadata** | 仅新文件和已更改的文件 | 同样的拆分归档；大小、修改时间（精确到纳秒）、权限模式和所有者都与上次备份相同的文件直接取自上次备份，不再读取 |

在 **Metadata** 模式下，首次运行会读取全部内容；之后的运行只读取变化的部分。如果文件的改动没有改变其大小和修改时间，就不会被发现，所以备份集还可以设置**每 N 次运行读取一次所有文件**。请为每个备份集设置单独的 Backup ID（备份 ID）：Metadata 模式会与备份组中最新的快照进行比较。在命令行中请使用 `-change-detection-mode`。

**在把备份集切换到 Data 或 Metadata 之前，** 请先把所有用于恢复的机器升级到 2.0：更早的版本会显示拆分归档中的文件夹，但不显示其中的文件。见 [GUI 手册第 7 节](docs/manual/GUI.md#7-incremental-folder-backups-change-detection)。

<a id="-physical-machines-into-virtual-machines-p2v"></a>

## 🔄 物理机转虚拟机（P2V）

物理 PC 或服务器的机器备份可以通过两种方式变成 Proxmox VE 虚拟机：

1. **在 Proxmox VE 中恢复。** 把备份集（或一次性备份）设置为 **Backup as: Proxmox VE virtual machine**（备份为：Proxmox VE 虚拟机）。备份以 `vm/<number>` 形式存储，因此 Proxmox VE 会把它与其他虚拟机备份一起列出，并恢复为虚拟机（一块 SATA 磁盘；启动前请添加网卡）。
2. **用恢复 ISO 恢复到空的虚拟机中**：虚拟机的固件需与原机器相同，SATA 磁盘至少同样大，网卡为 Intel E1000。这种方式对 `host` 和 `vm` 备份都适用，而且同一个备份仍然可以恢复到裸机上。

已测试的路径包括 Windows 10 和 Windows Server（UEFI/GPT）、Linux 桌面（UEFI/GPT 和 BIOS/MBR），以及一个密钥存放在 U 盘上的加密 Linux 备份。[裸机恢复手册](docs/manual/BMR.md#7-restoring-onto-different-hardware-p2v-and-replacement-machines)中有完整的表格、固件选择、驱动程序和分步说明。

## 🤝 与官方 Proxmox 客户端双向兼容

已用 `proxmox-backup-client` 3.4.9 和 PBS 4.2 验证：

- **本客户端生成的 Legacy、Data 和 Metadata 快照，用官方客户端恢复时逐字节一致**，无论快照是通过 GUI 还是命令行、在 Windows 还是 Linux 上生成的；PBS 4.2 的文件浏览器也能打开它们。
- **官方客户端生成的 Legacy、Data 和 Metadata 快照，用本客户端恢复时逐字节一致**，完整恢复和选择性恢复均可。
- 相同的拆分归档格式（pxar 版本 2）、相同的变更检测模式、相同的加密密钥文件格式、相同的 `PBS_REPOSITORY` 风格环境变量。

你的备份永远不会被本客户端绑定。标准归档之外还有两项附加内容，没有它们，官方客户端也能正常恢复文件：

- 文件夹快照根目录下的 `.proxmox_backup_client_meta.json` 记录原始路径，官方客户端会把它当作普通文件恢复。
- 每个文件夹快照中保存的 `proxmox-client-acls.json.gz.blob` 记录归档无法容纳的信息：Windows 上是 NTFS 权限、属性、创建时间、备用数据流和扩展属性；Linux 上是扩展属性和 POSIX ACL。官方客户端和 PBS 网页界面会忽略它；本客户端的恢复会把这些信息还原。

在 Linux 上，官方客户端把 POSIX ACL、扩展属性和文件 capabilities 保存在归档本身中。本客户端目前还不写入也不读取这些条目，因此在 Linux 上，ACL 在两个客户端之间的任何方向都不会保留。以官方的方式实现这一点（作为可选项）已列入[路线图](TODO.md)。

## 整机在线备份

`salvanza-cli backup-disk`（以及 GUI 的机器备份）可以在不停机的情况下备份完整的 Windows 10/11 或 Windows Server 系统，或一台 Linux 机器。

命令语法与文件夹备份基本相同，只是 `-backupdir` 换成了 `-backupdev`（多块磁盘时重复该参数）。例如：

`salvanza-cli backup-disk -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

上面的命令会查看磁盘 0，检测所有已挂载的分区，对它们做 VSS 快照，然后把整块磁盘生成为 fidx 格式的可启动备份映像。空闲空间、分页文件和休眠文件不经读取，直接以零写入映像（在 Linux 上，ext2/3/4 和 XFS 的空闲空间也是如此）；`-readfreespace` 则读取全部内容，用于制作取证副本。无法读取的扇区以零写入映像，运行结果会指出其位置。加上 `-type vm`（并使用数字形式的 `-backup-id`）可把备份存储为 Proxmox VE 虚拟机备份（[P2V](#-physical-machines-into-virtual-machines-p2v)）。

下一次备份是增量的。哈希计算是并行的，因此速度可达 1 GB/s。

要排除某个分区，例如游戏库或已由文件夹备份覆盖的数据分区，请加上 `-skippartition <disk>:<number>`（多个分区时重复该参数），例如 `-skippartition \\.\PhysicalDrive0:4` 或 `-skippartition /dev/sda:3`。该分区不会被读取，也不会做快照，在映像中以零填充：分区表会保留，因此恢复后该分区会回到原来的位置、保持原来的大小，但内容为空且未格式化。在 GUI 中，在备份集里取消勾选磁盘下的该分区即可。

### Linux 机器备份的前提条件（相当于 VSS）

Windows 机器备份使用 VSS。Linux 没有 VSS，因此备份正在运行的 Linux 主机时，会用一个内核模块为每个已挂载的分区做时间点快照：`elastio-snap`（仍在维护的分支 [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap)）或 `dattobd`。

必须满足两个条件，否则 Linux 机器备份会拒绝运行：

1. **内核模块必须已安装并加载。** 请通过 DKMS 安装，这样每次内核更新后都会自动重新构建，并设置为每次启动时加载。GUI 手册中的 [Linux 快照：安装 elastio-snap 或 dattobd](docs/manual/GUI.md#linux-snapshots-installing-elastio-snap-or-dattobd) 一节提供了在 Debian、Ubuntu、Mint 和 Fedora 上测试过的步骤，并介绍了 Secure Boot 和检查方法。截至 2026 年 11 月，这两个模块都无法在内核 7 上构建（更新后的 Fedora 44）。

2. **备份必须以 root 身份运行。** 该模块的控制接口仅限 root 使用，普通用户请求快照会被拒绝。后台服务以 root 身份运行；没有后台服务时，请在应用中使用 **Restart as administrator**（以管理员身份重新启动），或用 `sudo` 运行 `salvanza-cli backup-disk`。普通的文件夹备份不需要 root。

在 GUI 中，Linux 机器备份集可以取消勾选 **Use snapshots**（使用快照）并在线读取磁盘，但必须同时勾选第二个复选框（“Run without a snapshot: I accept the backup may be inconsistent”，即“不使用快照运行：我接受备份可能不一致”）。否则每次运行都会停止并说明原因。未挂载的分区会被直接读取。

### 从磁盘备份中恢复文件

GUI 的 Restore（恢复）页面会像打开文件夹备份一样打开机器备份：映像中的每个卷（NTFS、FAT、exFAT、ext2/3/4 或 XFS）都是一个可以浏览并从中恢复的文件夹。无需挂载任何内容。

在 Linux 上，也可以用 `salvanza-cli attach-nbd` 把磁盘备份挂接为块设备。请先运行 `modprobe nbd max_part=0`：如果 `max_part != 0`，内核可能会在探测分区时陷入无限循环。

`attach-nbd` 可以连接任何固定磁盘备份，无论是 VM 备份还是 host 备份（因此也适用于 Proxmox VE 备份）。例如：

`sudo salvanza-cli attach-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

如果省略 `-path`，会出现一个终端界面让你选择 fidx 文件；`-list` 则列出所有映像。

> 不要在运行重要任务的机器上这样做：损坏的文件系统可能导致操作系统崩溃，这也是 Proxmox VE 为此使用一个 QEMU 实例的原因。

在停止 `salvanza-cli attach-nbd` 之前，请先卸载 nbd 磁盘上的所有内容，否则分区很可能一直处于忙碌状态。如果出现 `Device or resource busy` 错误，可用 `nbd-client -d /dev/nbd0` 强制断开，或重新启动。

### 恢复到物理机或虚拟机

使用[裸机恢复 ISO](#-bare-metal-restore-iso)，恢复到裸机或空的虚拟机中。[裸机恢复手册](docs/manual/BMR.md)有分步说明。

## 用法：目录备份

完整参考（包括 Windows 和 Linux 的示例、退出代码和计划任务）见 **[命令行手册](docs/manual/CLI.md)**。一个典型的命令：

```shell
salvanza-cli backup-folder -baseurl "https://yourpbshost:8007" -certfingerprint pbsfingerprint -authid "user@realm!apiid" -secret "apisecret" -backupdir "C:\path\to\backup" -datastore "datastorename"
```

```
salvanza-cli backup-folder
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

官方客户端的 `PBS_REPOSITORY`、`PBS_SERVER`、`PBS_PORT`、`PBS_DATASTORE`、`PBS_AUTH_ID`、`PBS_PASSWORD` 和 `PBS_FINGERPRINT` 环境变量也会被读取。

JSON 配置见 [config.json.example](config.json.example)；只需填写你需要的字段。

### 一次运行备份多个目录

重复使用 `-backupdir`，即可用一条命令备份多个目录：

```shell
salvanza-cli backup-folder -baseurl "https://yourpbshost:8007" -certfingerprint pbsfingerprint -authid "user@realm!apiid" -secret "apisecret" -datastore "datastorename" -backupdir "C:\Data" -backupdir "D:\Photos"
```

每个目录都作为单独的备份组进行备份，备份 ID 为 `<backup-id>_<path>`（`-backup-id` 默认为主机名）。因此，保留策略、清理（prune）和恢复都把每个目录当作独立的序列。某个目录失败不会影响其他目录，退出代码会报告失败。`-pxarout` 不能与多个目录同时使用；如果两个目录最终会得到相同的备份 ID，会在上传任何内容之前被拒绝。

在 JSON 配置文件中使用 `"backupdirs": ["C:\\Data", "D:\\Photos"]`。单个的 `"backupdir"` 字段仍然有效，并且可以与 `"backupdirs"` 一起使用。加密方式与单个目录相同：密钥只读取一次，每个目录的备份组都用它加密。

### 不在归档中记录时间

每个文件和文件夹的修改时间都是归档的一部分，因此时间一变，归档的字节就会改变，该条目周围的数据块即使内容没变也会重新上传。对大多数数据来说，这是合理的取舍。但有些目录树会不断改动其文件夹，而文件本身保持不变：例如一个把数据存放在以哈希命名的文件夹（`aa/bb/<rest of the hash>`）下的存储，仅因为这个原因，每次运行就可能重新上传归档的很大一部分。

`-no-mtime`（在 JSON 配置文件中为 `"no-mtime": true`）会把每个文件和文件夹的时间都记录为 1970-01-01，这样只有真正的内容变化才会产生新的数据块。真实的时间不会保存在任何地方：恢复后每一项的日期都是 1970-01-01。使用 `-change-detection-mode metadata` 时，每个文件都会被读取，与 `data` 模式相同。该选项默认关闭；请只用于时间无关紧要的数据源。

### 邮件模板

不需要登录的中继服务器（通常在 25 端口）也可以使用：不填用户名和密码即可。发件人可以包含名称（`Backups <backup@example.com>`）。

邮件主题和正文使用 [Go 的模板引擎](https://pkg.go.dev/text/template)；语法请参阅其文档。可用的变量如下：

- `.NewChunks`：新建的数据块数量
- `.ReusedChunks`：复用的数据块数量
- `.Datastore`：数据存储名称
- `.Error`：错误（如有）（`.ErrorStr` 以文本形式给出，没有错误时为空）
- `.ReadErrors`：无法读取的文件或文件夹；`.ReadErrorCount` 为其数量
- `.Hostname`：机器的主机名
- `.StartTime`：备份开始时间
- `.EndTime`：备份结束时间
- `.Duration`：备份持续时间
- `.FromattedDuration`：格式化后的备份持续时间（代码中的名称就是这样拼写的）
- `.Success`：备份无错误完成且没有无法读取的内容时为 true
- `.Partial`：快照已保存但有部分文件无法读取时为 true
- `.Status`：`Success`、`Partial` 或 `Failed`

## 流备份

这种方式备份的是数据流，而不是 pxar 归档。例如：

```
mysqldump yourdatabase | salvanza-cli backup-folder -backupstream yourdatabase.sql [other options]
```

它使用 buzhash 分块，对数据流本身（例如 tar 归档或 SQL 转储）进行重复数据删除。

## 已知问题

Windows Defender 反恶意软件处于活动状态时，备份速度可能降到可达速度的约 25%。

在 Windows 上，`salvanza-cli backup-folder` 和 `backup-disk` 会使用进程锁：在一个备份运行期间启动第二个备份，会输出 "Another backup job is running" 并以代码 2 退出。Linux 上没有这样的锁，因此不要让两个命令行任务在同一时间运行。（GUI 和后台服务可以安全地同时运行多个备份，见[同时运行多个备份](docs/manual/GUI.md#67-several-backups-at-once)。）

## 📋 系统要求

- Windows 10/11 或 Windows Server（64 位）；安装时以及把应用连接到后台服务时需要管理员权限
- 或 Linux：软件包适用于 Debian、Ubuntu、Mint、Fedora 和 Arch；GUI 需要 WebKitGTK 运行时和 polkit；一致性机器备份需要 elastio-snap 或 dattobd 内核模块以及 root 权限
- 能够通过网络访问 Proxmox Backup Server
- P2V：已把 PBS 数据存储添加为存储的 Proxmox VE，或任何能启动恢复 ISO 的虚拟化平台

## 🔨 从源代码构建

**[BUILD.md](BUILD.md)** 有完整说明：GUI 和后台服务、命令行工具、MSI 以及 Linux 软件包。简而言之，使用 Go 1.25 或更高版本以及 Wails CLI v2.13：

```bash
cd gui
wails build                                                       # the app, gui/build/bin/salvanza-gui.exe
go build -tags service -o build/bin/salvanza-svc.exe              # the background service
cd ../cli
GOWORK=off go build -o ../gui/build/bin/salvanza-cli.exe .        # the command line
```

```bash
make cli          # salvanza-cli, in dist/
make deb          # .deb
make pkgfedora    # .rpm (in Docker)
make pkgarch      # Arch package (on Arch)
make msi          # Windows MSIs on Linux (Docker + Wine + WiX), one per brand
make clonezilla   # the bare-metal restore ISO, dist/Salvanza-BMR-<version>.iso
```

构建好的前端（`gui/frontend/dist`）已提交到仓库，因此只有修改前端时才需要 Node.js（在 `gui/frontend` 中运行 `npm install` 和 `npm run build`，或用 `wails dev` 进行热重载）。测试：`make test`。

## 🔧 高级用法与指南

### 多 PBS（多台 PBS 服务器）

可以配置多台 PBS 服务器，并为每个备份集或一次性备份选择目标（例如每天把 `C:\Users` 备份到一台快速 SSD 的 PBS，每周把 `D:\` 备份到一台大容量 PBS，再加一台灾难恢复服务器）。

- **[用户指南](MULTI_PBS_USER_GUIDE.md)**：添加和测试服务器、默认服务器、为每个备份选择服务器、常见问题和故障排除。
- **[实现指南](MULTI_PBS_GUIDE.md)**：数据模型、从单 PBS 配置自动迁移、后端方法。

旧的单 PBS 配置会在首次加载时自动迁移为名为 `default` 的服务器。

<a id="-bare-metal-restore-iso"></a>

### 💿 裸机恢复 ISO

恢复 ISO 名为 Salvanza-BMR，是在标准 Clonezilla Live ISO 的基础上，加入 `pbsnbd` 二进制文件（即 `salvanza-cli attach-nbd` 的代码，由 `nbd/cmd/pbsnbd` 单独构建）和两个菜单项制成的。它可以从光盘、U 盘（用 `dd` 写入，或放在 Ventoy U 盘中）启动，支持 UEFI 和 BIOS 机器：

- **`ocs-pbs-bare-metal-restore`**：启动菜单中的第一项，也是默认项，**PBS Bare Metal Restore**（PBS 裸机恢复）。它跳过 Clonezilla 的语言和键盘提示，从输入 PBS 凭据直接走到恢复完成：先尝试 DHCP（只有失败时才提示输入静态 IP），如果备份已加密则从 U 盘读取加密密钥，检测目标磁盘时绝不会把启动用的 U 盘列为目标。它同样能恢复 Windows 和 Linux 备份（恢复在块级别进行），恢复到裸机或空的虚拟机中。
- **`ocs-pbs-nbd`**：Clonezilla 自己的模式选择菜单中的 **pbs-nbd** 项。它会询问你的 PBS 连接信息和快照，把快照挂接到 `/dev/nbd0`，然后交给 Clonezilla 常规的磁盘恢复向导，由你手动操作。

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-37-amd64.iso \
  ./build/pbsnbd \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

`make clonezilla` 会下载标准 ISO，构建 `pbsnbd`，并完成同样的操作，生成 `dist/Salvanza-BMR-<version>.iso`。完整细节（为什么要完整重建 ISO 而不是就地替换、前提条件、菜单流程、验证）见 **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**；使用方法见 **[裸机恢复手册](docs/manual/BMR.md)**。

### 构建 Windows GUI

**原生 Windows**：工具链（来自 Chocolatey 的 Go 和 MinGW，以及 Wails）和构建应用、服务及 MSI 的命令见 **[BUILD.md](BUILD.md)**。

**Docker（在 Linux 上构建时）。** `./build_gui_windows_docker.sh` 在一个用完即弃的 `golang` 容器中构建 Windows 应用，并正确支持 WebView2。它不会构建后台服务或 MSI。

### 更新日志和内部文档

- **[CHANGELOG.md](CHANGELOG.md)**：各版本的变更历史。
- **[TODO.md](TODO.md)**：未完成的路线图和想法。
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)**：稳定的产品状态和可用的构建。
- **[installer/DEPLOYMENT.md](installer/DEPLOYMENT.md)**：把 MSI 部署到大量计算机。
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**：MSI 卸载对话框（保留/删除配置）及其测试计划。
- **[AGENTS.md](AGENTS.md)**：仓库结构和约定，供贡献者和编程代理参考。

## 🌐 意大利技术支持

需要说明的是，本项目及其**所有**贡献都将永远公开，并以 GPLv3 许可证授权。本项目以及最新机器备份功能的主要赞助方 E.T.I. Srl（ https://etitech.net ）可以为有需要的用户提供一般的 Proxmox 部署支持，特别是 Windows 备份工具方面的支持。

本项目永远不会区分“社区版”和“企业版”，只有技术支持是一项独立的服务。
你提出的任何定制，即使开发可能需要付费，也会像整个项目一样以 GPLv3 发布。

## 🖥️ GUI 来源说明

**Salvanza 应用**（原名 Proxmox Backup Client GUI）最初由 **[RDEM Systems](https://www.rdem-systems.com/)** 开发，名为 **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)**：他们设计并构建了最初的应用，并围绕其品牌版 Nimbus Backup 提供商业支持。之后 **[mjb-is](https://github.com/mjb-is)** 借鉴其他备份系统的工作流程和使用体验，对它进行了重新设计和扩展，这些工作已合并回本仓库。整个代码库，包括 GUI 及其所有功能，仍以 GPLv3 许可证开源。

## ⚠️ 警告

本软件按“原样”提供。尽管我们力求可靠，但对数据丢失或损坏不承担任何责任。在生产环境中依赖备份之前，请务必测试备份并验证恢复。任何备份只有在你真正从中恢复过之后才算得到证明，所以请亲自测试恢复，包括裸机恢复。

本软件仍处于 alpha 质量阶段，我们对任何形式的损坏或数据丢失（甚至包括源文件）不承担任何责任。

欢迎报告错误：请提交 [issue](https://github.com/salvanza/salvanza_client/issues)，说明你做了什么、发生了什么，并附上日志（Message Log（消息日志），或 **View Full Logs**（查看完整日志））。

## 📄 许可证

GPLv3：见 [LICENSE](LICENSE) 文件。

## 🏷️ 品牌

每位贡献了**至少 5 个**增加功能或修复问题的提交的贡献者，都有权把自己的品牌信息加入项目，用于商业用途。

唯一的条件是，品牌所指向的公司**没有**从事以下任何活动：

- 恶意软件活动
- 宣扬战争的业务（适用于任何国家，包括西方国家）
- 诈骗
- 数据窃取
- 儿童拐卖
- 暴力
- 歧视
- 毒品
- 任何普遍被认为违法的活动

如果任何贡献者收到投诉，我们会设法取得联系；如果无法给出合理的解释，我们将**立即终止**该权益。

**GPLv3 许可证仍然有效**，你仍然可以自由地分叉本项目并构建自己的可执行文件。

## 🤝 参与贡献

欢迎贡献，特别是：

1. 2.0 版 README 和用户手册的翻译
2. 数据块的异步 / 多核上传（机器备份已实现多核压缩）
3. Proxmox 端补丁：在 pxar 格式中增加一种包含 Windows 安全描述符的条目
4. 支持 Windows 符号链接和目录联接（目前会被跳过）
5. 在更多硬件、Linux 发行版和内核上进行测试，尤其是裸机恢复
6. 你能想到的任何有意思的东西 :)

## 关于 Salvanza 贡献者

Salvanza 贡献者（原 Proxmox Backup Client GO 贡献者）开发并维护本项目。本软件依赖 NTP/NTS 基础设施，以及社区参考列表中列出的 [11 个公共 NTS 服务器](https://github.com/jauderho/nts-servers)。

---

**© 2024-2026 Salvanza contributors and RDEM Systems.**
