# Proxmox Backup Client: عميل ويندوز ولينكس لخادم Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Proxmox Backup Client عميل نسخ احتياطي مفتوح المصدر (GPL-3.0) لخادم Proxmox Backup Server (PBS)، يعمل على ويندوز ولينكس.**

إنه **مجموعة أدوات** للنسخ الاحتياطي إلى PBS:

- **Proxmox Backup Client GUI** (مبني على Nimbus Backup GUI من RDEM Systems): عميل رسومي لخوادم ومحطات عمل ويندوز ولينكس، مع لقطات متسقة (VSS على ويندوز، وelastio-snap/dattobd على لينكس)، ومجموعات نسخ (Backup Sets) بجداول زمنية، ونسخ المجلدات والأقراص الكاملة، والاستعادة، وUndelete وRoll back، وعدة خوادم PBS، وخدمة خلفية على ويندوز ولينكس.
- **`proxmoxbackup-directory`**: نسخ المجلدات (pxar) من سطر الأوامر، مع إزالة التكرار واكتشاف التغييرات.
- **`proxmoxbackup-machine`**: نسخ الأقراص الكاملة (fidx) لأجهزة ويندوز (VSS) ولينكس (elastio-snap/dattobd) أثناء تشغيلها؛ والصورة قابلة للإقلاع بعد الاستعادة على جهاز فارغ (bare-metal).
- **`proxmoxbackup-nbd`**: يربط نسخة قرص كجهاز NBD لاستعادتها (لينكس).
- **قرص ISO للاستعادة على جهاز فارغ** مبني على Clonezilla Live.

