# Proxmox Backup Client: cliente para Windows y Linux de Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Proxmox Backup Client es un cliente de copia de seguridad de código abierto (GPL-3.0) para Proxmox Backup Server (PBS), que funciona en Windows y Linux.**

Es un **conjunto de herramientas** para hacer copias de seguridad en PBS:

- **Proxmox Backup Client GUI** (basada en la GUI Nimbus Backup de RDEM Systems): un cliente gráfico para copiar servidores y estaciones de trabajo Windows y Linux en PBS, con instantáneas coherentes (VSS en Windows, elastio-snap/dattobd en Linux), conjuntos de copias de seguridad con programación, copias de carpetas y de discos completos, restauración, Recuperación y Reversión, varios servidores PBS y un servicio en segundo plano en Windows y Linux.
- **`proxmoxbackup-directory`**: herramienta de línea de comandos para copias de carpetas (pxar) con deduplicación y detección de cambios.
- **`proxmoxbackup-machine`**: herramienta de línea de comandos para copias en vivo de discos completos (fidx) de equipos Windows (VSS) y Linux (elastio-snap/dattobd); la imagen arranca tras una restauración a metal desnudo (bare-metal).
- **`proxmoxbackup-nbd`**: conecta una copia de disco como dispositivo NBD para restaurar (Linux).
- Una **ISO de restauración bare-metal** basada en Clonezilla Live.

