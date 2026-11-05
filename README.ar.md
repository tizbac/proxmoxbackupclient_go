# Salvanza for Proxmox Backup Server: عميل Windows وLinux لـ Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

> ℹ️ هذه ترجمة لـ[ملف README الإنجليزي](README.md)؛ وحيثما اختلفا، فالنسخة الإنجليزية هي المعتمدة والمحدَّثة.

> ⚠️ **إشعار بشأن العلامة التجارية والانتساب.** هذا **مشروع مجتمعي مستقل**، وليس منتجًا رسميًا من Proxmox: فهو **غير تابع لشركة Proxmox Server Solutions GmbH، ولا معتمد منها، ولا برعايتها**. إن **Proxmox®** علامة تجارية مسجّلة لشركة Proxmox Server Solutions GmbH في الاتحاد الأوروبي والولايات المتحدة ودول أخرى، وتُستخدم هنا **فقط** لبيان التوافق. والتزامًا بإرشادات Proxmox نفسها بشأن العلامة التجارية (لا يجوز استخدام العلامات المسجّلة في أسماء المنتجات أو التطبيقات، فيُقال مثلًا "XY App for Proxmox Virtual Environment" بدلًا من "Proxmox XY App")، **تغيّر اسم العميل غير الموسوم بعلامة تجارية**: فالمنتج الذي كان يُسمّى سابقًا *Proxmox Backup Client* أصبح الآن **Salvanza for Proxmox Backup Server** (واختصارًا Salvanza)، وهو اسم لا يتضمن علامة Proxmox. راجع [حزمة Proxmox الإعلامية](https://www.proxmox.com/en/about/company-details/media-kit) لمعرفة القواعد الدقيقة.

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/salvanza/salvanza_client)](https://github.com/salvanza/salvanza_client/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Salvanza for Proxmox Backup Server (المعروف سابقًا باسم Proxmox Backup Client) عميل نسخ احتياطي مفتوح المصدر (GPL-3.0) لـ Proxmox Backup Server (PBS)، يعمل على Windows وLinux.** ينسخ المجلدات احتياطيًا كأرشيفات ملفات قابلة للتصفح، والأجهزة كاملةً كصور أقراص قابلة للإقلاع، وفق جداول زمنية أو عند الطلب. ويستعيد أي شيء، من ملف واحد إلى جهاز كامل: على الحاسوب نفسه، أو على جهاز بديل، أو داخل آلة افتراضية في Proxmox VE.

موقع المشروع: **[salvanza.org](https://salvanza.org/)** · الشيفرة المصدرية والإصدارات والبلاغات: **[github.com/salvanza/salvanza_client](https://github.com/salvanza/salvanza_client)**

إنه **مجموعة من الأدوات**:

- **تطبيق Salvanza**، `salvanza-gui` (أنشأته RDEM Systems في الأصل باسم Nimbus Backup GUI، ثم أعاد mjb-is صياغته ووسّعه): التطبيق لـ Windows وLinux. **Backup Sets** (مجموعات النسخ الاحتياطي) لكل منها عدة جداول زمنية، ونسخ احتياطي للمجلدات وللأقراص كاملة مع لقطات متسقة (VSS على Windows، وelastio-snap/dattobd على Linux)، واستعادة الملفات من نسخ المجلدات ونسخ الأجهزة، و**Undelete** (استرجاع المحذوفات) و**Roll back** (التراجع)، وعدة خوادم PBS، وتشفير من جهة العميل، وخدمة خلفية، `salvanza-svc`، تُشغّل الجداول الزمنية دون أن يكون أي مستخدم مسجّلًا للدخول.
- **`salvanza-cli`**: أداة سطر أوامر واحدة، لها أمر فرعي لكل مهمة:
  - **`salvanza-cli backup-folder`**: نسخ احتياطي للمجلدات (pxar) مع إزالة التكرار واكتشاف التغييرات.
  - **`salvanza-cli backup-disk`**: نسخ احتياطي حي للأقراص كاملة (fidx) لأجهزة Windows وLinux. تكون الصورة قابلة للإقلاع بعد الاستعادة.
  - **`salvanza-cli attach-nbd`**: يربط نسخة احتياطية لقرص كجهاز كتل NBD (Linux).
- **قرص ISO للاستعادة على جهاز فارغ (bare-metal)** (Salvanza-BMR) مبني على Clonezilla Live، مع استعادة موجَّهة من لحظة التشغيل حتى إعادة الإقلاع.

> كلمات مفتاحية: salvanza · proxmox backup server client windows · PBS client · Windows VSS backup · immutable offsite backups · physical to virtual (P2V) · bare-metal restore · Proxmox Backup Server interface.

<a id="-new-in-20"></a>

## 💡 لماذا هذا العميل

عميل `proxmox-backup-client` الرسمي أداة سطر أوامر لنظام Linux. يعمل هذا العميل مع الخادم نفسه وصيغ النسخ الاحتياطي نفسها، ويضيف:

- **Windows.** لا يوجد عميل رسمي لنظام Windows. هنا توجد أدوات سطر أوامر وتطبيق، ونسخ احتياطي للمجلدات عبر لقطات VSS، ونسخ احتياطي حي للأقراص كاملة.
- **تطبيق لـ Windows وLinux**: **Backup Sets** (مجموعات النسخ الاحتياطي) مجدولة تشغّلها خدمة خلفية فلا يلزم أن يسجّل أحد الدخول، واستعادة مع بحث، و**Undelete** (استرجاع المحذوفات) و**Roll back** (التراجع)، وتقارير وبريد إلكتروني.
- **أجهزة كاملة**: صور أقراص قابلة للإقلاع، وملفات مفردة تُستعاد منها، وملف ISO لاستعادة موجَّهة على عتاد فارغ (bare-metal)، على الحاسوب نفسه أو عتاد جديد أو آلة افتراضية في Proxmox VE.
- **الحفاظ على تفاصيل ملفات Windows**: تُسجَّل أذونات NTFS والسمات وتدفقات البيانات البديلة والسمات الموسعة وأوقات الإنشاء مع كل نسخة احتياطية لمجلد، وتعيدها الاستعادة في هذا العميل إلى مكانها.
- **دون ارتباط**: ما ينسخه هذا العميل يُستعاد بالعميل الرسمي، وما ينسخه العميل الرسمي يُستعاد بهذا العميل (انظر أدناه).

## 🆕 الجديد في الإصدار 2.0

يدمج الإصدار 2.0 في المشروع قدرًا كبيرًا من العمل على الواجهة الرسومية والخدمة الخلفية ومحرك النسخ الاحتياطي (راجع [CHANGELOG](CHANGELOG.md)). مقارنةً بالإصدار 1.2:

- **يتصل التطبيق بالخدمة الخلفية بصلاحيات المسؤول.** عند تثبيت الخدمة، تصبح خوادم PBS و**Backup Sets** (مجموعات النسخ الاحتياطي) الخاصة بالحاسوب ملكًا للخدمة، ولا يقرأ مجلد إعداداتها أحد سواها. يطلب التطبيق صلاحيات المسؤول مرة واحدة للاتصال (UAC على Windows، وpkexec على Linux؛ وعلى حساب قياسي، اسم مستخدم مسؤول وكلمة مروره). لا يُرفع إلى صلاحيات المسؤول إلا برنامج مساعد صغير، ويحتفظ التطبيق برمز الخدمة في الذاكرة فقط. يمكن للمستخدم الذي لا يستطيع الرد على هذا الطلب أن يختار **Use my own settings** (استخدام إعداداتي الخاصة) ويحتفظ بخوادمه ومجموعات نسخه، التي تعمل ما دام التطبيق مفتوحًا؛ ولاحقًا يعرض **Connect** (اتصال) نسخها إلى إعدادات الحاسوب. التطبيق نفسه يعمل دون صلاحيات المسؤول، ويعرض **Restart as administrator** (إعادة التشغيل كمسؤول) حيث تحتاج عملية ما إلى ذلك. راجع القسم 15 من [دليل الواجهة الرسومية](docs/manual/GUI.md#connecting-to-the-background-service).
- **يتحقق التطبيق من أنه يتحدث إلى الخدمة.** قبل أن يرسل أي شيء إلى واجهة API المحلية للخدمة، يتحقق التطبيق ممن يجيب: عملية الخدمة نفسها على Windows، ومقبس (socket) يملكه root على Linux. أي برنامج يستولي على المنفذ أثناء توقف الخدمة لا يحصل على شيء.
- **مجموعات نسخ احتياطي لكل منها عدة جداول زمنية**، من كل بضع دقائق إلى مرة في السنة (آخر يوم من الشهر، الثلاثاء الثاني)، ويُعرض كل جدول بكلمات بسيطة مع مواعيد تشغيله الثلاثة التالية.
- **نسخ احتياطي تزايدي للمجلدات** باكتشاف التغييرات المعتمد في العميل الرسمي (وضعا Data وMetadata): لا تُقرأ الملفات غير المتغيرة مرة أخرى.
- **نسخ احتياطي أسرع للأجهزة** يتخطى المساحة الحرة وملف ترحيل الصفحات (paging file) وملف الإسبات، ويكتب القطاعات غير القابلة للقراءة أصفارًا بدلًا من أن يفشل.
- **عدة نسخ احتياطية في آن واحد** (**Maximum Concurrent Backups** (الحد الأقصى للنسخ المتزامنة)، من 1 إلى 8، والقيمة الافتراضية 3)، وكذلك في الخدمة الخلفية، ولكل منها لقطتها وسجلها و**Pause** (إيقاف مؤقت) و**Stop** (إيقاف).
- **حدود للسرعة** للرفع إلى PBS، والتنزيل من PBS، والقراءة من القرص، وكل منها ميزانية واحدة يتقاسمها كل ما يعمل في الوقت نفسه.
- **Undelete** (استرجاع المحذوفات) و**Roll back** (التراجع) لمجموعات نسخ المجلدات والأجهزة، في الواجهة الرسومية ومن سطر الأوامر.
- **استعادة الملفات من نسخ الأجهزة**: تصفّح وحدات تخزين NTFS وFAT وexFAT وext2/3/4 وXFS داخل صورة القرص، واستعد ملفات مفردة.
- **مفاتيح تشفير يمكنك الاحتفاظ بها**: احفظ نسخة، واطبع مفتاحًا ورقيًا برمز QR، واستورد مفتاحًا من ملف أو من الكاميرا أو من صورة أو من نص مكتوب.
- **مجموعات نسخ الأجهزة تثبّت أقراصها بهويتها**، فالقرص الذي يحصل على رقم جديد بعد تغيير في العتاد يبقى هو القرص الذي يُنسخ، والقرص المفقود يُفشل التشغيل بدلًا من نسخ قرص آخر.
- **يمكن استبعاد أقسام من نسخة الجهاز**: أشّر على الأقسام المراد نسخها تحت كل قرص. القسم المستبعد لا يُقرأ ولا تؤخذ له لقطة، ويدخل الصورة أصفارًا، فتعيده الاستعادة في مكانه وبحجمه، فارغًا وغير مهيأ. ويميّز منتقي الأقراص الأقسام التي يحتاجها القرص المستعاد كي يُقلع.
- **نسخ احتياطي لأجهزة Linux باستخدام elastio-snap أو dattobd.** يمكن لمجموعة نسخ جهاز Linux أن توقف اللقطات وتقرأ الأقراص وهي تعمل، ولكن فقط عندما يؤشّر مالكها على أنه يقبل أن تكون النسخة غير متسقة.
- **يمكن لكل مستخدم اختيار سمة الألوان الخاصة به** في التطبيق غير الموسوم بعلامة تجارية. أما التطبيق الموسوم فيحتفظ بألوان علامته.
- **تمكين مجموعة نسخ وتعطيلها** من محررها وكذلك من قائمتها، مع تحذير عند الحفظ إذا كانت المجموعة لن تعمل كما حُفظت (معطلة، أو لا جدول مفعّلًا، أو مشكلة في اللقطة، أو قرص مفقود).
- **أداة سطر أوامر واحدة، `salvanza-cli`**، بالأوامر الفرعية `backup-folder` و`backup-disk` و`attach-nbd` بدلًا من الأدوات الثلاث المنفصلة؛ ولا تزال أسماؤها السابقة تعمل.
- **حزم Linux** (`.deb` و`.rpm` وArch)، باسم الحزمة `salvanza` والخدمة `salvanza.service`، مع الخدمة الخلفية.
- **قرص ISO موجَّه للاستعادة على جهاز فارغ**، يستعيد أيضًا داخل آلة افتراضية فارغة (P2V). والجهاز الذي نُسخ كآلة افتراضية لـ Proxmox VE يمكن أيضًا استعادته مباشرة من PBS في Proxmox VE، مثل أي آلة افتراضية نسخها Proxmox VE بنفسه.
- **إصدارات موسومة بعلامات تجارية**: تُوزَّع الملفات التنفيذية نفسها كمثبتات Salvanza وNimbus Backup وEtitech Backup، ولكل منها أسماء برامجها ومجلدات إعداداتها الخاصة.
- **18 لغة للواجهة.**

## 🔁 الترقية من Proxmox Backup Client

Salvanza هو الاسم الجديد لـ Proxmox Backup Client، والترقية تلقائية. يؤدي تثبيت Salvanza (مثبت MSI، أو حزمة Linux) إلى إزالة تثبيت Proxmox Backup Client السابق، أو تثبيت علامة تجارية أخرى، مع خدمته الخلفية. وعند تشغيل Salvanza أول مرة، ينقل الإعدادات السابقة (خوادم PBS، والمفاتيح، و**Backup Sets** (مجموعات النسخ الاحتياطي)، والسجل) من `C:\ProgramData\ProxmoxBackupClient` أو `%APPDATA%\ProxmoxBackupClient` أو `~/.proxmox-backup-guardian` أو `/var/lib/proxmoxbackupclient`، ويحتفظ بنسخة مؤرخة من ملفات الإعدادات في المجلد الجديد، تحت `pre-salvanza-<date>`. لا تُمسّ نسخك الاحتياطية على PBS.

| قبل | الآن |
|---|---|
| `ProxmoxBackupClient.exe`، `ProxmoxBackupClientSVC.exe` | `salvanza-gui.exe`، `salvanza-svc.exe` (خدمة Windows باسم `salvanza-svc`) |
| `proxmoxbackup-directory`، `proxmoxbackup-machine`، `proxmoxbackup-nbd` | `salvanza-cli backup-folder`، `salvanza-cli backup-disk`، `salvanza-cli attach-nbd` (لا تزال الأسماء السابقة تعمل) |
| `C:\Program Files\ProxmoxBackupClient`، `C:\ProgramData\ProxmoxBackupClient` | `C:\Program Files\Salvanza`، `C:\ProgramData\Salvanza` |
| حزمة Linux وخدمتها `proxmoxbackupclient` | الحزمة `salvanza`، والخدمة `salvanza.service`، والإعدادات في `/var/lib/salvanza`، وإعداداتك الخاصة في `~/.salvanza` |

## ⭐ ما يفعله، مع نتائج مقيسة

| | النتيجة |
|---|---|
| **نسخ احتياطي تزايدي للمجلدات** | مشاركة ملفات بحجم 480.9 GB تضم 761,866 ملفًا: التشغيل الأول 4 ساعات و17 دقيقة، وكل تشغيل لاحق من 10 إلى 15 دقيقة. الملفات غير المتغيرة لا تُفتح أصلًا (اكتشاف التغييرات بوضع Metadata، كما في العميل الرسمي). |
| **نسخ احتياطي سريع للأجهزة** | المساحة الحرة وملف ترحيل الصفحات وملف الإسبات لا تُقرأ، ومساحة القرص الفارغة لا تُحسب لها بصمة (hash). قرص حاسوب محمول بسعة 1 TB مستخدم منه 373 GB انخفض زمن نسخه من ساعة و23 دقيقة إلى ساعة و44 دقيقة، إلى 38 دقيقة فقط. |
| **الروابط البطيئة والمحسوبة بالاستهلاك** | تُرسل البيانات الجديدة فقط، مضغوطة، ويعرض التطبيق ما عبر الشبكة فعلًا. حاسوب Linux صغير في موقع بعيد على اتصال 4G (4.8 Mbit/s للرفع): صورة قرصه كاملة بحجم 128 GB مع مجلد المنزل في 11 دقيقة، بنقل نحو 172 MiB. |
| **استعادة الملفات من نسخ الأجهزة** | محرك C: بحجم 127 GB في حاسوب محمول يضم 430,000 ملف يُعرض محتواه في 4 إلى 15 ثانية. لا يُركَّب أي شيء ولا يُنزَّل مسبقًا. |
| **الاستعادة على جهاز فارغ** | أقلع من قرص ISO (ذاكرة USB أو قرص مضغوط أو عبر الشبكة)، وأدخل بيانات PBS، واختر النسخة: عاد جهاز Windows بحجم 32 GB إلى شاشة تسجيل الدخول بعد 8 دقائق من التشغيل. |
| **Undelete** (استرجاع المحذوفات) | بعد حذف 39 GB من مشاركة تضم 761,000 ملف، وجد بحث Undelete في نسخ آخر 7 أيام تلك الملفات بالضبط في نحو دقيقة، وأعادها إلى مكانها. |
| **دقة الاستعادة** | أذونات NTFS، والسمات، وتدفقات البيانات البديلة، والسمات الموسعة، وأوقات الإنشاء وأوقات المجلدات على Windows؛ والأوضاع، والمالكون، وقوائم ACL، والسمات الموسعة، والروابط الرمزية على Linux. |

## 📖 أدلة المستخدم

| الدليل | يغطي |
|---|---|
| **[دليل الواجهة الرسومية](docs/manual/GUI.md)** | التثبيت، وخوادم PBS، و**Backup Sets** (مجموعات النسخ الاحتياطي) والجداول الزمنية، والنسخ التزايدي، والتقدم، والاستعادة، و**Undelete** (استرجاع المحذوفات)، و**Roll back** (التراجع)، و**Reports** (التقارير)، والسجلات، و**Preferences** (التفضيلات)، والخدمة الخلفية، واستكشاف الأخطاء وإصلاحها |
| **[دليل سطر الأوامر](docs/manual/CLI.md)** | `salvanza-cli backup-folder` و`backup-disk` و`attach-nbd`: كل خيار، وملفات الإعداد، ورموز الخروج، والجدولة |
| **[دليل الاستعادة على جهاز فارغ](docs/manual/BMR.md)** | قرص ISO للاستعادة خطوة بخطوة؛ ونسخ `vm` على Proxmox VE؛ والتحويل من جهاز فعلي إلى افتراضي؛ والعتاد المختلف |
| [سيناريوهات الاستعادة](docs/RESTORE_GUIDE.md) | أي نوع من الاستعادة يُستخدم ومتى |
| [فهرس الأدلة](docs/manual/README.md) | نظرة عامة على الأدلة |

الأدلة نفسها مدمجة في التطبيق (**Help** (المساعدة)، F1)، فهي تعمل دون اتصال بالإنترنت وتطابق الإصدار المثبت.

<a id="-download"></a>

## 📦 التنزيل

👉 **[نزّل أحدث إصدار](https://github.com/salvanza/salvanza_client/releases)**

| لديك | استخدم |
|---|---|
| Windows | **مثبت MSI**، `Salvanza-<version>-x64.msi`. يثبّت التطبيق والخدمة الخلفية و`salvanza-cli` في `C:\Program Files\Salvanza`، ويضيف ذلك المجلد إلى PATH، ويشغّل الخدمة، ويضيف عنصرًا إلى قائمة Start. المثبتات الموسومة (`NimbusBackup-<version>-x64.msi`، `EtitechBackup-<version>-x64.msi`) هي الإصدار نفسه تحت علامة تجارية أخرى |
| Debian، Ubuntu، Mint | `sudo apt install ./salvanza_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./salvanza-<version>-*.rpm` |
| Arch | الحزمة المبنية من [packaging/arch/PKGBUILD](packaging/arch/PKGBUILD) (`make pkgarch`) |
| سطر الأوامر فقط | أرشيفات `salvanza-cli` الخاصة بكل منصة |
| جهاز تريد استعادته | قرص ISO للاستعادة على جهاز فارغ ([دليل الاستعادة على جهاز فارغ](docs/manual/BMR.md#3-get-the-iso-and-make-the-boot-media))، أو ابنه بنفسك ([أدناه](#-bare-metal-restore-iso)) |

عندما لا يتضمن إصدار ما حزمة Linux، ابنها باستخدام `make deb` أو `make pkgfedora` أو `make pkgarch` ([BUILD.md](BUILD.md)، [packaging/debian/README.md](packaging/debian/README.md)).

تثبّت حزم Linux عنصر قائمة واحدًا، **Salvanza for Proxmox Backup Server**، يشغّل التطبيق باسم المستخدم الحالي. التثبيت الجديد يفعّل الخدمة الخلفية (`salvanza.service`)؛ والترقية تحافظ على الخيار الذي اتخذته. تحل الحزمة `salvanza` محل الحزمتين السابقتين `proxmoxbackupclient` و`pbsgo`. تحتاج الحزم إلى polkit (pkexec)، الذي يستخدمه التطبيق للاتصال بالخدمة.

> ⚠️ **هل يُظهر Windows رسالة "virus detected" (مثل `Trojan:Win32/Sabsik.FL.A!ml`) أو تحذير SmartScreen؟**
> هذه **نتيجة إيجابية كاذبة معروفة** لتطبيقات Go/Wails: إنها *ليست* فيروسًا. تشير اللاحقة `!ml` إلى اكتشاف بنموذج تعلّم آلي يضع علامة على الملفات التنفيذية *غير الموقّعة وغير الشائعة*. تحقق من الملف المنزَّل كما هو موضح أدناه.

### 🔎 التحقق من أي ملف منزَّل

يوفر كل إصدار مجاميع اختبارية SHA-256 (`SHA256SUMS.txt`، وتُطبع أيضًا في صفحة الإصدار) و**شهادة مصدر موقّعة** (provenance attestation): إثبات تشفيري بأن الملف أنتجه نظام CI الخاص بهذا المستودع، من إيداع (commit) محدد.

```powershell
Get-FileHash .\Salvanza-<version>-x64.msi -Algorithm SHA256   # compare with SHA256SUMS.txt
gh attestation verify .\Salvanza-<version>-x64.msi --repo salvanza/salvanza_client
```

على Linux: `sha256sum -c SHA256SUMS.txt --ignore-missing`.

عندما تجتاز مثبتات إصدار ما فحص VirusTotal دون اكتشافات، تتضمن صفحة الإصدار روابط إلى التقارير.

> ℹ️ **توقيع الشيفرة:** ملفات Windows التنفيذية **ليست موقّعة بعد بتوقيع Authenticode** (شهادة للبرمجيات مفتوحة المصدر عبر [SignPath Foundation](https://signpath.org) قيد الانتظار). وإلى ذلك الحين، يُثبَت المصدر عبر الشهادة والمجاميع الاختبارية المذكورة أعلاه.

## 🚀 بدء سريع (الواجهة الرسومية)

1. ثبّته: MSI على Windows، أو حزمة `.deb` أو `.rpm` أو Arch على Linux ([التنزيل](#-download)).
2. شغّل **Salvanza for Proxmox Backup Server**. تُعدّ المثبتات الخدمة الخلفية، فيطلب التطبيق مرة واحدة صلاحيات المسؤول للاتصال بها (UAC على Windows، وكلمة مرورك عبر pkexec على Linux). إن لم تستطع منحها، فاختر **Use my own settings** (استخدام إعداداتي الخاصة).
3. أضف خادم PBS في **Preferences > Account Information** (التفضيلات > معلومات الحساب) واختبره بـ **Test** (اختبار).
4. أنشئ مجموعة نسخ احتياطي (أو نسخة لمرة واحدة) وحدد المجلدات أو الأقراص المراد نسخها.
5. شغّلها. مع الخدمة الخلفية، تعمل جداولها الزمنية سواء كان أحد مسجّلًا للدخول أم لا.

يشرح [دليل الواجهة الرسومية](docs/manual/GUI.md) كل خطوة.

### 🪟 تثبيت غير مراقَب / صامت (MSI)

يدعم MSI خيار التثبيت الصامت القياسي في Windows Installer، إضافة إلى خصائص عامة لكل قيمة افتراضية:

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

| الخاصية | الافتراضي | المعنى |
| --- | --- | --- |
| `SERVICEAUTOSTART` | `1` | `0` = تُثبَّت الخدمة بنوع بدء **Manual** (يدوي) وتُترك **متوقفة**: مثبتة، لكنها لا تُفعَّل ولا تبدأ تلقائيًا أبدًا |
| `AUTOSTART` | `1` | `0` = لا يبدأ التطبيق في علبة النظام عند تسجيل دخول كل مستخدم |
| `DESKTOPSHORTCUTS` | `1` | `0` = لا اختصار على سطح المكتب (يُضاف عنصر قائمة Start دائمًا) |
| `CONFIGFILE` | | المسار الكامل لملف محفوظ عبر **Preferences > Export Settings** (التفضيلات > تصدير الإعدادات): تُدمج خوادم PBS ومجموعات النسخ الاحتياطي التي فيه قبل بدء الخدمة. معرّفات النسخ المشتقة من اسم الحاسوب المصدِّر تُشتق من اسم كل حاسوب على حدة. راجع [installer/DEPLOYMENT.md](installer/DEPLOYMENT.md) |
| `KEEP_CONFIG` | `1` | عند إلغاء التثبيت فقط: `0` = يحذف أيضًا مجلد الحالة `%ProgramData%\Salvanza` (أو مجلد الإصدار الموسوم الخاص به، مثل `%ProgramData%\NimbusBackup`) |
| `INSTALLFOLDER` | `C:\Program Files\Salvanza` | مجلد التثبيت (قابل للتعديل أيضًا في واجهة المثبت) |

شغّله بصلاحيات مرتفعة (مسؤول)؛ رمز الخروج `0` = نجاح، و`3010` = نجاح مع الحاجة إلى إعادة التشغيل. تقبل مثبتات MSI الموسومة الخيارات نفسها تمامًا.

## ✨ الميزات

### الواجهة الرسومية
الميزات الرئيسية مدرجة تحت [الجديد في الإصدار 2.0](#-new-in-20). وإضافة إلى ذلك:
- 18 لغة للواجهة: الفرنسية والإنجليزية والإيطالية والألمانية والبولندية والإسبانية والبلغارية والتشيكية واليونانية والمجرية واللاتفية والليتوانية والهولندية والبرتغالية والرومانية والسلوفاكية والتركية والأوكرانية
- **نسخ احتياطية لمرة واحدة** للمجلدات أو للأقراص كاملة، ونسخ احتياطية للأجهزة تُخزَّن كـ `host` أو كنسخة `vm` لـ Proxmox VE، مع خيار **نسخة جنائية** (forensic copy) يقرأ المساحة الحرة أيضًا
- **الاستعادة** مع البحث عن الملفات (أحرف البدل، والتعابير النمطية، والمسارات الكاملة)، والاستعادة الانتقائية، وقوائم NTFS/POSIX ACL، وتدفقات البيانات البديلة، وأوقات الإنشاء في Windows، وجولة تحقق اختيارية
- **Pause** و**Resume** و**Stop** (إيقاف مؤقت، استئناف، إيقاف) لنسخة جارية، من علبة النظام أيضًا على Windows، وللعمليات التي تجريها الخدمة الخلفية
- بطاقة تقدم حية (الانتهاء المتوقع، والملفات المنجزة، وسرعة الرفع، والبيانات المنقولة، ونسبة الضغط)، و**Reports** (التقارير)، و**Message Log** (سجل الرسائل)، و**View Full Logs** (عرض السجلات الكاملة)
- إشعارات البريد الإلكتروني، وإجراءات ما بعد النسخ، واستنساخ مجموعات النسخ، وتصدير الإعدادات واستيرادها
- عدة خوادم PBS، مع تثبيت بصمة الشهادة (الثقة عند الاستخدام الأول)، وتسجيل الدخول برمز API أو باسم مستخدم وكلمة مرور
- خمس سمات ألوان ولون مخصص، يختارها كل مستخدم (في التطبيق غير الموسوم؛ أما التطبيق الموسوم فيحتفظ بألوان علامته)

### سطر الأوامر: `salvanza-cli`
- `salvanza-cli backup-folder`: نسخ احتياطي للمجلدات (pxar) مع إزالة التكرار و`-change-detection-mode legacy|data|metadata`
- `salvanza-cli backup-disk`: نسخ احتياطي حي للأقراص كاملة لأجهزة Windows (VSS) وLinux (elastio-snap/dattobd) (fidx، تزايدي، مع تخطي المساحة الحرة ما لم يُستخدم `-readfreespace`، و`-type vm` لنسخة Proxmox VE، و`-skippartition <disk>:<number>` لاستبعاد قسم)
- `salvanza-cli attach-nbd`: خادم NBD لاستعادة النسخ الاحتياطية للأقراص (Linux)
- يطبع `salvanza-cli version` الإصدار، ويسرد `salvanza-cli help` الأوامر الفرعية، ويسرد `salvanza-cli <command> -help` خيارات أمر فرعي
- برنامج الخدمة، `salvanza-svc` (`salvanza-svc.exe` على Windows؛ وعلى Linux التطبيق أيضًا، `salvanza-gui`): `-list-sets` و`-undelete` و`-rollback` لمجموعات النسخ المعرّفة في الواجهة الرسومية

تقبل الأوامر الفرعية الثلاثة `-keyfile` و`-keyfile-passphrase` للقطات المشفرة. ولا تزال الأسماء السابقة للأدوات، `proxmoxbackup-directory` و`proxmoxbackup-machine` و`proxmoxbackup-nbd` (و`pbsdirectorybackup` و`pbsmachinebackup` و`pbsnbd`)، تشغّل الأوامر الفرعية نفسها بالخيارات نفسها، فتستمر السكربتات الحالية في العمل. المرجع الكامل: [دليل سطر الأوامر](docs/manual/CLI.md).

### 📸 لقطات الشاشة

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Backup Sets** (مجموعات النسخ الاحتياطي): مهام مسماة مع شارات الجدول الزمني والنوع، وزر **Run Now** (تشغيل الآن) لكل منها

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**تقدم النسخ الاحتياطي**: الإنتاجية، والانتهاء المتوقع، والكتل (chunks) الجديدة والمعاد استخدامها

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Restore** (الاستعادة): اختر خادم PBS ومعرّف النسخة، واعرض اللقطات أو ابحث فيها باسم الملف

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-schedule-tab.png" width="280">](docs/screenshots/gui-schedule-tab.png)
**الجداول الزمنية**: عدة جداول لكل مجموعة، مع مواعيد التشغيل الثلاثة التالية

</td>
<td width="33%">

[<img src="docs/screenshots/backup-set-as-vm.png" width="280">](docs/screenshots/backup-set-as-vm.png)
**النسخ كآلة افتراضية لـ Proxmox VE**: يظهر معرّف النسخة في PBS أثناء الكتابة

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Reports** (التقارير): سجل كامل لعمليات التشغيل مع لوحة تفاصيل

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-machine-partitions.png" width="280">](docs/screenshots/gui-machine-partitions.png)
**مجموعات نسخ الأجهزة**: أشّر تحت كل قرص على الأقسام المراد نسخها

</td>
<td width="33%">

[<img src="docs/screenshots/gui-themes.png" width="280">](docs/screenshots/gui-themes.png)
**Themes** (السمات): ست سمات لونية، يختارها كل مستخدم لنفسه

</td>
<td width="33%">

[<img src="docs/screenshots/gui-languages.png" width="280">](docs/screenshots/gui-languages.png)
**اللغات**: 18 لغة، وهنا الفرنسية والألمانية والإيطالية والإسبانية والبولندية والأوكرانية

</td>
</tr>
</table>

### استثناءات النظام الذكية (نسخ المجلدات)
تستبعد نسخ المجلدات هذه العناصر تلقائيًا:

**مجلدات النظام:** `System Volume Information` (تخزين VSS، وقد يتجاوز 100 GB) و`$RECYCLE.BIN` أينما وُجدا؛ و`Recovery` في جذر محرك الأقراص فقط؛ و`.pbs-rollback` (مجلد الأمان الخاص بـ Roll back).
**ملفات النظام:** `pagefile.sys` و`hiberfil.sys` و`swapfile.sys` و`DumpStack.log.tmp`.

**لماذا يهم هذا:** قد يُظهر محرك الأقراص 1.03 TB مستخدمة بينما الملفات الحقيقية نحو 141 GB. دون هذه الاستثناءات ستتضمن النسخة لقطات VSS (هدر في المساحة والوقت)؛ ومعها، يطابق الحجم البيانات الحقيقية.

**التوصية:** استخدم نسخة **مجلد** للنسخ على مستوى الملفات؛ واستخدم نسخة **جهاز** في مجموعة نسخ منفصلة للاستعادة على جهاز فارغ ولـ P2V (صورة قرص كاملة، تدخلها المساحة الحرة وملفا ترحيل الصفحات والإسبات أصفارًا).

### الأمان والجودة
- مجلد إعدادات الخدمة مفتوح لـ SYSTEM وAdministrators فقط (Windows) أو لـ root فقط (Linux)؛ وتستمع واجهة API المحلية الخاصة بها على 127.0.0.1 وتحتاج إلى رمز يُستبدل عند كل بدء وكل 24 ساعة
- لا يرسل التطبيق شيئًا إلى تلك الواجهة حتى يتحقق من أن الخدمة نفسها هي التي تجيب (عملية الخدمة على Windows، ومقبس يملكه root على Linux)
- على Windows، تُشفَّر رموز PBS وكلمات المرور في `config.json` على مستوى الحاسوب (DPAPI)
- التحقق من المدخلات، وتنقية بيانات الاعتماد، ومنع اجتياز المسارات، وإعادة المحاولة مع تراجع أسّي
- اختبارات وحدات في كل وحدة برمجية، إضافة إلى اختبارات حية اختيارية واختبارات شاملة من طرف إلى طرف على خادم PBS حقيقي

## 🔐 التشفير من جهة العميل

يمكن تشفير اللقطات قبل أن تغادر الجهاز، باستخدام ملف مفتاح لـ Proxmox Backup Server (بالصيغة التي يكتبها `proxmox-backup-client key create`، لذا فالمفاتيح قابلة للتبادل مع العميل الرسمي). تُشفَّر كل كتلة بـ AES-256-GCM ويُوقَّع البيان (manifest). لا يرى خادم PBS المفتاح أبدًا.

**الواجهة الرسومية:** افتح **Preferences > Account Information** (التفضيلات > معلومات الحساب)، وحرّر خادمًا واستخدم تبويب **Encryption** (التشفير) الخاص به. استورد بـ **Import...** (استيراد) مفتاحًا لديك (نسخة من ملف المفتاح، أو مفتاحًا ورقيًا يُقرأ بالكاميرا أو من مسح ضوئي أو صورة، أو يُكتب يدويًا) أو أنشئ مفتاحًا جديدًا بـ **Generate...** (إنشاء). يعرض التبويب بصمة المفتاح. يتيح **Save a copy...** (حفظ نسخة) و**Print a paper key...** (طباعة مفتاح ورقي) (برمز QR) الاحتفاظ بنسخة بعيدًا عن الحاسوب. تُظهر التقارير ما إذا كانت كل نسخة مشفرة. **دون ملف المفتاح لا يمكن استعادة اللقطة: احتفظ بنسخة في مكان آمن، بعيدًا عن هذا الجهاز.**

**المفاتيح المحمية بعبارة مرور (scrypt أو PBKDF2):** يحتفظ **Ask me** (اسألني) بعبارة المرور في الذاكرة طوال جلسة واحدة من التطبيق؛ ويخزّنها **Remember on this computer** (التذكر على هذا الحاسوب) مثل رمز PBS، وهو الخيار المناسب للجداول الزمنية وللخدمة الخلفية.

**سطر الأوامر:**
```shell
salvanza-cli backup-folder -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
لا يلزم `-keyfile-passphrase` (أو `PBS_ENCRYPTION_PASSWORD`) إلا لمفتاح محمي، ويُطلب تفاعليًا عند إغفاله. يقبل `salvanza-cli backup-disk` و`salvanza-cli attach-nbd` الخيارات نفسها. يتحقق `attach-nbd` من المفتاح مقابل اللقطة قبل لمس جهاز NBD، لذا يفشل المفتاح الخاطئ فورًا.

**الاستعادة على جهاز فارغ:** يطلب قرص ISO للاستعادة المفتاح (لا شيء، أو ملف مفتاح على ذاكرة USB، أو مسارًا مكتوبًا) وعبارة المرور لمفتاح محمي. لا يُكتب المفتاح أبدًا على قرص ISO.

تتطلب استعادة لقطة مشفرة ملف المفتاح نفسه؛ ولا سبيل إلى الاسترداد إذا فُقد.

## نسخ احتياطي أسرع للمجلدات: اكتشاف التغييرات

في وضع Legacy الافتراضي، تقرأ كل نسخة مجلد كل ملف، فيستغرق التشغيل زمن قراءة المجلد كله حتى عندما لا يتغير شيء. يمكن لمجموعات النسخ الاحتياطي ولـ `salvanza-cli backup-folder` أن تستخدم بدلًا من ذلك أوضاع اكتشاف التغييرات الخاصة بـ `proxmox-backup-client` الرسمي:

| الوضع | يقرأ | يُخزَّن كـ |
|---|---|---|
| **Legacy** (الافتراضي) | كل ملف | ملف `<name>.pxar.didx` واحد لكل مجلد |
| **Data** | كل ملف | أرشيف مقسوم: `<name>.mpxar.didx` (البيانات الوصفية) + `<name>.ppxar.didx` (محتوى الملفات) |
| **Metadata** | الملفات الجديدة والمتغيرة فقط | الأرشيف المقسوم نفسه؛ الملفات التي يطابق حجمها ووقت تعديلها (حتى النانوثانية) ووضعها ومالكها النسخة السابقة تؤخذ منها دون قراءتها |

في وضع **Metadata** يقرأ التشغيل الأول كل شيء؛ والتشغيلات اللاحقة تقرأ ما تغيّر فقط. الملف الذي يتغير بطريقة تحافظ على حجمه ووقت تعديله لا يُلاحَظ، ولهذا يمكن للمجموعة أيضًا أن **تقرأ كل ملف كل N تشغيلات**. امنح كل مجموعة نسخ احتياطي معرّف نسخ (Backup ID) خاصًا بها: فوضع Metadata يقارن بأحدث لقطة في مجموعة النسخ (backup group). في سطر الأوامر استخدم `-change-detection-mode`.

**قبل تحويل مجموعة إلى Data أو Metadata،** حدّث إلى الإصدار 2.0 كل جهاز تستعيد منه: فالإصدارات السابقة تعرض مجلدات الأرشيف المقسوم دون ملفاتها. راجع [القسم 7 من دليل الواجهة الرسومية](docs/manual/GUI.md#7-incremental-folder-backups-change-detection).

<a id="-physical-machines-into-virtual-machines-p2v"></a>

## 🔄 الأجهزة الفعلية إلى آلات افتراضية (P2V)

يمكن أن تصبح نسخة احتياطية لحاسوب أو خادم فعلي آلةً افتراضية في Proxmox VE، بطريقتين:

1. **استعدها في Proxmox VE.** اضبط مجموعة النسخ (أو النسخة لمرة واحدة) على **Backup as: Proxmox VE virtual machine** (النسخ كـ: آلة افتراضية لـ Proxmox VE). تُخزَّن النسخة كـ `vm/<number>`، فيُدرجها Proxmox VE مع نسخ الآلات الافتراضية الأخرى ويستعيدها كآلة افتراضية (قرص SATA واحد؛ أضف بطاقة شبكة قبل تشغيلها).
2. **استعدها بقرص ISO للاستعادة داخل آلة افتراضية فارغة** لها البرنامج الثابت (firmware) نفسه الذي للجهاز الأصلي، وقرص SATA لا يقل عنه حجمًا، وبطاقة شبكة Intel E1000. يصلح هذا لنسخ `host` و`vm` على حد سواء، وتبقى النسخة نفسها قابلة للاستعادة على جهاز فعلي فارغ.

تشمل المسارات المختبرة Windows 10 وWindows Server (UEFI/GPT)، وأجهزة Linux المكتبية (UEFI/GPT وBIOS/MBR)، ونسخة Linux مشفرة مع مفتاحها على ذاكرة USB. يتضمن [دليل الاستعادة على جهاز فارغ](docs/manual/BMR.md#7-restoring-onto-different-hardware-p2v-and-replacement-machines) الجدول الكامل وخيارات البرنامج الثابت وبرامج التشغيل والتعليمات خطوة بخطوة.

## 🤝 متوافق مع عميل Proxmox الرسمي في الاتجاهين

جرى التحقق مع `proxmox-backup-client` 3.4.9 وPBS 4.2:

- **لقطات Legacy وData وMetadata التي يصنعها هذا العميل تُستعاد بالعميل الرسمي مطابقةً بايتًا ببايت**، سواء صُنعت من الواجهة الرسومية أو من سطر الأوامر، على Windows أو Linux، ويفتحها مستعرض الملفات في PBS 4.2.
- **لقطات Legacy وData وMetadata التي يصنعها العميل الرسمي تُستعاد بهذا العميل مطابقةً بايتًا ببايت**، كليًا أو انتقائيًا.
- صيغة الأرشيف المقسوم نفسها (pxar الإصدار 2)، وأوضاع اكتشاف التغييرات نفسها، وصيغة ملف مفتاح التشفير نفسها، ومتغيرات البيئة نفسها على نمط `PBS_REPOSITORY`.

نسخك الاحتياطية ليست مقيدة بهذا العميل أبدًا. ترافق الأرشيفات القياسية إضافتان، ويستعيد العميل الرسمي الملفات كالمعتاد من دونهما:

- يسجّل `.proxmox_backup_client_meta.json` في جذر لقطة المجلد المسار الأصلي، ويستعيده العميل الرسمي كملف عادي.
- يسجّل `proxmox-client-acls.json.gz.blob`، المحفوظ مع كل لقطة مجلد، ما لا يتسع له الأرشيف على Windows (أذونات NTFS والسمات وأوقات الإنشاء وتدفقات البيانات البديلة والسمات الموسعة)، وعلى Linux السمات الموسعة وقوائم POSIX ACL. يتجاهله العميل الرسمي وواجهة PBS على الويب، بينما تعيدها الاستعادة في هذا العميل إلى مكانها.

على Linux يحفظ العميل الرسمي قوائم POSIX ACL والسمات الموسعة وقدرات الملفات (capabilities) داخل الأرشيف نفسه. لا يكتب هذا العميل تلك الإدخالات ولا يقرؤها بعد، لذلك لا تنتقل قوائم ACL على Linux بين العميلين في أي اتجاه. القيام بذلك على الطريقة الرسمية، كخيار، مدرج في [خارطة الطريق](TODO.md).

## نسخ احتياطي حي للجهاز كاملًا

ينسخ `salvanza-cli backup-disk` (ونسخ الأجهزة في الواجهة الرسومية) نظام Windows 10/11 أو Windows Server كاملًا، أو جهاز Linux، دون أي توقف عن العمل.

صيغة الأمر مماثلة في معظمها لنسخ المجلدات، إلا أن `-backupdir` يُستبدل بـ `-backupdev` (كرّره لعدة أقراص). على سبيل المثال:

`salvanza-cli backup-disk -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

يفحص الأمر أعلاه القرص 0، ويكتشف جميع الأقسام المركّبة، ويأخذ لها لقطة VSS، ثم ينشئ صورة نسخ احتياطي قابلة للإقلاع للقرص كله بصيغة fidx. تدخل المساحة الحرة وملفا ترحيل الصفحات والإسبات الصورةَ أصفارًا دون قراءتها (وعلى Linux، المساحة الحرة في ext2/3/4 وXFS أيضًا)؛ أما `-readfreespace` فيقرأ كل شيء بدلًا من ذلك، للحصول على نسخة جنائية. القطاع الذي تتعذر قراءته يدخل الصورة أصفارًا، ويذكر التشغيل موضعه. أضف `-type vm` (مع `-backup-id` رقمي) لتخزين النسخة كنسخة آلة افتراضية لـ Proxmox VE ([P2V](#-physical-machines-into-virtual-machines-p2v)).

النسخة التالية تزايدية. يجري حساب البصمات (hashing) بالتوازي، لذا يمكن بلوغ سرعات تصل إلى 1 GB/s.

لاستبعاد قسم، مثل مكتبة ألعاب أو قسم بيانات تغطيه أصلًا نسخة مجلد، أضف `-skippartition <disk>:<number>` (كرّره لعدة أقسام)، مثل `-skippartition \\.\PhysicalDrive0:4` أو `-skippartition /dev/sda:3`. لا يُقرأ القسم ولا تؤخذ له لقطة، ويدخل الصورة أصفارًا: يُحتفظ بجدول الأقسام، فتعيد الاستعادة القسم في مكانه وبحجمه، فارغًا وغير مهيأ. في الواجهة الرسومية، أزل التأشير عن القسم تحت قرصه في مجموعة النسخ الاحتياطي.

### متطلبات نسخ أجهزة Linux (ما يعادل VSS)

يستخدم نسخ أجهزة Windows تقنية VSS. لا يوجد VSS في Linux، لذا يأخذ نسخ جهاز Linux قيد التشغيل لقطة في لحظة زمنية محددة لكل قسم مركّب باستخدام وحدة نواة: `elastio-snap` (التفرّع المصان، [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap)) أو `dattobd`.

يلزم أمران، وإلا رفض نسخ جهاز Linux العمل:

1. **يجب أن تكون وحدة النواة مثبتة ومحمّلة.** ثبّتها عبر DKMS لكي يُعاد بناؤها بعد كل تحديث للنواة، واجعلها تُحمَّل عند كل إقلاع. يحتوي قسم [لقطات Linux: تثبيت elastio-snap أو dattobd](docs/manual/GUI.md#linux-snapshots-installing-elastio-snap-or-dattobd) في دليل الواجهة الرسومية على خطوات مُختبرة لـ Debian وUbuntu وMint وFedora، إضافة إلى Secure Boot وطرق التحقق. في نوفمبر 2026 لا تُبنى أيّ من الوحدتين على النواة 7 (Fedora 44 بعد تحديثاته).

2. **يجب أن يعمل النسخ بصلاحيات root.** واجهة التحكم في الوحدة مقصورة على root، وتُرفض اللقطة لمستخدم عادي. تعمل الخدمة الخلفية بصلاحيات root؛ ودونها، استخدم **Restart as administrator** (إعادة التشغيل كمسؤول) في التطبيق، أو شغّل `salvanza-cli backup-disk` مع `sudo`. لا يحتاج نسخ المجلدات العادي إلى root.

في الواجهة الرسومية، يمكن لمجموعة نسخ جهاز Linux إزالة التأشير عن **Use snapshots** (استخدام اللقطات) وقراءة أقراصها وهي تعمل، ولكن فقط مع التأشير على المربع الثاني ("Run without a snapshot: I accept the backup may be inconsistent"، أي: التشغيل دون لقطة، وأقبل أن تكون النسخة غير متسقة). ودونه، يتوقف كل تشغيل ويذكر السبب. الأقسام غير المركّبة تُقرأ مباشرة.

### استعادة الملفات من نسخة قرص احتياطية

تفتح صفحة **Restore** (الاستعادة) في الواجهة الرسومية نسخة الجهاز كما تفتح نسخة المجلد: كل وحدة تخزين في الصورة (NTFS أو FAT أو exFAT أو ext2/3/4 أو XFS) مجلد يمكنك تصفحه والاستعادة منه. لا يُركَّب أي شيء.

على Linux يمكنك أيضًا ربط نسخة قرص احتياطية كجهاز كتل باستخدام `salvanza-cli attach-nbd`. شغّل أولًا `modprobe nbd max_part=0`: فمع `max_part != 0` قد تدخل النواة في حلقة لا نهائية أثناء فحص الأقسام.

يتصل `attach-nbd` بأي نسخة احتياطية لقرص ثابت، سواء كانت نسخة VM أو host (لذا تعمل أيضًا مع نسخ Proxmox VE). على سبيل المثال:

`sudo salvanza-cli attach-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

إذا أغفلت `-path`، تتيح لك واجهة نصية في الطرفية اختيار ملف fidx؛ أما `-list` فيطبع كل الصور بدلًا من ذلك.

> لا تستخدم هذا على جهاز يشغّل أي شيء مهم: فنظام ملفات تالف قد يتسبب في انهيار نظام التشغيل، ولهذا يستخدم Proxmox VE مثيل QEMU لهذا الغرض.

ألغِ تركيب كل شيء على قرص nbd قبل إيقاف `salvanza-cli attach-nbd`، وإلا فمن المرجح أن يبقى القسم مشغولًا. إذا ظهر لك الخطأ `Device or resource busy`، فافرض قطع الاتصال باستخدام `nbd-client -d /dev/nbd0`، أو أعد التشغيل.

### الاستعادة إلى جهاز فعلي أو آلة افتراضية

استخدم [قرص ISO للاستعادة على جهاز فارغ](#-bare-metal-restore-iso)، على جهاز فعلي أو داخل آلة افتراضية فارغة. يشرح [دليل الاستعادة على جهاز فارغ](docs/manual/BMR.md) ذلك خطوة بخطوة.

## الاستخدام: نسخ المجلدات احتياطيًا

المرجع الكامل، مع أمثلة لـ Windows وLinux ورموز الخروج والجدولة، موجود في **[دليل سطر الأوامر](docs/manual/CLI.md)**. أمر نموذجي:

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

تُقرأ أيضًا متغيرات البيئة `PBS_REPOSITORY` و`PBS_SERVER` و`PBS_PORT` و`PBS_DATASTORE` و`PBS_AUTH_ID` و`PBS_PASSWORD` و`PBS_FINGERPRINT` الخاصة بالعميل الرسمي.

لإعداد JSON راجع [config.json.example](config.json.example)؛ واملأ الحقول التي تحتاجها فقط.

### عدة مجلدات في تشغيل واحد

كرّر `-backupdir` لنسخ أكثر من مجلد بأمر واحد:

```shell
salvanza-cli backup-folder -baseurl "https://yourpbshost:8007" -certfingerprint pbsfingerprint -authid "user@realm!apiid" -secret "apisecret" -datastore "datastorename" -backupdir "C:\Data" -backupdir "D:\Photos"
```

يُنسخ كل مجلد كمجموعة نسخ (backup group) مستقلة، بمعرّف النسخة `<backup-id>_<path>` (القيمة الافتراضية لـ `-backup-id` هي اسم المضيف). لذلك تتعامل سياسات الاحتفاظ والتقليم (pruning) والاستعادة مع كل مجلد كسلسلة مستقلة. المجلد الذي يفشل لا يوقف المجلدات الأخرى، ويُبلغ رمز الخروج عن الفشل. لا يمكن الجمع بين `-pxarout` وعدة مجلدات، ويُرفض مجلدان سينتهي بهما الأمر إلى معرّف النسخة نفسه قبل رفع أي شيء.

في ملف إعداد JSON استخدم `"backupdirs": ["C:\\Data", "D:\\Photos"]`. يستمر الحقل المفرد `"backupdir"` في العمل ويمكن جمعه مع `"backupdirs"`. يعمل التشفير كما في المجلد الواحد: يُقرأ المفتاح مرة واحدة، وتُشفَّر به مجموعة النسخ الخاصة بكل مجلد.

### استبعاد الأوقات من الأرشيف

وقت تعديل كل ملف ومجلد جزء من الأرشيف، لذا فإن تغيّر الوقت يغيّر بايتات الأرشيف، وتُرفع الكتل المحيطة بذلك الإدخال مرة أخرى مع أن شيئًا فيها لم يتغير. وهذه المقايضة صحيحة لمعظم البيانات. غير أن بعض الأشجار تظل تلمس مجلداتها بينما تبقى الملفات كما هي: فالمخزن الذي يحفظ بياناته تحت مجلدات مسماة بالبصمات (`aa/bb/<rest of the hash>`) قد ينتهي به الأمر إلى رفع جزء كبير من الأرشيف مجددًا في كل تشغيل لهذا السبب وحده.

يسجّل `-no-mtime` (`"no-mtime": true` في ملف إعداد JSON) التاريخ 1970-01-01 وقتًا لكل ملف ومجلد بدلًا من ذلك، فلا تكلّف كتلًا جديدة إلا التغييرات الحقيقية في المحتوى. لا تُحفظ الأوقات الحقيقية في أي مكان: فالاستعادة تؤرّخ كل عنصر بـ 1970-01-01. ومع `-change-detection-mode metadata` يُقرأ كل ملف، كما في وضع `data`. الخيار معطّل افتراضيًا؛ استخدمه لمصدر لا تهم فيه الأوقات.

### قوالب البريد

يعمل مُرحِّل البريد الذي لا يحتاج إلى تسجيل دخول (غالبًا على المنفذ 25): اترك اسم المستخدم وكلمة المرور فارغين. ويمكن أن يتضمن المرسل اسمًا (`Backups <backup@example.com>`).

يُستخدم [محرك القوالب في Go](https://pkg.go.dev/text/template) لعناوين الرسائل ونصوصها؛ راجع وثائقه لمعرفة الصيغة. المتغيرات المتاحة:

- `.NewChunks`: عدد الكتل الجديدة المنشأة
- `.ReusedChunks`: عدد الكتل المعاد استخدامها
- `.Datastore`: اسم مخزن البيانات (datastore)
- `.Error`: الخطأ، إن وُجد (يعطيه `.ErrorStr` كنص، ويكون فارغًا عند عدم وجوده)
- `.ReadErrors`: الملفات أو المجلدات التي تعذرت قراءتها؛ و`.ReadErrorCount` عددها
- `.Hostname`: اسم مضيف الجهاز
- `.StartTime`: وقت بدء النسخ
- `.EndTime`: وقت انتهاء النسخ
- `.Duration`: مدة النسخ
- `.FromattedDuration`: مدة النسخ منسّقة (الاسم مكتوب بهذا الشكل في الشيفرة)
- `.Success`: true عندما ينتهي النسخ دون أخطاء ودون أي شيء غير مقروء
- `.Partial`: true عندما تُحفظ لقطة لكن تتعذر قراءة بعض الملفات
- `.Status`: `Success` أو `Partial` أو `Failed`

## نسخ التدفق احتياطيًا

ينسخ هذا تدفقًا (stream) بدلًا من أرشيف pxar. على سبيل المثال:

```
mysqldump yourdatabase | salvanza-cli backup-folder -backupstream yourdatabase.sql [other options]
```

يستخدم هذا تقطيع buzhash لإزالة التكرار في التدفق نفسه، مثل أرشيف tar أو تفريغ SQL.

## المشكلات المعروفة

قد يؤدي تفعيل مكافحة البرمجيات الخبيثة في Windows Defender إلى إبطاء النسخ إلى نحو 25% من السرعة الممكنة.

على Windows يأخذ `salvanza-cli backup-folder` و`backup-disk` قفل عملية: النسخة الثانية التي تبدأ أثناء تشغيل أخرى تطبع "Another backup job is running" وتخرج بالرمز 2. على Linux لا يوجد مثل هذا القفل، لذا لا تجدول تشغيلين لسطر الأوامر في الوقت نفسه. (تشغّل الواجهة الرسومية والخدمة الخلفية عدة نسخ في آن واحد بأمان، راجع [عدة نسخ احتياطية في آن واحد](docs/manual/GUI.md#67-several-backups-at-once).)

## 📋 المتطلبات المسبقة

- Windows 10/11 أو Windows Server (64-bit)؛ وصلاحيات المسؤول للتثبيت، ولربط التطبيق بالخدمة الخلفية
- أو Linux: الحزم مخصصة لـ Debian وUbuntu وMint وFedora وArch؛ وتحتاج الواجهة الرسومية إلى بيئة تشغيل WebKitGTK وإلى polkit؛ ويحتاج النسخ المتسق للأجهزة إلى وحدة النواة elastio-snap أو dattobd وصلاحيات root
- وصول شبكي إلى Proxmox Backup Server
- لـ P2V: Proxmox VE مع إضافة مخزن بيانات PBS كوحدة تخزين، أو أي برنامج مراقبة أجهزة افتراضية (hypervisor) يستطيع الإقلاع من قرص ISO للاستعادة

## 🔨 البناء من المصدر

يتضمن **[BUILD.md](BUILD.md)** التعليمات الكاملة: الواجهة الرسومية والخدمة الخلفية، وأدوات سطر الأوامر، وMSI، وحزم Linux. باختصار، مع Go 1.25 أو أحدث وWails CLI v2.13:

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

الواجهة الأمامية المبنية (`gui/frontend/dist`) مودعة في المستودع، لذا لا حاجة إلى Node.js إلا لتعديل الواجهة الأمامية (`npm install` و`npm run build` في `gui/frontend`، أو `wails dev` لإعادة التحميل الفوري). الاختبارات: `make test`.

## 🔧 الاستخدام المتقدم والأدلة

### Multi-PBS (عدة خوادم PBS)

اضبط عدة خوادم PBS واختر الهدف لكل مجموعة نسخ أو نسخة لمرة واحدة (مثلًا `C:\Users` إلى خادم PBS سريع على SSD يوميًا، و`D:\` إلى خادم PBS للبيانات الضخمة أسبوعيًا، إضافة إلى خادم للتعافي من الكوارث).

- **[دليل المستخدم](MULTI_PBS_USER_GUIDE.md)**: إضافة الخوادم واختبارها، والخادم الافتراضي، واختيار خادم لكل نسخة، والأسئلة الشائعة واستكشاف الأخطاء وإصلاحها.
- **[دليل التنفيذ](MULTI_PBS_GUIDE.md)**: نموذج البيانات، والترحيل التلقائي من إعداد بخادم PBS واحد، ودوال الخلفية (backend).

يُرحَّل إعداد الخادم الواحد القديم تلقائيًا إلى خادم باسم `default` عند أول تحميل.

<a id="-bare-metal-restore-iso"></a>

### 💿 قرص ISO للاستعادة على جهاز فارغ

قرص ISO للاستعادة، Salvanza-BMR، هو قرص Clonezilla Live ISO قياسي، مرقّع بالملف التنفيذي `pbsnbd` (شيفرة `salvanza-cli attach-nbd`، مبنية وحدها من `nbd/cmd/pbsnbd`) إضافة إلى عنصرين في القائمة. يُقلع من قرص مضغوط، أو من ذاكرة USB (مكتوبة بـ `dd`، أو من ذاكرة Ventoy)، وعلى أجهزة UEFI أو BIOS:

- **`ocs-pbs-bare-metal-restore`**: العنصر الأول والافتراضي في قائمة الإقلاع، **PBS Bare Metal Restore**. يتخطى مطالبات اللغة ولوحة المفاتيح في Clonezilla وينتقل مباشرة من بيانات اعتماد PBS إلى استعادة مكتملة: DHCP أولًا (ومطالبة بعنوان IP ثابت فقط إذا فشل)، ومفتاح تشفير من ذاكرة USB إذا كانت النسخة مشفرة، واكتشاف للقرص الهدف لا يعرض أبدًا الذاكرة التي أقلع منها. يستعيد نسخ Windows وLinux على حد سواء (فالاستعادة على مستوى الكتل)، على جهاز فارغ أو داخل آلة افتراضية فارغة.
- **`ocs-pbs-nbd`**: عنصر **pbs-nbd** في قائمة اختيار الوضع الخاصة بـ Clonezilla. يطلب بيانات الاتصال بـ PBS واللقطة، ويربطها بـ `/dev/nbd0`، ثم يسلّم الأمر إلى معالج استعادة الأقراص العادي في Clonezilla لتتولاه يدويًا.

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-37-amd64.iso \
  ./build/pbsnbd \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

ينزّل `make clonezilla` قرص ISO القياسي، ويبني `pbsnbd`، ويفعل الشيء نفسه، فيكتب `dist/Salvanza-BMR-<version>.iso`. التفاصيل الكاملة (لماذا إعادة بناء ISO كاملة بدلًا من الاستبدال في المكان، والمتطلبات، وتسلسل القائمة، والتحقق) موجودة في **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**؛ وطريقة استخدامه في **[دليل الاستعادة على جهاز فارغ](docs/manual/BMR.md)**.

### بناء واجهة Windows الرسومية

**على Windows مباشرة**: راجع **[BUILD.md](BUILD.md)** لسلسلة الأدوات (Go وMinGW من Chocolatey، وWails) والأوامر الخاصة بالتطبيق والخدمة وMSI.

**Docker (عند البناء على Linux).** يبني `./build_gui_windows_docker.sh` تطبيق Windows، مع دعم سليم لـ WebView2، داخل حاوية `golang` مؤقتة. لا يبني الخدمة الخلفية ولا MSI.

### سجل التغييرات والوثائق الداخلية

- **[CHANGELOG.md](CHANGELOG.md)**: سجل التغييرات لكل إصدار.
- **[TODO.md](TODO.md)**: خارطة الطريق المفتوحة والأفكار.
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)**: حالة المنتج المستقرة والإصدارات المتاحة.
- **[installer/DEPLOYMENT.md](installer/DEPLOYMENT.md)**: نشر MSI على حواسيب كثيرة.
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**: مربع حوار إلغاء تثبيت MSI (الاحتفاظ بالإعدادات أو حذفها) وخطة اختباره.
- **[AGENTS.md](AGENTS.md)**: بنية المستودع وأعرافه، للمساهمين ولوكلاء البرمجة.

## 🌐 الدعم التقني في إيطاليا

مع التأكيد على أن المشروع و**جميع** المساهمات فيه ستبقى عامة إلى الأبد ومرخصة بموجب رخصة GPLv3، فإن الراعي الرئيسي لهذا المشروع ولأحدث ميزات نسخ الأجهزة، E.T.I. Srl ( https://etitech.net )، يمكنه أن يقدّم لمن يحتاج دعمًا لعمليات نشر Proxmox عمومًا ولأدوات النسخ الاحتياطي في Windows خصوصًا.

لن تكون هناك أبدًا نسختان مختلفتان "community" و"enterprise"؛ وسيكون الدعم التقني وحده خدمة مستقلة.
وأي تخصيص تطلبه، حتى لو كان تطويره مدفوعًا، سيُصدر بموجب GPLv3 مثل المشروع كله.

## 🖥️ نسب الواجهة الرسومية

نشأ **تطبيق Salvanza** (المعروف سابقًا باسم Proxmox Backup Client GUI) على يد **[RDEM Systems](https://www.rdem-systems.com/)** باسم **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)**: فهي التي صممت التطبيق الأصلي وبنته، وتقدّم دعمًا تجاريًا حول Nimbus Backup، نسختها الموسومة بعلامتها. ثم أعاد **[mjb-is](https://github.com/mjb-is)** صياغته ووسّعه، مستلهمًا أسلوب عمل أنظمة نسخ احتياطي أخرى وطابعها، ودُمج ذلك العمل مجددًا في هذا المستودع. وتبقى قاعدة الشيفرة بأكملها، بما فيها الواجهة الرسومية وجميع ميزاتها، مفتوحة المصدر بموجب رخصة GPLv3.

## ⚠️ تحذير

يُقدَّم هذا البرنامج "كما هو". ومع أننا نسعى إلى الموثوقية، فإننا نخلي مسؤوليتنا عن أي فقدان للبيانات أو ضرر يلحق بها. اختبر نسخك الاحتياطية دائمًا وتحقق من الاستعادة قبل الاعتماد عليها في بيئة الإنتاج. لا تُعدّ أي نسخة احتياطية موثوقة حتى تستعيد منها فعلًا، فاختبر عمليات الاستعادة بنفسك، بما في ذلك الاستعادة على جهاز فارغ.

لا يزال البرنامج بجودة ألفا، ولا نتحمل أي مسؤولية عن أي نوع من الضرر أو فقدان البيانات، حتى للملفات المصدر.

نرحب بتقارير الأخطاء: افتح [بلاغًا (issue)](https://github.com/salvanza/salvanza_client/issues) يتضمن ما فعلته، وما حدث، والسجل (**Message Log** (سجل الرسائل)، أو **View Full Logs** (عرض السجلات الكاملة)).

## 📄 الرخصة

GPLv3: راجع ملف [LICENSE](LICENSE).

## 🏷️ العلامة التجارية

لكل مساهم قدّم **5 إيداعات على الأقل** تضيف وظائف أو إصلاحات الحقُّ في إضافة بيانات علامته التجارية للاستخدام التجاري.

الشروط الوحيدة هي ألا تمارس الشركة التي تشير إليها العلامة التجارية **أيًّا** من الأنشطة التالية:

- حملات البرمجيات الخبيثة
- الأعمال التي تروّج للحرب (ينطبق هذا على أي دولة، بما فيها الدول الغربية)
- الاحتيال
- سرقة البيانات
- الاتجار بالأطفال
- العنف
- التمييز
- المخدرات
- أي نشاط يُعترف به عمومًا على أنه غير قانوني

إذا وصلت أي شكوى إلى أي من المساهمين، فسنحاول التواصل؛ وإذا لم يُقدَّم تفسير مقنع، فسنقوم **بإنهاء تلك الميزة فورًا**.

**تبقى رخصة GPLv3 سارية**، وستظل حرًا في تفريع المشروع وبناء ملفاتك التنفيذية الخاصة.

## 🤝 المساهمة

نرحب بالمساهمات، ولا سيما:

1. ترجمات ملفات README وأدلة المستخدم للإصدار 2.0
2. الرفع غير المتزامن / متعدد الأنوية للكتل (الضغط متعدد الأنوية منفّذ بالفعل لنسخ الأجهزة)
3. ترقيع من جهة Proxmox لإضافة نوع آخر من الإدخالات إلى صيغة pxar يحتوي على واصفات الأمان في Windows
4. دعم الروابط الرمزية ونقاط الوصل (junctions) في Windows (يجري تخطيها حاليًا)
5. الاختبار على مزيد من العتاد وتوزيعات Linux والأنوية، ولا سيما الاستعادة على أجهزة فارغة
6. أي شيء مثير للاهتمام يمكنك التوصل إليه :)

## عن المساهمين في Salvanza

يطوّر المساهمون في Salvanza (المعروفون سابقًا بالمساهمين في Proxmox Backup Client GO) هذا المشروع ويصونونه. يعتمد البرنامج على البنية التحتية لـ NTP/NTS وعلى [خوادم NTS العامة الـ 11](https://github.com/jauderho/nts-servers) المدرجة في المرجع المجتمعي.

---

**© 2024-2026 Salvanza contributors and RDEM Systems.**
