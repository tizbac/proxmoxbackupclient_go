#!/bin/bash
# Runs the GUI as root via pkexec (native graphical password prompt), for
# VSS/machine backups which require EUID 0 (see snapshot/linux_snapshot.go's
# CreateVSSSnapshot). DISPLAY/XAUTHORITY/HOME are captured here, in the
# normal user's own session, and passed explicitly through to pkexec's
# environment -- pkexec does not forward the caller's environment by
# default, and without an explicit HOME the elevated process sees root's
# own home (/root), not this user's, so it can't find the existing PBS
# server config under ~/.proxmox-backup-guardian and looks freshly
# unconfigured. Confirmed live 2026-09-24.
#
# The elevated process runs as root but with HOME pointed at this user's
# config directory, so any file it creates/rewrites there (config.json,
# scheduled_jobs.json, job_history.json, message_log.json) ends up owned by
# root, mode 600, completely unreadable by this user afterward -- confirmed
# live 2026-09-24: launching normally right after a root run showed "no
# server configured" because config.json couldn't even be read anymore.
# Both the app run and the ownership fix happen inside ONE pkexec call (a
# single `bash -c '...; chown ...'`) so this is still one password prompt,
# not two, and ownership is guaranteed to be handed back the moment the
# elevated app closes, every single time.
#
# NOTE: this file must be copied back into gui/build/bin/ after every
# `wails build -clean` on this dev machine -- -clean wipes the whole
# build/bin directory, and this script is hand-maintained, not part of
# Wails' own build output, so a clean rebuild silently deletes it. Found
# live 2026-09-25: the "Run as Root" desktop shortcut broke after several
# -clean rebuilds tonight while the plain shortcut kept working, since
# that one points straight at the ProxmoxBackupClient binary Wails does
# regenerate. Kept in packaging/linux/run-as-root.sh (source of truth) so
# a rebuild can restore it instead of losing it for good.
UID_GID="$(id -u):$(id -g)"
pkexec env DISPLAY="$DISPLAY" XAUTHORITY="$HOME/.Xauthority" HOME="$HOME" \
  bash -c "/home/mickbeeby/proxmoxbackupclient_go/gui/build/bin/ProxmoxBackupClient; chown -R $UID_GID \"\$HOME/.proxmox-backup-guardian\""
