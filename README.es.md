# Proxmox Backup Client — Cliente de Windows para Proxmox Backup Server

[🇬🇧 English](README.md) · [🇫🇷 Français](README.fr.md) · [🇮🇹 Italiano](README.it.md) · [🇩🇪 Deutsch](README.de.md) · [🇪🇸 Español](README.es.md) · [🇷🇺 Русский](README.ru.md) · [🇨🇳 中文](README.zh.md) · [🇯🇵 日本語](README.ja.md) · [🇬🇷 Ελληνικά](README.el.md) · [🇷🇴 Română](README.ro.md) · [🇸🇪 Svenska](README.sv.md) · [🇸🇦 العربية](README.ar.md) · [🇮🇷 فارسی](README.fa.md)

[![Licence](https://img.shields.io/badge/license-GPLv3-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/tizbac/proxmoxbackupclient_go)](https://github.com/tizbac/proxmoxbackupclient_go/releases)
[![Documentation](https://img.shields.io/badge/docs-github-orange)](https://github.com/tizbac/proxmoxbackupclient_go)

**Proxmox Backup Client es un cliente de copia de seguridad de código abierto (GPL-3.0) para Proxmox Backup Server (PBS), que funciona en Windows y Linux.**
Es un **conjunto de herramientas** para hacer copias de seguridad en PBS:

- **Proxmox Backup Client GUI** (basada en la GUI Nimbus Backup de RDEM Systems) — interfaz gráfica moderna para respaldar servidores y estaciones de trabajo Windows en PBS: snapshots coherentes vía VSS, tareas programadas, modos de archivo y disco, navegación/restauración de snapshots, soporte multi-PBS y modo de servicio Windows.
- **`proxmoxbackup-directory`** — herramienta de línea de comandos para copias de seguridad de directorios (PXAR) con deduplicación.
- **`proxmoxbackup-machine`** — herramienta de línea de comandos para copias de seguridad completas en vivo de sistemas Windows (FIDX, VSS, incremental).
- **`proxmoxbackup-nbd`** — servidor NBD para restaurar copias de seguridad de disco (Linux).

> Palabras clave: cliente proxmox backup windows · cliente PBS · copia de seguridad Windows VSS · copia de seguridad remota inmutable · interfaz Proxmox Backup Server.

> ⚠️ **Aviso legal:** este proyecto **no está afiliado de ninguna manera** con **Proxmox Server Solutions GmbH**. «Proxmox», el logotipo de Proxmox y los nombres relacionados pertenecen a sus respectivos propietarios; aquí se utilizan **solo** para indicar compatibilidad. Consulte [proxmox.com](https://www.proxmox.com/) para conocer sus productos.

> 🤖 **Esta traducción fue generada con IA y puede contener pequeños errores. Las contribuciones para mejorarla son bienvenidas.**

## 📦 Descarga

👉 **[Descargar la última versión](https://github.com/tizbac/proxmoxbackupclient_go/releases)**

> ⚠️ **¿Windows muestra «virus detectado» (p. ej., `Trojan:Win32/Sabsik.FL.A!ml`) o una advertencia de SmartScreen?**
> Es un **falso positivo** conocido para las aplicaciones Go/Wails — *no* es un virus. El sufijo `!ml` indica una detección por modelo de machine learning que marca los ejecutables *no firmados y poco comunes*.
> Lea [por qué ocurre esto y cómo verificar la descarga](https://github.com/tizbac/proxmoxbackupclient_go).

### 🔎 Verificar cualquier descarga

Cada release proporciona sumas de verificación SHA-256 y una **atestación de procedencia firmada** (prueba criptográfica de que el binario fue producido por la CI de este repositorio, a partir de un commit preciso):

```powershell
Get-FileHash .\ProxmoxBackupClient.exe -Algorithm SHA256   # comparar con SHA256SUMS.txt
gh attestation verify .\ProxmoxBackupClient.exe --repo tizbac/proxmoxbackupclient_go
```

**VirusTotal — 0 detecciones.** Informes independientes de múltiples motores de los instaladores MSI recientes:
[0.2.108](https://www.virustotal.com/gui/file/6e8fb7ce9af740d470e947addb8daba4331c0b88e8bfdec9e0697ea8f7f29e9e/detection) ·
[0.2.107](https://www.virustotal.com/gui/file/6fd6c6fa77e0305c129ef882a3745100aa6033187a6d52a4af94149ab6b666d2/detection) ·
[0.2.106](https://www.virustotal.com/gui/file/ad6e56700ed9df8e088906e38cee2e2882fc7045f4e39269de0e379a01784ad7/detection)

> ℹ️ **Firma de código:** los binarios de Windows **aún no están firmados con Authenticode** (se espera un certificado OSS a través de [SignPath Foundation](https://signpath.org)). Mientras tanto, la procedencia se establece mediante la atestación y las sumas de verificación anteriores.

## 📚 Documentación

- **Guía completa de copia de seguridad de Proxmox** — mejores prácticas de despliegue de PBS
- **Copiar Windows con Proxmox Backup Server** — guía de despliegue específica para Windows
- **PBS vs Veeam** — comparativa de copias de seguridad Proxmox

## ✨ Funcionalidades

### Proxmox Backup Client GUI (recomendada)
- **🌍 Multilingüe** — interfaz en español, inglés y otros idiomas
- Configuración cómoda con prueba de conexión
- Progreso de la copia de seguridad en tiempo real con velocidad y tiempo restante
- Soporte VSS (Volume Shadow Copy) para copias coherentes
- Copia de seguridad de varias carpetas, modos de archivo y disco
- Navegación de snapshots, búsqueda de archivos (comodines) y restauración
- Soporte de múltiples servidores PBS, fijación de huella de certificado (TOFU)
- Modo servicio de Windows + copias programadas
- Registro de depuración para diagnóstico

### 📸 Capturas de pantalla

![Configuración de servidores](docs/screenshots/nimbus-gui-liste-servers.png)
*Gestión multi-servidor PBS con indicadores de estado*

![Formulario de añadir servidor](docs/screenshots/nimbus-gui-add-server-form.png)
*Configuración sencilla de servidor con prueba de conexión*

![Copia inmediata](docs/screenshots/nimbus-gui-one-shot-backup.png)
*Progreso de copia en tiempo real con ETA y velocidad*

### Exclusiones inteligentes del sistema (modo archivo)
Al respaldar un disco completo (p. ej., `D:\`), Proxmox Backup Client excluye automáticamente:

**Carpetas del sistema:** `System Volume Information` (almacenamiento VSS, puede alcanzar 100+ GB), `$RECYCLE.BIN`, `Recovery`.
**Archivos del sistema:** `pagefile.sys`, `hiberfil.sys`, `swapfile.sys`.

**Por qué es importante:** un disco puede mostrar 1,03 TB usados mientras que los archivos reales son ~141 GB. Sin exclusiones, la copia incluiría los snapshots VSS (espacio y tiempo desperdiciados); con ellas, el tamaño corresponde a los datos reales.

**Recomendación:** use el **modo archivo** (predeterminado) con autoexclusiones para copias a nivel de archivo; use el **modo disco** en una tarea separada para la restauración bare-metal (incluye todo).

### Seguridad y calidad
- Validación de entradas y saneamiento de credenciales
- Prevención de path traversal (recorrido de rutas)
- Lógica de reintentos con backoff exponencial
- Manejo completo de errores y pruebas, 100 % de conformidad con lint

## 🚀 Inicio rápido

1. Descargue `ProxmoxBackupClient.exe` (o el `.msi`) desde las releases
2. Ejecútelo con derechos de administrador (requerido para VSS)
3. Configure su conexión PBS y pruébela
4. Seleccione las carpetas a respaldar
5. Inicie la copia de seguridad

## 📋 Requisitos

- Windows 10/11 (64 bits)
- Derechos de administrador (para snapshots VSS)
- Acceso de red a un servidor Proxmox Backup Server

## 🔨 Compilación desde el código fuente

### Requisitos
- Go 1.22 o superior
- Node.js 20 o superior
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### Compilación
```bash
cd gui
npm install --prefix frontend
wails build      # o: wails dev  (recarga en caliente)
```

## 🔧 Uso avanzado y guías

### Multi-PBS (varios servidores PBS)

Configure varios servidores PBS y elija el destino para cada copia (p. ej., `C:\Users` → PBS SSD rápido, diario; `C:\` → PBS de big-data, semanal; más un servidor DR).

- **[Guía de usuario](MULTI_PBS_USER_GUIDE.md)** — añadir/probar servidores, servidor por defecto, FAQ y solución de problemas.
- **[Guía de implementación](MULTI_PBS_GUIDE.md)** — modelo de datos, migración automática desde config mono-PBS, métodos de la API backend.

La configuración mono-PBS existente se migra automáticamente a un servidor `default` en la primera carga.

### ISO Clonezilla (restauración bare-metal)

El flujo de rescate se construye parcheando una ISO Clonezilla Live con los binarios `pbsnbd` / `machinebackup` y una entrada **pbs-nbd** en el menú principal de Clonezilla (arranque desde CD, USB vía `dd` y UEFI):

```bash
./patch-clonezilla.sh \
  -o clonezilla-live-patched.iso \
  clonezilla-live-3.3.3-15-amd64.iso \
  ./build/pbsnbd ./build/machinebackup \
  ./clonezilla-patch/ocs-pbs-nbd
```

Detalles completos (por qué una reconstrucción completa en lugar de un reemplazo in situ, requisitos, flujo del menú, verificación) en **[PATCH-CLONEZILLA.md](PATCH-CLONEZILLA.md)**.

### Compilación de la GUI de Windows

**Docker (recomendado, sobre todo al compilar bajo Linux).** El script de un solo comando produce un `ProxmoxBackupClientGO.exe` con el soporte adecuado de WebView2, mediante un contenedor `golang` desechable (instalación de mingw + Wails, compilación del frontend, ejecución de `wails build`):

```bash
./build_gui_windows_docker.sh
```

**Windows nativo (Chocolatey).** Vea **[BUILD.md](BUILD.md)** para la configuración completa de la toolchain de Windows:

```powershell
choco install go
choco install mingw
# luego, en un shell no elevado:
build.bat          # GUI
build_cli.bat      # CLI
```

### Estado de las funciones, changelog y documentación interna

- **[FEATURES_STATUS.md](FEATURES_STATUS.md)** — matriz de estado por función (implementado / probado / hoja de ruta).
- **[CHANGELOG.md](CHANGELOG.md)** — historial de cambios por versión.
- **[TODO.md](TODO.md)** — hoja de ruta abierta e ideas.
- **[RELEASE_NOTES.md](RELEASE_NOTES.md)** — estado estable del producto y compilaciones disponibles.
- **[MSI_UNINSTALL_TEST.md](MSI_UNINSTALL_TEST.md)** — diálogo de desinstalación MSI (conservar/eliminar config) y su plan de prueba.
- **[FIXES_SUMMARY.md](FIXES_SUMMARY.md)** — notas de correcciones de la GUI (conmutación modo directorio vs. máquina).

## 🖥️ Atribución de la interfaz gráfica

La **Proxmox Backup Client GUI** está basada en la **[GUI Nimbus Backup](https://nimbus.rdem-systems.com)**, desarrollada y mantenida por **[RDEM Systems](https://www.rdem-systems.com/)**.

La GUI (originalmente un fork de este proyecto) se fusionó de nuevo en este repositorio: la totalidad del código, incluida la GUI y todas sus funciones, permanece de código abierto bajo GPLv3. RDEM Systems patrocina el desarrollo de la GUI y ofrece soporte comercial para ella.

**Autor del CLI original:** Tiziano Bacocco (tizbac) · **Licencia:** GPLv3

## ⚠️ Aviso

Este software se proporciona «tal cual». Aunque aspiramos a la fiabilidad, declinamos toda responsabilidad por pérdida o daño de datos. Pruebe siempre sus copias de seguridad y verifique la restauración antes de confiar en ellas en producción.

## 📄 Licencia

GPLv3 — consulte el archivo [LICENSE](LICENSE).

## 🏷️ Marca (Branding)

Todo contribuyente que haya aportado **al menos 5 commits** que añadan funcionalidad o correcciones tiene derecho a que se añadan sus datos de marca para uso comercial.

Las únicas condiciones son que la empresa a la que apunta la marca **no** lleve a cabo ninguna de las siguientes actividades:

- Campañas de malware
- Empresas que promueven la guerra (esto se aplica a cualquier país, incluidos los occidentales)
- Estafas
- Robo de datos
- Tráfico de personas/menores
- Violencia
- Discriminación
- Drogas
- Cualquier actividad generalmente reconocida como ilegal

Si surge alguna queja contra cualquiera de los contribuyentes, intentaremos ponernos en contacto; si no se da una explicación válida, **terminaremos inmediatamente** ese beneficio.

La **licencia GPLv3 sigue activa**, y usted seguirá siendo libre de hacer un fork del proyecto y compilar sus propios ejecutables.

## Acerca de los contribuyentes de Proxmox Backup Client GO

Los contribuyentes de Proxmox Backup Client GO desarrollan y mantienen este proyecto. El software se apoya en la infraestructura NTP/NTS y los [11 servidores NTS públicos](https://github.com/jauderho/nts-servers) enumerados en la referencia de la comunidad.

## 🤝 Contribuir

La GUI ya está totalmente implementada, pero las contribuciones siguen siendo bienvenidas, en especial:

1. Soporte de cifrado (aún falta)
2. Migración físico-a-virtual (P2V), restauración de una copia bare-metal en una máquina virtual (aún incompleta)
3. Subida asíncrona / subida multicore de chunks (la compresión multicore ya está implementada para machine backup)
4. Parche del lado de Proxmox para añadir otro tipo de entrada al formato pxar con descriptores de seguridad de Windows
5. Soporte de enlaces simbólicos de Windows
6. Cualquier cosa interesante que se le ocurra :)

---

**© 2024-2026 Proxmox Backup Client GO Contributors and RDEM Systems.**