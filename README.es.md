# Salvanza for Proxmox Backup Server: cliente para Windows y Linux de Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

> ℹ️ Esta es una traducción del [README en inglés](README.md). Si difieren, la versión en inglés es la actualizada.

> ⚠️ **Aviso sobre marcas y afiliación.** Este es un **proyecto comunitario independiente**, no un producto oficial de Proxmox: **no está afiliado a Proxmox Server Solutions GmbH, ni respaldado ni patrocinado por ella**. **Proxmox®** es una marca registrada de Proxmox Server Solutions GmbH en la UE, en EE. UU. y en otros países, y aquí se usa **solo** para describir la compatibilidad. Siguiendo las directrices de Proxmox sobre el uso de sus marcas (las marcas registradas no deben usarse en nombres de productos o aplicaciones, por ejemplo "XY App for Proxmox Virtual Environment" en lugar de "Proxmox XY App"), **el cliente sin marca ha cambiado de nombre**: el producto antes llamado *Proxmox Backup Client* ahora se llama **Salvanza for Proxmox Backup Server** (Salvanza, para abreviar), un nombre que no contiene la marca Proxmox. Las reglas exactas están en el [kit de prensa de Proxmox](https://www.proxmox.com/en/about/company-details/media-kit).

[![Licencia](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Versión](https://img.shields.io/github/v/release/salvanza/salvanza_client)](https://github.com/salvanza/salvanza_client/releases)
[![Documentación](https://img.shields.io/badge/docs-manuals-orange)](docs/manual/README.md)

**Salvanza for Proxmox Backup Server (antes Proxmox Backup Client) es un cliente de copias de seguridad de código abierto (GPL-3.0) para Proxmox Backup Server (PBS), para Windows y Linux.** Hace copias de seguridad de carpetas como archivos de ficheros explorables y de máquinas enteras como imágenes de disco arrancables, según una programación o a petición. Restaura cualquier cosa, desde un solo archivo hasta una máquina entera: en el mismo equipo, en uno de sustitución o en una máquina virtual de Proxmox VE.

Sitio del proyecto: **[salvanza.org](https://salvanza.org/)** · Código fuente, versiones e incidencias: **[github.com/salvanza/salvanza_client](https://github.com/salvanza/salvanza_client)**

Es un **conjunto de herramientas**:

- **La aplicación Salvanza**, `salvanza-gui` (creada por RDEM Systems como Nimbus Backup GUI, rediseñada y ampliada por mjb-is): la aplicación para Windows y Linux. Conjuntos de copia de seguridad con varias programaciones cada uno, copias de carpetas y de discos enteros con instantáneas coherentes (VSS en Windows, elastio-snap/dattobd en Linux), restauración de archivos desde copias de carpetas y de máquinas, Recuperación y Reversión, varios servidores PBS, cifrado en el cliente y un servicio en segundo plano, `salvanza-svc`, para que las programaciones se ejecuten sin que nadie haya iniciado sesión.
- **`salvanza-cli`**, una única herramienta de línea de comandos con un subcomando para cada tarea:
  - **`salvanza-cli backup-folder`**: copias de seguridad de carpetas (pxar) con deduplicación y detección de cambios.
  - **`salvanza-cli backup-disk`**: copias en caliente de discos enteros (fidx) de máquinas Windows y Linux. La imagen es arrancable tras una restauración.
  - **`salvanza-cli attach-nbd`**: conecta una copia de disco como dispositivo de bloques NBD (Linux).
- **Una ISO de restauración bare-metal** (Salvanza-BMR) basada en Clonezilla Live, con una restauración guiada desde el encendido hasta el reinicio.

> Palabras clave: salvanza · cliente proxmox backup server windows · cliente PBS · copia de seguridad VSS de Windows · copias externas inmutables · físico a virtual (P2V) · restauración bare-metal · interfaz de Proxmox Backup Server.

<a id="-new-in-20"></a>
## 💡 Por qué usar este cliente

El `proxmox-backup-client` oficial es una herramienta de línea de comandos para Linux. Este cliente trabaja con el mismo servidor y los mismos formatos de copia, y añade:

- **Windows.** No existe un cliente oficial para Windows. Aquí hay herramientas de línea de comandos y una aplicación, copias de carpetas mediante instantáneas VSS y copias en caliente de discos enteros.
- **Una aplicación para Windows y Linux**: conjuntos de copia de seguridad programados, ejecutados por un servicio en segundo plano sin que nadie tenga que iniciar sesión, restauración con búsqueda, Recuperación y Reversión, informes y correo electrónico.
- **Máquinas enteras**: imágenes de disco arrancables, archivos sueltos recuperados de ellas y una ISO de restauración bare-metal guiada, en el mismo equipo, en hardware nuevo o en una máquina virtual de Proxmox VE.
- **Se conservan los detalles de los archivos de Windows**: permisos NTFS, atributos, flujos de datos alternativos, atributos extendidos y fechas de creación se registran con cada copia de carpeta, y la restauración de este cliente los vuelve a aplicar.
- **Sin ataduras**: lo que copia este cliente se restaura con el cliente oficial, y lo que copia el cliente oficial se restaura con este (vea más abajo).

## 🆕 Novedades de la 2.0

La versión 2.0 incorpora al proyecto un gran volumen de trabajo en la interfaz gráfica, el servicio en segundo plano y el motor de copias de seguridad (consulte el [CHANGELOG](CHANGELOG.md)). En comparación con la 1.2:

- **La aplicación se conecta al servicio en segundo plano con derechos de administrador.** Con el servicio instalado, los servidores PBS y los conjuntos de copia de seguridad del equipo pertenecen al servicio, y solo el servicio lee su carpeta de configuración. La aplicación pide una vez derechos de administrador para conectarse (UAC en Windows, pkexec en Linux; en una cuenta estándar, el nombre y la contraseña de un administrador). Solo se eleva un pequeño programa auxiliar, y la aplicación guarda el token del servicio únicamente en memoria. Un usuario que no pueda responder a la petición puede elegir **Usar mi propia configuración** y conservar sus propios servidores y conjuntos de copia de seguridad, que se ejecutan mientras la aplicación está abierta; más tarde, **Conectar** ofrece copiarlos a los del equipo. La propia aplicación se ejecuta sin derechos de administrador y ofrece **Reiniciar como administrador** cuando una operación lo necesita. Consulte la sección 15 del [manual de la interfaz gráfica](docs/manual/GUI.md#connecting-to-the-background-service).
- **La aplicación comprueba que habla con el servicio.** Antes de enviar nada a la API local del servicio, la aplicación comprueba quién responde: el propio proceso del servicio en Windows, un socket propiedad de root en Linux. Un programa que ocupe el puerto mientras el servicio está detenido no recibe nada.
- **Conjuntos de copia de seguridad con varias programaciones cada uno**, desde cada pocos minutos hasta una vez al año (el último día del mes, el 2.º martes), cada una descrita en lenguaje claro con sus 3 próximas ejecuciones.
- **Copias incrementales de carpetas** con la detección de cambios del cliente oficial (modos Datos y Metadatos): los archivos sin cambios no se vuelven a leer.
- **Copias de máquina más rápidas**, que omiten el espacio libre, el archivo de paginación y el archivo de hibernación, y escriben ceros en lugar de los sectores ilegibles en vez de fallar.
- **Varias copias a la vez** (**Número máximo de copias de seguridad simultáneas**, de 1 a 8, 3 por defecto), también en el servicio en segundo plano, cada una con su propia instantánea, su registro, Pausar y Detener.
- **Límites de velocidad** para la subida a PBS, la descarga desde PBS y la lectura de disco, cada uno un único presupuesto compartido por todo lo que se ejecuta al mismo tiempo.
- **Recuperación y Reversión** para conjuntos de copia de seguridad de carpetas y de máquinas, en la interfaz gráfica y desde la línea de comandos.
- **Archivos recuperados de copias de máquina**: explore volúmenes NTFS, FAT, exFAT, ext2/3/4 y XFS dentro de una imagen de disco y restaure archivos individuales.
- **Claves de cifrado que puede conservar**: guarde una copia, imprima una clave en papel con un código QR e importe una clave desde un archivo, la cámara, una foto o un texto escrito.
- **Los conjuntos de copia de máquina identifican sus discos por su identidad**, de modo que un disco que recibe un número nuevo tras un cambio de hardware sigue siendo el que se copia, y un disco que falta hace fallar la ejecución en lugar de copiar otro disco.
- **Se pueden excluir particiones de una copia de máquina**: marque bajo cada disco las particiones que quiere copiar. Una partición excluida no se lee ni se incluye en la instantánea, y entra en la imagen como ceros, de modo que una restauración la devuelve en su lugar y con su tamaño, vacía y sin formatear. El selector de discos señala las particiones que un disco restaurado necesita para arrancar.
- **Copias de máquina Linux con elastio-snap o dattobd.** Un conjunto de máquina Linux puede desactivar las instantáneas y leer los discos en caliente, pero solo si su propietario marca que acepta que la copia pueda ser incoherente.
- **Cada usuario puede elegir su propio tema de colores** en la aplicación sin marca. Una aplicación con marca mantiene los colores de su marca.
- **Habilitar y deshabilitar un conjunto de copia de seguridad** desde su editor además de desde su menú, y un aviso al guardar cuando un conjunto no se ejecutaría tal como está guardado (deshabilitado, sin ninguna programación activa, un problema con la instantánea, un disco que falta).
- **Una única herramienta de línea de comandos, `salvanza-cli`**, con los subcomandos `backup-folder`, `backup-disk` y `attach-nbd` en lugar de las tres herramientas separadas; sus nombres anteriores siguen funcionando.
- **Paquetes para Linux** (`.deb`, `.rpm` y Arch), con el paquete `salvanza` y el servicio `salvanza.service`, junto con el servicio en segundo plano.
- **Una ISO de restauración bare-metal guiada**, que también restaura en una máquina virtual vacía (P2V). Una máquina copiada como máquina virtual de Proxmox VE también puede restaurarse directamente desde PBS en Proxmox VE, como cualquier VM que el propio Proxmox VE haya copiado.
- **Compilaciones con marca**: los mismos binarios se distribuyen como instaladores de Salvanza, Nimbus Backup y Etitech Backup, cada uno con sus propios nombres de programa y carpetas de configuración.
- **18 idiomas de interfaz.**

## 🔁 Actualizar desde Proxmox Backup Client

Salvanza es el nuevo nombre de Proxmox Backup Client, y la actualización es automática. Al instalar Salvanza (el MSI o el paquete para Linux) se quita la instalación anterior de Proxmox Backup Client, o la de otra marca, y su servicio en segundo plano. La primera vez que se inicia, Salvanza traslada la configuración anterior (servidores PBS, claves, conjuntos de copia de seguridad e historial) desde `C:\ProgramData\ProxmoxBackupClient`, `%APPDATA%\ProxmoxBackupClient`, `~/.proxmox-backup-guardian` o `/var/lib/proxmoxbackupclient`, y guarda una copia fechada de los archivos de configuración en la nueva carpeta, en `pre-salvanza-<date>`. Sus copias de seguridad en PBS no se tocan.

| Antes | Ahora |
|---|---|
| `ProxmoxBackupClient.exe`, `ProxmoxBackupClientSVC.exe` | `salvanza-gui.exe`, `salvanza-svc.exe` (servicio de Windows `salvanza-svc`) |
| `proxmoxbackup-directory`, `proxmoxbackup-machine`, `proxmoxbackup-nbd` | `salvanza-cli backup-folder`, `salvanza-cli backup-disk`, `salvanza-cli attach-nbd` (los nombres anteriores siguen funcionando) |
| `C:\Program Files\ProxmoxBackupClient`, `C:\ProgramData\ProxmoxBackupClient` | `C:\Program Files\Salvanza`, `C:\ProgramData\Salvanza` |
| Paquete y servicio para Linux `proxmoxbackupclient` | Paquete `salvanza`, servicio `salvanza.service`, configuración en `/var/lib/salvanza`, la suya propia en `~/.salvanza` |

## ⭐ Qué hace, con resultados medidos

| | Resultado |
|---|---|
| **Copias incrementales de carpetas** | Un recurso compartido de 480,9 GB con 761.866 archivos: primera ejecución 4 h 17 min, cada ejecución posterior de 10 a 15 minutos. Los archivos sin cambios ni siquiera se abren (detección de cambios Metadatos, como en el cliente oficial). |
| **Copias de máquina rápidas** | El espacio libre, el archivo de paginación y el archivo de hibernación no se leen, y el espacio vacío del disco no se somete a hash. El disco de 1 TB de un portátil, con 373 GB ocupados, pasó de tardar entre 1 h 23 min y 1 h 44 min a solo 38 minutos. |
| **Conexiones lentas y de pago por uso** | Solo se envían los datos nuevos, comprimidos, y la aplicación muestra lo que realmente ha pasado por la red. Un pequeño PC Linux en una sede remota con conexión 4G (4,8 Mbit/s de subida): toda la imagen de su disco de 128 GB más su carpeta personal en 11 minutos, con unos 172 MiB transferidos. |
| **Archivos recuperados de copias de máquina** | La unidad C: de 127 GB de un portátil, con 430.000 archivos, se lista en 4 a 15 segundos. No se monta ni se descarga nada antes. |
| **Restauración bare-metal** | Arranque la ISO (memoria USB, CD o red), introduzca los datos de PBS, elija la copia: una máquina Windows de 32 GB volvió a su pantalla de inicio de sesión 8 minutos después del encendido. |
| **Recuperación** | Tras borrar 39 GB de un recurso compartido de 761.000 archivos, una búsqueda de Recuperación en las copias de los últimos 7 días encontró exactamente esos archivos en un minuto aproximadamente y los devolvió a su sitio. |
| **Fidelidad de la restauración** | Permisos NTFS, atributos, flujos de datos alternativos, atributos extendidos, fechas de creación y fechas de carpetas en Windows; modos, propietarios, ACL, atributos extendidos y enlaces simbólicos en Linux. |

## 📖 Manuales de usuario

| Manual | Contenido |
|---|---|
| **[Manual de la interfaz gráfica](docs/manual/GUI.md)** | Instalación, servidores PBS, conjuntos de copia de seguridad y programaciones, copias incrementales, progreso, restauración, Recuperación, Reversión, Informes, registros, Preferencias, el servicio en segundo plano, solución de problemas |
| **[Manual de la línea de comandos](docs/manual/CLI.md)** | `salvanza-cli backup-folder`, `backup-disk`, `attach-nbd`: todas las opciones, archivos de configuración, códigos de salida, programación |
| **[Manual de restauración bare-metal](docs/manual/BMR.md)** | La ISO de restauración, paso a paso; copias `vm` en Proxmox VE; físico a virtual; hardware diferente |
| [Escenarios de restauración](docs/RESTORE_GUIDE.md) | Qué restauración usar y cuándo |
| [Índice de manuales](docs/manual/README.md) | Una visión general de los manuales |

Los mismos manuales están integrados en la aplicación (**Ayuda**, F1), así que funcionan sin conexión y corresponden a la versión instalada.

<a id="-download"></a>
## 📦 Descarga

👉 **[Descargar la última versión](https://github.com/salvanza/salvanza_client/releases)**

| Si tiene | Use |
|---|---|
| Windows | El **instalador MSI**, `Salvanza-<version>-x64.msi`. Instala la aplicación, el servicio en segundo plano y `salvanza-cli` en `C:\Program Files\Salvanza`, añade esa carpeta al PATH, inicia el servicio y añade una entrada en el menú Inicio. Los instaladores con marca (`NimbusBackup-<version>-x64.msi`, `EtitechBackup-<version>-x64.msi`) son la misma compilación con otra marca |
| Debian, Ubuntu, Mint | `sudo apt install ./salvanza_<version>_amd64.deb` |
| Fedora | `sudo dnf install ./salvanza-<version>-*.rpm` |
| Arch | El paquete generado a partir de [packaging/arch/PKGBUILD](packaging/arch/PKGBUILD) (`make pkgarch`) |
| Solo la línea de comandos | Los archivos comprimidos de `salvanza-cli` por plataforma |
| Una máquina que restaurar | La ISO de restauración bare-metal ([manual de restauración bare-metal](docs/manual/BMR.md#3-get-the-iso-and-make-the-boot-media)), o constrúyala usted mismo ([más abajo](#-bare-metal-restore-iso)) |

Cuando una versión no incluye un paquete para Linux, constrúyalo con `make deb`, `make pkgfedora` o `make pkgarch` ([BUILD.md](BUILD.md), [packaging/debian/README.md](packaging/debian/README.md)).

Los paquetes para Linux instalan una sola entrada de menú, **Salvanza for Proxmox Backup Server**, que inicia la aplicación con su usuario. Una instalación nueva activa el servicio en segundo plano (`salvanza.service`); una actualización conserva la elección que hizo. El paquete `salvanza` sustituye a los anteriores paquetes `proxmoxbackupclient` y `pbsgo`. Los paquetes necesitan polkit (pkexec), que la aplicación usa para conectarse al servicio.

> ⚠️ **¿Windows muestra "virus detectado" (p. ej. `Trojan:Win32/Sabsik.FL.A!ml`) o un aviso de SmartScreen?**
> Es un **falso positivo conocido** en las aplicaciones Go/Wails: *no* es un virus. El sufijo `!ml` indica una detección por un modelo de aprendizaje automático que marca los ejecutables *sin firmar y poco comunes*. Verifique la descarga como se indica a continuación.

### 🔎 Verificar una descarga

Cada versión proporciona sumas de comprobación SHA-256 (`SHA256SUMS.txt`, que también aparecen en la página de la versión) y una **atestación de procedencia firmada**: la prueba criptográfica de que el archivo lo produjo la CI de este repositorio, a partir de un commit concreto.

```powershell
Get-FileHash .\Salvanza-<version>-x64.msi -Algorithm SHA256   # compare with SHA256SUMS.txt
gh attestation verify .\Salvanza-<version>-x64.msi --repo salvanza/salvanza_client
```

En Linux: `sha256sum -c SHA256SUMS.txt --ignore-missing`.

Cuando los instaladores de una versión salen limpios en VirusTotal, la página de la versión enlaza los informes.

> ℹ️ **Firma de código:** los binarios de Windows **aún no están firmados con Authenticode** (hay pendiente un certificado OSS a través de [SignPath Foundation](https://signpath.org)). Mientras tanto, la procedencia se acredita mediante la atestación y las sumas de comprobación anteriores.

## 🚀 Inicio rápido (interfaz gráfica)

1. Instálelo: el MSI en Windows, el paquete `.deb`, `.rpm` o de Arch en Linux ([Descarga](#-download)).
2. Inicie **Salvanza for Proxmox Backup Server**. Los instaladores configuran el servicio en segundo plano, así que la aplicación pide una vez derechos de administrador para conectarse a él (UAC en Windows, su contraseña mediante pkexec en Linux). Si no puede concederlos, elija **Usar mi propia configuración**.
3. Añada su servidor PBS en **Preferencias > Información de la cuenta** y compruébelo con **Probar**.
4. Cree un conjunto de copia de seguridad (o una copia de seguridad puntual) y seleccione las carpetas o los discos que quiere copiar.
5. Ejecútelo. Con el servicio en segundo plano, sus programaciones se ejecutan haya o no alguien con la sesión iniciada.

El [manual de la interfaz gráfica](docs/manual/GUI.md) explica cada paso.

### 🪟 Instalación desatendida / silenciosa (MSI)

El MSI admite la opción silenciosa estándar de Windows Installer, además de propiedades públicas para cada valor predeterminado:

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

| Propiedad | Predeterminado | Significado |
| --- | --- | --- |
| `SERVICEAUTOSTART` | `1` | `0` = servicio instalado como **Manual** y **detenido**: instalado, pero nunca habilitado ni iniciado automáticamente |
| `AUTOSTART` | `1` | `0` = no iniciar la aplicación en la bandeja cuando cada usuario inicia sesión |
| `DESKTOPSHORTCUTS` | `1` | `0` = sin acceso directo en el escritorio (la entrada del menú Inicio se añade siempre) |
| `CONFIGFILE` | | Ruta completa de un archivo guardado con **Preferencias > Exportar configuración...**: sus servidores PBS y conjuntos de copia de seguridad se combinan antes de que se inicie el servicio. Los ID de copia de seguridad formados con el nombre del equipo que exportó se forman con el nombre de cada equipo. Consulte [installer/DEPLOYMENT.md](installer/DEPLOYMENT.md) |
| `KEEP_CONFIG` | `1` | Solo al desinstalar: `0` = borrar también el directorio de estado `%ProgramData%\Salvanza` (el propio de una compilación con marca, como `%ProgramData%\NimbusBackup`) |
| `INSTALLFOLDER` | `C:\Program Files\Salvanza` | Directorio de instalación (también se puede cambiar en la interfaz) |

Ejecútelo con privilegios elevados (administrador); código de salida `0` = éxito, `3010` = éxito + reinicio necesario. Los MSI con marca aceptan exactamente las mismas opciones.

## ✨ Funciones

### Interfaz gráfica
Las funciones principales se enumeran en [Novedades de la 2.0](#-new-in-20). Además:
- 18 idiomas de interfaz: francés, inglés, italiano, alemán, polaco, español, búlgaro, checo, griego, húngaro, letón, lituano, neerlandés, portugués, rumano, eslovaco, turco y ucraniano
- **Copias de seguridad puntuales** de carpetas o de discos enteros, y copias de máquina guardadas como `host` o como copia `vm` de Proxmox VE, con una opción de **copia forense** que también lee el espacio libre
- **Restauración** con búsqueda de archivos (comodines, expresiones regulares, rutas completas), restauración selectiva, ACL NTFS/POSIX, flujos de datos alternativos, fechas de creación de Windows y una pasada de verificación opcional
- **Pausar, Reanudar y Detener** una copia en curso, también desde la bandeja en Windows y para las ejecuciones del servicio en segundo plano
- Tarjeta de progreso en directo (fin previsto, archivos procesados, velocidad de subida, datos transferidos, tasa de compresión), **Informes**, un **Registro de mensajes** y **Ver registros completos**
- Notificaciones por correo electrónico, acciones tras la copia, clonación de conjuntos de copia de seguridad, exportación e importación de la configuración
- Varios servidores PBS, con fijación de la huella del certificado (confianza en el primer uso), inicio de sesión con token de API o con usuario y contraseña
- Cinco temas de colores y un color personalizado, elegidos por cada usuario (en la aplicación sin marca; una aplicación con marca mantiene los colores de su marca)

### Línea de comandos: `salvanza-cli`
- `salvanza-cli backup-folder`: copias de carpetas (pxar) con deduplicación y `-change-detection-mode legacy|data|metadata`
- `salvanza-cli backup-disk`: copias en caliente de discos enteros de máquinas Windows (VSS) y Linux (elastio-snap/dattobd) (fidx, incrementales, espacio libre omitido salvo con `-readfreespace`, `-type vm` para una copia de Proxmox VE, `-skippartition <disk>:<number>` para excluir una partición)
- `salvanza-cli attach-nbd`: servidor NBD para restaurar copias de disco (Linux)
- `salvanza-cli version` muestra la versión, `salvanza-cli help` enumera los subcomandos y `salvanza-cli <command> -help` enumera las opciones de un subcomando
- El programa del servicio, `salvanza-svc` (`salvanza-svc.exe` en Windows; en Linux también la aplicación, `salvanza-gui`): `-list-sets`, `-undelete` y `-rollback` para los conjuntos de copia de seguridad definidos en la interfaz gráfica

Los tres subcomandos aceptan `-keyfile` y `-keyfile-passphrase` para instantáneas cifradas. Los nombres anteriores de las herramientas, `proxmoxbackup-directory`, `proxmoxbackup-machine` y `proxmoxbackup-nbd` (y `pbsdirectorybackup`, `pbsmachinebackup`, `pbsnbd`), siguen ejecutando los mismos subcomandos con las mismas opciones, así que los scripts existentes siguen funcionando. Referencia completa: [manual de la línea de comandos](docs/manual/CLI.md).

### 📸 Capturas de pantalla

<table>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-backup-sets.png" width="280">](docs/screenshots/gui-backup-sets.png)
**Conjuntos de copia de seguridad**: tareas con nombre, con distintivos de programación y de tipo, y Ejecutar ahora en cada una

</td>
<td width="33%">

[<img src="docs/screenshots/gui-backup-progress.png" width="280">](docs/screenshots/gui-backup-progress.png)
**Progreso de la copia de seguridad**: rendimiento, fin previsto, fragmentos nuevos y reutilizados

</td>
<td width="33%">

[<img src="docs/screenshots/gui-restore.png" width="280">](docs/screenshots/gui-restore.png)
**Restaurar**: elija un servidor PBS y un ID de copia de seguridad, liste las instantáneas o búsquelas por nombre de archivo

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-schedule-tab.png" width="280">](docs/screenshots/gui-schedule-tab.png)
**Programaciones**: varias por conjunto, con las 3 próximas ejecuciones

</td>
<td width="33%">

[<img src="docs/screenshots/backup-set-as-vm.png" width="280">](docs/screenshots/backup-set-as-vm.png)
**Copia como VM de Proxmox VE**: el ID de copia de seguridad de PBS se muestra mientras escribe

</td>
<td width="33%">

[<img src="docs/screenshots/gui-reports.png" width="280">](docs/screenshots/gui-reports.png)
**Informes**: historial completo de ejecuciones con un panel de detalle

</td>
</tr>
<tr>
<td width="33%">

[<img src="docs/screenshots/gui-machine-partitions.png" width="280">](docs/screenshots/gui-machine-partitions.png)
**Conjuntos de máquina**: marque bajo cada disco las particiones que quiere copiar

</td>
<td width="33%">

[<img src="docs/screenshots/gui-themes.png" width="280">](docs/screenshots/gui-themes.png)
**Temas**: seis temas de color, elegidos por cada usuario

</td>
<td width="33%">

[<img src="docs/screenshots/gui-languages.png" width="280">](docs/screenshots/gui-languages.png)
**Idiomas**: 18, aquí francés, alemán, italiano, español, polaco y ucraniano

</td>
</tr>
</table>

### Exclusiones inteligentes del sistema (copias de carpetas)
Las copias de carpetas excluyen por sí solas estos elementos:

**Carpetas del sistema:** `System Volume Information` (almacenamiento de VSS, puede superar los 100 GB) y `$RECYCLE.BIN` estén donde estén; `Recovery` solo en la raíz de una unidad; `.pbs-rollback` (la carpeta de seguridad de Reversión).
**Archivos del sistema:** `pagefile.sys`, `hiberfil.sys`, `swapfile.sys` y `DumpStack.log.tmp`.

**Por qué importa:** una unidad puede mostrar 1,03 TB ocupados cuando los archivos reales suman ~141 GB. Sin exclusiones la copia incluiría las instantáneas de VSS (espacio y tiempo desperdiciados); con ellas, el tamaño coincide con los datos reales.

**Recomendación:** use una copia de **carpetas** para las copias a nivel de archivo; use una copia de **máquina** en un conjunto de copia de seguridad aparte para la restauración bare-metal y el P2V (una imagen de disco entero, en la que el espacio libre y los archivos de paginación e hibernación entran como ceros).

### Seguridad y calidad
- La carpeta de configuración del servicio solo es accesible para SYSTEM y los administradores (Windows) o solo para root (Linux); su API local escucha en 127.0.0.1 y exige un token, que se sustituye en cada inicio y cada 24 horas
- La aplicación no envía nada a esa API hasta comprobar que responde el propio servicio (el proceso del servicio en Windows, un socket propiedad de root en Linux)
- En Windows, los tokens y contraseñas de PBS de `config.json` se cifran para el equipo (DPAPI)
- Validación de entradas, saneamiento de credenciales, prevención de path traversal, reintentos con espera exponencial
- Pruebas unitarias en cada módulo, además de pruebas en vivo opcionales y pruebas de extremo a extremo contra un PBS real

## 🔐 Cifrado en el cliente

Las instantáneas pueden cifrarse antes de salir de la máquina, usando un archivo de clave de Proxmox Backup Server (el formato que escribe `proxmox-backup-client key create`, así que las claves son intercambiables con el cliente oficial). Cada fragmento se cifra con AES-256-GCM y el manifiesto se firma. El servidor PBS nunca ve la clave.

**Interfaz gráfica:** abra **Preferencias > Información de la cuenta**, edite un servidor y use su pestaña **Cifrado**. Con **Importar...** carga una clave que ya tenga (una copia del archivo de clave, o una clave en papel leída con la cámara, desde un escaneo o una foto, o escrita a mano) y con **Generar...** crea una nueva. La pestaña muestra la huella de la clave. **Guardar una copia…** e **Imprimir una clave en papel…** (con un código QR) le permiten guardar una copia lejos del equipo. Los informes indican si cada copia estaba cifrada. **Sin el archivo de clave no se puede restaurar una instantánea: guarde una copia en un lugar seguro, lejos de esta máquina.**

**Claves protegidas con frase de contraseña (scrypt o PBKDF2):** **Pedírmela** guarda la frase de contraseña en memoria durante una sesión de la aplicación; **Recordar en este equipo** la almacena como el token de PBS, y es la opción adecuada para las programaciones y el servicio en segundo plano.

**Línea de comandos:**
```shell
salvanza-cli backup-folder -baseurl ... -backupdir "C:\data" -datastore store -keyfile "C:\keys\pbs-key.json" -keyfile-passphrase "..."
```
`-keyfile-passphrase` (o `PBS_ENCRYPTION_PASSWORD`) solo hace falta para una clave protegida, y si se omite se pide la frase de contraseña. `salvanza-cli backup-disk` y `salvanza-cli attach-nbd` aceptan las mismas opciones. `attach-nbd` comprueba la clave contra la instantánea antes de tocar el dispositivo NBD, así que una clave incorrecta falla de inmediato.

**Restauración bare-metal:** la ISO de restauración pide la clave (ninguna, un archivo de clave en una memoria USB o una ruta escrita) y la frase de contraseña de una clave protegida. La clave nunca se escribe en la ISO.

Restaurar una instantánea cifrada requiere el mismo archivo de clave; si se pierde, no hay forma de recuperarla.

## Copias de carpetas más rápidas: detección de cambios

En el modo predeterminado Clásico, cada copia de carpeta lee todos los archivos, así que una ejecución tarda lo mismo que leer la carpeta entera aunque no haya cambiado nada. Los conjuntos de copia de seguridad y `salvanza-cli backup-folder` pueden usar en su lugar los modos de detección de cambios del `proxmox-backup-client` oficial:

| Modo | Lee | Se guarda como |
|---|---|---|
| **Clásico** (Legacy, predeterminado) | todos los archivos | un `<name>.pxar.didx` por carpeta |
| **Datos** (Data) | todos los archivos | un archivo dividido: `<name>.mpxar.didx` (metadatos) + `<name>.ppxar.didx` (contenido de los archivos) |
| **Metadatos** (Metadata) | solo los archivos nuevos y modificados | el mismo archivo dividido; los archivos cuyo tamaño, fecha de modificación (al nanosegundo), modo y propietario coinciden con la copia anterior se toman de ella sin leerlos |

En modo **Metadatos** la primera ejecución lo lee todo; las siguientes solo leen lo que ha cambiado. Un archivo modificado de forma que conserve su tamaño y su fecha de modificación no se detecta, y por eso un conjunto también puede **leer todos los archivos cada N ejecuciones**. Dé a cada conjunto de copia de seguridad su propio ID de copia de seguridad: el modo Metadatos compara con la instantánea más reciente del grupo de copias. En la línea de comandos use `-change-detection-mode`.

**Antes de pasar un conjunto a Datos o Metadatos,** actualice a la 2.0 todas las máquinas desde las que restaura: las versiones anteriores muestran las carpetas de un archivo dividido sin sus archivos. Consulte la [sección 7 del manual de la interfaz gráfica](docs/manual/GUI.md#7-incremental-folder-backups-change-detection).

<a id="-physical-machines-into-virtual-machines-p2v"></a>
## 🔄 De máquinas físicas a máquinas virtuales (P2V)

La copia de máquina de un PC o servidor físico puede convertirse en una máquina virtual de Proxmox VE de dos maneras:

1. **Restaurarla en Proxmox VE.** Configure el conjunto de copia de seguridad (o la copia puntual) con **Copia de seguridad como: Máquina virtual de Proxmox VE**. La copia se guarda como `vm/<number>`, así que Proxmox VE la muestra junto a sus otras copias de VM y la restaura como una VM (un disco SATA; añada una tarjeta de red antes de iniciarla).
2. **Restaurarla con la ISO de restauración en una VM vacía** con el mismo firmware que la máquina original, un disco SATA al menos igual de grande y una tarjeta de red Intel E1000. Esto funciona igual para copias `host` y `vm`, y la misma copia se sigue pudiendo restaurar en hardware físico.

Entre los recorridos probados están Windows 10 y Windows Server (UEFI/GPT), equipos de escritorio Linux (UEFI/GPT y BIOS/MBR) y una copia Linux cifrada con su clave en una memoria USB. El [manual de restauración bare-metal](docs/manual/BMR.md#7-restoring-onto-different-hardware-p2v-and-replacement-machines) contiene la tabla completa, las opciones de firmware, los controladores y las instrucciones paso a paso.

## 🤝 Compatible con el cliente oficial de Proxmox, en ambos sentidos

Verificado con `proxmox-backup-client` 3.4.9 y PBS 4.2:

- **Las instantáneas Clásico, Datos y Metadatos creadas por este cliente se restauran idénticas byte a byte con el cliente oficial**, tanto desde la interfaz gráfica como desde la línea de comandos, creadas en Windows o en Linux, y el explorador de archivos de PBS 4.2 las abre.
- **Las instantáneas Clásico, Datos y Metadatos creadas por el cliente oficial se restauran idénticas byte a byte con este cliente**, completas o de forma selectiva.
- El mismo formato de archivo dividido (pxar versión 2), los mismos modos de detección de cambios, el mismo formato de archivo de clave de cifrado, las mismas variables de entorno al estilo de `PBS_REPOSITORY`.

Sus copias nunca quedan atadas a este cliente. Dos añadidos acompañan a los archivos estándar, y el cliente oficial restaura los archivos con normalidad sin ellos:

- `.proxmox_backup_client_meta.json`, en la raíz de una instantánea de carpeta, registra la ruta original. El cliente oficial lo restaura como un archivo normal.
- `proxmox-client-acls.json.gz.blob`, guardado con cada instantánea de carpeta, registra lo que el archivo de copia no puede contener en Windows (permisos NTFS, atributos, fechas de creación, flujos de datos alternativos y atributos extendidos) y, en Linux, los atributos extendidos y las ACL POSIX. El cliente oficial y la interfaz web de PBS lo omiten; la restauración de este cliente los vuelve a aplicar.

En Linux, el cliente oficial guarda las ACL POSIX, los atributos extendidos y las capacidades de archivo dentro del propio archivo de copia. Este cliente todavía no escribe ni lee esas entradas, así que en Linux las ACL no pasan de un cliente a otro en ningún sentido. Hacerlo al modo oficial, como opción, figura en la [hoja de ruta](TODO.md).

## Copia en caliente de la máquina completa

`salvanza-cli backup-disk` (y las copias de máquina de la interfaz gráfica) copian un sistema Windows 10/11 o Windows Server completo, o una máquina Linux, sin ninguna interrupción del servicio.

La sintaxis del comando es en gran parte la misma que para las copias de carpetas, salvo que `-backupdir` se sustituye por `-backupdev` (repítalo para varios discos). Por ejemplo:

`salvanza-cli backup-disk -authid yourapikey -backupdev \\.\PhysicalDrive0 -baseurl https://yourpbs:8007 -certfingerprint xx:xx:xx... -datastore zfs -secret L4m3r -backup-id testfull1`

El comando anterior examina el disco 0, detecta todas las particiones montadas, toma una instantánea VSS de ellas y luego crea una imagen de copia arrancable del disco entero en formato fidx. El espacio libre y los archivos de paginación e hibernación entran en la imagen como ceros sin leerse (en Linux, también el espacio libre de ext2/3/4 y XFS); `-readfreespace` lo lee todo, para una copia forense. Un sector que no se puede leer entra en la imagen como ceros, y la ejecución indica dónde. Añada `-type vm` (con un `-backup-id` numérico) para guardar la copia como copia de VM de Proxmox VE ([P2V](#-physical-machines-into-virtual-machines-p2v)).

La siguiente copia es incremental. El cálculo de hash está paralelizado, por lo que se pueden alcanzar velocidades de 1 GB/s.

Para excluir una partición, por ejemplo una biblioteca de juegos o una partición de datos ya cubierta por una copia de carpetas, añada `-skippartition <disk>:<number>` (repítalo para varias), como `-skippartition \\.\PhysicalDrive0:4` o `-skippartition /dev/sda:3`. La partición no se lee ni se incluye en la instantánea, y entra en la imagen como ceros: la tabla de particiones se conserva, así que una restauración devuelve la partición en su lugar y con su tamaño, vacía y sin formatear. En la interfaz gráfica, desmarque la partición bajo su disco en el conjunto de copia de seguridad.

### Requisitos previos de las copias de máquina en Linux (equivalente a VSS)

La copia de máquina en Windows usa VSS. Linux no tiene VSS, así que la copia de máquina de un host Linux en funcionamiento toma una instantánea en un momento dado de cada partición montada con un módulo del kernel: `elastio-snap` (el fork mantenido, [`Axcient/elastio-snap`](https://github.com/Axcient/elastio-snap)) o `dattobd`.

Se necesitan dos cosas; si no, una copia de máquina Linux se niega a ejecutarse:

1. **El módulo del kernel debe estar instalado y cargado.** Instálelo mediante DKMS, para que se recompile tras cada actualización del kernel, y haga que se cargue en cada arranque. La sección [Instantáneas en Linux: instalar elastio-snap o dattobd](docs/manual/GUI.md#linux-snapshots-installing-elastio-snap-or-dattobd) del manual de la interfaz gráfica tiene pasos probados para Debian, Ubuntu, Mint y Fedora, además de Secure Boot y comprobaciones. En noviembre de 2026 ninguno de los dos módulos compila con el kernel 7 (Fedora 44 tras sus actualizaciones).

2. **La copia debe ejecutarse como root.** La interfaz de control del módulo es solo para root, y la instantánea se rechaza para un usuario normal. El servicio en segundo plano se ejecuta como root; sin él, use **Reiniciar como administrador** en la aplicación, o ejecute `salvanza-cli backup-disk` con `sudo`. Una simple copia de carpetas no necesita root.

En la interfaz gráfica, un conjunto de copia de máquina Linux puede desmarcar **Usar instantáneas** y leer sus discos en caliente, pero solo con la segunda casilla marcada ("Ejecutar sin instantánea: acepto que la copia pueda ser incoherente"). Sin ella, cada ejecución se detiene e indica el motivo. Las particiones que no están montadas se leen directamente.

### Restaurar archivos desde una copia de disco

La página Restaurar de la interfaz gráfica abre una copia de máquina como una copia de carpetas: cada volumen de la imagen (NTFS, FAT, exFAT, ext2/3/4 o XFS) es una carpeta que puede explorar y desde la que puede restaurar. No se monta nada.

En Linux también puede conectar una copia de disco como dispositivo de bloques con `salvanza-cli attach-nbd`. Ejecute primero `modprobe nbd max_part=0`: con `max_part != 0` el kernel puede quedarse en un bucle infinito sondeando particiones.

`attach-nbd` conecta cualquier copia de disco fijo, sea una copia de VM o de host (así que también funciona con las copias de Proxmox VE). Por ejemplo:

`sudo salvanza-cli attach-nbd -authid 'apikey' -baseurl https://yourpbs:8007 -secret 'yoursecret' -certfingerprint 'aa:...:xx' -datastore test -namespace test1 -path "vm/107/2025-08-02T23:13:01Z/drive-virtio0.img.fidx"`

Si omite `-path`, una interfaz de texto le permite seleccionar el archivo fidx; `-list` en cambio muestra todas las imágenes.

> No use esto en una máquina que ejecute algo importante: un sistema de archivos dañado puede bloquear el sistema operativo, y por eso Proxmox VE usa una instancia de QEMU para esto.

Desmonte todo lo que haya en el disco nbd antes de detener `salvanza-cli attach-nbd`, o es probable que la partición siga ocupada. Si obtiene un error `Device or resource busy`, fuerce la desconexión con `nbd-client -d /dev/nbd0`, o reinicie.

### Restaurar en una máquina física o en una VM

Use la [ISO de restauración bare-metal](#-bare-metal-restore-iso), en hardware físico o en una máquina virtual vacía. El [manual de restauración bare-metal](docs/manual/BMR.md) lo explica paso a paso.

## Uso: copia de seguridad de directorios

La referencia completa, con ejemplos para Windows y Linux, códigos de salida y programación, está en el **[manual de la línea de comandos](docs/manual/CLI.md)**. Un comando típico:

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

También se leen las variables de entorno `PBS_REPOSITORY`, `PBS_SERVER`, `PBS_PORT`, `PBS_DATASTORE`, `PBS_AUTH_ID`, `PBS_PASSWORD` y `PBS_FINGERPRINT` del cliente oficial.

Para la configuración JSON consulte [config.json.example](config.json.example); rellene solo los campos que necesite.

### Varios directorios en una sola ejecución

Repita `-backupdir` para copiar más de un directorio con un solo comando:

```shell
salvanza-cli backup-folder -baseurl "https://yourpbshost:8007" -certfingerprint pbsfingerprint -authid "user@realm!apiid" -secret "apisecret" -datastore "datastorename" -backupdir "C:\Data" -backupdir "D:\Photos"
```

Cada directorio se copia en su propio grupo de copias, con el ID de copia de seguridad `<backup-id>_<path>` (`-backup-id` es por defecto el nombre del host). Por tanto, la retención, la poda y la restauración tratan cada directorio como una serie independiente. Un directorio que falla no detiene a los demás, y el código de salida informa del fallo. `-pxarout` no se puede combinar con varios directorios, y dos directorios que acabarían con el mismo ID de copia de seguridad se rechazan antes de subir nada.

En un archivo de configuración JSON use `"backupdirs": ["C:\\Data", "D:\\Photos"]`. El campo único `"backupdir"` sigue funcionando y se puede combinar con `"backupdirs"`. El cifrado funciona igual que con un solo directorio: la clave se lee una vez, y el grupo de copias de cada directorio se cifra con ella.

### Dejar las fechas fuera del archivo

La fecha de modificación de cada archivo y carpeta forma parte del archivo de copia, así que una fecha que cambia cambia los bytes del archivo, y los fragmentos alrededor de esa entrada se vuelven a subir aunque en ellos no haya cambiado nada. Para la mayoría de los datos es el equilibrio adecuado. Sin embargo, algunos árboles tocan continuamente sus carpetas mientras los archivos siguen iguales: un almacén que guarda sus datos en carpetas con nombres de hash (`aa/bb/<rest of the hash>`) puede acabar volviendo a subir gran parte del archivo en cada ejecución solo por ese motivo.

`-no-mtime` (`"no-mtime": true` en un archivo de configuración JSON) registra en su lugar 1970-01-01 como fecha de cada archivo y carpeta, de modo que solo los cambios reales de contenido cuestan fragmentos nuevos. Las fechas reales no se guardan en ningún sitio: una restauración pone a cada elemento la fecha 1970-01-01. Con `-change-detection-mode metadata` se lee cada archivo, como en el modo `data`. La opción está desactivada por defecto; úsela para un origen en el que las fechas no importen.

### Plantillas de correo

Funciona un relé que no requiere autenticación (a menudo en el puerto 25): omita el nombre de usuario y la contraseña. El remitente puede incluir un nombre (`Backups <backup@example.com>`).

Para el asunto y el cuerpo de los correos se usa el [motor de plantillas de Go](https://pkg.go.dev/text/template); consulte su documentación para la sintaxis. Están disponibles estas variables:

- `.NewChunks`: número de fragmentos nuevos creados
- `.ReusedChunks`: número de fragmentos reutilizados
- `.Datastore`: nombre del datastore
- `.Error`: el error, si lo hay (`.ErrorStr` lo da como texto, vacío si no hay ninguno)
- `.ReadErrors`: los archivos o carpetas que no se pudieron leer; `.ReadErrorCount` indica cuántos son
- `.Hostname`: nombre de host de la máquina
- `.StartTime`: hora de inicio de la copia
- `.EndTime`: hora de fin de la copia
- `.Duration`: duración de la copia
- `.FromattedDuration`: duración formateada de la copia (el nombre se escribe así en el código)
- `.Success`: verdadero cuando la copia terminó sin errores y sin nada ilegible
- `.Partial`: verdadero cuando se guardó una instantánea pero algunos archivos no se pudieron leer
- `.Status`: `Success`, `Partial` o `Failed`

## Copia de flujos

Esto copia un flujo en lugar de un archivo pxar. Por ejemplo:

```
mysqldump yourdatabase | salvanza-cli backup-folder -backupstream yourdatabase.sql [other options]
```

Usa fragmentación buzhash para deduplicar el propio flujo, por ejemplo un archivo tar o un volcado SQL.

## Problemas conocidos

Con el antimalware de Windows Defender activo, una copia puede ralentizarse hasta un 25 % aproximadamente de la velocidad alcanzable.

En Windows, `salvanza-cli backup-folder` y `backup-disk` toman un bloqueo de proceso: una segunda copia iniciada mientras otra está en curso muestra "Another backup job is running" y termina con el código 2. En Linux no existe ese bloqueo, así que no programe dos ejecuciones de línea de comandos a la misma hora. (La interfaz gráfica y el servicio en segundo plano ejecutan varias copias a la vez de forma segura, consulte [Varias copias a la vez](docs/manual/GUI.md#67-several-backups-at-once).)

## 📋 Requisitos previos

- Windows 10/11 o Windows Server (64 bits); derechos de administrador para instalar y para conectar la aplicación al servicio en segundo plano
- o Linux: los paquetes son para Debian, Ubuntu, Mint, Fedora y Arch; la interfaz gráfica necesita el entorno de ejecución WebKitGTK y polkit; las copias de máquina coherentes necesitan el módulo del kernel elastio-snap o dattobd y root
- Acceso de red a un Proxmox Backup Server
- Para P2V: Proxmox VE con el datastore de PBS añadido como almacenamiento, o cualquier hipervisor que pueda arrancar la ISO de restauración

## 🔨 Compilar desde el código fuente

**[BUILD.md](BUILD.md)** contiene las instrucciones completas: la interfaz gráfica y el servicio en segundo plano, las herramientas de línea de comandos, el MSI y los paquetes para Linux. En resumen, con Go 1.25 o posterior y la CLI de Wails v2.13:

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

El frontend compilado (`gui/frontend/dist`) está incluido en el repositorio, así que Node.js solo hace falta para modificar el frontend (`npm install` y `npm run build` en `gui/frontend`, o `wails dev` para la recarga en caliente). Pruebas: `make test`.

## 🔧 Uso avanzado y guías

### Multi-PBS (varios servidores PBS)

Configure varios servidores PBS y elija el destino para cada conjunto de copia de seguridad o copia puntual (p. ej. `C:\Users` a diario a un PBS rápido en SSD, `D:\` cada semana a un PBS de gran capacidad, además de un servidor de recuperación ante desastres).

- **[Guía de usuario](MULTI_PBS_USER_GUIDE.md)**: añadir y probar servidores, el servidor predeterminado, elegir un servidor por copia, preguntas frecuentes y solución de problemas.
- **[Guía de implementación](MULTI_PBS_GUIDE.md)**: modelo de datos, migración automática desde una configuración de un solo PBS, métodos del backend.

Una configuración antigua de un solo PBS se migra automáticamente a un servidor `default` en la primera carga.

<a id="-bare-metal-restore-iso"></a>
### 💿 ISO de restauración bare-metal

La ISO de restauración, Salvanza-BMR, es una ISO estándar de Clonezilla Live modificada con el binario `pbsnbd` (el código de `salvanza-cli attach-nbd`, compilado por separado desde `nbd/cmd/pbsnbd`) más dos entradas de menú. Arranca desde CD, desde una memoria USB (escrita con `dd`, o desde una memoria Ventoy), y en máquinas UEFI o BIOS:

- **`ocs-pbs-bare-metal-restore`**: la primera entrada del menú de arranque, la predeterminada, **PBS Bare Metal Restore**. Se salta las preguntas de idioma y teclado de Clonezilla y va directamente de las credenciales de PBS a una restauración terminada: primero DHCP (una petición de IP estática solo si eso falla), una clave de cifrado desde una memoria USB si la copia está cifrada, y una detección del disco de destino que nunca ofrece la memoria desde la que arrancó. Restaura igual copias de Windows y de Linux (la restauración es a nivel de bloque), en hardware físico o en una VM vacía.
- **`ocs-pbs-nbd`**: una entrada **pbs-nbd** en el propio menú de selección de modo de Clonezilla. Pide los datos de conexión a PBS y la instantánea, la conecta a `/dev/nbd0` y después pasa al asistente normal de restauración de discos de Clonezilla, que usted maneja a mano.

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-37-amd64.iso \
  ./build/pbsnbd \
  ./clonezilla-patch/ocs-pbs-nbd ./clonezilla-patch/ocs-pbs-bare-metal-restore
```

`make clonezilla` descarga la ISO estándar, compila `pbsnbd` y hace lo mismo, generando `dist/Salvanza-BMR-<version>.iso`. Todos los detalles (por qué se reconstruye la ISO completa en lugar de sustituir archivos en ella, requisitos previos, flujo de los menús, verificación) están en **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**; su uso, en el **[manual de restauración bare-metal](docs/manual/BMR.md)**.

### Compilar la interfaz gráfica para Windows

**Windows nativo**: consulte **[BUILD.md](BUILD.md)** para la cadena de herramientas (Go y MinGW desde Chocolatey, Wails) y los comandos para la aplicación, el servicio y el MSI.

**Docker (al compilar en Linux).** `./build_gui_windows_docker.sh` compila la aplicación de Windows, con compatibilidad completa con WebView2, en un contenedor `golang` desechable. No compila el servicio en segundo plano ni el MSI.

### Registro de cambios y documentación interna

- **[CHANGELOG.md](CHANGELOG.md)**: historial de cambios por versión.
- **[TODO.md](TODO.md)**: hoja de ruta abierta e ideas.
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)**: estado estable del producto y compilaciones disponibles.
- **[installer/DEPLOYMENT.md](installer/DEPLOYMENT.md)**: desplegar el MSI en muchos equipos.
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)**: diálogo de desinstalación del MSI (conservar o borrar la configuración) y su plan de pruebas.
- **[AGENTS.md](AGENTS.md)**: estructura del repositorio y convenciones, para colaboradores y agentes de programación.

## 🌐 Soporte técnico en Italia

Dejando claro que el proyecto y **TODAS** sus contribuciones seguirán siendo siempre públicos y con licencia GPLv3, el principal patrocinador de este proyecto, y también de las últimas funciones de copia de máquina, E.T.I. Srl ( https://etitech.net ), puede ofrecer a quien lo necesite soporte para implantaciones de Proxmox en general y, en particular, para las herramientas de copia de seguridad de Windows.

Nunca habrá ediciones "community" y "enterprise" distintas; solo el soporte técnico será un servicio independiente.
Cualquier personalización que solicite, aunque su desarrollo pueda ser de pago, se publicará con licencia GPLv3, como todo el proyecto.

## 🖥️ Atribución de la interfaz gráfica

La **aplicación Salvanza** (antes la Proxmox Backup Client GUI) fue creada por **[RDEM Systems](https://www.rdem-systems.com/)** como **[Nimbus Backup GUI](https://nimbus.rdem-systems.com)**: RDEM Systems diseñó y construyó la aplicación original, y ofrece soporte comercial en torno a Nimbus Backup, su versión con marca. **[mjb-is](https://github.com/mjb-is)** la rediseñó y amplió después, inspirándose en el flujo de trabajo y el estilo de otros sistemas de copia de seguridad, y ese trabajo se ha incorporado de nuevo a este repositorio. Todo el código, incluida la interfaz gráfica y todas sus funciones, sigue siendo de código abierto con licencia GPLv3.

## ⚠️ Advertencia

Este software se proporciona "tal cual". Aunque buscamos la fiabilidad, declinamos toda responsabilidad por la pérdida o el daño de datos. Pruebe siempre sus copias de seguridad y verifique la restauración antes de confiar en ellas en producción. Ninguna copia está probada hasta que ha restaurado a partir de ella, así que pruebe sus propias restauraciones, incluida una restauración bare-metal.

El software sigue teniendo calidad alfa y no asumimos ninguna responsabilidad por ningún tipo de daño o pérdida de datos, ni siquiera de los archivos de origen.

Los informes de errores son bienvenidos: abra una [issue](https://github.com/salvanza/salvanza_client/issues) con lo que hizo, lo que ocurrió y el registro (el Registro de mensajes, o **Ver registros completos**).

## 📄 Licencia

GPLv3: consulte el archivo [LICENSE](LICENSE).

## 🏷️ Marca personalizada

Todo colaborador que haya aportado **al menos 5 commits** que añadan funciones o correcciones tiene derecho a que se añadan sus datos de marca para uso comercial.

Las únicas condiciones son que la empresa a la que remite la marca **no** se dedique a ninguna de las siguientes actividades:

- Campañas de malware
- Negocios que promuevan la guerra (esto se aplica a cualquier país, incluidos los occidentales)
- Estafas
- Robo de datos
- Tráfico de menores
- Violencia
- Discriminación
- Drogas
- Cualquier actividad generalmente reconocida como ilegal

Si a alguno de los colaboradores le llega una queja, intentaremos ponernos en contacto; si no se da una explicación válida, **pondremos fin de inmediato** a ese beneficio.

La **licencia GPLv3 sigue vigente**, y seguirá teniendo libertad para hacer un fork del proyecto y compilar sus propios ejecutables.

## 🤝 Contribuir

Las contribuciones son bienvenidas, sobre todo:

1. Traducciones de los README y de los manuales de usuario para la versión 2.0
2. Subida asíncrona / multinúcleo de fragmentos (la compresión multinúcleo ya está implementada para la copia de máquina)
3. Un parche en el lado de Proxmox para añadir al formato pxar otro tipo de entrada con los descriptores de seguridad de Windows
4. Compatibilidad con los enlaces simbólicos y las uniones de Windows (hoy se omiten)
5. Pruebas en más hardware, distribuciones Linux y kernels, sobre todo de restauraciones bare-metal
6. Cualquier cosa interesante que se le ocurra :)

## Acerca de los colaboradores de Salvanza

Los colaboradores de Salvanza (antes los colaboradores de Proxmox Backup Client GO) desarrollan y mantienen este proyecto. El software se apoya en la infraestructura NTP/NTS y en los [11 servidores NTS públicos](https://github.com/jauderho/nts-servers) que figuran en la referencia de la comunidad.

---

**© 2024-2026 Los colaboradores de Salvanza y RDEM Systems.**
