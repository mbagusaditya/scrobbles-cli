#!/usr/bin/env bash

set -euo pipefail

SYSTEMD_USER_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
SERVICE_NAME="scrobbles.service"
TIMER_NAME="scrobbles.timer"


# Cari path binary scrobbles
BIN_PATH="$(command -v scrobbles || true)"
if [ -z "$BIN_PATH" ]; then
    if [ -f "$HOME/.local/bin/scrobbles" ]; then

        BIN_PATH="$HOME/.local/bin/scrobbles"
    else
        echo "Error: Binary 'scrobbles' tidak ditemukan di PATH atau ~/.local/bin/scrobbles."
        echo "Jalankan './build.sh' terlebih dahulu."
        exit 1

    fi
fi

# Fungsi uninstall / stop
uninstall() {
    echo "Menonaktifkan dan menghapus systemd user units..."
    systemctl --user stop "$TIMER_NAME" "$SERVICE_NAME" 2>/dev/null || true
    systemctl --user disable "$TIMER_NAME" "$SERVICE_NAME" 2>/dev/null || true
    rm -f "$SYSTEMD_USER_DIR/$SERVICE_NAME" "$SYSTEMD_USER_DIR/$TIMER_NAME"
    systemctl --user daemon-reload
    echo "Selesai: Service dan timer scrobbles berhasil dihapus."
    exit 0
}


if [ "${1:-}" = "uninstall" ] || [ "${1:-}" = "--uninstall" ]; then

    uninstall

fi


echo "Memasang systemd user units..."
mkdir -p "$SYSTEMD_USER_DIR"


# 1. Tulis scrobbles.service
cat <<EOF > "$SYSTEMD_USER_DIR/$SERVICE_NAME"
[Unit]
Description=Last.fm Scrobbles Auto Sync and Resolve
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/bin/sh -c '$BIN_PATH sync --week && $BIN_PATH resolve -b 500'
Environment="XDG_CONFIG_HOME=%h/.config"

[Install]
WantedBy=default.target
EOF

# 2. Tulis scrobbles.timer
cat <<EOF > "$SYSTEMD_USER_DIR/$TIMER_NAME"
[Unit]
Description=Periodic Timer for Last.fm Scrobbles Sync

[Timer]
OnBootSec=5min
OnUnitActiveSec=2h
RandomizedDelaySec=2min
Persistent=true

[Install]
WantedBy=timers.target
EOF

echo "File unit tersimpan di: $SYSTEMD_USER_DIR"


# 3. Reload daemon dan aktifkan timer
systemctl --user daemon-reload
systemctl --user enable --now "$TIMER_NAME"

echo ""
echo "Berhasil! Timer aktif dan akan berjalan otomatis setiap 2 jam."
echo "Status timer:"
systemctl --user list-timers "$TIMER_NAME" --no-pager