> ⚠️ **إخلاء مسؤولية:** هذا المشروع **غير مرتبط بأي شكل** بشركة **Proxmox Server Solutions GmbH**. اسم «Proxmox» وشعاره والأسماء المرتبطة به ملك لأصحابها، وتُستخدم هنا **فقط** للإشارة إلى التوافق. راجع [proxmox.com](https://www.proxmox.com/).

> ℹ️ **عن هذه الصفحة.** هذه خلاصة مترجمة. المرجع هو ملف [README.md](README.md) الإنجليزي، وفيه كل التفاصيل. الأدلة مكتوبة بالإنجليزية. واجهة البرنامج غير مترجمة إلى العربية، لذلك تُذكر أسماء القوائم والأزرار بالإنجليزية كما تظهر في البرنامج. أُعدّت الترجمة بمساعدة الذكاء الاصطناعي، والتصحيحات مرحّب بها.

## 📖 أدلة المستخدم

| الدليل | المحتوى |
|---|---|
| **[الواجهة الرسومية](docs/manual/GUI.md)** | التثبيت، خوادم PBS، Backup Sets والجداول، النسخ التزايدي، الاستعادة، Undelete، Roll back، Reports، Preferences، الخدمة الخلفية، حل المشكلات |
| **[سطر الأوامر](docs/manual/CLI.md)** | `proxmoxbackup-directory` و`proxmoxbackup-machine` و`proxmoxbackup-nbd`: كل الخيارات، الإعداد، رموز الخروج، الجدولة؛ وUndelete وRoll back من سطر الأوامر |
| **[الاستعادة على جهاز فارغ](docs/manual/BMR.md)** | قرص ISO للاستعادة خطوة بخطوة؛ لقطات `vm` على Proxmox VE؛ عتاد مختلف |
| [سيناريوهات الاستعادة](docs/RESTORE_GUIDE.md) | أي استعادة تناسب أي حالة |
| [الفهرس](docs/manual/README.md) | نظرة عامة على الأدلة الثلاثة |

## 🆕 في master، للإصدار القادم

مذكورة في [CHANGELOG](CHANGELOG.md) تحت [Unreleased] ولم تصدر بعد:

- **برامج التثبيت**: **مثبّت MSI لويندوز** (البرنامج والخدمة في Program Files، وقائمة ابدأ، والترقية فوق الإصدار الحالي، والاختيار عند الإزالة بين إبقاء الإعدادات أو حذفها) و**حزم لينكس** `.deb` (Debian وUbuntu وMint) و`.rpm` (Fedora).
- **خدمة خلفية على لينكس** (`proxmoxbackupclient.service`): تعمل Backup Sets المجدولة بصلاحيات root دون مستخدم مسجّل. تُفعَّل كما في ويندوز من Preferences > Advanced > Run as Service. وصار **Restart as administrator** يعمل على لينكس أيضًا (عبر pkexec).
- **Pause وResume وStop للنسخ التي تشغّلها الخدمة**.
- **تحذير عندما تكتب مجموعتا نسخ في مجموعة نسخ واحدة على الخادم** (الخادم والنوع وBackup ID نفسها)، مع شارة **SHARED ID**.
- **إعدادات محمية**: المجلد `C:\ProgramData\ProxmoxBackupClient` للمسؤولين فقط؛ وعلى لينكس مجلد الإعدادات خاص بمالكه.
- **ملفات من نسخ الأجهزة بأنظمة FAT وexFAT وext2/3/4 وXFS**، لا NTFS فقط؛ واستعادة صور ويندوز تعيد الأذونات وتدفقات البيانات البديلة.
- **نسخ أجهزة لينكس تتخطى المساحة الفارغة** (ext2/3/4 وXFS).
- **Undelete وRoll back لأجهزة لينكس**، و**من سطر الأوامر**: `-list-sets` و`-undelete` و`-rollback` (القسم 12 من [دليل سطر الأوامر](docs/manual/CLI.md)).
- أدوات سطر الأوامر تكتب الأخطاء إلى stderr بدل نوافذ الرسائل؛ و`proxmoxbackup-machine` يرسل بريد الإشعار.

## 💽 الجديد في v0.8.0

- **نسخ الأجهزة على ويندوز تتخطى المساحة الفارغة** (وملفات الترحيل والسبات): يُقرأ القرص نصف الفارغ في نحو نصف الوقت. و**القطاع التالف** لم يعد يوقف النسخ: يُكتب أصفارًا ويُذكر مكانه.
- **استعادة الملفات من نسخة جهاز**: كل وحدة NTFS في الصورة تُفتح كمجلد، دون تركيب أو تنزيل مسبق (قرص C: بحجم 127 GB فيه 430,000 ملف يُعرض خلال 4 إلى 15 ثانية). نسخة جهاز واحدة تكفي للاستعادة على جهاز فارغ ولطلب «أحتاج ذلك الملف».
- **Undelete وRoll back** لمجموعات نسخ الأجهزة (ويندوز)، على المجلدات المختارة (افتراضيًا Desktop وDocuments وPictures وMusic وVideos وDownloads).
- **Pause وResume**، وشارة **VSS / Snapshot ON-OFF**، وحفظ **أوقات الإنشاء** في ويندوز.
- **v0.8.2:** لم تعد المساحة الفارغة تُحسب بصمتها، فيمر القرص شبه الفارغ في نحو ثلث الوقت (50 GB: من 3m36s إلى 1m15s).

## 🗑️ الجديد في v0.7.0: Undelete وRoll back

- **Undelete** يجد الملفات الموجودة في نسخ المجموعة لكنها اختفت من القرص (في أحدث نسخة أو خلال 7 أو 30 يومًا)، ويعرضها شجرة مجلدات مع وقت اختفائها، ويستعيدها إلى مكانها أو إلى مجلد آخر. بعد حذف 39 GB من مشاركة فيها 761,000 ملف، وجدها بحث 7 أيام في نحو دقيقة.
- **Roll back** يعيد مجلدات المجموعة إلى نسخة مختارة: معاينة أولًا ثم استعادة. ما يُستبدل يُنقل جانبًا على القرص نفسه، فيعيد **Undo** كل شيء.

وأيضًا: Backup ID مقترح، ووقت التشغيل التالي على كل بطاقة، والخدمة الخلفية ضمن تنزيل ويندوز. راجع [دليل الواجهة الرسومية](docs/manual/GUI.md#10-undelete).

## ⚡ الجديد في v0.6.0: نسخ تزايدي للمجلدات

في وضع **Metadata** (كما في `proxmox-backup-client` الرسمي) لا تُقرأ الملفات غير المتغيرة (الحجم نفسه، ووقت التعديل حتى النانوثانية، والأذونات والمالك): تؤخذ بياناتها من اللقطة السابقة. خادم ملفات بحجم 480.9 GB فيه 761,866 ملفًا:

| التشغيل | الوقت | القطع الجديدة المرفوعة |
|---|---|---|
| أول تشغيل Metadata | 4 ساعات و17 دقيقة | 135,549 |
| التشغيلات التالية | نحو 10 إلى 15 دقيقة | من 4 إلى 40 |

تنخفض حركة الشبكة إلى PBS بعيد إلى ما تغيّر فعلًا تقريبًا. الإعداد موضّح أدناه في قسم اكتشاف التغييرات.

## 🤝 متوافق مع عميل Proxmox الرسمي

تم التحقق مع `proxmox-backup-client` 3.4.9 وPBS 4.2: لقطات Legacy وData وMetadata **تُستعاد مطابقة بايتًا ببايت في الاتجاهين** (من الواجهة الرسومية وسطر الأوامر، على ويندوز ولينكس)، ومتصفح الملفات في PBS 4.2 يفتحها. صيغة pxar الإصدار 2 نفسها، وأوضاع اكتشاف التغييرات نفسها، وصيغة مفتاح التشفير نفسها (`proxmox-backup-client key create`)، ومتغيرات `PBS_REPOSITORY` نفسها. الإضافة الوحيدة ملف `.proxmox_backup_client_meta.json` (المسار الأصلي) في جذر لقطة المجلد، ويستعيده العميل الرسمي كملف عادي.

## 🔧 عن هذا الفرع (fork)

فرع **mjb-is**: **https://github.com/mjb-is/proxmoxbackupclient_go**. الهدف: أن يكون PBS نظام النسخ الوحيد في مختبر منزلي، مع واجهة رسومية موثوقة واستعادة آلية بالكامل على جهاز فارغ. ما يفيد الجميع يُرسل إلى المشروع الأصلي؛ وقد دُمج سبعة عشر طلب دمج (القائمة في [README الإنجليزي](README.md#-about-this-fork)).

## 📦 التنزيل والتثبيت

👉 **[تنزيل أحدث إصدار](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| لديك | استخدم |
|---|---|
| Windows | **MSI** (`ProxmoxBackupClient-<version>-x64.msi`): البرنامج والخدمة الخلفية في Program Files، وقائمة ابدأ. أو `ProxmoxBackupClient-v<version>-windows-amd64.zip` (البرنامج و`ProxmoxBackupClientSVC.exe` في المجلد نفسه) |
| Debian, Ubuntu, Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| توزيعة لينكس أخرى | فك ضغط `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` (يحتاج WebKitGTK) |
| سطر الأوامر فقط | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` أو `-linux-amd64.tar.gz` |

ملفات MSI و`.deb` و`.rpm` موجودة في master فقط (الإصدارات حتى v0.8.1 فيها zip وtar.gz فقط)؛ ابنها كما يشرح [BUILD.md](BUILD.md). على لينكس يشغّل مدخل **Proxmox Backup Client** البرنامج باسمك، ويشغّله **Proxmox Backup Client (administrator)** بصلاحيات root عبر polkit لنسخ الأجهزة. تُثبَّت الخدمة دون تفعيل: راجع [packaging/debian/README.md](packaging/debian/README.md).

> ⚠️ **هل يبلّغ ويندوز عن فيروس (مثل `Trojan:Win32/Sabsik.FL.A!ml`) أو يعرض تحذير SmartScreen؟** هذا **إنذار كاذب معروف** لتطبيقات Go/Wails غير الموقعة (اللاحقة `!ml` تعني نموذج تعلم آلي). [التفاصيل](https://github.com/tizbac/proxmoxbackupclient_go).

### 🔎 التحقق من التنزيل

كل إصدار يتضمن `SHA256SUMS.txt`:

```powershell
Get-FileHash .\ProxmoxBackupClient-v0.8.1-windows-amd64.zip -Algorithm SHA256   # compare with SHA256SUMS.txt
```

على لينكس: `sha256sum -c SHA256SUMS.txt --ignore-missing`. لا يوجد بعد توقيع للشيفرة ولا شهادة CI ولا فحص VirusTotal (راجع [TODO.md](TODO.md)).

## ✨ الميزات

### الواجهة الرسومية (موصى بها)
- **18 لغة للواجهة** (الفرنسية والإنجليزية والإيطالية والألمانية والبولندية والإسبانية والبلغارية والتشيكية واليونانية والمجرية واللاتفية والليتوانية والهولندية والبرتغالية والرومانية والسلوفاكية والتركية والأوكرانية)
- القوائم: Backup وRestore وUndelete وRoll back وReports وMessage Log وPreferences وAbout
- **Backup Sets**: جدولة (يومية / دورية / يدوية)، وشارتا النوع وVSS / Snapshot، و**Run Now**، وتحذير **SHARED ID**؛ ونسخ لمرة واحدة للمجلدات والأقراص
- **نسخ تزايدي للمجلدات** (Metadata) مع قراءة كاملة كل N تشغيلات
- **نسخ الأجهزة** على ويندوز ولينكس دون المساحة الفارغة ومع تحمّل القطاعات التالفة، كـ`host` أو كلقطة `vm` لـProxmox VE
- **Restore**: تصفح، وبحث (أحرف بدل، تعابير نمطية)، واستعادة انتقائية مع ACL الخاصة بـNTFS/POSIX وأوقات المجلدات، وتحقق اختياري؛ وملفات من صور الأقراص
- **Undelete** و**Roll back** و**Pause وResume وStop**
- **Run as Service** (Preferences > Advanced): جداول دون مستخدم مسجّل (ويندوز منذ v0.7.0؛ ولينكس في master)
- بطاقة تقدّم، وReports، وMessage Log، وخمس سمات ألوان، وإشعارات بريد، وإجراءات بعد النسخ، وتصدير الإعدادات واستيرادها
- عدة خوادم PBS، وتثبيت بصمة الشهادة، والدخول برمز API أو بكلمة مرور
- **تشفير من جهة العميل** (AES-256-GCM، وبيان manifest موقّع)

### أدوات سطر الأوامر
- `proxmoxbackup-directory`: نسخ المجلدات مع `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`: نسخ الأقراص الكاملة (fidx، تزايدي؛ و`-readfreespace` يقرأ المساحة الفارغة أيضًا)
- `proxmoxbackup-nbd`: خادم NBD للاستعادة (لينكس)
- `ProxmoxBackupClientSVC` (ويندوز) و`proxmoxbackupclient` (لينكس): `-list-sets` و`-undelete` و`-rollback` (في master)

الأدوات الثلاث الأولى تقبل `-keyfile` و`-keyfile-passphrase`. المرجع: [دليل سطر الأوامر](docs/manual/CLI.md).

### 📸 لقطات الشاشة

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Backup Sets**: مهام بشارتي الجدولة والنوع، وزر Run Now لكل منها

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**تقدّم النسخ**: معدّل النقل، والوقت المتبقي، والقطع الجديدة مقابل المعاد استخدامها

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Restore**: خادم PBS وBackup ID، وقائمة اللقطات أو البحث باسم الملف

</td>
</tr>
</table>

بقية لقطات الشاشة في [README الإنجليزي](README.md#-screenshots).

### الاستثناءات والأمان
تستثني نسخ المجلدات تلقائيًا `System Volume Information` و`$RECYCLE.BIN` و`Recovery` (في جذر القرص) و`.pbs-rollback` و`pagefile.sys` و`hiberfil.sys` و`swapfile.sys` و`DumpStack.log.tmp`. للملفات استخدم نسخ **المجلدات**، وللاستعادة على جهاز فارغ استخدم Backup Set منفصلًا لنسخ **الجهاز**.

الأمان: تشفير القطع، ومجلد إعدادات محمي (في master)، وواجهة API محلية للخدمة على 127.0.0.1 فقط برمز، والتحقق من المدخلات، ومنع اجتياز المسارات، وإعادة المحاولة، واختبارات وحدات واختبارات شاملة.

## 🔐 التشفير من جهة العميل

تُشفَّر اللقطات قبل مغادرتها الجهاز بملف مفتاح Proxmox Backup Server (صيغة `proxmox-backup-client key create`): AES-256-GCM وبيان manifest موقّع. لا يرى خادم PBS المفتاح أبدًا.

**الواجهة الرسومية:** **Preferences > Account Information**، حرّر الخادم، لسان **Encryption**: اختر ملف مفتاح أو **Generate...**. للمفتاح المحمي بعبارة مرور: **Ask me** (العبارة في الذاكرة طوال الجلسة؛ والنسخ المجدولة ونسخ الخدمة تفشل فورًا دونها) أو **Remember on this computer** (العبارة في `config.json`؛ للمهام دون إشراف وللخدمة). **دون ملف المفتاح لا يمكن الاستعادة: احتفظ بنسخة منه بعيدًا عن هذا الجهاز.**

```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```

يطلب قرص Clonezilla المفتاح والعبارة عند الاستعادة؛ ولا يُكتب المفتاح في ISO أبدًا. يحفظ `config.json` الرموز والعبارات نصًا صريحًا (وحتى v0.8.2 كان على ويندوز مقروءًا للمستخدمين المحليين).

## نسخ أسرع للمجلدات: اكتشاف التغييرات

| الوضع | ما يُقرأ | طريقة الحفظ |
|---|---|---|
| **Legacy** (افتراضي) | كل الملفات | `<name>.pxar.didx` |
| **Data** | كل الملفات | أرشيف مقسوم `<name>.mpxar.didx` + `<name>.ppxar.didx` |
| **Metadata** | الجديد والمعدَّل فقط | الأرشيف المقسوم نفسه |

**الواجهة الرسومية:** Backup Set، لسان **Destination**، ثم **Change detection**. اضبط **Read every file every** N تشغيلات لالتقاط التغييرات النادرة التي تُبقي الحجم والوقت (مثلًا 42 لتشغيل كل 4 ساعات). **أعطِ كل مجموعة Backup ID خاصًا بها**، وإلا أجبرت المجموعات بعضها على قراءة كل شيء. لمجموعات المجلدات فقط؛ والنسخ لمرة واحدة تستخدم Legacy. **سطر الأوامر:** `-change-detection-mode legacy|data|metadata`.

قبل التحويل إلى Data أو Metadata حدّث كل جهاز تستعيد منه إلى v0.6.0 أو أحدث. أول نسخة مقسومة تكلّف تقريبًا نسخة كاملة إضافية حتى تُحذف اللقطات القديمة.

## 🚀 البدء السريع (الواجهة الرسومية)

1. ثبّت MSI أو `.deb` أو `.rpm`، أو فك ضغط zip أو tar.gz من [الإصدارات](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
2. على ويندوز استخدم **التشغيل كمسؤول** من أجل VSS ونسخ الأجهزة؛ وعلى لينكس استخدم **Proxmox Backup Client (administrator)** لنسخ الأجهزة
3. أضف الخادم في **Preferences > Account Information** واضغط **Test**
4. أنشئ Backup Set (أو نسخة لمرة واحدة) واختر المجلدات أو الأقراص
5. شغّله. لتعمل الجداول دون مستخدم مسجّل، فعّل **Preferences > Advanced > Run as Service**

## النسخ الكامل للجهاز أثناء التشغيل

يُستبدل `-backupdir` بـ`-backupdev` (يمكن تكراره):

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

يأخذ الأمر لقطة VSS لكل أقسام القرص 0 وينشئ صورة fidx قابلة للإقلاع. تُكتب المساحة الفارغة أصفارًا دون قراءة؛ والنسخ التالية تزايدية بسرعة تصل إلى 1 GB/s.

### متطلبات نسخ أجهزة لينكس (بديل VSS)

يلزم وحدة النواة [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap) (أو `dattobd` محمّل مسبقًا). التثبيت من مستودع الحزم لا يعمل حاليًا على Ubuntu 22.04؛ ابنها من المصدر:

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

يجب تشغيل نسخ الجهاز **بصلاحيات root** (مدخل **Proxmox Backup Client (administrator)** أو الخدمة أو `sudo`)؛ ومجموعة `disk` لا تكفي.

### استعادة الملفات من نسخة قرص

تفتح صفحة Restore نسخة الجهاز كمجلدات (NTFS منذ v0.8.0؛ وFAT وexFAT وext2/3/4 وXFS في master). وعلى لينكس يمكن ربطها عبر NBD بعد `modprobe nbd max_part=0`:

`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

دون `-path` تظهر قائمة لاختيار الملف. لا تفعل ذلك على جهاز مهم؛ وافصل كل شيء قبل الإيقاف؛ وعند `Device or resource busy` نفّذ `nbd-client -d /dev/nbd0`.

### الاستعادة إلى جهاز فعلي

قرص Clonezilla Live من هذا الفرع فيه مدخل آلي **«PBS Bare Metal Restore»**: بيانات PBS، ثم اللقطة، ثم القرص الهدف، والباقي يتم وحده حتى إعادة التشغيل. والمدخل شبه اليدوي «pbs-nbd» موجود أيضًا. راجع [الدليل](docs/manual/BMR.md).

## الاستخدام: نسخ المجلدات

المرجع الكامل: **[دليل سطر الأوامر](docs/manual/CLI.md)**.

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

تُقرأ أيضًا متغيرات البيئة الخاصة بالعميل الرسمي (`PBS_REPOSITORY` و`PBS_SERVER` و`PBS_PORT` و`PBS_DATASTORE` و`PBS_AUTH_ID` و`PBS_PASSWORD` و`PBS_FINGERPRINT`). تستخدم رسائل البريد [قوالب Go](https://pkg.go.dev/text/template). على ويندوز لا تبدأ نسخة ثانية أثناء عمل الأولى؛ ولا يوجد قفل كهذا على لينكس. ويبطئ Windows Defender النسخ حتى 25%.

## 🔨 البناء من المصدر

كل التفاصيل في **[BUILD.md](BUILD.md)** (Go 1.25 أو أحدث، وWails CLI v2.13):

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

## 🔧 الاستخدام المتقدم والأدلة

- **عدة خوادم PBS:** [دليل المستخدم](MULTI_PBS_USER_GUIDE.md) و[دليل التنفيذ](MULTI_PBS_GUIDE.md).
- **قرص Clonezilla Live:** طريقة بنائه في **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**:

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)**: ما اكتمل ومنذ أي إصدار. **[CHANGELOG.md](CHANGELOG.md)**: سجل التغييرات. **[TODO.md](TODO.md)**: الخطط. **[RELEASE_NOTES.md](RELEASE_NOTES.md)** و**[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)** و**[AGENTS.md](AGENTS.md)**.

## 🌐 الدعم الفني في إيطاليا

يبقى المشروع و**كل** المساهمات فيه مفتوحة إلى الأبد بترخيص GPLv3. الراعي الرئيسي، E.T.I. Srl ( https://etitech.net )، يقدّم الدعم لـProxmox ولأدوات نسخ ويندوز. لن تكون هناك نسختا «community» و«enterprise» منفصلتان؛ وأي تخصيص يصدر بترخيص GPLv3.

## 🖥️ نسبة الواجهة الرسومية

**Proxmox Backup Client GUI** مبني على **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)** من **[RDEM Systems](https://www.rdem-systems.com/)**، وقد دُمج في هذا المستودع بترخيص GPLv3. وترعى RDEM Systems تطوير الواجهة الرسومية وتقدّم لها دعمًا تجاريًا.

## ⚠️ تحذير

يُقدَّم البرنامج «كما هو»، وهو بجودة ألفا، ولا نتحمل مسؤولية فقدان البيانات أو تلفها. اختبر نسخك والاستعادة دائمًا.

## 📄 الترخيص

GPLv3: راجع [LICENSE](LICENSE).

## 🏷️ العلامة التجارية

لكل مساهم قدّم **5 إيداعات على الأقل** لوظائف أو إصلاحات أن يضيف علامته التجارية للاستخدام التجاري، بشرط ألا ترتبط الشركة **بأي** من: البرمجيات الخبيثة، أو الترويج للحرب، أو الاحتيال، أو سرقة البيانات، أو الاتجار بالأطفال، أو العنف، أو التمييز، أو المخدرات، أو أي نشاط غير قانوني. **يبقى ترخيص GPLv3 ساريًا.**

## 🤝 المساهمة

المساعدة مرحّب بها: مراجعة ترجمات نصوص التشفير وإدارة المفاتيح خارج الملف (مثل Windows DPAPI)، وP2V (لقطة `vm` تعمل بالفعل، وينقصها بطاقة الشبكة وTPM وتعريفات virtio)، والرفع غير المتزامن للقطع، وواصفات أمان ويندوز في pxar، والروابط الرمزية في ويندوز، وأي فكرة مثيرة للاهتمام :)

## عن مساهمي Proxmox Backup Client GO

يطوّر مساهمو Proxmox Backup Client GO هذا المشروع ويصونونه. ويعتمد البرنامج على بنية NTP/NTS وعلى [11 خادم NTS عامًا](https://github.com/jauderho/nts-servers).

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**
