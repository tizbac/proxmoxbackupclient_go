# Official Proxmox Backup Server client, built from Proxmox's own Debian
# repository.
#
# Why this exists
# ---------------
# The E2E suite needs a *current* proxmox-backup-client, not a stale one:
#
#   * the pre-built community images are far behind (fdrake/... is 2.4.4, which
#     predates both chunk encryption and the return of namespaces in PBS 4.x);
#   * the client installed on a developer machine is whatever their distro
#     happened to ship (3.2.7 on the machine this was developed on).
#
# An old client silently changes what the suite can assert: it cannot restore
# namespaces, and it predates the encryption-key handling we want to verify.
# So we build the client from the official repo instead of trusting a tag.
#
# Debian 13 "trixie" is the current stable and matches the PBS 4.x packages.
# Note the repo key is named after the Proxmox VE release, not the Debian
# suite: the trixie key is proxmox-release-trixie.gpg on enterprise.proxmox.com
# (download.proxmox.com only carries the bookworm/bullseye keys).
#
# Build:
#   docker build -t proxmoxbackupclient-e2e-client testing/e2e/official-client
FROM debian:trixie-slim

# Proxmox's own instructions use plain http for both the key and the repo and
# rely on apt's GPG verification; that keeps this working on a slim base image
# that ships no CA bundle.
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends ca-certificates curl gnupg; \
    curl -fsSL -o /usr/share/keyrings/proxmox-release-trixie.gpg \
        http://enterprise.proxmox.com/debian/proxmox-release-trixie.gpg; \
    printf '%s\n' \
        'deb [signed-by=/usr/share/keyrings/proxmox-release-trixie.gpg] http://download.proxmox.com/debian/pbs trixie pbs-no-subscription' \
        > /etc/apt/sources.list.d/pbs.list; \
    apt-get update; \
    apt-get install -y --no-install-recommends proxmox-backup-client; \
    rm -rf /var/lib/apt/lists/*; \
    proxmox-backup-client version

# FUSE is not needed for `restore` (only for proxmox-file-restore), but the
# package pulls in libfuse3 anyway; make sure the loader finds it regardless of
# how the image is run.
ENV LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu

ENTRYPOINT ["proxmox-backup-client"]