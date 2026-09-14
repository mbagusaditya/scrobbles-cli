#!/usr/bin/env bash
set -e

APP_NAME="scrobbles"
PKG="github.com/mbagusaditya/scrobbles-cli/cmd"
INSTALL_DIR="$HOME/.local/bin"
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/$APP_NAME"

echo "==> 1. Memeriksa konfigurasi XDG..."
mkdir -p "$CONFIG_DIR"
if [ ! -f "$CONFIG_DIR/config.env" ]; then
    if [ -f ".env" ]; then
        echo "Menyalin .env lokal ke $CONFIG_DIR/config.env"
        cp .env "$CONFIG_DIR/config.env"
        chmod 600 "$CONFIG_DIR/config.env"
    else
        echo "Peringatan: File .env maupun $CONFIG_DIR/config.env tidak ditemukan."
    fi
else
    echo "Konfigurasi ditemukan di $CONFIG_DIR/config.env"
fi

echo "==> 2. Mengambil metadata git & kompilasi..."
VERSION=$(git describe --tags --always 2>/dev/null || echo "v1.1.0-dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')

mkdir -p "$INSTALL_DIR"

go build -ldflags=" \
  -s -w \
  -X '${PKG}.Version=${VERSION}' \
  -X '${PKG}.GitCommit=${COMMIT}' \
  -X '${PKG}.BuildDate=${BUILD_DATE}'" \
  -o "$INSTALL_DIR/$APP_NAME" main.go

echo "Binary berhasil di-build ke: $INSTALL_DIR/$APP_NAME"

echo "==> 3. Memeriksa environment PATH..."
case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *)
        echo "Catatan: $INSTALL_DIR belum ada di \$PATH kamu."
        echo "Tambahkan baris berikut ke ~/.bashrc atau ~/.zshrc:"
        echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
        ;;
esac

echo "==> Selesai! Coba jalankan: $APP_NAME version"
