# Proxmox Backup Client: cliente para Windows y Linux de Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/mjb-is/proxmoxbackupclient_go)](https://github.com/mjb-is/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)
[![Ko-fi](https://img.shields.io/badge/Ko--fi-buy%20me%20a%20coffee-FF5E5B?logo=ko-fi&logoColor=white)](https://ko-fi.com/mickbeeby)

> **Construido sobre el trabajo de otros.** Este fork se basa en el [proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) original de Tiziano Bacocco (tizbac) y en el cliente gráfico Nimbus Backup de RDEM Systems, que se integró en él y del que surgió la interfaz gráfica de esta aplicación.
>
> ☕ El cliente es gratuito y seguirá siéndolo. Si le ahorra tiempo, puede [invitarme a un café en Ko-fi](https://ko-fi.com/mickbeeby). Es totalmente opcional y no compra soporte ni promesas de funciones.

**Proxmox Backup Client es un cliente de copia de seguridad de código abierto (GPL-3.0) para Proxmox Backup Server (PBS), para Windows y Linux.** Copia carpetas como archivos que se pueden explorar y máquinas completas como imágenes de disco arrancables, de forma programada o a petición, y recupera desde un solo archivo hasta una máquina entera, en el mismo equipo, en uno de sustitución o en una máquina virtual de Proxmox VE.

Lo desarrollo para mi propio homelab, para que PBS sea el único sistema de copias de seguridad de una mezcla de servidores, equipos de escritorio y portátiles Windows y Linux, en lugar de Backup for Workgroups, réplicas con rsync e imágenes de Clonezilla/RescueZilla. Empezó en septiembre de 2026 como un fork de [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) y ha ido mucho más allá: más de 350 commits desde entonces, con 18 correcciones y funciones enviadas al proyecto original ([lista más abajo](#-contribuciones-al-proyecto-original)).

**Puede usarlo tal como está.** Es oficialmente **software alfa**, se usa bajo su propia responsabilidad y sin garantía de ningún tipo. Pruebo yo mismo cada versión y lo uso habitualmente en mis propios equipos, pero lea [Estado, informes de errores y solicitudes de funciones](#-estado-informes-de-errores-y-solicitudes-de-funciones) antes de confiar en él.

El conjunto de herramientas:

- **Proxmox Backup Client GUI**: la aplicación para Windows y Linux. Conjuntos de copias de seguridad con varias programaciones cada uno, copias de carpetas y de discos completos con instantáneas coherentes (VSS en Windows, elastio-snap/dattobd en Linux), restauración de archivos desde copias de carpetas y de máquina, Recuperación y Reversión, varios servidores PBS, cifrado en el cliente y un servicio en segundo plano para que las programaciones se ejecuten sin nadie con la sesión iniciada.
- **`proxmoxbackup-directory`**: copias de carpetas (pxar) desde la línea de comandos, con deduplicación y detección de cambios.
- **`proxmoxbackup-machine`**: copias en vivo de discos completos (fidx) de equipos Windows y Linux desde la línea de comandos; la imagen arranca tras una restauración.
- **`proxmoxbackup-nbd`**: conecta una copia de disco como dispositivo NBD (Linux).
- **Una ISO de restauración bare-metal** basada en Clonezilla Live, con una restauración guiada desde el encendido hasta el reinicio.

> Palabras clave: cliente proxmox backup para windows · cliente PBS · copia de seguridad VSS de Windows · físico a virtual (P2V) · restauración bare-metal · interfaz para Proxmox Backup Server.

## ⭐ Qué hace, con resultados medidos

| | Resultado |
|---|---|
| **Copias de carpetas incrementales** | Un recurso compartido de un servidor de archivos de 480,9 GB con 761.866 archivos: primera ejecución 4 h 17 min, cada ejecución posterior de 10 a 15 minutos. Los archivos sin cambios ni siquiera se abren (la detección de cambios Metadatos del cliente oficial). |
| **Copias de máquina rápidas** | No se leen el espacio libre, el archivo de paginación ni el de hibernación, y el espacio vacío del disco no se calcula. Un disco de portátil de 1 TB con 373 GB ocupados: antes de 1 h 23 min a 1 h 44 min, ahora 38 minutos. Un disco de prueba de 50 GB: 3 min 36 s, ahora 1 min 15 s. |
| **Archivos desde copias de máquina** | Explore una imagen de disco como una carpeta y restaure archivos sueltos, desde NTFS, FAT, exFAT, ext2/3/4 y XFS. La unidad C: de 127 GB de un portátil con 430.000 archivos se lista en 4 a 15 segundos; no se monta ni se descarga nada antes. |
| **De máquina física a máquina virtual** | Copie un PC o un servidor y luego arránquelo como VM de Proxmox VE: restaurado mediante el propio Proxmox VE o con la ISO de restauración. [Véase más abajo](#-de-máquinas-físicas-a-máquinas-virtuales-p2v). |
| **Restauración bare-metal** | Arranque la ISO (memoria USB, CD o PXE), introduzca los datos de PBS y elija la copia: un equipo Windows de 32 GB volvió a su pantalla de inicio de sesión 8 minutos después del encendido. |
| **Recuperación** | Tras borrar 39 GB de un recurso de 761.000 archivos, una búsqueda de 7 días encontró exactamente esos archivos en un minuto aproximadamente y los devolvió a su sitio. |
| **Reversión** | Devuelve las carpetas de un conjunto al estado de una copia elegida, con una vista previa antes y Deshacer después. |
| **Programaciones** | Varias por conjunto de copias de seguridad, desde cada pocos minutos hasta una vez al año: «todos los días a las 02:00» más «el último día de cada mes» o «el 2.º martes», cada una descrita con palabras y con sus 3 próximas ejecuciones. |
| **Fidelidad de la restauración** | Permisos NTFS, atributos, flujos de datos alternativos y atributos extendidos, fechas de creación y de carpetas en Windows; modos, propietarios, ACL, atributos extendidos y enlaces simbólicos en Linux. Una pasada opcional vuelve a leer lo restaurado y lo compara con la copia. |
| **Funciona con las propias herramientas de Proxmox** | Las copias hechas aquí se restauran byte a byte idénticas con el `proxmox-backup-client` oficial, y al revés. [Detalles](#-compatible-con-el-cliente-oficial-de-proxmox-en-ambos-sentidos). |

## 🧩 Qué añade respecto al proyecto original

Además de los [18 cambios incorporados al proyecto original](#-contribuciones-al-proyecto-original), este cliente añade:

**Copias de seguridad**
- Copias de carpetas incrementales con la detección de cambios del cliente oficial (modos Datos y Metadatos, archivos pxar v2 divididos), con una lectura completa cada N ejecuciones.
- Copias de máquina que omiten el espacio libre, el archivo de paginación y el de hibernación (NTFS en Windows; ext2/3/4 y XFS en Linux), escriben como ceros los sectores ilegibles y continúan, y muestran mientras se ejecutan los recuentos de bloques, disco, partición y fragmentos.
- Pausar, Reanudar y Detener para copias de carpetas y de máquina, también para las copias que ejecuta el servicio en segundo plano.
- Flujos de datos alternativos, atributos extendidos NTFS y fechas de creación en las copias de carpetas de Windows.

**Programaciones y conjuntos de copias de seguridad**
- Varias programaciones por conjunto, desde cada pocos minutos hasta una vez al año (el enésimo día de la semana, el último día del mes), cada una descrita con palabras y con sus 3 próximas ejecuciones.
- Un servicio en segundo plano en Linux además de en Windows, y Reiniciar como administrador en Linux.
- Ejecutar ahora rápido o completo, una insignia VSS / Instantánea, un aviso **ID compartido**, nombres de conjunto escritos en PBS como comentarios de las instantáneas, alertas por correo, acciones tras la copia y exportación e importación de la configuración.

**Restauraciones**
- Archivos sueltos desde copias de máquina (NTFS, FAT, exFAT, ext2/3/4, XFS), sin montar ni descargar la imagen.
- Recuperación y Reversión, para conjuntos de carpetas y de máquina, en la GUI y desde la línea de comandos.
- Una pasada de verificación opcional tras una restauración, fechas de creación de Windows y un árbol de instantáneas que carga una carpeta cada vez, para que las copias muy grandes se abran al instante.
- Una ISO de restauración bare-metal guiada que también restaura en una VM vacía, y rutas probadas de físico a virtual ([P2V](#-de-máquinas-físicas-a-máquinas-virtuales-p2v)).
- Compatibilidad byte a byte con el `proxmox-backup-client` oficial en ambos sentidos, incluidas sus instantáneas Data y Metadata (archivo dividido).

**La aplicación**
- Una interfaz rehecha: barra de menús, menú lateral, Informes con el historial completo de copias y restauraciones, un Registro de mensajes, cinco temas de color y un color personalizado, notificaciones en la bandeja del sistema y emergentes.
- 18 idiomas de interfaz, todos revisados (el original tiene 5).
- Carpeta de configuración bloqueada para administradores (Windows) o para su propietario (Linux), y un token de la API del servicio renovado en cada arranque.
- Paquetes MSI, `.deb` y `.rpm`, y los manuales de usuario de la GUI, de la línea de comandos y de la restauración bare-metal.

## 🔄 De máquinas físicas a máquinas virtuales (P2V)

Una copia de máquina de un PC o servidor físico puede convertirse en una máquina virtual de Proxmox VE, de dos maneras:

**1. Restaurarla en Proxmox VE.** Configure el conjunto de copias de seguridad (o la copia puntual) con **Copia de seguridad como: Máquina virtual de Proxmox VE**. La copia se guarda como `vm/<número>` con una pequeña configuración de VM, de modo que Proxmox VE la muestra junto a sus otras copias de VM: abra el almacenamiento PBS en Proxmox VE, seleccione la copia, **Restaurar**, y elija un ID de VM y un almacenamiento. La VM recibe un disco SATA y el firmware adecuado para el disco (OVMF para UEFI/GPT, SeaBIOS para BIOS/MBR); añada una tarjeta de red antes de arrancarla, y un disco EFI (con «Pre-Enroll keys» desmarcado) si una VM UEFI no arranca.

**2. Restaurarla con la ISO de restauración bare-metal en una VM vacía.** Cree una VM con el mismo firmware que la máquina original (OVMF con un disco EFI para UEFI/GPT, SeaBIOS para BIOS/MBR), un disco SATA al menos tan grande como el original y una tarjeta de red Intel E1000, arránquela desde la ISO y ejecute la restauración. Funciona igual para copias `host` y `vm`, y la misma copia se sigue pudiendo restaurar en una máquina física.

Rutas probadas:

| Desde | Hacia | Ruta | Resultado |
|---|---|---|---|
| Portátil Windows 10, UEFI/GPT | VM de Proxmox VE | ISO de restauración | Arrancó en el escritorio del usuario |
| Windows Server, UEFI/GPT | VM de Proxmox VE | Restauración de Proxmox VE | Arrancó sin cambios |
| Escritorio Xubuntu, UEFI/GPT | VM de Proxmox VE | Restauración de Proxmox VE, y la ISO de restauración | Arrancó en el escritorio |
| Linux, BIOS/MBR | VM de Proxmox VE | ISO de restauración, y restauración de Proxmox VE | Arrancó en el inicio de sesión |
| Linux Mint, copia cifrada | VM de Proxmox VE | ISO de restauración, clave desde una memoria USB | Arrancó |
| PC Linux | el mismo PC con un disco nuevo | ISO de restauración | Arrancó |

Windows configura el nuevo hardware virtual en el primer arranque («Getting devices ready»). El [manual de restauración bare-metal](docs/manual/BMR.md#7-restoring-onto-different-hardware-p2v-and-replacement-machines) contiene la tabla de firmware, los controladores y las instrucciones paso a paso.

## 📖 Manuales de usuario

| Manual | Contenido |
|---|---|
| **[Manual de la GUI](docs/manual/GUI.md)** | Instalación, servidores PBS, conjuntos de copias de seguridad y programaciones, copias incrementales, progreso, restauración, Recuperación, Reversión, Informes, registros, Preferencias, el servicio en segundo plano, solución de problemas |
| **[Manual de la línea de comandos](docs/manual/CLI.md)** | `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd`: todas las opciones, archivos de configuración, códigos de salida, programación; Recuperación y Reversión desde la línea de comandos |
| **[Manual de restauración bare-metal](docs/manual/BMR.md)** | La ISO de restauración paso a paso; copias `vm` en Proxmox VE; de físico a virtual; hardware distinto |
| [Escenarios de restauración](docs/RESTORE_GUIDE.md) | Qué restauración usar en cada caso |
| [Índice de manuales](docs/manual/README.md) | Un resumen de los tres manuales |

## 🆕 Novedades de v0.9.0: varias programaciones por conjunto, instaladores, un servicio en Linux

- **Varias programaciones por conjunto de copias de seguridad**, desde cada pocos minutos hasta una vez al año: «todos los días a las 02:00» más «cada mes el último día» o «el 2.º martes», cada programación descrita con palabras y con sus 3 próximas ejecuciones. Los conjuntos existentes conservan su programación.
- **Las copias de carpetas conservan los flujos de datos alternativos y los atributos extendidos NTFS** (Windows), como la marca «descargado de Internet», y la restauración los repone. La lista de limitaciones conocidas ya no contiene ninguna.
- **Instaladores**: un **MSI para Windows** (aplicación y servicio en segundo plano en Program Files, entrada en el menú Inicio, actualizaciones en el sitio, conservar o borrar la configuración al desinstalar) y **paquetes Linux**, `.deb` (Debian, Ubuntu, Mint) y `.rpm` (Fedora), con la aplicación, las tres herramientas de línea de comandos, los manuales y entradas de menú.
- **Un servicio en segundo plano en Linux** (`proxmoxbackupclient.service`): los conjuntos programados se ejecutan como root, también cuando no hay nadie conectado. Se activa en Preferencias > Avanzado > Ejecutar como servicio, igual que en Windows. **Reiniciar como administrador** ya funciona también en Linux (mediante pkexec).
- **Pausar, Reanudar y Detener las copias que ejecuta el servicio**, con su tarjeta de progreso en la aplicación abierta.
- **Un aviso cuando dos conjuntos comparten un grupo de copias** (mismo servidor, tipo e ID de copia de seguridad), con una insignia **ID compartido** en ambas tarjetas.
- **Configuración protegida**: en Windows `C:\ProgramData\ProxmoxBackupClient` ahora solo es accesible para los administradores; en Linux la carpeta de configuración es privada de su propietario.
- **Archivos desde copias de máquina con todos los sistemas de archivos habituales**: FAT, exFAT, ext2/3/4 y XFS además de NTFS. Las restauraciones de imágenes de Windows reponen también los permisos y los flujos de datos alternativos.
- **Las copias de máquina Linux omiten el espacio libre** (ext2/3/4, XFS), como las de Windows desde v0.8.0.
- **Recuperación y Reversión para conjuntos de máquina Linux**, y **desde la línea de comandos** para cualquier conjunto: `-list-sets`, `-undelete`, `-rollback` (sección 12 del [manual de la línea de comandos](docs/manual/CLI.md)).
- Las herramientas de línea de comandos escriben los errores en stderr en lugar de mostrar ventanas de mensaje, y `proxmoxbackup-machine` envía su correo de notificación.
- **La ISO de restauración bare-metal** nunca ofrece como disco de destino la memoria USB desde la que arrancó, y el manual explica cómo hacer coincidir el firmware (UEFI o BIOS) cuando una máquina física pasa a ser una VM.
- **Todos los idiomas revisados** (17 traducciones, unas 1.850 cadenas), y las fechas y los números siguen el idioma de la aplicación.

Versiones anteriores: **v0.8** copias de máquina más rápidas (espacio libre omitido, sectores defectuosos superados), archivos desde copias de máquina, Pausar y Reanudar; **v0.7** Recuperación y Reversión; **v0.6** copias de carpetas incrementales. El [CHANGELOG](CHANGELOG.md) recoge todos los cambios.

## 🤝 Compatible con el cliente oficial de Proxmox, en ambos sentidos

Verificado con `proxmox-backup-client` 3.4.9 y PBS 4.2:

- **Las instantáneas Legacy, Data y Metadata hechas por este cliente se restauran byte a byte idénticas con el cliente oficial**, desde la GUI y desde la línea de comandos, hechas en Windows o en Linux, y el explorador de archivos de PBS 4.2 las abre. En Linux los archivos restaurados conservan también el modo exacto, la fecha de modificación al nanosegundo y los destinos de los enlaces simbólicos.
- **Las instantáneas Legacy, Data y Metadata hechas por el cliente oficial se restauran byte a byte idénticas con este cliente**, completas o de forma selectiva.
- Mismo formato de archivo dividido (pxar versión 2), mismos modos de detección de cambios, mismo formato de archivo de clave de cifrado (las claves de `proxmox-backup-client key create` sirven aquí, y las claves generadas aquí sirven allí), mismas variables de entorno del estilo `PBS_REPOSITORY`.

Así que sus copias nunca quedan atadas a este cliente: las propias herramientas de Proxmox restauran todas las instantáneas que hace, y él restaura todas las instantáneas que hace el cliente oficial. Lo único extra es un pequeño archivo que este cliente añade en la raíz de una instantánea de carpeta, `.proxmox_backup_client_meta.json` (la ruta original), que el cliente oficial restaura como un archivo normal.

> ⚠️ **Aviso legal:** este proyecto **no está afiliado de ninguna manera** con **Proxmox Server Solutions GmbH**. «Proxmox», el logotipo de Proxmox y los nombres relacionados pertenecen a sus respectivos propietarios; aquí se utilizan **solo** para indicar compatibilidad. Consulte [proxmox.com](https://www.proxmox.com/) para conocer sus productos.

## 📦 Descarga e instalación

👉 **[Descargar la última versión](https://github.com/mjb-is/proxmoxbackupclient_go/releases)**

| Tiene | Use |
|---|---|
| Windows | El **instalador MSI** (`ProxmoxBackupClient-<version>-x64.msi`) instala la aplicación y el servicio en segundo plano en Program Files, con una entrada en el menú Inicio. O descomprima `ProxmoxBackupClient-v<version>-windows-amd64.zip` (la aplicación y `ProxmoxBackupClientSVC.exe`; manténgalos en la misma carpeta) |
| Debian, Ubuntu, Mint | `sudo apt install ./proxmoxbackupclient_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./proxmoxbackupclient-<version>-1.fc44.x86_64.rpm` |
| Otro Linux | Descomprima `ProxmoxBackupClient-v<version>-linux-amd64.tar.gz` (necesita el entorno de ejecución WebKitGTK 4.1) |
| Solo línea de comandos | `ProxmoxBackupClient-CLI-v<version>-windows-amd64.zip` o `-linux-amd64.tar.gz` |
| Una máquina que restaurar | `ProxmoxBackupClient-GO-BMR-Restore-v<version>.iso`, la ISO de restauración bare-metal |

El MSI, el `.deb` y el `.rpm` son nuevos en v0.9.0: las versiones hasta v0.8.1 solo tienen los archivos zip y tar.gz. [BUILD.md](BUILD.md) explica cómo compilarlos usted mismo.

Los paquetes Linux instalan dos entradas de menú: **Proxmox Backup Client** ejecuta la aplicación con su usuario (copias de carpetas de sus propios archivos) y **Proxmox Backup Client (administrator)** la ejecuta como root, pidiendo su contraseña mediante polkit, para copias de máquina e instantáneas de bloque. El servicio en segundo plano se instala pero no se activa; consulte [packaging/debian/README.md](packaging/debian/README.md).

> ⚠️ **¿Windows muestra «virus detectado» (p. ej. `Trojan:Win32/Sabsik.FL.A!ml`) o un aviso de SmartScreen?**
> Es un **falso positivo conocido** de las aplicaciones Go/Wails: *no* es un virus. El sufijo `!ml` indica una detección por un modelo de aprendizaje automático que marca los ejecutables *sin firmar y poco comunes*. Compruebe la descarga con `SHA256SUMS.txt` (más abajo).

### 🔎 Verificar una descarga

Cada versión incluye un `SHA256SUMS.txt`:

```powershell
Get-FileHash .\ProxmoxBackupClient-0.9.0-x64.msi -Algorithm SHA256   # compare with SHA256SUMS.txt
```

En Linux: `sha256sum -c SHA256SUMS.txt --ignore-missing`.

> ℹ️ **Todavía no hay firma de código, atestación firmada por CI ni análisis de VirusTotal.** Las versiones se compilan y se suben a mano, todavía no mediante una canalización de CI que pueda producir una atestación verificable del origen de la compilación. Consulte [TODO.md](TODO.md) si quiere ayudar a configurarlo.

## ✨ Funciones

### GUI
- **🌍 Multilingüe**: 18 idiomas de interfaz: francés, inglés, italiano, alemán, polaco, español, búlgaro, checo, griego, húngaro, letón, lituano, neerlandés, portugués, rumano, eslovaco, turco y ucraniano
- Barra de menús nativa y menú lateral (Copia de seguridad, Restaurar, Recuperación, Reversión, Informes, Registro de mensajes, Preferencias, Acerca de)
- **Conjuntos de copias de seguridad**: tareas con nombre reutilizables con **varias programaciones cada una** (desde cada pocos minutos hasta anual, o manual), una insignia de tipo carpeta o máquina, una insignia VSS / Instantánea, «Ejecutar ahora» (rápido o con lectura completa para los conjuntos Metadatos) y un aviso **ID compartido** cuando dos conjuntos escribirían en un mismo grupo de copias
- **Copias puntuales** de carpetas o discos completos
- Página **Informes** con el historial completo de copias y restauraciones, y un **Registro de mensajes** limitado para diagnóstico
- Diálogo **Preferencias** con cinco temas de color (Ámbar, Azul, Verde, Rojo, Oscuro) más una opción de color personalizado
- **Copias de carpetas incrementales** (detección de cambios, modo Metadatos): los archivos sin cambios se reutilizan sin leerlos, como en el cliente oficial, con una lectura completa opcional cada N ejecuciones
- **Copias de máquina (disco completo)** en Windows y Linux que omiten el espacio libre y superan los sectores defectuosos, guardadas como `host` o como copia `vm` de Proxmox VE
- **De físico a virtual**: una copia de máquina se restaura en una VM de Proxmox VE, mediante Proxmox VE o la ISO de restauración
- **Restaurar**: exploración de instantáneas, búsqueda de archivos (comodines, expresiones regulares, rutas completas) y restauración selectiva, con ACL NTFS/POSIX, flujos de datos alternativos, fechas de carpetas, fechas de creación de Windows y una pasada de verificación opcional
- **Archivos desde copias de máquina**: explore y restaure archivos de volúmenes NTFS, FAT, exFAT, ext2/3/4 y XFS dentro de una imagen de disco
- **Recuperación** y **Reversión**: encuentre los archivos que las copias de un conjunto aún tienen pero el disco ya no, y devuélvalos a su sitio; o devuelva las carpetas de un conjunto al estado de una copia elegida, con una vista previa antes y Deshacer después. Conjuntos de carpetas y de máquina. Consulte el [manual de la GUI](docs/manual/GUI.md#10-undelete)
- **Pausar, Reanudar y Detener** una copia en curso
- **Ejecutar como servicio**: instale, inicie, detenga o elimine el servicio en segundo plano desde Preferencias > Avanzado, para que las programaciones se ejecuten sin nadie con la sesión iniciada (Windows y Linux)
- Tarjeta de progreso en vivo: hora de inicio y fin previsto, archivos hechos sobre el total, velocidad de procesamiento (datos y archivos por segundo) y velocidad real de subida, archivo actual y el motivo por el que una ejecución lee todos los archivos
- VSS (Volume Shadow Copy) en Windows, instantáneas de bloque elastio-snap/dattobd en Linux
- Notificaciones por correo y acciones tras la copia, clonación de conjuntos, exportación e importación de la configuración
- Varios servidores PBS, con fijación de la huella del certificado (confianza en el primer uso), inicio de sesión con token de API o con usuario/contraseña
- **Cifrado en el cliente**: archivo de clave PBS por servidor (AES-256-GCM, manifiesto firmado), protección opcional con frase de paso, véase [Cifrado en el cliente](#-cifrado-en-el-cliente)
- Protección de administrador para modificar conjuntos de copias de seguridad, con Reiniciar como administrador
- Registro de depuración para diagnóstico

### Herramientas de línea de comandos
- `proxmoxbackup-directory`: copias de carpetas (pxar) con deduplicación y `-change-detection-mode legacy|data|metadata`
- `proxmoxbackup-machine`: copias en vivo de discos completos de equipos Windows (VSS) y Linux (elastio-snap/dattobd) (fidx, incrementales, espacio libre omitido salvo con `-readfreespace`, `-type vm` para una copia de Proxmox VE)
- `proxmoxbackup-nbd`: servidor NBD para restaurar copias de disco (Linux)
- `ProxmoxBackupClientSVC` (Windows) y `proxmoxbackupclient` (paquetes Linux): `-list-sets`, `-undelete` y `-rollback` para los conjuntos definidos en la GUI

Las tres primeras aceptan `-keyfile` y `-keyfile-passphrase` para instantáneas cifradas. Referencia completa: [manual de la línea de comandos](docs/manual/CLI.md).

### 📸 Capturas de pantalla

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Conjuntos de copias de seguridad**: tareas con nombre con una insignia para la programación que se ejecuta a continuación (DIARIO, SEMANAL, MENSUAL, +N para las demás) y una insignia de tipo carpeta/máquina, Ejecutar ahora en cada una

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Progreso de la copia en vivo**: rendimiento, tiempo restante y recuento de fragmentos nuevos frente a reutilizados mientras se ejecuta

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Restaurar**: elija un servidor PBS y un ID de copia, liste las instantáneas o busque en ellas por nombre de archivo

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-schedule-tab.png" width="280">](docs/screenshots/gui-schedule-tab.png)
**Programaciones**: varias por conjunto, desde cada pocos minutos hasta una vez al año («cada mes el último día», «el 2.º martes»), con las 3 próximas ejecuciones

</td>
<td width="33%">

[<img src="docs/screenshots/backup-set-as-vm.png" width="280">](docs/screenshots/backup-set-as-vm.png)
**Copia como VM de Proxmox VE**: elija el tipo máquina virtual y un ID numérico; el ID de copia de PBS se muestra mientras escribe

</td>
<td width="33%">

[<img src="docs/screenshots/backup-sets-vm-badge.png" width="280">](docs/screenshots/backup-sets-vm-badge.png)
**Conjuntos como VM de Proxmox VE**: un conjunto guardado como copia `vm` lleva una insignia `MACHINE (VM)`

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/oneoff-as-vm.png" width="280">](docs/screenshots/oneoff-as-vm.png)
**Copias puntuales**: la misma opción en el formulario de copia puntual

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore-progress.png" width="280">](docs/screenshots/gui-restore-progress.png)
**Restauración selectiva**: marque archivos o carpetas sueltos en el árbol de la instantánea y restaure en su sitio o en otro lugar

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Informes**: historial completo de ejecuciones con un panel de detalle (duración, fragmentos, carpetas copiadas)

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-message-log.png" width="280">](docs/screenshots/gui-message-log.png)
**Registro de mensajes**: un registro de diagnóstico limitado y con marca de tiempo de todas las copias y restauraciones

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-servers.png" width="280">](docs/screenshots/gui-preferences-servers.png)
**Multi-PBS**: gestione varios servidores PBS, pruebe la conexión, elija uno por defecto

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-edit-server.png" width="280">](docs/screenshots/gui-preferences-edit-server.png)
**Añadir o editar un servidor**: URL, usuario/contraseña o token de API, almacén de datos y espacio de nombres

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-theme-amber.png" width="280">](docs/screenshots/gui-theme-amber.png)
**Temas**: cinco paletas de color integradas más un color personalizado, que cambian el color de toda la aplicación al instante (véase la tira de abajo)

</td>
<td width="33%">

[<img src="docs/screenshots/gui-preferences-advanced.png" width="280">](docs/screenshots/gui-preferences-advanced.png)
**Opciones de copia**: explicadas con honestidad: la extracción paralela en la restauración es experimental y en nuestro propio hardware de prueba resultó *más lenta*

</td>
</tr>
</table>

**🎨 Temas de color**: el propio diálogo Preferencias cambia de color con cada elección, no solo una muestra:

<table>
<tr>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-amber.png" width="180">](docs/screenshots/gui-theme-amber.png)
Ámbar

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-blue.png" width="180">](docs/screenshots/gui-theme-blue.png)
Azul clásico

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-green.png" width="180">](docs/screenshots/gui-theme-green.png)
Verde

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-red.png" width="180">](docs/screenshots/gui-theme-red.png)
Rojo

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-dark.png" width="180">](docs/screenshots/gui-theme-dark.png)
Oscuro

</td>
<td width="16.6%">

[<img src="docs/screenshots/gui-theme-custom.png" width="180">](docs/screenshots/gui-theme-custom.png)
Personalizado

</td>
</tr>
</table>

**🌍 Multilingüe**: la página Informes en francés, italiano y español (y 15 más, entre ellos inglés, alemán y polaco):

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

### Exclusiones inteligentes del sistema (copias de carpetas)
Las copias de carpetas omiten por sí solas lo siguiente:

**Carpetas del sistema:** `System Volume Information` (almacenamiento de VSS, puede superar los 100 GB) y `$RECYCLE.BIN` estén donde estén; `Recovery` solo en la raíz de una unidad; `.pbs-rollback` (la carpeta de seguridad de Reversión).
**Archivos del sistema:** `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` y `DumpStack.log.tmp`.

**Por qué importa:** una unidad puede mostrar 1,03 TB ocupados cuando los archivos reales ocupan ~141 GB. Sin exclusiones la copia incluiría las instantáneas VSS (espacio y tiempo desperdiciados); con ellas, el tamaño coincide con los datos reales.

**Recomendación:** use una copia de **carpetas** con estas exclusiones para las copias a nivel de archivo; use una copia de **máquina** en un conjunto aparte para la restauración bare-metal y P2V (es una imagen del disco completo; el espacio libre y los archivos de paginación e hibernación entran como ceros).

### Seguridad y calidad
- Cifrado opcional en el cliente de cada fragmento, con un manifiesto firmado
- Carpeta de configuración accesible solo para administradores en Windows, privada de su propietario en Linux
- La API local del servicio escucha solo en 127.0.0.1 y necesita un token, renovado cada vez que el servicio arranca
- Validación de entradas y saneamiento de credenciales
- Prevención de path traversal
- Lógica de reintentos con espera exponencial
- Gestión de errores y pruebas unitarias en cada módulo, además de pruebas en vivo opcionales y pruebas de extremo a extremo contra un PBS real

## 🔐 Cifrado en el cliente

Las instantáneas pueden cifrarse antes de salir del equipo, con un archivo de clave de Proxmox Backup Server (el formato que escribe `proxmox-backup-client key create`, así que las claves son intercambiables con el cliente oficial). Cada fragmento se cifra con AES-256-GCM y el manifiesto se firma. El servidor PBS nunca ve la clave.

**GUI:** abra **Preferencias > Información de la cuenta**, edite un servidor y use su pestaña **Cifrado**. Elija un archivo de clave existente o pulse **Generar...** para crear uno nuevo (la opción avanzada lo protege con una frase de paso). La pestaña muestra la huella de la clave. Los Informes indican si cada copia estaba cifrada. **Sin el archivo de clave no se puede restaurar una instantánea: guarde una copia en un lugar seguro, fuera de este equipo.**

**Claves protegidas por frase de paso (scrypt o PBKDF2):** elija cómo desbloquea la clave la aplicación:
- **Pedírmela**: la frase de paso se pide una vez por sesión de la aplicación y solo se guarda en memoria. Las copias programadas y las de inicio nunca la piden: fallan enseguida, con un mensaje claro, hasta que se introduce la frase de paso en esa sesión. Una copia ejecutada por el servicio en segundo plano también falla enseguida con una clave en modo Pedírmela.
- **Recordar en este equipo**: la frase de paso se guarda en la configuración igual que el token de PBS, así que cualquiera que pueda leer `config.json` puede desbloquear la clave. Es la opción para tareas programadas desatendidas y para el servicio en segundo plano.

La restauración pide la frase de paso cuando se selecciona una instantánea cifrada.

**CLI:**
```shell
proxmoxbackup-directory.exe -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
`-keyfile-passphrase` solo hace falta para una clave protegida y se pide si se omite. `proxmoxbackup-machine` y `proxmoxbackup-nbd` aceptan las mismas dos opciones; `proxmoxbackup-nbd` las necesita para conectar una imagen de disco cifrada y comprueba el primer fragmento antes de tocar el dispositivo NBD, así que una clave incorrecta falla de inmediato.

**Restauración bare-metal:** la ISO de restauración pide la clave (ninguna, un archivo de clave en una memoria USB o una ruta escrita a mano) y la frase de paso de una clave protegida. La clave nunca se escribe en la ISO. Véase [PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md).

**Limitaciones conocidas**
- Una clave generada por la GUI se escribe con una ACL protegida (solo el usuario actual, SYSTEM y Administradores). Los archivos de clave que guarde en otro sitio tienen los permisos que les dé su carpeta.
- `config.json` contiene en texto plano los tokens de PBS, las contraseñas y cualquier frase de paso recordada. Desde v0.9.0 su carpeta solo es accesible para administradores (Windows) o solo para su propietario (Linux); hasta v0.8.2 los Usuarios locales de Windows podían leerla, así que restrínjala usted mismo en un equipo compartido.
- Restaurar una instantánea cifrada requiere el mismo archivo de clave; si se pierde, no hay forma de recuperarla.
- El flujo de clave Pedírmela y el servicio son procesos separados: use Recordar para todo lo que se ejecute mediante el servicio.

## Copias de carpetas más rápidas: detección de cambios

En el modo Clásico por defecto cada copia de carpetas lee todos los archivos, así que una ejecución tarda lo mismo que leer la carpeta entera aunque no haya cambiado nada. Los conjuntos de copias de seguridad (y la CLI) pueden usar en su lugar los modos de detección de cambios del `proxmox-backup-client` oficial. El modo Metadatos se usa a diario en un servidor de 480,9 GB y 761.866 archivos, donde una ejecución con pocos cambios tarda de 10 a 15 minutos en lugar de más de 4 horas.

| Modo | Lee | Se guarda como |
|---|---|---|
| **Legacy** (Clásico, por defecto) | todos los archivos | un `<name>.pxar.didx` por carpeta, como antes |
| **Data** (Datos) | todos los archivos | un archivo dividido: `<name>.mpxar.didx` (metadatos) + `<name>.ppxar.didx` (contenido de los archivos) |
| **Metadata** (Metadatos) | solo los archivos nuevos y modificados | el mismo archivo dividido; los archivos cuyo tamaño y fecha de modificación (al nanosegundo), modo y propietario coinciden con la copia anterior se toman de ella sin leerlos |

En modo **Metadatos** la primera ejecución lo lee todo (todavía no hay nada con qué comparar); las siguientes solo leen lo que ha cambiado. Un archivo modificado de forma que conserva su tamaño y su fecha de modificación no se detecta, y por eso un conjunto también puede **leer todos los archivos cada N ejecuciones** (una ejecución Datos): esa ejecución detecta esos cambios y elimina el relleno que la reutilización deja en el archivo. Un archivo que no se pudo leer por completo (rellenado con ceros tras un error de lectura) siempre se vuelve a leer en la ejecución siguiente. La reutilización solo se produce a partir de una copia anterior cifrada de la misma forma, con la misma clave.

Como los datos sin cambios no se leen ni se envían, el tráfico hacia un PBS remoto o externo también se reduce a poco más de lo que realmente ha cambiado.

**GUI:** edite un conjunto, pestaña **Destino**, **Detección de cambios**. Solo se aplica a los conjuntos de carpetas: las copias de máquina (disco completo) son imágenes de disco y no se ven afectadas, así que las restauraciones bare-metal y en Proxmox VE funcionan exactamente igual que antes. Las copias puntuales usan Clásico. Solo las ejecuciones que terminan correctamente cuentan para la próxima lectura completa; la lista de conjuntos muestra «Próxima lectura completa dentro de N ejecuciones». La tarjeta de progreso indica cuándo y por qué una ejecución lee todos los archivos. **Informes** muestra el modo de cada ejecución y cuántos archivos sin cambios se reutilizaron.

**Un ID de copia de seguridad por conjunto.** El modo Metadatos compara con la instantánea más reciente del grupo de copias. Si dos conjuntos (o dos equipos) usan el mismo ID de copia en el mismo servidor, cada ejecución compara con la instantánea del otro y lo lee todo. El editor de conjuntos avisa de ello.

**CLI:** `-change-detection-mode legacy|data|metadata`, o `"change-detection-mode"` en la configuración JSON. No hay una opción de CLI «cada N ejecuciones»: programe en su lugar una ejecución ocasional con `-change-detection-mode data`.

**Antes de pasar un conjunto a Datos o Metadatos,** actualice a v0.6.0 o posterior todos los equipos desde los que restaure: las compilaciones anteriores al soporte de archivos divididos no pueden leerlos (muestran las carpetas sin sus archivos). El `proxmox-backup-client` oficial los restaura byte a byte idénticos (probado con 3.4.9), y el explorador de archivos de PBS 4.2 los abre. La primera copia dividida de una carpeta comparte poco con sus copias Clásico anteriores, así que cuesta aproximadamente una copia completa más de esa carpeta en el almacén de datos hasta que se purguen las instantáneas antiguas.

## 🚀 Inicio rápido (GUI)

1. Instálela: el MSI en Windows, el `.deb` o el `.rpm` en Linux, o descomprima el zip o el tar.gz de las [versiones](https://github.com/mjb-is/proxmoxbackupclient_go/releases) (véase [Descarga e instalación](#-descarga-e-instalación))
2. Iníciela. En Windows, **Ejecutar como administrador** para VSS, copias de máquina y cambios en los conjuntos. En Linux, las copias de carpetas de sus propios archivos funcionan con su usuario; use **Proxmox Backup Client (administrator)** (o ejecútela como root) para copias de máquina e instantáneas de bloque
3. Añada su servidor PBS en **Preferencias > Información de la cuenta** y pulse **Probar**
4. Cree un conjunto de copias de seguridad (o una copia puntual) y seleccione las carpetas o discos que copiar
5. Ejecútelo. Para que las programaciones se ejecuten sin nadie con la sesión iniciada, active **Preferencias > Avanzado > Ejecutar como servicio**

El [manual de la GUI](docs/manual/GUI.md) explica cada paso.

## Copia de máquina completa en vivo

`proxmoxbackup-machine` (y las copias de máquina de la GUI) copian un sistema Windows 10/11 o Windows Server completo, o un equipo Linux, sin ninguna interrupción.

La sintaxis del comando es casi la misma que para las copias de carpetas, salvo que `-backupdir` se sustituye por `-backupdev` (repítalo para varios discos).
Por ejemplo, una invocación podría ser:

`proxmoxbackup-machine.exe -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

El comando anterior examina el disco 0, detecta todas las particiones montadas, crea una instantánea VSS de ellas y luego genera una imagen de copia arrancable de todo el disco como fidx. El espacio libre y los archivos de paginación e hibernación entran en la imagen como ceros sin leerse (en Linux también el espacio libre de ext2/3/4 y XFS); `-readfreespace` lo lee todo, para una copia forense. Un sector que no se puede leer entra en la imagen como ceros, y la ejecución indica dónde. Añada `-type vm` para guardar la copia como copia de VM de Proxmox VE ([P2V](#-de-máquinas-físicas-a-máquinas-virtuales-p2v)).

La siguiente copia será incremental; el cálculo de hash se ha paralelizado, así que se alcanzan con facilidad velocidades de 1 GB/s.

### Requisitos de la copia de máquina en Linux (equivalente a VSS)

La copia de máquina en Windows usa VSS (de ahí el «Ejecutar como administrador» de arriba); Linux no tiene VSS, así que este
proyecto usa el módulo del kernel `elastio-snap` (un fork de `dattobd`) para la misma tarea: una
instantánea coherente en un momento dado de un dispositivo de bloque en vivo mientras se sigue escribiendo en él. El repositorio original
`elastio/elastio-snap` está archivado; la versión mantenida es
[`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap), que es la que hay que instalar. El propio `dattobd`
también es compatible si ya está cargado.

Las dos condiciones siguientes son obligatorias, cada una por separado, o la copia de máquina en Linux se niega a ejecutarse:

1. **El módulo del kernel debe estar instalado y cargado.** La instalación documentada de `elastio-snap` mediante el paquete
   del repositorio (véase [su INSTALL.md](https://github.com/Axcient/elastio-snap/blob/develop/INSTALL.md)) apunta actualmente
   a una URL movida o rota al menos para Ubuntu 22.04 (confirmado el 2026-09-24: el paquete del repositorio
   da 404 tras una redirección). Compilar desde el código fuente funciona de forma fiable:
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
   Esto lo compila y lo carga solo para el arranque actual; consulte la documentación de su distribución para cargarlo automáticamente
   en cada arranque (p. ej. una entrada en `/etc/modules-load.d/`) si lo desea.

2. **El propio proceso de copia debe ejecutarse como root (EUID 0).** Incluso con el módulo cargado, `CreateVSSSnapshot`
   (la ruta de instantáneas de Linux a pesar de su nombre centrado en Windows) se niega expresamente a ejecutarse como usuario no root:
   una instantánea coherente de disco completo necesita la interfaz de control del módulo, reservada a root, así que es un requisito
   estricto, no algo que pueda sustituir la pertenencia al grupo `disk` o similar. Para una copia de máquina, use
   la entrada de menú **Proxmox Backup Client (administrator)**, el servicio en segundo plano, o ejecute la GUI o
   `proxmoxbackup-machine` con `sudo` (o como root); una copia normal de archivos o carpetas no tiene ese requisito.

Si una copia de máquina falla con `backup device /dev/sdX: open /dev/sdX: permission denied`, normalmente
basta con añadir su usuario al grupo `disk` para el acceso básico de lectura (`sudo usermod -aG disk $USER`, y luego cerrar
y volver a abrir la sesión), pero eso por sí solo NO basta para una instantánea real; se topará con el requisito de root anterior
inmediatamente después.

### Restaurar archivos de una copia de disco

La página Restaurar de la GUI abre una copia de máquina como una copia de carpetas: cada volumen de la imagen (NTFS, FAT, exFAT, ext2/3/4 o XFS) es una carpeta que se puede explorar y desde la que se puede restaurar. No se monta nada.

En Linux también puede conectar una copia de disco como dispositivo de bloque con la herramienta nbd.

Para usar nbd, ejecute primero `modprobe nbd max_part=0`.

Por razones desconocidas, usar `max_part != 0` provoca un bucle infinito de sondeo de particiones.

La herramienta NBD conecta cualquier copia de disco fijo, sea una copia de VM o de host (dicho esto, también funciona con copias de PVE).

Para usarla, use una línea de comandos similar a esta:
`./proxmoxbackup-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

Si omite `-path`, aparece una interfaz de terminal para elegir el archivo fidx.

> No lo use en un equipo que ejecute algo importante: un sistema de archivos dañado puede bloquear el sistema operativo, y por eso Proxmox VE usa una instancia de QEMU para esto.

Desmonte todo lo que haya en el disco nbd antes de detener `proxmoxbackup-nbd`, o es probable que la partición quede ocupada y sea imposible desmontarla.

Si obtiene un error `Device or resource busy`, tiene que forzar la desconexión ejecutando `nbd-client -d /dev/nbd0` o simplemente reiniciar.

### Restaurar en una máquina física o en una VM

La ISO de restauración bare-metal tiene una entrada del menú de arranque **«PBS Bare Metal Restore»** totalmente automática: véase [ISO de restauración bare-metal](#-iso-de-restauración-bare-metal) más abajo. Es la primera entrada, la elegida por defecto al agotarse el tiempo, y omite las preguntas de idioma y teclado del propio Clonezilla: introduzca los datos de conexión a PBS, elija una instantánea, confirme el disco de destino detectado, y se encarga de la configuración de red, la conexión NBD, la propia restauración y un reinicio automático sin más intervención. La misma ISO restaura en una máquina virtual vacía para [P2V](#-de-máquinas-físicas-a-máquinas-virtuales-p2v).

La ruta semimanual anterior también sigue incluida en la misma ISO, como entrada propia en el menú normal de selección de modo de Clonezilla («pbs-nbd»): le guía por los datos de conexión a PBS y conecta en segundo plano la instantánea elegida a `/dev/nbd0`, y después le deja hacer la restauración a mano con el asistente estándar de restauración de discos del propio Clonezilla.

## Uso: copia de directorios

La referencia completa, con ejemplos para Windows y Linux, códigos de salida y programación, está en el **[manual de la línea de comandos](docs/manual/CLI.md)**. Un comando típico:

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

También se leen las variables de entorno `PBS_REPOSITORY`, `PBS_SERVER`, `PBS_PORT`, `PBS_DATASTORE`, `PBS_AUTH_ID`, `PBS_PASSWORD` y `PBS_FINGERPRINT` del cliente oficial.

Para la configuración JSON se incluye un ejemplo JSON; rellene solo los campos necesarios.

Nota sobre las plantillas de correo:
se usa el [motor de plantillas de Go](https://pkg.go.dev/text/template) para los asuntos y cuerpos de los correos; consulte su documentación para la sintaxis.
Las siguientes variables están disponibles en las plantillas:

- `.NewChunks`: número de fragmentos nuevos creados
- `.ReusedChunks`: número de fragmentos reutilizados
- `.Datastore`: nombre del almacén de datos
- `.Error`: el error, si lo hay (`.ErrorStr` lo da como texto, vacío cuando no hay ninguno)
- `.ReadErrors`: los archivos o carpetas que no se pudieron leer; `.ReadErrorCount` indica cuántos
- `.Hostname`: nombre de host del equipo
- `.StartTime`: hora de inicio de la copia
- `.EndTime`: hora de fin de la copia
- `.Duration`: duración de la copia
- `.FromattedDuration`: duración formateada de la copia (el nombre se escribe así en el código)
- `.Success`: verdadero cuando la copia terminó sin errores y sin nada ilegible
- `.Partial`: verdadero cuando se guardó una instantánea pero algunos archivos no se pudieron leer
- `.Status`: `Success`, `Partial` o `Failed`

## Copia de flujos

Permite copiar un flujo en lugar de un pxar, lo que abre infinitas posibilidades. Por ejemplo, puede ejecutar:

```
mysqldump yourdatabase | ./proxmoxbackup-directory -backupstream yourdatabase.sql [other options]
```

Esto permite aprovechar buzhash para la deduplicación incluso usando tar, por ejemplo, o el propio volcado SQL.

## Problemas conocidos

Con el antimalware de Windows Defender activo, la copia se ralentiza hasta un 25 % de la velocidad alcanzable.

En Windows las herramientas de línea de comandos toman un bloqueo de proceso: una segunda copia iniciada mientras otra está en curso muestra «Another backup job is running» y termina. En Linux no existe ese bloqueo, así que no programe dos ejecuciones de la CLI a la misma hora.

## 📋 Requisitos previos

- Windows 10/11 o Windows Server (64 bits), con derechos de administrador para VSS y copias de máquina
- o Linux (los paquetes son para Debian, Ubuntu, Mint y Fedora), con el entorno de ejecución WebKitGTK para la GUI, y root más el módulo del kernel elastio-snap o dattobd para copias de máquina coherentes
- Acceso de red a un Proxmox Backup Server
- Para P2V: Proxmox VE con el almacén de datos PBS añadido como almacenamiento, o cualquier hipervisor que pueda arrancar la ISO de restauración

## 🔨 Compilar desde el código fuente

**[BUILD.md](BUILD.md)** tiene las instrucciones completas: la GUI y el servicio en segundo plano, las herramientas de línea de comandos, el MSI y los paquetes Linux. En resumen, con Go 1.25 o posterior y Wails CLI v2.13:

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

La interfaz compilada (`gui/frontend/dist`) está incluida en el repositorio, así que Node.js solo hace falta para modificar la interfaz (`npm install` y `npm run build` en `gui/frontend`, o `wails dev` para la recarga en caliente). Pruebas: `make test`.

## 🔧 Uso avanzado y guías

### Multi-PBS (varios servidores PBS)

Configure varios servidores PBS y elija el destino para cada conjunto o copia puntual (p. ej. `C:\Users` a un PBS rápido con SSD a diario, `D:\` a un PBS de gran capacidad cada semana, más un servidor de recuperación ante desastres).

- **[Guía de usuario](MULTI_PBS_USER_GUIDE.md)**: añadir y probar servidores, el servidor por defecto, elegir un servidor por copia, preguntas frecuentes y solución de problemas.
- **[Guía de implementación](MULTI_PBS_GUIDE.md)**: modelo de datos, migración automática desde una configuración de un solo PBS, métodos del backend.

Una configuración antigua de un solo PBS se migra automáticamente a un servidor `default` en la primera carga.

### 💿 ISO de restauración bare-metal

La ISO de restauración es una ISO estándar de Clonezilla Live parcheada con los binarios `pbsnbd` / `machinebackup` más dos entradas de menú. Arranca desde CD, desde una memoria USB (escrita con `dd`, o desde una memoria Ventoy), por PXE, y en equipos UEFI o BIOS:

- **`ocs-pbs-bare-metal-restore`**: la primera entrada del menú de arranque, elegida por defecto al agotarse el tiempo, **«Proxmox Backup Client Go - PBS Bare Metal Restore»**. Omite las preguntas de idioma y teclado de Clonezilla y su asistente genérico, y va directamente de las credenciales de PBS a una restauración completada con reinicio automático: primero DHCP (una petición de IP estática solo si eso falla), sincronización NTP, una clave de cifrado desde una memoria USB si la copia está cifrada, y una detección del disco de destino que nunca ofrece la memoria desde la que arrancó. Si todo va bien muestra un mensaje de finalización y reinicia. Restaura copias de Windows y de Linux por igual (la restauración es a nivel de bloque), en una máquina física o en una VM vacía.
- **`ocs-pbs-nbd`**: una entrada **«pbs-nbd»** en el propio menú de selección de modo de Clonezilla. Pide los datos de conexión a PBS y la instantánea que restaurar, la conecta a `/dev/nbd0` en segundo plano y después pasa al asistente normal de restauración de discos de Clonezilla para que lo maneje usted a mano.

También corregido en la parte de restauración: un bloqueo mutuo de `zstd.NewReader` en la ruta de obtención de fragmentos de PBS que colgaba las restauraciones de máquina completa (aportado al proyecto original como [PR #85](https://github.com/tizbac/proxmoxbackupclient_go/pull/85)), un error de «la primera pasada de restauración falla con 'no partition' y la repetición funciona» causado por una condición de carrera entre `is_disk_without_part_and_fs()` del propio Clonezilla y udev, y equipos BIOS/MBR que no arrancaban tras una restauración (ahora también se restauran los datos entre el MBR y la primera partición).

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-37-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

Todos los detalles (por qué una reconstrucción completa de la ISO en lugar de un reemplazo en el sitio, requisitos previos, flujo del menú, verificación) en **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**; su uso, en el **[manual de restauración bare-metal](docs/manual/BMR.md)**.

### Compilar la GUI de Windows

**Windows nativo**: consulte **[BUILD.md](BUILD.md)** para la cadena de herramientas (Go, MinGW, Wails) y los comandos para la aplicación, el servicio y el MSI.

**Docker (al compilar en Linux).** `./build_gui_windows_docker.sh` compila la aplicación de Windows como `ProxmoxBackupClientGO.exe`, con soporte correcto de WebView2, en un contenedor `golang` desechable (instala mingw y Wails, compila la interfaz y ejecuta `wails build`). No compila el servicio en segundo plano ni el MSI.

### Estado de las funciones, registro de cambios y documentación interna

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)**: qué está hecho, parcial o pendiente, y desde qué versión.
- **[CHANGELOG.md](CHANGELOG.md)**: historial de cambios por versión, y [Unreleased] para lo que está en master.
- **[TODO.md](TODO.md)**: hoja de ruta abierta e ideas.
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)**: qué contiene cada versión.
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**: diálogo de desinstalación del MSI (conservar o borrar la configuración) y su plan de pruebas.
- **[AGENTS.md](AGENTS.md)**: estructura del repositorio y convenciones, para colaboradores y agentes de programación.

## 🔁 Contribuciones al proyecto original

Las correcciones y funciones de uso general se ofrecen a [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go) como pull requests. Se han incorporado dieciocho:

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

## 🧪 Estado, informes de errores y solicitudes de funciones

**Este es software alfa, proporcionado tal cual, sin garantía de ningún tipo, y se usa enteramente bajo su propia responsabilidad.** No asumo ninguna responsabilidad por datos perdidos o dañados, incluidos los archivos de origen.

Está hecho para mi propio homelab. Pruebo yo mismo cada versión antes de publicarla, y se usa allí con regularidad: copias programadas de carpetas y de máquina de equipos Windows y Linux, con restauraciones, Recuperación, Reversión, restauraciones bare-metal y restauraciones P2V probadas por el camino. Ninguna copia está demostrada hasta que se ha restaurado desde ella, así que pruebe sus propias restauraciones, incluida una bare-metal, antes de confiar en él.

- **Los informes de errores son bienvenidos**: abra una [incidencia](https://github.com/mjb-is/proxmoxbackupclient_go/issues) con lo que hizo, lo que ocurrió y el registro (el Registro de mensajes, o los archivos de la carpeta de configuración).
- **Solicitudes de funciones**: añado funciones que ayudan a mi propio uso, así que no acepto peticiones para otros usos. Envíelas al proyecto original, [tizbac/proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go/issues). Los pull requests que corrigen errores, o que claramente también ayudarían a mi uso, son bienvenidos.

Los informes de errores y las correcciones son la ayuda más útil. Si quiere dar las gracias de otra forma, puede [invitarme a un café en Ko-fi](https://ko-fi.com/mickbeeby).

## 🙏 Créditos e historia

El cliente de línea de comandos original lo escribió Tiziano Bacocco (tizbac) a partir de 2023. Más tarde se le incorporó el cliente gráfico de RDEM Systems, y con él la primera GUI, el soporte multi-PBS y el servicio de Windows. Este fork empezó en septiembre de 2026 para cubrir las carencias que impedían que PBS fuera mi único sistema de copias de seguridad; sus versiones empiezan en v0.3.0, y las correcciones de uso general vuelven al proyecto original.

- El cliente original, [proxmoxbackupclient_go](https://github.com/tizbac/proxmoxbackupclient_go), de Tiziano Bacocco (tizbac) y sus colaboradores.
- El cliente gráfico Nimbus Backup de RDEM Systems, punto de partida de la interfaz gráfica de esta aplicación.
- El flujo de trabajo y el estilo de Backup for Workgroups (BFW), que siguen los conjuntos de copias de seguridad, Recuperación y Reversión.
- [Clonezilla Live](https://clonezilla.org/), sobre la que se construye la ISO de restauración bare-metal.

## 📄 Licencia

GPLv3: consulte el archivo [LICENSE](LICENSE).

---

**Cambios desde el fork © 2026 mjb-is, sobre el trabajo de Tiziano Bacocco (tizbac), los colaboradores de Proxmox Backup Client GO y RDEM Systems (© 2024-2026).**

☕ Si este cliente le resulta útil, puede [invitarme a un café en Ko-fi](https://ko-fi.com/mickbeeby). Gracias.
