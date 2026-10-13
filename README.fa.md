# Proxmox Backup Client: کلاینت ویندوز و لینوکس برای Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Proxmox Backup Client یک کلاینت پشتیبان‌گیری متن‌باز (GPL-3.0) برای Proxmox Backup Server (PBS) است که روی ویندوز و لینوکس کار می‌کند.**

این یک **مجموعه ابزار** برای پشتیبان‌گیری روی PBS است:

- **Proxmox Backup Client GUI** (بر پایه Nimbus Backup GUI از RDEM Systems): کلاینت گرافیکی برای پشتیبان‌گیری از سرورها و ایستگاه‌های کاری ویندوز و لینوکس روی PBS، با اسنپ‌شات‌های سازگار (VSS در ویندوز، elastio-snap/dattobd در لینوکس)، Backup Sets زمان‌بندی‌شده، پشتیبان‌گیری از پوشه‌ها و کل دیسک، بازیابی، Undelete و Roll back، چند سرور PBS و یک سرویس پس‌زمینه در ویندوز و لینوکس.
- **`proxmoxbackup-directory`**: ابزار خط فرمان برای پشتیبان‌گیری از پوشه‌ها (pxar) با حذف داده‌های تکراری و تشخیص تغییرات.
- **`proxmoxbackup-machine`**: ابزار خط فرمان برای پشتیبان‌گیری زنده از کل دیسک (fidx) در ماشین‌های ویندوز (VSS) و لینوکس (elastio-snap/dattobd)؛ ایمیج پس از بازیابی bare-metal قابل بوت است.
- **`proxmoxbackup-nbd`**: یک پشتیبان دیسک را برای بازیابی به‌صورت دستگاه NBD وصل می‌کند (لینوکس).
- یک **ISO بازیابی bare-metal** ساخته‌شده بر پایه Clonezilla Live.