> ⚠️ **Aviso legal:** este proyecto **no está afiliado de ninguna manera** con **Proxmox Server Solutions GmbH**. «Proxmox», el logotipo de Proxmox y los nombres relacionados pertenecen a sus respectivos propietarios; aquí se utilizan **solo** para indicar compatibilidad. Consulte [proxmox.com](https://www.proxmox.com/) para conocer sus productos.

> ℹ️ **Esta página es un resumen traducido.** La referencia es el [README.md en inglés](README.md), que contiene todos los detalles. Los manuales de usuario están en inglés: [GUI](docs/manual/GUI.md), [línea de comandos](docs/manual/CLI.md) y [restauración bare-metal](docs/manual/BMR.md).

## 📖 Manuales de usuario

| Manual | Contenido |
|---|---|
| **[Manual de la GUI](docs/manual/GUI.md)** | Instalación, servidores PBS, conjuntos de copias de seguridad y programación, copias incrementales, progreso, restauración, Recuperación, Reversión, Informes, registros, Preferencias, el servicio en segundo plano, solución de problemas |
| **[Manual de la línea de comandos](docs/manual/CLI.md)** | `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd`: todas las opciones, archivos de configuración, códigos de salida, programación; Recuperación y Reversión desde la línea de comandos |
| **[Manual de restauración bare-metal](docs/manual/BMR.md)** | La ISO de restauración paso a paso; instantáneas `vm` en Proxmox VE; hardware distinto |
| [Escenarios de restauración](docs/RESTORE_GUIDE.md) | Qué restauración usar en cada caso |
| [Índice de manuales](docs/manual/README.md) | Resumen de los tres manuales |

## 🆕 En master, para la próxima versión

Figuran en el [CHANGELOG](CHANGELOG.md) bajo [Unreleased] y aún no están en ninguna versión publicada:

- **Instaladores**: un **MSI para Windows** (aplicación y servicio en Program Files, entrada en el menú Inicio, actualizaciones en el sitio, conservar o borrar la configuración al desinstalar) y **paquetes Linux** `.deb` (Debian, Ubuntu, Mint) y `.rpm` (Fedora), con la aplicación, las tres herramientas de línea de comandos, los manuales y entradas de menú.
- **Servicio en segundo plano en Linux** (`proxmoxbackupclient.service`): los conjuntos programados se ejecutan como root, también sin nadie conectado. Se activa en Preferencias > Avanzado > Ejecutar como servicio, igual que en Windows. **Reiniciar como administrador** ya funciona también en Linux (mediante pkexec).
- **Pausar, Reanudar y Detener las copias que ejecuta el servicio**, con su tarjeta de progreso en la aplicación abierta.
- **Aviso cuando dos conjuntos comparten un grupo de copias** (mismo servidor, tipo e ID de copia de seguridad), con una insignia **ID compartido** en ambas tarjetas.
- **Configuración protegida**: en Windows `C:\ProgramData\ProxmoxBackupClient` solo es accesible para administradores; en Linux la carpeta de configuración es privada de su propietario.
- **Recuperar archivos de copias de máquina con todos los sistemas de archivos habituales**: FAT, exFAT, ext2/3/4 y XFS además de NTFS. Las restauraciones de imágenes de Windows reponen también permisos y flujos de datos alternativos.
- **Las copias de máquina Linux omiten el espacio libre** (ext2/3/4, XFS), como las de Windows desde v0.8.0.
- **Recuperación y Reversión para conjuntos de máquina Linux**, y **desde la línea de comandos** para cualquier conjunto: `-list-sets`, `-undelete`, `-rollback` (sección 12 del [manual de la línea de comandos](docs/manual/CLI.md)).
- Las herramientas de línea de comandos escriben los errores en stderr en lugar de mostrar ventanas, y `proxmoxbackup-machine` envía su correo de notificación.

## 💽 Novedades de v0.8.0: copias de máquina más rápidas, archivos desde ellas, Pausar/Reanudar

- **Las copias de máquina omiten el espacio libre** (y los archivos de paginación e hibernación) en Windows, así que un disco medio vacío se lee en la mitad de tiempo, y un **sector defectuoso** ya no detiene la copia: entra en la imagen como ceros y el informe indica dónde.
- La tarjeta de progreso muestra bloques sobre el total, el disco y la partición, fragmentos nuevos / reutilizados / vacíos y la velocidad de subida.
- **Restaurar archivos de una copia de máquina**: cada volumen NTFS de la imagen es una carpeta que se puede explorar, buscar y marcar. No se monta ni se descarga nada antes; solo se lee la tabla de archivos y luego los datos de los archivos elegidos (un C: de portátil de 127 GB con 430.000 archivos se lista en 4 a 15 segundos). Una sola copia de máquina por portátil sirve para la restauración bare-metal y para «necesito recuperar ese archivo».
- **Recuperación y Reversión** funcionan con conjuntos de máquina (Windows), sobre las carpetas que elija para el conjunto (por defecto Escritorio, Documentos, Imágenes, Música, Vídeos y Descargas).
- **Pausar y Reanudar** una copia en curso, una **insignia VSS / Instantánea** en cada tarjeta y las **fechas de creación** de Windows copiadas y restauradas.
- **v0.8.2:** el espacio vacío ya no se calcula bloque a bloque, así que un disco casi vacío tarda un tercio (un disco de prueba de 50 GB: de 3m36s a 1m15s).

## 🗑️ Novedades de v0.7.0: Recuperación y Reversión

Dos páginas nuevas en la GUI, para conjuntos de carpetas:

- **Recuperación** encuentra los archivos que están en las copias de un conjunto pero ya no en el disco (en la copia más reciente o en todas las de los últimos 7 o 30 días), los muestra en árbol con la fecha en que desaparecieron (y «probablemente movido a ...» si el mismo archivo está en otro sitio) y restaura los marcados en su ubicación original o en otra carpeta. Tras borrar 39 GB de un recurso de 761.000 archivos, una búsqueda de 7 días encontró exactamente esos archivos en un minuto.
- **Reversión** devuelve las carpetas de un conjunto al estado de una copia elegida. Primero muestra qué cambiaría (faltantes, modificados, añadidos después) y luego restaura. Todo lo que sustituye o elimina se aparta antes en la misma unidad, así que **Deshacer** lo devuelve todo.

Solo se leen las listas de archivos de las copias y los listados de las carpetas; ningún archivo se abre hasta restaurarlo. También: diálogos de confirmación propios, la Próxima ejecución en cada tarjeta, un ID de copia sugerido por conjunto (`deepthought-data`), nombres de conjunto únicos y el servicio incluido en la descarga de Windows. Consulte el [manual de la GUI](docs/manual/GUI.md#10-undelete).

## ⚡ Novedades de v0.6.0: copias de carpetas incrementales

Las copias de carpetas pueden omitir los archivos sin cambios, con la misma **detección de cambios** que el `proxmox-backup-client` oficial. En modo **Metadatos** el cliente compara el tamaño, la fecha de modificación (al nanosegundo), el modo y el propietario de cada archivo con la instantánea anterior. Los archivos sin cambios ni se abren: sus datos se reutilizan. Solo se leen y suben los archivos nuevos y modificados.

Medido en un servidor de archivos real (480,9 GB, 761.866 archivos en discos USB):

| Ejecución | Tiempo | Lectura | Fragmentos nuevos subidos |
|---|---|---|---|
| Primera ejecución Metadatos (nada con qué comparar) | 4 h 17 min | todos los archivos | 135.549 |
| Ejecuciones siguientes | unos 10 a 15 min | solo los modificados; los 761.866 sin cambios reutilizados | 4 a 40 |

También reduce el tráfico hacia un PBS remoto a poco más de lo que ha cambiado. Para usarlo: edite un conjunto de carpetas, pestaña **Destino**, **Detección de cambios: Metadatos**. Con **Leer todos los archivos cada N ejecuciones** cada N ejecuciones se hace una lectura completa (por ejemplo 42 para un conjunto que se ejecuta cada 4 horas, más o menos semanal). Dé a cada conjunto **su propio ID de copia de seguridad**. Línea de comandos: `-change-detection-mode metadata`. Clásico sigue siendo el modo por defecto.

## 🤝 Compatible con el cliente oficial de Proxmox, en ambos sentidos

Verificado con `proxmox-backup-client` 3.4.9 y PBS 4.2:

- **Las instantáneas Legacy, Data y Metadata de este cliente se restauran byte a byte idénticas con el cliente oficial**, desde la GUI y la línea de comandos, hechas en Windows o Linux, y el explorador de archivos de PBS 4.2 las abre. En Linux se conservan también el modo exacto, la fecha de modificación al nanosegundo y los destinos de los enlaces simbólicos.
- **Las instantáneas Legacy, Data y Metadata del cliente oficial se restauran idénticas con este cliente**, completas o de forma selectiva.
- Mismo formato de archivo dividido (pxar versión 2), mismos modos de detección de cambios, mismo formato de archivo de clave (las claves de `proxmox-backup-client key create` sirven aquí y al revés), mismas variables de entorno del estilo `PBS_REPOSITORY`.

Sus copias nunca dependen de este cliente. El único añadido es un pequeño archivo en la raíz de una instantánea de carpeta, `.proxmox_backup_client_meta.json` (la ruta original), que el cliente oficial restaura como un archivo normal.

## 📦 Descarga e instalación

👉 **[Descargar la última versión](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| Tiene | Use |
|---|---|
| Windows | El **instalador MSI** (`ProxmoxBackupClient-<version>-x64.msi`) instala la aplicación y el servicio en Program Files, con entrada en el menú Inicio. O descomprima `ProxmoxBackupClient-v<version>-windows-amd64.zip` (la aplicación y `ProxmoxBackupClientSVC.exe`; manténgalos en la misma carpeta) |
| Debian, Ubuntu, Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| Otro Linux | Descomprima `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` (necesita WebKitGTK) |
| Solo línea de comandos | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` o `-linux-amd64.tar.gz` |

El MSI, el `.deb` y el `.rpm` son nuevos en master: las versiones hasta v0.8.1 solo tienen los zip y tar.gz. Mientras ninguna versión incluya los instaladores, constrúyalos según [BUILD.md](BUILD.md).

Los paquetes Linux instalan dos entradas de menú: **Proxmox Backup Client** ejecuta la aplicación con su usuario (copias de sus propios archivos) y **Proxmox Backup Client (administrator)** la ejecuta como root, pidiendo la contraseña mediante polkit, para copias de máquina e instantáneas de bloque. El servicio se instala pero no se activa; consulte [packaging/debian/README.md](packaging/debian/README.md).

> ⚠️ **¿Windows muestra «virus detectado» (p. ej. `Trojan:Win32/Sabsik.FL.A!ml`) o un aviso de SmartScreen?** Es un **falso positivo conocido** de las aplicaciones Go/Wails para ejecutables sin firmar y poco comunes. Más detalles en el [proyecto original](https://github.com/tizbac/proxmoxbackupclient_go).

### 🔎 Verificar una descarga

Cada versión de este fork incluye un `SHA256SUMS.txt`:

```powershell
Get-FileHash .\ProxmoxBackupClient-v0.8.1-windows-amd64.zip -Algorithm SHA256   # compare with SHA256SUMS.txt
```

En Linux: `sha256sum -c SHA256SUMS.txt --ignore-missing`. Las compilaciones de este fork aún no tienen firma de código, atestación de CI ni análisis de VirusTotal (véase [TODO.md](TODO.md)).

## ✨ Funciones

### GUI: Proxmox Backup Client GUI (recomendada)
- **🌍 Multilingüe**: 18 idiomas de interfaz (francés, inglés, italiano, alemán, polaco, español, búlgaro, checo, griego, húngaro, letón, lituano, neerlandés, portugués, rumano, eslovaco, turco y ucraniano)
- Barra de menús nativa y menú lateral (Copia de seguridad, Restaurar, Recuperación, Reversión, Informes, Registro de mensajes, Preferencias, Acerca de)
- **Conjuntos de copias de seguridad**: tareas con nombre, programación diaria / por intervalo / manual, insignias de tipo y de VSS / Instantánea, «Ejecutar ahora» y aviso **ID compartido**
- **Copias puntuales** de carpetas o discos completos; página **Informes** con el historial completo y un **Registro de mensajes** limitado
- **Preferencias** con cinco temas de color (Ámbar, Azul clásico, Verde, Rojo, Oscuro) y un color personalizado
- **Copias de carpetas incrementales** (Metadatos), con lectura completa opcional cada N ejecuciones
- **Copias de máquina** en Windows y Linux que omiten el espacio libre y toleran sectores defectuosos, guardadas como `host` o como instantánea `vm` de Proxmox VE
- **Restaurar**: exploración de instantáneas, búsqueda (comodines, expresiones regulares, rutas completas), restauración selectiva con ACL NTFS/POSIX, fechas de carpetas, fechas de creación de Windows y verificación opcional
- **Archivos desde copias de máquina** (NTFS; FAT, exFAT, ext2/3/4 y XFS en master)
- **Recuperación** y **Reversión** (desde v0.7.0), para conjuntos de carpetas y de máquina, con vista previa y Deshacer
- **Pausar, Reanudar y Detener** una copia en curso
- **Ejecutar como servicio** desde Preferencias > Avanzado, para que las programaciones se ejecuten sin sesión iniciada (Windows desde v0.7.0; Linux en master)
- Tarjeta de progreso en vivo, VSS en Windows, instantáneas de bloque elastio-snap/dattobd en Linux
- Notificaciones por correo, acciones tras la copia, clonación de conjuntos, exportar e importar la configuración
- Varios servidores PBS, con fijación de huella del certificado, token de API o usuario/contraseña
- **Cifrado en el cliente** con un archivo de clave PBS por servidor
- Protección de administrador para modificar conjuntos (Windows), con Reiniciar como administrador

### Herramientas de línea de comandos
- `proxmoxbackup-directory`: copias de carpetas (pxar) con deduplicación y `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`: copias en vivo de discos completos de Windows (VSS) y Linux (elastio-snap/dattobd) (fidx, incrementales, espacio libre omitido salvo con `-readfreespace`)
- `proxmoxbackup-nbd`: servidor NBD para restaurar copias de disco (Linux)
- `ProxmoxBackupClientSVC` (Windows) y `proxmoxbackupclient` (paquetes Linux): `-list-sets`, `-undelete` y `-rollback` para los conjuntos definidos en la GUI (en master)

Las tres primeras aceptan `-keyfile` y `-keyfile-passphrase`. Referencia completa: [manual de la línea de comandos](docs/manual/CLI.md).

### 📸 Capturas de pantalla

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Conjuntos de copias de seguridad**: tareas con nombre, insignias de programación y de tipo, Ejecutar ahora en cada una

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Progreso en vivo**: rendimiento, tiempo restante y fragmentos nuevos frente a reutilizados

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Restaurar**: elija servidor PBS e ID de copia, liste instantáneas o busque por nombre de archivo

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-restore-progress.png" width="280">](docs/screenshots/gui-restore-progress.png)
**Restauración selectiva**: marque archivos o carpetas del árbol y restaure en su sitio o en otro lugar

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Informes**: historial completo con panel de detalle (duración, fragmentos, carpetas copiadas)

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-servers.png" width="280">](docs/screenshots/gui-preferences-servers.png)
**Multi-PBS**: gestione varios servidores PBS, pruebe la conexión, elija uno por defecto

</td>
</tr>
</table>

Más capturas (instantáneas `vm`, temas, idiomas) en el [README en inglés](README.md).

### Exclusiones automáticas del sistema (copias de carpetas)
Se omiten solos: `System Volume Information` y `$RECYCLE.BIN` estén donde estén, `Recovery` solo en la raíz de una unidad (en master), `.pbs-rollback` (la carpeta de seguridad de Reversión), y los archivos `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` y `DumpStack.log.tmp`. Una unidad puede mostrar 1,03 TB ocupados con solo ~141 GB de archivos reales; sin exclusiones la copia incluiría las instantáneas VSS. Recomendación: copia de **carpetas** para los archivos y copia de **máquina** en otro conjunto para la restauración bare-metal.

### Seguridad y calidad
- Cifrado opcional en el cliente de cada fragmento, con manifiesto firmado
- Carpeta de configuración solo para administradores en Windows, privada en Linux (en master)
- La API local del servicio escucha solo en 127.0.0.1 y exige un token renovado en cada arranque
- Validación de entradas, saneamiento de credenciales, prevención de path traversal, reintentos con espera exponencial
- Pruebas unitarias en cada módulo, además de pruebas en vivo y de extremo a extremo contra un PBS real

## 🔐 Cifrado en el cliente

Las instantáneas pueden cifrarse antes de salir del equipo con un archivo de clave de Proxmox Backup Server (el formato de `proxmox-backup-client key create`, intercambiable con el cliente oficial). Cada fragmento se cifra con AES-256-GCM y el manifiesto se firma. El servidor PBS nunca ve la clave.

**GUI:** **Preferencias > Información de la cuenta**, edite un servidor y use la pestaña **Cifrado**. Elija un archivo de clave existente o pulse **Generar...**. Con una clave protegida por frase de paso elija **Pedírmela** (una vez por sesión; las copias programadas y las del servicio fallan enseguida sin ella) o **Recordar en este equipo** (guardada en `config.json`; la opción para tareas desatendidas y el servicio). **Sin el archivo de clave no se puede restaurar una instantánea: guarde una copia en un lugar seguro, fuera de este equipo.**

**CLI:**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```

**Bare-metal:** la ISO de Clonezilla pide la clave y la frase de paso; la clave nunca se escribe en la ISO. Véase [PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md).

## Copias de carpetas más rápidas: detección de cambios

| Modo | Lee | Se guarda como |
|---|---|---|
| **Legacy** (Clásico, por defecto) | todos los archivos | un `<name>.pxar.didx` por carpeta |
| **Data** (Datos) | todos los archivos | archivo dividido: `<name>.mpxar.didx` (metadatos) + `<name>.ppxar.didx` (contenido) |
| **Metadata** (Metadatos) | solo nuevos y modificados | el mismo archivo dividido; los archivos con mismo tamaño, fecha, modo y propietario se toman de la copia anterior sin leerlos |

La primera ejecución Metadatos lo lee todo. Un cambio que conserva tamaño y fecha no se detecta, por eso existe **Leer todos los archivos cada N ejecuciones**. Solo afecta a los conjuntos de carpetas: las copias de máquina y las restauraciones bare-metal o en Proxmox VE no cambian. Las copias puntuales usan Clásico. **Un ID de copia de seguridad por conjunto:** si dos conjuntos comparten ID en el mismo servidor, cada uno se compara con la instantánea del otro y lo lee todo. CLI: `-change-detection-mode legacy|data|metadata` (no hay opción «cada N ejecuciones»: programe de vez en cuando una ejecución `data`). **Antes de pasar un conjunto a Datos o Metadatos,** actualice a v0.6.0 o posterior todos los equipos desde los que restaure.

## 🚀 Inicio rápido (GUI)

1. Instálela: el MSI en Windows, el `.deb` o `.rpm` en Linux, o el zip o tar.gz de las [versiones](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
2. Iníciela. En Windows, **Ejecutar como administrador** para VSS, copias de máquina y cambios en los conjuntos. En Linux, use **Proxmox Backup Client (administrator)** (o root) para copias de máquina e instantáneas de bloque
3. Añada su servidor PBS en **Preferencias > Información de la cuenta** y pulse **Probar**
4. Cree un conjunto de copias de seguridad (o una copia puntual) y elija carpetas o discos
5. Ejecútelo. Para que las programaciones se ejecuten sin sesión iniciada, active **Preferencias > Avanzado > Ejecutar como servicio**

El [manual de la GUI](docs/manual/GUI.md) explica cada paso.

## Copia de máquina completa en vivo

`proxmoxbackup-machine` (y las copias de máquina de la GUI) copian un sistema Windows 10/11, Windows Server o Linux completo sin interrupción. La sintaxis es como la de las copias de carpetas, pero `-backupdir` se sustituye por `-backupdev` (repítalo para varios discos):

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

Esto detecta las particiones montadas del disco 0, crea una instantánea VSS y genera una imagen arrancable del disco completo como fidx. El espacio libre y los archivos de paginación e hibernación entran como ceros sin leerse; `-readfreespace` lo lee todo (copia forense). La siguiente copia es incremental, a velocidades de hasta 1 GB/s.

### Requisitos de la copia de máquina en Linux (equivalente a VSS)

Linux no tiene VSS; el proyecto usa el módulo del kernel `elastio-snap` (un fork de `dattobd`), en su versión mantenida [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap). `dattobd` también sirve si ya está cargado. Hacen falta las dos cosas:

1. **El módulo instalado y cargado.** El paquete de repositorio documentado falla al menos en Ubuntu 22.04; compilar desde el código fuente funciona:
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
   Esto lo carga solo para el arranque actual (para cargarlo siempre, p. ej. una entrada en `/etc/modules-load.d/`).

2. **El proceso de copia debe ejecutarse como root (EUID 0).** Pertenecer al grupo `disk` no basta. Use la entrada **Proxmox Backup Client (administrator)**, el servicio, o `sudo`. Las copias de carpetas no lo necesitan.

### Restaurar archivos de una copia de disco

La página Restaurar de la GUI abre una copia de máquina como una de carpetas: cada volumen es una carpeta que se puede explorar (NTFS desde v0.8.0; FAT, exFAT, ext2/3/4 y XFS en master). No se monta nada.

En Linux también puede conectar una copia de disco como dispositivo de bloque con la herramienta nbd. Antes ejecute `modprobe nbd max_part=0` (con `max_part != 0` se produce un bucle infinito de sondeo de particiones):

`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

Sin `-path` aparece una interfaz de terminal para elegir el fidx. No lo use en un equipo con servicios importantes (un sistema de archivos dañado puede bloquear el sistema operativo), y desmonte todo antes de detener pbsnbd. Ante `Device or resource busy`, ejecute `nbd-client -d /dev/nbd0` o reinicie.

### Restaurar en una máquina física

El fork incluye una ISO de Clonezilla Live parcheada con una entrada de arranque **«PBS Bare Metal Restore»** totalmente automática (primera posición, por defecto al agotar el tiempo): introduzca los datos de PBS, elija una instantánea, confirme el disco destino detectado y se encarga de la red, la conexión NBD, la restauración y el reinicio. La ruta semimanual anterior («pbs-nbd» en el menú de modos de Clonezilla) sigue disponible.

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

Detalles en **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)** y en el [manual de restauración bare-metal](docs/manual/BMR.md).

## Uso: copia de directorios

La referencia completa está en el **[manual de la línea de comandos](docs/manual/CLI.md)**. Un comando típico:

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

También se leen las variables de entorno `PBS_REPOSITORY`, `PBS_SERVER`, `PBS_PORT`, `PBS_DATASTORE`, `PBS_AUTH_ID`, `PBS_PASSWORD` y `PBS_FINGERPRINT` del cliente oficial. Las plantillas de correo usan el [motor de plantillas de Go](https://pkg.go.dev/text/template); la lista de variables está en el README en inglés.

## 📋 Requisitos previos

- Windows 10/11 o Windows Server (64 bits), con derechos de administrador para VSS y copias de máquina
- o Linux (paquetes para Debian, Ubuntu, Mint y Fedora), con WebKitGTK para la GUI, y root más el módulo elastio-snap o dattobd para copias de máquina coherentes
- Acceso de red a un Proxmox Backup Server

## 🔨 Compilar desde el código fuente

**[BUILD.md](BUILD.md)** tiene las instrucciones completas (GUI, servicio, herramientas de línea de comandos, MSI y paquetes Linux). En resumen, con Go 1.25 o posterior y Wails CLI v2.13:

```bash
cd gui
wails build                                    # the app, in gui/build/bin
go build -tags service -o build/bin/ProxmoxBackupClientSVC.exe   # the background service
```

Pruebas: `make test`.

## 🔧 Uso avanzado y guías

- **[MULTI_PBS_USER_GUIDE.md](MULTI_PBS_USER_GUIDE.md)** y **[MULTI_PBS_GUIDE.md](MULTI_PBS_GUIDE.md)**: varios servidores PBS (guía de usuario e implementación).
- **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**: la ISO de restauración bare-metal.
- **[FEATURES_STATUS.md](FEATURES_STATUS.md)**: qué está hecho, parcial o pendiente.
- **[CHANGELOG.md](CHANGELOG.md)**: historial de cambios por versión.
- **[TODO.md](TODO.md)**: hoja de ruta e ideas.

## 🌐 Soporte técnico en Italia

El proyecto y **TODAS** sus contribuciones seguirán siendo públicos bajo GPLv3. E.T.I. Srl ( https://etitech.net ), patrocinador principal, puede ofrecer soporte para despliegues de Proxmox y herramientas de copia para Windows. Nunca habrá ediciones «community» y «enterprise»; cualquier personalización, aunque sea de pago, se publicará como GPLv3.

## 🖥️ Atribución de la GUI

La **Proxmox Backup Client GUI** se basa en la **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)**, desarrollada y mantenida por **[RDEM Systems](https://www.rdem-systems.com/)**. Se ha reintegrado en este repositorio y sigue siendo código abierto bajo GPLv3. RDEM Systems patrocina su desarrollo y ofrece soporte comercial.

## ⚠️ Advertencia

Este software se proporciona «tal cual» y aún tiene calidad alfa. Declinamos toda responsabilidad por pérdida o daño de datos. Pruebe siempre sus copias y verifique la restauración antes de confiar en ellas en producción.

## 📄 Licencia

GPLv3: consulte el archivo [LICENSE](LICENSE).

## 🏷️ Marca

Todo colaborador con **al menos 5 commits** que añadan funciones o correcciones tiene derecho a que se añadan sus datos de marca para uso comercial, siempre que la empresa **no** se dedique a malware, a promover guerras, estafas, robo de datos, tráfico de menores, violencia, discriminación, drogas ni actividades ilegales. Ante una queja sin explicación válida, el beneficio se **retira de inmediato**. La **licencia GPLv3 sigue vigente**.

## 🤝 Contribuir

Las contribuciones son bienvenidas, sobre todo: revisión nativa de los textos de cifrado y una gestión de claves más allá del archivo de clave; migración física a virtual (P2V); subida asíncrona o multinúcleo de fragmentos; un parche en Proxmox para guardar descriptores de seguridad de Windows en pxar; enlaces simbólicos de Windows; y cualquier idea interesante :)

## Acerca de los colaboradores de Proxmox Backup Client GO

Los colaboradores de Proxmox Backup Client GO desarrollan y mantienen este proyecto. El software usa la infraestructura NTP/NTS y los [11 servidores NTS públicos](https://github.com/jauderho/nts-servers) de la referencia comunitaria.

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**
