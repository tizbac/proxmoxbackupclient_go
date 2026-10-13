# Proxmox Backup Client: клиент для Windows и Linux к Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Proxmox Backup Client: клиент резервного копирования с открытым исходным кодом (GPL-3.0) для Proxmox Backup Server (PBS), работающий в Windows и Linux.**

Это **набор инструментов** для резервного копирования в PBS:

- **Proxmox Backup Client GUI** (на основе Nimbus Backup GUI от RDEM Systems): графический клиент для серверов и рабочих станций Windows и Linux. Согласованные снимки (VSS в Windows, elastio-snap/dattobd в Linux), наборы заданий (Backup Sets) с расписанием, копии папок и целых дисков, восстановление, Undelete и Roll back, несколько серверов PBS, фоновая служба в Windows и Linux.
- **`proxmoxbackup-directory`**: копии папок (pxar) из командной строки, с дедупликацией и обнаружением изменений.
- **`proxmoxbackup-machine`**: копии целых дисков (fidx) работающих машин Windows (VSS) и Linux (elastio-snap/dattobd); после восстановления на «голое железо» образ загружается.
- **`proxmoxbackup-nbd`**: подключает копию диска как устройство NBD для восстановления (Linux).
- **ISO для восстановления на «голое железо»** на базе Clonezilla Live.