> ⚠️ **سلب مسئولیت:** این پروژه **به هیچ وجه** با **Proxmox Server Solutions GmbH** وابستگی ندارد. «Proxmox»، لوگوی Proxmox و نام‌های مرتبط متعلق به صاحبان آن‌هاست و در اینجا **فقط** برای بیان سازگاری به کار رفته‌اند. برای محصولات آن‌ها به [proxmox.com](https://www.proxmox.com/) مراجعه کنید.

> ℹ️ **درباره این صفحه:** این صفحه خلاصه‌ای ترجمه‌شده از README انگلیسی است. **[README.md انگلیسی](README.md) مرجع اصلی است** و همه جزئیات را دارد. راهنماهای کاربر فقط به انگلیسی هستند (جدول زیر). رابط برنامه فعلاً فارسی ندارد، بنابراین نام منوها، زبانه‌ها و دکمه‌ها همان‌طور که در برنامه به انگلیسی نوشته شده‌اند آمده‌اند.

## 📖 راهنماهای کاربر

| راهنما | موضوع |
|---|---|
| **[راهنمای GUI](docs/manual/GUI.md)** | نصب، سرورهای PBS، Backup Sets و زمان‌بندی، پشتیبان‌گیری افزایشی، پیشرفت کار، بازیابی، Undelete، Roll back، Reports، لاگ‌ها، Preferences، سرویس پس‌زمینه، عیب‌یابی |
| **[راهنمای خط فرمان](docs/manual/CLI.md)** | `proxmoxbackup-directory`، `proxmoxbackup-machine`، `proxmoxbackup-nbd`: همه پرچم‌ها، فایل‌های پیکربندی، کدهای خروج، زمان‌بندی؛ Undelete و Roll back از خط فرمان |
| **[راهنمای بازیابی bare-metal](docs/manual/BMR.md)** | ISO بازیابی گام به گام؛ اسنپ‌شات‌های `vm` روی Proxmox VE؛ سخت‌افزار متفاوت |
| [سناریوهای بازیابی](docs/RESTORE_GUIDE.md) | کدام بازیابی در چه موقعیتی |
| [فهرست راهنماها](docs/manual/README.md) | نمای کلی سه راهنما |

## 🆕 تازه در v0.9.0: چند زمان‌بندی برای هر Backup Set، نصب‌کننده‌ها، سرویس لینوکس

- **چند زمان‌بندی برای هر Backup Set**، از هر چند دقیقه تا سالی یک بار: «هر روز ساعت 02:00» به‌علاوه «آخرین روز هر ماه» یا «دومین سه‌شنبه»، هر کدام با توضیح ساده و 3 اجرای بعدی. Backup Sets موجود زمان‌بندی خود را نگه می‌دارند.
- **پشتیبان پوشه‌ها alternate data stream ها و ویژگی‌های توسعه‌یافته NTFS را حفظ می‌کند** (ویندوز)، مثل نشان «دانلودشده از اینترنت»، و بازیابی آن‌ها را برمی‌گرداند.
- **نصب‌کننده‌ها**: **MSI ویندوز** (Program Files، منوی Start، ارتقای درجا، نگه داشتن یا حذف تنظیمات هنگام حذف) و **بسته‌های لینوکس** `.deb` (Debian، Ubuntu، Mint) و `.rpm` (Fedora) با برنامه، سه ابزار خط فرمان، راهنماها و ورودی‌های منو.
- **سرویس پس‌زمینه در لینوکس** (`proxmoxbackupclient.service`): Backup Sets زمان‌بندی‌شده با root و بدون ورود کاربر اجرا می‌شوند؛ از **Preferences > Advanced > Run as Service** فعال می‌شود. **Restart as administrator** هم در لینوکس کار می‌کند (pkexec).
- **Pause، Resume و Stop** برای پشتیبان‌هایی که سرویس اجرا می‌کند.
- **هشدار SHARED ID** وقتی دو Backup Set یک گروه پشتیبان مشترک دارند (سرور، نوع و Backup ID یکسان).
- **قفل تنظیمات**: `C:\ProgramData\ProxmoxBackupClient` فقط برای مدیران ویندوز؛ در لینوکس فقط برای مالک.
- **بازگرداندن فایل از پشتیبان ماشین** برای FAT، exFAT، ext2/3/4 و XFS علاوه بر NTFS؛ بازیابی ایمیج ویندوز مجوزها و alternate data stream ها را هم برمی‌گرداند.
- **پشتیبان ماشین لینوکس از فضای خالی می‌گذرد** (ext2/3/4، XFS)، مثل ویندوز از v0.8.0.
- **Undelete و Roll back برای Backup Sets ماشین لینوکس**، و **از خط فرمان** برای هر Backup Set: `-list-sets`، `-undelete`، `-rollback` (بخش 12 [راهنمای خط فرمان](docs/manual/CLI.md)).
- خطاهای ابزارهای خط فرمان در stderr چاپ می‌شوند؛ `proxmoxbackup-machine` ایمیل اعلان می‌فرستد.
- **ISO بازیابی bare-metal** هرگز فلشی را که از آن بوت شده به‌عنوان دیسک مقصد پیشنهاد نمی‌کند، و راهنما توضیح می‌دهد وقتی ماشین فیزیکی به VM تبدیل می‌شود firmware (UEFI یا BIOS) چگونه باید جور شود.
- **همه زبان‌ها بازبینی شدند** (17 ترجمه، حدود 1,850 رشته)، و تاریخ‌ها و اعداد از زبان برنامه پیروی می‌کنند.
- بهبود سرعت v0.8.2 برای فضای خالی (tag شده، هرگز جداگانه منتشر نشد) هم در این نسخه هست.

فهرست کامل را در [CHANGELOG](CHANGELOG.md) ببینید.

## 💽 تازه در v0.8.0: پشتیبان ماشین سریع‌تر، بازگرداندن فایل از آن، Pause/Resume

- در ویندوز **پشتیبان ماشین از فضای خالی** (و فایل‌های paging و hibernation) **می‌گذرد**، پس دیسک نیمه‌خالی در حدود نصف زمان خوانده می‌شود؛ یک **سکتور خراب** هم دیگر کار را متوقف نمی‌کند و به‌صورت صفر در ایمیج می‌رود.
- **بازیابی فایل از پشتیبان ماشین**: هر حجم NTFS در ایمیج پوشه‌ای قابل مرور و جستجوست، بدون mount یا دانلود (درایو C: یک لپ‌تاپ با 127 گیگابایت و 430,000 فایل در 4 تا 15 ثانیه فهرست می‌شود).
- **Undelete و Roll back** برای Backup Sets ماشین (ویندوز)، روی پوشه‌های انتخاب‌شده مجموعه (پیش‌فرض: Desktop، Documents، Pictures، Music، Videos، Downloads).
- **Pause و Resume**، نشان **VSS / Snapshot ON-OFF** روی هر کارت، و حفظ **زمان ایجاد** فایل‌ها در ویندوز.
- **v0.8.2:** فضای خالی دیگر بلوک به بلوک hash نمی‌شود؛ دیسک عمدتاً خالی در حدود یک‌سوم زمان پردازش می‌شود (دیسک آزمایشی 50 گیگابایتی: 3m36s به 1m15s).

[CHANGELOG](CHANGELOG.md) را ببینید.

## 🗑️ تازه در v0.7.0: Undelete و Roll back

دو صفحه جدید در GUI، برای Backup Sets پوشه:

- **Undelete** فایل‌هایی را که در پشتیبان‌ها هستند ولی دیگر روی دیسک نیستند (در جدیدترین پشتیبان یا پشتیبان‌های 7 یا 30 روز گذشته) به‌صورت درخت پوشه با زمان ناپدید شدن نشان می‌دهد و فایل‌های علامت‌خورده را به جای قبلی یا پوشه‌ای دیگر برمی‌گرداند. پس از حذف 39 گیگابایت از یک اشتراک 761,000 فایلی، جستجوی 7 روزه همان فایل‌ها را در حدود یک دقیقه پیدا کرد.
- **Roll back** پوشه‌های مجموعه را به وضعیتشان در یک پشتیبان انتخاب‌شده برمی‌گرداند، با پیش‌نمایش تغییرات. هر فایل جایگزین‌شده یا حذف‌شده ابتدا روی همان درایو کنار گذاشته می‌شود، پس **Undo** همه چیز را برمی‌گرداند.

همچنین تازه: سرویس پس‌زمینه در بسته دانلود ویندوز (Preferences > Advanced > Run as Service). [راهنمای GUI](docs/manual/GUI.md#10-undelete) را ببینید.

## ⚡ تازه در v0.6.0: پشتیبان‌گیری افزایشی از پوشه‌ها

پشتیبان‌گیری از پوشه‌ها اکنون با همان **تشخیص تغییرات** کلاینت رسمی `proxmox-backup-client` از فایل‌های تغییرنکرده می‌گذرد. در حالت **Metadata** اندازه، زمان تغییر (نانوثانیه)، mode و مالک هر فایل با اسنپ‌شات قبلی مقایسه می‌شود؛ فایل‌های تغییرنکرده باز نمی‌شوند و فقط فایل‌های جدید و تغییرکرده خوانده و آپلود می‌شوند.

اندازه‌گیری روی یک فایل‌سرور واقعی (480.9 گیگابایت، 761,866 فایل روی دیسک‌های USB):

| اجرا | زمان | خوانده‌شده | chunk های جدید آپلودشده |
|---|---|---|---|
| اولین اجرای Metadata (چیزی برای مقایسه نیست) | 4 ساعت و 17 دقیقه | همه فایل‌ها | 135,549 |
| اجراهای بعدی | حدود 10 تا 15 دقیقه (اگر اجرای قبلی چند دقیقه پیش بوده، سریع‌تر) | فقط فایل‌های تغییرکرده؛ همه 761,866 فایل تغییرنکرده دوباره استفاده شدند | 4 تا 40 |

نحوه استفاده: در **Backup Set** پوشه، زبانه **Destination**، گزینه **Change detection: Metadata** را انتخاب کنید و **Read every file every N runs** را تنظیم کنید (مثلاً 42 برای مجموعه‌ای که هر 4 ساعت اجرا می‌شود). به هر مجموعه **Backup ID جداگانه** بدهید. خط فرمان: `-change-detection-mode metadata`. پیش‌فرض همچنان Legacy است؛ جزئیات در [راهنمای GUI](docs/manual/GUI.md#7-incremental-folder-backups-change-detection).

## 🤝 سازگار با کلاینت رسمی Proxmox، در هر دو جهت

با `proxmox-backup-client` 3.4.9 و PBS 4.2 بررسی شده است:

- **اسنپ‌شات‌های Legacy، Data و Metadata این کلاینت با کلاینت رسمی بایت به بایت یکسان بازیابی می‌شوند** (از GUI یا خط فرمان، ساخته‌شده روی ویندوز یا لینوکس) و مرورگر فایل PBS 4.2 آن‌ها را باز می‌کند.
- **اسنپ‌شات‌های کلاینت رسمی هم با این کلاینت بایت به بایت یکسان بازیابی می‌شوند**، کامل یا انتخابی.
- همان قالب آرشیو تقسیم‌شده (pxar version 2)، همان حالت‌های تشخیص تغییرات، همان قالب فایل کلید (کلیدها در هر دو جهت کار می‌کنند) و همان متغیرهای محیطی به سبک `PBS_REPOSITORY`.

تنها چیز اضافه فایل کوچک `.proxmox_backup_client_meta.json` (مسیر اصلی) در ریشه اسنپ‌شات پوشه است که کلاینت رسمی آن را مثل یک فایل عادی بازیابی می‌کند.

## 🔧 درباره این فورک

این **فورک mjb-is** از پروژه اصلی است: **https://github.com/mjb-is/proxmoxbackupclient_go**. هدف: PBS تنها سیستم پشتیبان‌گیری سرورمحور homelab شود، کلاینت GUI مقاوم‌تر شود و یک مسیر کاملاً خودکار بازیابی bare-metal بر پایه Clonezilla ساخته شود. تاکنون 17 PR از این فورک در پروژه اصلی ادغام شده و یکی ([#105](https://github.com/tizbac/proxmoxbackupclient_go/pull/105)) باز است. فهرست بهبودها و PR ها در [README انگلیسی](README.md#-about-this-fork) آمده است.

## 📦 دانلود و نصب

👉 **[دانلود آخرین نسخه](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| سیستم شما | استفاده کنید از |
|---|---|
| ویندوز | **نصب‌کننده MSI** (`ProxmoxBackupClient-<version>-x64.msi`) برنامه و سرویس پس‌زمینه را در Program Files نصب می‌کند و میانبر منوی Start می‌سازد. یا `ProxmoxBackupClient-v<version>-windows-amd64.zip` را باز کنید (برنامه و `ProxmoxBackupClientSVC.exe`؛ هر دو را در یک پوشه نگه دارید) |
| Debian، Ubuntu، Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| لینوکس دیگر | `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` را باز کنید (به runtime ابزار WebKitGTK نیاز دارد) |
| فقط خط فرمان | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` یا `-linux-amd64.tar.gz` |

MSI، `.deb` و `.rpm` روی master تازه هستند: نسخه‌ها تا v0.8.1 فقط فایل‌های zip و tar.gz دارند. تا وقتی نسخه‌ای همراه نصب‌کننده‌ها منتشر شود، آن‌ها را طبق [BUILD.md](BUILD.md) بسازید.

بسته‌های لینوکس دو ورودی منو دارند: **Proxmox Backup Client** با کاربر خودتان (پشتیبان پوشه‌های خودتان) و **Proxmox Backup Client (administrator)** با root از طریق polkit (پشتیبان ماشین و اسنپ‌شات بلوکی). سرویس پس‌زمینه نصب ولی فعال نمی‌شود؛ [packaging/debian/README.md](packaging/debian/README.md).

> ⚠️ **ویندوز پیام «ویروس شناسایی شد» (مثلاً `Trojan:Win32/Sabsik.FL.A!ml`) یا هشدار SmartScreen نشان می‌دهد؟**
> این یک **مثبت کاذب شناخته‌شده** برای برنامه‌های Go/Wails است، نه ویروس: پسوند `!ml` یعنی تشخیص مدل یادگیری ماشین برای فایل‌های اجرایی *بدون امضا و کم‌رایج*. [توضیح بیشتر در پروژه اصلی](https://github.com/tizbac/proxmoxbackupclient_go).

### 🔎 بررسی دانلود

هر نسخه از این فورک یک `SHA256SUMS.txt` دارد:

```powershell
Get-FileHash .\ProxmoxBackupClient-v0.8.1-windows-amd64.zip -Algorithm SHA256   # compare with SHA256SUMS.txt
```

در لینوکس: `sha256sum -c SHA256SUMS.txt --ignore-missing`.

> ℹ️ ساخت‌های این فورک **هنوز امضای کد، گواهی CI یا اسکن VirusTotal ندارند** و دستی ساخته می‌شوند. برای کمک [TODO.md](TODO.md) را ببینید.

## ✨ امکانات

### GUI: Proxmox Backup Client GUI (پیشنهادی)
- **🌍 چندزبانه**: 18 زبان رابط (فرانسوی، انگلیسی، ایتالیایی، آلمانی، لهستانی، اسپانیایی، بلغاری، چکی، یونانی، مجاری، لتونیایی، لیتوانیایی، هلندی، پرتغالی، رومانیایی، اسلواکی، ترکی و اوکراینی)
- نوار منو و منوی کناری (Backup، Restore، Undelete، Roll back، Reports، Message Log، Preferences، About)
- **Backup Sets**: کارهای نام‌دار با زمان‌بندی روزانه / بازه‌ای / دستی، نشان نوع و نشان VSS / Snapshot، «Run Now»، و هشدار **SHARED ID** برای گروه پشتیبان مشترک
- **پشتیبان‌گیری یک‌باره** از پوشه‌ها یا کل دیسک؛ صفحه **Reports** و **Message Log**؛ پنجره **Preferences** با پنج تم رنگی و رنگ دلخواه
- **پشتیبان‌گیری افزایشی از پوشه‌ها** (حالت Metadata) با خواندن کامل اختیاری هر N اجرا
- **پشتیبان ماشین (کل دیسک)** در ویندوز و لینوکس، با گذشتن از فضای خالی و تحمل سکتور خراب، به‌صورت `host` یا اسنپ‌شات `vm` در Proxmox VE
- **بازیابی**: مرور اسنپ‌شات، جستجو (wildcard، عبارت منظم، مسیر کامل)، بازیابی انتخابی با ACL های NTFS/POSIX، زمان پوشه‌ها، زمان ایجاد در ویندوز و بررسی اختیاری
- **بازگرداندن فایل از پشتیبان ماشین** (NTFS؛ روی master همچنین FAT، exFAT، ext2/3/4 و XFS)
- **Undelete** و **Roll back** (از v0.7.0) برای مجموعه‌های پوشه و ماشین، با پیش‌نمایش و Undo؛ [راهنمای GUI](docs/manual/GUI.md#10-undelete)
- **Pause، Resume و Stop**؛ **Run as Service** در Preferences > Advanced (ویندوز از v0.7.0؛ لینوکس روی master)
- کارت پیشرفت زنده با زمان پیش‌بینی پایان، سرعت پردازش و آپلود و فایل جاری
- VSS در ویندوز، اسنپ‌شات بلوکی elastio-snap/dattobd در لینوکس
- اعلان ایمیلی، اقدامات پس از پشتیبان‌گیری، clone کردن Backup Set، خروجی و ورودی تنظیمات
- چند سرور PBS با سنجاق کردن اثر انگشت گواهی، ورود با توکن API یا نام کاربری/رمز عبور
- **رمزنگاری سمت کلاینت** (AES-256-GCM، manifest امضاشده، عبارت عبور اختیاری)
- محافظت مدیر برای تغییر Backup Sets (ویندوز) با Restart as administrator؛ لاگ اشکال‌زدایی

### ابزارهای خط فرمان
- `proxmoxbackup-directory`: پشتیبان‌گیری از پوشه‌ها (pxar) با حذف داده‌های تکراری و `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`: پشتیبان‌گیری زنده از کل دیسک در ماشین‌های ویندوز (VSS) و لینوکس (elastio-snap/dattobd) (fidx، افزایشی، گذشتن از فضای خالی مگر با `-readfreespace`)
- `proxmoxbackup-nbd`: سرور NBD برای بازیابی پشتیبان‌های دیسک (لینوکس)
- `ProxmoxBackupClientSVC` (ویندوز) و `proxmoxbackupclient` (بسته‌های لینوکس): `-list-sets`، `-undelete` و `-rollback` برای Backup Sets تعریف‌شده در GUI (روی master)

سه ابزار اول `-keyfile` و `-keyfile-passphrase` را برای اسنپ‌شات‌های رمزنگاری‌شده می‌پذیرند. مرجع کامل: [راهنمای خط فرمان](docs/manual/CLI.md).

### 📸 تصاویر

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Backup Sets**: کارهای نام‌دار با نشان راه‌انداز تکراری/دستی و نشان نوع پوشه/ماشین، با Run Now روی هر کدام

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**پیشرفت زنده پشتیبان‌گیری**: توان عملیاتی، زمان باقی‌مانده و تعداد chunk های جدید در برابر استفاده‌شده دوباره

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**بازیابی**: یک سرور PBS و Backup ID انتخاب کنید، اسنپ‌شات‌ها را فهرست کنید یا در میان آن‌ها با نام فایل جستجو کنید

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/backup-sets-vm-badge.png" width="280">](docs/screenshots/backup-sets-vm-badge.png)
**Backup Sets به‌عنوان VM در Proxmox VE**: مجموعه‌ای که به‌صورت اسنپ‌شات `vm` ذخیره می‌شود نشان `MACHINE (VM)` دارد

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore-progress.png" width="280">](docs/screenshots/gui-restore-progress.png)
**بازیابی انتخابی**: فایل‌ها/پوشه‌ها را از درخت اسنپ‌شات علامت بزنید و در جای اصلی یا جای دیگر بازیابی کنید

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Reports**: تاریخچه کامل اجراها با پنل جزئیات (مدت، chunk ها، پوشه‌های پشتیبان‌گیری‌شده)

</td>
</tr>
</table>

تصاویر بیشتر (چند PBS، ویرایش سرور، Message Log، تم‌ها، زبان‌ها) در [README انگلیسی](README.md#-screenshots).

### استثناهای هوشمند سیستمی (پشتیبان‌گیری از پوشه)
پشتیبان‌گیری از پوشه این موارد را خودبه‌خود کنار می‌گذارد:

**پوشه‌های سیستمی:** `System Volume Information` (فضای ذخیره VSS، می‌تواند به بیش از 100 گیگابایت برسد) و `$RECYCLE.BIN` در هر جا که باشند؛ `Recovery` فقط در ریشه درایو (روی master؛ نسخه‌های قبلی پوشه‌ای با این نام را در هر عمقی رد می‌کردند)؛ `.pbs-rollback` (پوشه ایمنی Roll back).
**فایل‌های سیستمی:** `pagefile.sys`، `hiberfil.sys`، `swapfile.sys` و `DumpStack.log.tmp`.

**چرا مهم است:** درایوی با 1.03 ترابایت فضای استفاده‌شده ممکن است فقط حدود 141 گیگابایت فایل واقعی داشته باشد؛ بقیه اسنپ‌شات‌های VSS است.

**توصیه:** برای سطح فایل پشتیبان **پوشه**، و برای بازیابی bare-metal پشتیبان **ماشین** در یک Backup Set جداگانه.

### امنیت و کیفیت
- رمزنگاری اختیاری سمت کلاینت برای هر chunk، با manifest امضاشده
- پوشه تنظیمات در ویندوز فقط برای مدیران و در لینوکس فقط برای مالک آن باز است (روی master)
- API سرویس محلی فقط روی 127.0.0.1 گوش می‌دهد و به توکنی نیاز دارد که با هر بار شروع سرویس تازه می‌شود
- اعتبارسنجی ورودی، جلوگیری از path traversal و تلاش مجدد با backoff نمایی
- مدیریت خطا و آزمون‌های واحد در همه ماژول‌ها، به‌علاوه آزمون‌های زنده اختیاری و آزمون‌های end-to-end روی یک PBS واقعی

## 🔐 رمزنگاری سمت کلاینت

اسنپ‌شات‌ها پیش از خروج از ماشین با فایل کلید Proxmox Backup Server رمزنگاری می‌شوند (همان قالب `proxmox-backup-client key create`، قابل تبادل با کلاینت رسمی): AES-256-GCM برای هر chunk و manifest امضاشده. سرور PBS هرگز کلید را نمی‌بیند.

**GUI:** در **Preferences > Account Information** یک سرور را ویرایش کنید، زبانه **Encryption**: فایل کلید موجود را انتخاب کنید یا با **Generate...** بسازید. **بدون فایل کلید هیچ اسنپ‌شاتی بازیابی نمی‌شود: یک نسخه را جای امنی دور از این ماشین نگه دارید.**

**عبارت عبور:** **Ask me** آن را در هر نشست یک بار می‌پرسد و فقط در حافظه نگه می‌دارد (پشتیبان‌های زمان‌بندی‌شده و سرویس فوراً شکست می‌خورند)؛ **Remember on this computer** آن را در پیکربندی ذخیره می‌کند و برای کارهای بدون حضور کاربر و سرویس مناسب است.

**خط فرمان:**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```

**بازیابی bare-metal:** ISO کلونزیلا کلید و عبارت عبور را می‌پرسد و کلید را روی ISO نمی‌نویسد ([PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)). `config.json` توکن‌ها، رمزها و عبارت عبور به‌خاطرسپرده را به‌صورت متن ساده نگه می‌دارد؛ محدودیت‌ها در [README انگلیسی](README.md#-client-side-encryption).

## پشتیبان‌گیری سریع‌تر از پوشه‌ها: تشخیص تغییرات

در حالت پیش‌فرض Legacy هر پشتیبان‌گیری همه فایل‌ها را می‌خواند. Backup Sets (و خط فرمان) می‌توانند به جای آن از حالت‌های تشخیص تغییرات `proxmox-backup-client` رسمی استفاده کنند:

| حالت | می‌خواند | ذخیره به‌صورت |
|---|---|---|
| **Legacy** (پیش‌فرض) | همه فایل‌ها | یک `<name>.pxar.didx` برای هر پوشه، مثل قبل |
| **Data** | همه فایل‌ها | آرشیو تقسیم‌شده: `<name>.mpxar.didx` (فراداده) + `<name>.ppxar.didx` (محتوای فایل‌ها) |
| **Metadata** | فقط فایل‌های جدید و تغییرکرده | همان آرشیو تقسیم‌شده؛ فایل‌هایی که اندازه، زمان تغییر (نانوثانیه)، mode و مالکشان با پشتیبان قبلی یکی است بدون خواندن از آن برداشته می‌شوند |

اجرای اول Metadata همه چیز را می‌خواند و بعدی‌ها فقط تغییرات را؛ چون تغییری که اندازه و زمان را حفظ کند دیده نمی‌شود، مجموعه می‌تواند **هر N اجرا همه فایل‌ها را بخواند**.

**GUI:** Backup Set، زبانه **Destination**، گزینه **Change detection** (فقط مجموعه‌های پوشه؛ پشتیبان یک‌باره Legacy است). **برای هر مجموعه یک Backup ID.**

**خط فرمان:** `-change-detection-mode legacy|data|metadata`، یا `"change-detection-mode"` در پیکربندی JSON.

**پیش از تغییر یک مجموعه به Data یا Metadata،** همه ماشین‌هایی را که از آن‌ها بازیابی می‌کنید به v0.6.0 یا بالاتر ارتقا دهید: نسخه‌های قدیمی‌تر آرشیوهای تقسیم‌شده را نمی‌خوانند.

## 🚀 شروع سریع (GUI)

1. نصب کنید: MSI در ویندوز، `.deb` یا `.rpm` در لینوکس، یا فایل zip یا tar.gz را از [صفحه نسخه‌ها](https://github.com/mjb-is/proxmoxbackupclient_go/releases) باز کنید
2. اجرا کنید. در ویندوز برای VSS، پشتیبان ماشین و تغییر Backup Sets آن را **با دسترسی مدیر اجرا کنید**. در لینوکس پشتیبان‌گیری از پوشه‌های فایل‌های خودتان با کاربر خودتان کار می‌کند؛ برای پشتیبان ماشین و اسنپ‌شات بلوکی از **Proxmox Backup Client (administrator)** استفاده کنید (یا با root اجرا کنید)
3. سرور PBS خود را در **Preferences > Account Information** اضافه کنید و **Test** را بزنید
4. یک Backup Set (یا یک پشتیبان‌گیری یک‌باره) بسازید و پوشه‌ها یا دیسک‌ها را انتخاب کنید
5. اجرا کنید. برای اجرای زمان‌بندی‌ها بدون ورود کاربر، **Preferences > Advanced > Run as Service** را روشن کنید

[راهنمای GUI](docs/manual/GUI.md) همه مراحل را توضیح می‌دهد.

## پشتیبان‌گیری زنده از کل ماشین

`proxmoxbackup-machine` (و پشتیبان ماشین در GUI) از یک سیستم کامل Windows 10/11، Windows Server یا یک ماشین لینوکس بدون هیچ توقفی پشتیبان می‌گیرد. نحو فرمان تقریباً مثل پشتیبان‌گیری از پوشه است، با این تفاوت که `-backupdev` جای `-backupdir` را می‌گیرد (برای چند دیسک تکرارش کنید). برای نمونه:

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

این فرمان دیسک 0 را بررسی می‌کند، از پارتیشن‌های mount شده اسنپ‌شات VSS می‌گیرد و یک ایمیج قابل بوت از کل دیسک به‌صورت fidx می‌سازد. فضای خالی و فایل‌های paging و hibernation بدون خواندن به‌صورت صفر ذخیره می‌شوند (روی master فضای خالی ext2/3/4 و XFS لینوکس هم)؛ `-readfreespace` برای نسخه forensic همه چیز را می‌خواند. پشتیبان بعدی افزایشی است و hash گیری موازی، سرعت 1 گیگابایت بر ثانیه را ممکن می‌کند.

### پیش‌نیازهای پشتیبان ماشین لینوکس (معادل VSS)

لینوکس VSS ندارد، پس این پروژه از ماژول کرنل `elastio-snap` (فورکی از `dattobd`) برای گرفتن اسنپ‌شات سازگار از دستگاه بلوکی زنده استفاده می‌کند. نسخه نگهداری‌شده [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap) را نصب کنید (مخزن اصلی بایگانی شده است). `dattobd` هم اگر بارگذاری شده باشد پشتیبانی می‌شود.

هر دو مورد زیر به‌طور مستقل لازم‌اند، وگرنه پشتیبان ماشین لینوکس اجرا نمی‌شود:

1. **ماژول کرنل باید نصب و بارگذاری شده باشد.** نصب با بسته مخزن ([INSTALL.md](https://github.com/Axcient/elastio-snap/blob/develop/INSTALL.md)) دست‌کم روی Ubuntu 22.04 خراب است (2026-09-24)؛ ساخت از سورس کار می‌کند:
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
   این کار ماژول را فقط برای بوت فعلی می‌سازد و بارگذاری می‌کند؛ برای بارگذاری خودکار در هر بوت به مستندات توزیع خود مراجعه کنید (مثلاً یک ورودی در `/etc/modules-load.d/`).

2. **فرایند پشتیبان‌گیری باید با root (EUID 0) اجرا شود.** `CreateVSSSnapshot` (مسیر اسنپ‌شات لینوکس، با وجود نامش) با کاربر غیر root اجرا نمی‌شود و عضویت در گروه `disk` جایگزین آن نیست. از ورودی منوی **Proxmox Backup Client (administrator)**، سرویس پس‌زمینه، یا `sudo` استفاده کنید؛ پشتیبان‌گیری ساده از فایل/پوشه چنین شرطی ندارد.

خطای `backup device /dev/sdX: open /dev/sdX: permission denied` معمولاً با افزودن کاربر به گروه `disk` (`sudo usermod -aG disk $USER`، سپس ورود دوباره) برطرف می‌شود، ولی برای اسنپ‌شات واقعی همچنان root لازم است.

### بازیابی فایل از پشتیبان دیسک

صفحه Restore در GUI پشتیبان ماشین را مثل پشتیبان پوشه باز می‌کند: هر حجم در ایمیج پوشه‌ای است که می‌توانید مرور و از آن بازیابی کنید (NTFS از v0.8.0؛ FAT، exFAT، ext2/3/4 و XFS روی master). چیزی mount نمی‌شود.

در لینوکس می‌توانید با ابزار nbd یک پشتیبان دیسک (VM، host یا PVE) را به‌صورت دستگاه بلوکی وصل کنید. ابتدا `modprobe nbd max_part=0` را اجرا کنید (`max_part != 0` باعث حلقه بی‌پایان جستجوی پارتیشن می‌شود). فرمانی شبیه این:

`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

اگر `-path` را حذف کنید، یک رابط متنی برای انتخاب فایل fidx نمایش داده می‌شود.

> این را روی ماشینی که کارهای مهم اجرا می‌کند به کار نبرید (یک فایل‌سیستم خراب ممکن است سیستم‌عامل را از کار بیندازد؛ به همین دلیل Proxmox VE برای این کار از یک نمونه QEMU استفاده می‌کند).

پیش از متوقف کردن pbsnbd همه چیز را روی دیسک nbd از حالت mount خارج کنید. در صورت خطای `Device or resource busy`، با `nbd-client -d /dev/nbd0` اتصال را به‌زور قطع کنید یا سیستم را راه‌اندازی مجدد کنید.

### بازیابی روی ماشین فیزیکی

این فورک یک ISO وصله‌شده Clonezilla Live با ورودی بوت کاملاً خودکار **«PBS Bare Metal Restore»** ارائه می‌کند (بخش «ISO زنده Clonezilla» در پایین): اطلاعات PBS را وارد کنید، اسنپ‌شات را انتخاب و دیسک مقصد را تأیید کنید؛ شبکه، اتصال NBD، بازیابی و راه‌اندازی مجدد خودکار انجام می‌شود.

روش نیمه‌دستی قدیمی‌تر هم به‌صورت ورودی «pbs-nbd» در منوی Clonezilla باقی است: اسنپ‌شات را به `/dev/nbd0` وصل می‌کند و بازیابی را به ویزارد استاندارد Clonezilla می‌سپارد.

## کاربرد: پشتیبان‌گیری از پوشه

مرجع کامل، با نمونه‌هایی برای ویندوز و لینوکس، کدهای خروج و زمان‌بندی، در **[راهنمای خط فرمان](docs/manual/CLI.md)** است. یک فرمان معمول:

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

متغیرهای محیطی `PBS_REPOSITORY`، `PBS_SERVER`، `PBS_PORT`، `PBS_DATASTORE`، `PBS_AUTH_ID`، `PBS_PASSWORD` و `PBS_FINGERPRINT` کلاینت رسمی هم خوانده می‌شوند. یک نمونه پیکربندی JSON وجود دارد. قالب ایمیل با [موتور قالب Go](https://pkg.go.dev/text/template) است؛ متغیرها (`.NewChunks`، `.Status` و غیره) در [README انگلیسی](README.md#usage-directory-backup).

**پشتیبان‌گیری از جریان (stream):** با `-backupstream` می‌توانید به جای pxar از یک جریان پشتیبان بگیرید، مثلاً `mysqldump yourdatabase | ./proxmoxbackup-directory -backupstream yourdatabase.sql [other options]`.

**مشکلات شناخته‌شده:** Windows Defender سرعت را تا 25٪ کم می‌کند. در ویندوز ابزارهای خط فرمان قفل فرایند دارند («Another backup job is running»)؛ در لینوکس چنین قفلی نیست، پس دو اجرا را هم‌زمان زمان‌بندی نکنید.

## 📋 پیش‌نیازها

- Windows 10/11 یا Windows Server (64 بیتی)، با دسترسی مدیر برای VSS و پشتیبان ماشین
- یا لینوکس (بسته‌ها برای Debian، Ubuntu، Mint و Fedora هستند)، با runtime ابزار WebKitGTK برای GUI، و root به‌علاوه ماژول کرنل elastio-snap یا dattobd برای پشتیبان ماشین سازگار
- دسترسی شبکه به یک Proxmox Backup Server

## 🔨 ساخت از سورس

**[BUILD.md](BUILD.md)** دستورالعمل کامل را دارد: GUI و سرویس پس‌زمینه، ابزارهای خط فرمان، MSI و بسته‌های لینوکس. به‌طور خلاصه، با Go 1.25 یا بالاتر و Wails CLI v2.13:

```bash
cd gui
wails build                                    # the app, in gui/build/bin
go build -tags service -o build/bin/ProxmoxBackupClientSVC.exe   # the background service
```

فرانت‌اند ساخته‌شده (`gui/frontend/dist`) در مخزن قرار دارد، پس Node.js فقط برای تغییر فرانت‌اند لازم است. آزمون‌ها: `make test`.

## 🔧 استفاده پیشرفته و راهنماها

### چند PBS (چند سرور PBS)

چند سرور PBS پیکربندی کنید و مقصد را برای هر Backup Set یا پشتیبان‌گیری یک‌باره انتخاب کنید (مثلاً `C:\Users` روزانه به یک PBS سریع SSD، `D:\` هفتگی به یک PBS بزرگ، به‌علاوه یک سرور DR).

- **[راهنمای کاربر](MULTI_PBS_USER_GUIDE.md)**: افزودن و آزمودن سرورها، سرور پیش‌فرض، انتخاب سرور برای هر پشتیبان، پرسش‌های متداول و عیب‌یابی.
- **[راهنمای پیاده‌سازی](MULTI_PBS_GUIDE.md)**: مدل داده، مهاجرت خودکار از پیکربندی تک‌PBS، متدهای بک‌اند.

پیکربندی قدیمی تک‌PBS در اولین بارگذاری خودکار به سرور `default` منتقل می‌شود.

### ISO زنده Clonezilla (بازیابی bare-metal)

جریان کار نجات با وصله کردن یک ISO استاندارد Clonezilla Live با باینری‌های `pbsnbd` / `machinebackup` به‌علاوه دو ورودی منو ساخته می‌شود (بوت از CD، USB از طریق `dd` و UEFI):

- **`ocs-pbs-nbd`**: ورودی **«pbs-nbd»** که به منوی انتخاب حالت Clonezilla اضافه می‌شود؛ اسنپ‌شات را وصل می‌کند و سپس کار را به ویزارد عادی بازیابی دیسک Clonezilla برای انجام دستی می‌سپارد.
- **`ocs-pbs-bare-metal-restore`**: ورودی جدید و پیش‌فرض اول منوی بوت، **«Proxmox Backup Client Go - PBS Bare Metal Restore»**: از اطلاعات PBS تا بازیابی کامل و راه‌اندازی مجدد خودکار (DHCP با جایگزین IP ثابت، همگام‌سازی NTP، شناسایی خودکار دیسک مقصد)، برای پشتیبان‌های ویندوز و لینوکس.

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

جزئیات کامل در **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**.

### ساخت GUI ویندوز

**ویندوز بومی:** برای ابزارها (Go، MinGW، Wails) و فرمان‌های ساخت برنامه، سرویس و MSI به **[BUILD.md](BUILD.md)** مراجعه کنید. **Docker (هنگام ساخت روی لینوکس):** `./build_gui_windows_docker.sh` برنامه ویندوز را به نام `ProxmoxBackupClientGO.exe` در یک کانتینر یک‌بارمصرف `golang` می‌سازد؛ سرویس پس‌زمینه و MSI را نمی‌سازد.

### وضعیت امکانات، تاریخچه تغییرات و مستندات داخلی

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)**: چه چیزی انجام شده، نیمه‌کاره است یا انجام نشده، و از کدام نسخه.
- **[CHANGELOG.md](CHANGELOG.md)**: تاریخچه تغییرات به تفکیک نسخه، و [Unreleased] برای آنچه روی master است.
- **[TODO.md](TODO.md)**: نقشه راه باز و ایده‌ها.
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)**: محتوای هر نسخه.
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**: پنجره حذف نصب MSI (نگه داشتن/حذف پیکربندی) و طرح آزمون آن.
- **[AGENTS.md](AGENTS.md)**: ساختار مخزن و قراردادها، برای مشارکت‌کنندگان و عامل‌های برنامه‌نویسی.

## 🌐 پشتیبانی فنی در ایتالیا

پروژه و **همه** مشارکت‌های آن برای همیشه عمومی و تحت GPLv3 می‌مانند. حامی اصلی پروژه و امکانات پشتیبان ماشین، E.T.I. Srl ( https://etitech.net )، می‌تواند برای استقرار Proxmox و به‌ویژه ابزارهای پشتیبان‌گیری ویندوز پشتیبانی ارائه دهد.

نسخه جداگانه «community» و «enterprise» هرگز وجود نخواهد داشت؛ پشتیبانی فنی یک خدمت مستقل است و هر سفارشی‌سازی، حتی پولی، با GPLv3 منتشر می‌شود.

## 🖥️ منشأ GUI

**Proxmox Backup Client GUI** بر پایه **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)** است که **[RDEM Systems](https://www.rdem-systems.com/)** آن را توسعه و نگهداری می‌کند.

این GUI (در اصل فورکی از این پروژه) به این مخزن بازگشته و کل کد تحت GPLv3 متن‌باز می‌ماند. RDEM Systems حامی توسعه GUI است و پشتیبانی تجاری ارائه می‌دهد.

## ⚠️ هشدار

این نرم‌افزار «همان‌گونه که هست» ارائه می‌شود. با اینکه به دنبال قابلیت اطمینان هستیم، هیچ مسئولیتی در قبال از دست رفتن یا آسیب داده‌ها نمی‌پذیریم. همیشه پیش از تکیه بر پشتیبان‌ها در محیط عملیاتی، آن‌ها را آزمایش و بازیابی را بررسی کنید.

این نرم‌افزار هنوز در کیفیت آلفا است و ما هیچ مسئولیتی در قبال هیچ نوع آسیب یا از دست رفتن داده، حتی فایل‌های منبع، نمی‌پذیریم.

## 📄 مجوز

GPLv3: فایل [LICENSE](LICENSE) را ببینید.

## 🏷️ برندینگ

هر مشارکت‌کننده‌ای که **دست‌کم 5 کامیت** با افزودن قابلیت یا رفع اشکال داشته باشد، حق دارد اطلاعات برند خود برای استفاده تجاری افزوده شود.

تنها شرط این است که شرکتی که برند به آن اشاره می‌کند در **هیچ** یک از این فعالیت‌ها نباشد:

- کارزارهای بدافزار
- کسب‌وکارهای ترویج‌کننده جنگ (برای هر کشوری، از جمله کشورهای غربی)
- کلاهبرداری
- سرقت داده
- قاچاق کودکان
- خشونت
- تبعیض
- مواد مخدر
- هر فعالیتی که به‌طور کلی غیرقانونی شناخته شود

در صورت شکایت تلاش می‌کنیم تماس بگیریم؛ اگر توضیح معتبری داده نشود، آن امتیاز **فوراً لغو** می‌شود.

**مجوز GPLv3 همچنان برقرار است** و شما همچنان آزادید پروژه را فورک کنید و فایل‌های اجرایی خودتان را بسازید.

## 🤝 مشارکت

GUI اکنون کاملاً پیاده‌سازی شده است، ولی مشارکت‌ها همچنان خوشایند است، به‌ویژه:

1. رمزنگاری: بازبینی متن‌های ترجمه ماشینی توسط اهل زبان، و مدیریت کلید فراتر از فایل کلید (مثلاً Windows DPAPI یا Credential Manager)
2. مهاجرت فیزیکی به مجازی (P2V): کارهای پایه با نوع اسنپ‌شات `vm` انجام می‌شود (با دیسک UEFI ویندوز بررسی شده)؛ کارت شبکه، TPM، درایورهای virtio، CPU/RAM ماشین واقعی و گذرگاه‌های غیر SATA هنوز کم است
3. آپلود ناهمگام / چندهسته‌ای chunk ها (فشرده‌سازی چندهسته‌ای برای پشتیبان ماشین پیاده‌سازی شده است)
4. وصله‌ای در سمت Proxmox برای افزودن نوع دیگری از ورودی با توصیفگرهای امنیتی ویندوز به قالب pxar
5. پشتیبانی از symlink های ویندوز
6. هر چیز جالب دیگری که به ذهنتان برسد :)

## درباره مشارکت‌کنندگان Proxmox Backup Client GO

مشارکت‌کنندگان Proxmox Backup Client GO این پروژه را توسعه و نگهداری می‌کنند. این نرم‌افزار به زیرساخت NTP/NTS و [11 سرور عمومی NTS](https://github.com/jauderho/nts-servers) فهرست‌شده در مرجع جامعه متکی است.

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**