> ⚠️ **Отказ от ответственности:** этот проект **никак не связан** с **Proxmox Server Solutions GmbH**. «Proxmox», логотип Proxmox и связанные названия принадлежат их владельцам и используются здесь **только** для указания совместимости. См. [proxmox.com](https://www.proxmox.com/).

> ℹ️ **Об этой странице.** Это сокращённый перевод. Основной документ со всеми подробностями: английский [README.md](README.md). Руководства написаны на английском. Интерфейс программы не переведён на русский, поэтому названия меню и кнопок даны по-английски, как в программе. Перевод сделан с помощью ИИ; исправления приветствуются.

## 📖 Руководства пользователя

| Руководство | Содержание |
|---|---|
| **[GUI](docs/manual/GUI.md)** | Установка, серверы PBS, Backup Sets и расписания, инкрементные копии, восстановление, Undelete, Roll back, Reports, Preferences, фоновая служба, неполадки |
| **[Командная строка](docs/manual/CLI.md)** | `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd`: все флаги, конфигурация, коды выхода, планирование; Undelete и Roll back из командной строки |
| **[Восстановление на «голое железо»](docs/manual/BMR.md)** | ISO для восстановления по шагам; снимки `vm` в Proxmox VE; другое оборудование |
| [Сценарии восстановления](docs/RESTORE_GUIDE.md) | Какой способ выбрать |
| [Оглавление](docs/manual/README.md) | Обзор трёх руководств |

## 🆕 Новое в v0.9.0: несколько расписаний на Backup Set, установщики, служба в Linux

- **Несколько расписаний на Backup Set**, от раза в несколько минут до раза в год («каждый день в 02:00» плюс «в последний день месяца» или «во 2-й вторник»), каждое описано словами, с 3 ближайшими запусками. Существующие Backup Sets сохраняют своё расписание.
- **Копии папок сохраняют альтернативные потоки данных и расширенные атрибуты NTFS** (Windows), например отметку «загружено из интернета», и восстановление возвращает их.
- **Установщики**: **MSI для Windows** (программа и служба в Program Files, меню «Пуск», обновление поверх, выбор при удалении: сохранить или удалить настройки) и **пакеты Linux** `.deb` (Debian, Ubuntu, Mint) и `.rpm` (Fedora).
- **Фоновая служба в Linux** (`proxmoxbackupclient.service`): Backup Sets по расписанию выполняются от root без вошедшего пользователя. Включается как в Windows: Preferences > Advanced > Run as Service. **Restart as administrator** работает и в Linux (через pkexec).
- **Pause, Resume и Stop для копий, которые выполняет служба**.
- **Предупреждение, если два Backup Set пишут в одну группу копий** (тот же сервер, тип и Backup ID), значок **SHARED ID**.
- **Закрытые настройки**: `C:\ProgramData\ProxmoxBackupClient` доступна только администраторам; в Linux папка настроек доступна только владельцу.
- **Файлы из копий машин с FAT, exFAT, ext2/3/4 и XFS**, а не только NTFS; восстановление из образа Windows возвращает права доступа и альтернативные потоки данных.
- **Копии машин Linux пропускают свободное место** (ext2/3/4, XFS).
- **Undelete и Roll back для машин Linux**, а также **из командной строки**: `-list-sets`, `-undelete`, `-rollback` (раздел 12 [руководства по командной строке](docs/manual/CLI.md)).
- Утилиты командной строки пишут ошибки в stderr вместо окон сообщений; `proxmoxbackup-machine` отправляет письмо-уведомление.
- **ISO для восстановления на «голое железо»** никогда не предлагает в качестве целевого диска флешку, с которой загрузился, а руководство объясняет, как согласовать прошивку (UEFI или BIOS), когда физическая машина становится виртуальной.
- **Все языки вычитаны** (17 переводов, около 1850 строк); даты и числа следуют языку программы.
- Ускорение обработки пустого места из v0.8.2 (помечено тегом, отдельно не выпускалось) тоже вошло в этот выпуск.

Полный список см. в [CHANGELOG](CHANGELOG.md).

## 💽 Новое в v0.8.0

- **Копии машин в Windows пропускают свободное место** (и файлы подкачки и гибернации): наполовину пустой диск читается примерно вдвое быстрее. **Сбойный сектор** больше не останавливает копию: он записывается нулями, а место указывается в отчёте.
- **Восстановление файлов из копии машины**: каждый том NTFS образа открывается как папка, без монтирования и предварительной загрузки (диск C: на 127 ГБ с 430 000 файлов открывается за время от 4 до 15 секунд). Одна копия машины покрывает и восстановление на «голое железо», и «верните мне этот файл».
- **Undelete и Roll back** для Backup Sets машин (Windows), на выбранных папках (по умолчанию Desktop, Documents, Pictures, Music, Videos, Downloads).
- **Pause и Resume**, значок **VSS / Snapshot ON-OFF**, сохранение **времени создания** файлов Windows.
- **v0.8.2:** пустое место больше не хешируется, почти пустой диск обрабатывается примерно втрое быстрее (50 ГБ: было 3m36s, стало 1m15s).

## 🗑️ Новое в v0.7.0: Undelete и Roll back

- **Undelete** находит файлы, которые есть в копиях набора, но исчезли с диска (в последней копии или за 7 или 30 дней), показывает их деревом папок с датой исчезновения и восстанавливает на место или в другую папку. После удаления 39 ГБ с ресурса на 761 000 файлов поиск за 7 дней нашёл их примерно за минуту.
- **Roll back** возвращает папки набора к выбранной копии: сначала показывает изменения, затем восстанавливает. Заменяемые файлы откладываются на тот же диск, поэтому **Undo** возвращает всё обратно.

Также: предлагаемый Backup ID, время следующего запуска на карточке и фоновая служба в загрузке для Windows. См. [руководство по GUI](docs/manual/GUI.md#10-undelete).

## ⚡ Новое в v0.6.0: инкрементные копии папок

В режиме **Metadata** (как в официальном `proxmox-backup-client`) неизменённые файлы (тот же размер, время изменения до наносекунды, режим и владелец) не читаются: их данные берутся из предыдущего снимка. Файловый сервер на 480,9 ГБ и 761 866 файлов:

| Запуск | Время | Отправлено новых фрагментов |
|---|---|---|
| Первый запуск Metadata | 4 ч 17 мин | 135 549 |
| Следующие запуски | примерно от 10 до 15 мин | от 4 до 40 |

Трафик к удалённому PBS почти равен объёму реальных изменений. Настройка описана ниже, в разделе об обнаружении изменений.

## 🤝 Совместимость с официальным клиентом Proxmox

Проверено с `proxmox-backup-client` 3.4.9 и PBS 4.2: снимки Legacy, Data и Metadata **восстанавливаются байт в байт в обе стороны** (из GUI и командной строки, в Windows и Linux), файловый браузер PBS 4.2 их открывает. Тот же формат pxar версии 2, те же режимы обнаружения изменений, тот же формат ключа шифрования (`proxmox-backup-client key create`), те же переменные `PBS_REPOSITORY`. Единственное дополнение: файл `.proxmox_backup_client_meta.json` (исходный путь) в корне снимка папки, который официальный клиент восстанавливает как обычный файл.

## 🔧 Об этом форке

Это форк **mjb-is**: **https://github.com/mjb-is/proxmoxbackupclient_go**. Цель: сделать PBS единственной системой резервного копирования в домашней лаборатории, с надёжным GUI и полностью автоматическим восстановлением на «голое железо». Общеполезные изменения уходят в исходный проект; принято семнадцать pull request (список в [английском README](README.md#-about-this-fork)).

## 📦 Загрузка и установка

👉 **[Скачать последний выпуск](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| У вас | Что использовать |
|---|---|
| Windows | **MSI** (`ProxmoxBackupClient-<version>-x64.msi`): программа и фоновая служба в Program Files, пункт в меню «Пуск». Или `ProxmoxBackupClient-v<version>-windows-amd64.zip` (программа и `ProxmoxBackupClientSVC.exe` в одной папке) |
| Debian, Ubuntu, Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| Другой Linux | Распакуйте `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` (нужна среда WebKitGTK) |
| Только командная строка | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` или `-linux-amd64.tar.gz` |

MSI, `.deb` и `.rpm` пока только в master (выпуски до v0.8.1 содержат только zip и tar.gz); соберите их по [BUILD.md](BUILD.md). В Linux пункт меню **Proxmox Backup Client** запускает программу от вашего имени, а **Proxmox Backup Client (administrator)** от root через polkit, для копий машин. Служба устанавливается, но не включается: см. [packaging/debian/README.md](packaging/debian/README.md).

> ⚠️ **Windows сообщает о вирусе (например, `Trojan:Win32/Sabsik.FL.A!ml`) или SmartScreen предупреждает?** Это **известное ложное срабатывание** для неподписанных приложений Go/Wails (суффикс `!ml`: модель машинного обучения). [Подробнее](https://github.com/tizbac/proxmoxbackupclient_go).

### 🔎 Проверка загрузки

В каждом выпуске есть `SHA256SUMS.txt`:

```powershell
Get-FileHash .\ProxmoxBackupClient-v0.8.1-windows-amd64.zip -Algorithm SHA256   # compare with SHA256SUMS.txt
```

В Linux: `sha256sum -c SHA256SUMS.txt --ignore-missing`. Подписи кода, аттестации CI и проверки VirusTotal пока нет (см. [TODO.md](TODO.md)).

## ✨ Возможности

### GUI (рекомендуется)
- **18 языков интерфейса** (французский, английский, итальянский, немецкий, польский, испанский, болгарский, чешский, греческий, венгерский, латышский, литовский, нидерландский, португальский, румынский, словацкий, турецкий, украинский)
- Меню: Backup, Restore, Undelete, Roll back, Reports, Message Log, Preferences, About
- **Backup Sets**: расписание (ежедневно / с интервалом / вручную), значки типа и VSS / Snapshot, **Run Now**, предупреждение **SHARED ID**; разовые копии папок и дисков
- **Инкрементные копии папок** (Metadata) с полным чтением каждые N запусков
- **Копии машин** в Windows и Linux без свободного места и устойчивые к сбойным секторам, как `host` или как снимок `vm` для Proxmox VE
- **Restore**: просмотр снимков, поиск (шаблоны, регулярные выражения), выборочное восстановление с ACL NTFS/POSIX и временем папок, необязательная проверка; файлы из образов дисков
- **Undelete**, **Roll back**, **Pause, Resume и Stop**
- **Run as Service** (Preferences > Advanced): расписания без вошедшего пользователя (Windows с v0.7.0; Linux в master)
- Карточка хода выполнения, Reports, Message Log, пять цветовых тем, уведомления по e-mail, действия после копии, экспорт и импорт настроек
- Несколько серверов PBS, закрепление отпечатка сертификата, вход по токену API или паролю
- **Шифрование на стороне клиента** (AES-256-GCM, подписанный манифест)

### Утилиты командной строки
- `proxmoxbackup-directory`: копии папок с `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`: копии целых дисков (fidx, инкрементно; `-readfreespace` читает и свободное место)
- `proxmoxbackup-nbd`: сервер NBD для восстановления (Linux)
- `ProxmoxBackupClientSVC` (Windows) и `proxmoxbackupclient` (Linux): `-list-sets`, `-undelete`, `-rollback` (в master)

Первые три принимают `-keyfile` и `-keyfile-passphrase`. Справочник: [руководство по командной строке](docs/manual/CLI.md).

### 📸 Снимки экрана

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Backup Sets**: задания со значками расписания и типа, Run Now на каждом

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Ход копирования**: скорость, оставшееся время, новые и повторные фрагменты

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Restore**: сервер PBS и Backup ID, список снимков или поиск по имени файла

</td>
</tr>
</table>

Остальные снимки экрана: в [английском README](README.md#-screenshots).

### Системные исключения и безопасность
Копии папок сами пропускают `System Volume Information`, `$RECYCLE.BIN`, `Recovery` (в корне диска), `.pbs-rollback`, `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` и `DumpStack.log.tmp`. Для файлов используйте копию **папок**, для восстановления на «голое железо» отдельный Backup Set с копией **машины**.

Безопасность: шифрование фрагментов, закрытая папка настроек (в master), локальный API службы только на 127.0.0.1 с токеном, проверка ввода, защита от обхода путей, повторные попытки, модульные и сквозные тесты.

## 🔐 Шифрование на стороне клиента

Снимки шифруются до отправки файлом ключа Proxmox Backup Server (формат `proxmox-backup-client key create`): AES-256-GCM, подписанный манифест. Сервер PBS ключа не видит.

**GUI:** **Preferences > Account Information**, измените сервер, вкладка **Encryption**: выберите файл ключа или **Generate...**. Для ключа с парольной фразой: **Ask me** (фраза в памяти на сеанс; копии по расписанию и через службу без неё сразу завершаются ошибкой) или **Remember on this computer** (фраза в `config.json`; нужна для заданий без присмотра и службы). **Без файла ключа снимок не восстановить: храните копию ключа вне этой машины.**

```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```

ISO Clonezilla запрашивает ключ и фразу при восстановлении; ключ в ISO не записывается. `config.json` хранит токены и фразы открытым текстом (до v0.8.2 в Windows его могли читать локальные пользователи).

## Ускорение копий папок: обнаружение изменений

| Режим | Что читается | Как хранится |
|---|---|---|
| **Legacy** (по умолчанию) | все файлы | `<name>.pxar.didx` |
| **Data** | все файлы | разделённый архив `<name>.mpxar.didx` + `<name>.ppxar.didx` |
| **Metadata** | только новые и изменённые | тот же разделённый архив |

**GUI:** Backup Set, вкладка **Destination**, **Change detection**. Задайте **Read every file every** N запусков, чтобы ловить редкие изменения без смены размера и времени (например 42 при запуске каждые 4 часа). **Дайте каждому набору свой Backup ID**, иначе наборы заставляют друг друга читать всё. Только для наборов папок; разовые копии используют Legacy. **CLI:** `-change-detection-mode legacy|data|metadata`.

Перед переходом на Data или Metadata обновите до v0.6.0 или новее все машины, где вы восстанавливаете. Первая разделённая копия занимает примерно ещё одну полную копию, пока старые снимки не удалены.

## 🚀 Быстрый старт (GUI)

1. Установите MSI, `.deb` или `.rpm`, либо распакуйте zip или tar.gz из [выпусков](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
2. В Windows используйте **Запуск от имени администратора** для VSS и копий машин; в Linux для копий машин используйте **Proxmox Backup Client (administrator)**
3. Добавьте сервер в **Preferences > Account Information** и нажмите **Test**
4. Создайте Backup Set (или разовую копию) и выберите папки или диски
5. Запустите. Для расписаний без вошедшего пользователя включите **Preferences > Advanced > Run as Service**

## Полная копия работающей машины

Вместо `-backupdir` указывается `-backupdev` (можно повторять):

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

Команда делает снимок VSS всех разделов диска 0 и создаёт загрузочный образ fidx. Свободное место записывается нулями без чтения; следующие копии инкрементные, до 1 ГБ/с.

### Требования для копий машин Linux (аналог VSS)

Нужен модуль ядра [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap) (или уже загруженный `dattobd`). Установка из пакетного репозитория для Ubuntu 22.04 сейчас не работает; соберите из исходников:

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

Копия машины должна выполняться **от root** (пункт **Proxmox Backup Client (administrator)**, служба или `sudo`); группы `disk` недостаточно.

### Восстановление файлов из копии диска

Страница Restore открывает копию машины как папки (NTFS с v0.8.0; FAT, exFAT, ext2/3/4 и XFS в master). В Linux копию можно также подключить через NBD после `modprobe nbd max_part=0`:

`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

Без `-path` открывается выбор файла. Не делайте этого на важной машине; размонтируйте всё перед остановкой; при `Device or resource busy` выполните `nbd-client -d /dev/nbd0`.

### Восстановление на физическую машину

ISO Clonezilla Live из этого форка содержит автоматический пункт **«PBS Bare Metal Restore»**: данные PBS, снимок, целевой диск, и дальше всё само до перезагрузки. Полуручной пункт «pbs-nbd» тоже есть. См. [руководство](docs/manual/BMR.md).

## Использование: копирование папок

Полный справочник: **[руководство по командной строке](docs/manual/CLI.md)**.

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

Читаются и переменные окружения официального клиента (`PBS_REPOSITORY`, `PBS_SERVER`, `PBS_PORT`, `PBS_DATASTORE`, `PBS_AUTH_ID`, `PBS_PASSWORD`, `PBS_FINGERPRINT`). Письма используют [шаблоны Go](https://pkg.go.dev/text/template). В Windows вторая копия во время первой не запускается; в Linux блокировки нет. Windows Defender замедляет копирование до 25%.

## 🔨 Сборка из исходников

Всё описано в **[BUILD.md](BUILD.md)** (Go 1.25+, Wails CLI v2.13):

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

## 🔧 Расширенное использование и руководства

- **Несколько серверов PBS:** [руководство пользователя](MULTI_PBS_USER_GUIDE.md) и [руководство по реализации](MULTI_PBS_GUIDE.md).
- **ISO Clonezilla Live:** сборка описана в **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**:

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)**: что готово и с какой версии. **[CHANGELOG.md](CHANGELOG.md)**: история изменений. **[TODO.md](TODO.md)**: планы. **[RELEASE_NOTES.md](RELEASE_NOTES.md)**, **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**, **[AGENTS.md](AGENTS.md)**.

## 🌐 Техническая поддержка в Италии

Проект и **ВСЕ** вклады навсегда остаются открытыми под GPLv3. Главный спонсор, E.T.I. Srl ( https://etitech.net ), предлагает поддержку по Proxmox и инструментам копирования Windows. Отдельных редакций «community» и «enterprise» не будет; любые доработки выпускаются под GPLv3.

## 🖥️ Происхождение GUI

**Proxmox Backup Client GUI** основан на **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)** от **[RDEM Systems](https://www.rdem-systems.com/)**, который влит обратно в этот репозиторий под GPLv3. RDEM Systems спонсирует разработку GUI и предоставляет коммерческую поддержку.

## ⚠️ Предупреждение

Программа предоставляется «как есть», находится в стадии альфа, и мы не несём ответственности за потерю или повреждение данных. Всегда проверяйте копии и восстановление.

## 📄 Лицензия

GPLv3: см. [LICENSE](LICENSE).

## 🏷️ Брендинг

Участник с **не менее чем 5 коммитами** функций или исправлений может добавить свой бренд для коммерческого использования, если компания **не** связана с вредоносным ПО, продвижением войны, мошенничеством, кражей данных, торговлей детьми, насилием, дискриминацией, наркотиками или другой незаконной деятельностью. **Лицензия GPLv3 остаётся в силе.**

## 🤝 Участие

Помощь приветствуется: проверка переводов строк шифрования и хранение ключей вне файла (например, Windows DPAPI), P2V (снимок `vm` уже работает, не хватает сетевой карты, TPM, драйверов virtio), асинхронная отправка фрагментов, дескрипторы безопасности Windows в pxar, символические ссылки Windows и любые интересные идеи :)

## Об участниках Proxmox Backup Client GO

Участники Proxmox Backup Client GO разрабатывают и поддерживают проект. Программа опирается на инфраструктуру NTP/NTS и [11 публичных серверов NTS](https://github.com/jauderho/nts-servers).

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**
