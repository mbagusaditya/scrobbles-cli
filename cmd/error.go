package cmd

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// humanizeError mengubah error teknis driver/network menjadi pesan yang jelas dan actionable.
func humanizeError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	// 1. Masalah DNS / Host salah ketik
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) || strings.Contains(errMsg, "no such host") {
		return fmt.Errorf("gagal terhubung ke database: host tidak ditemukan.\n-> Periksa kembali URL database di file konfigurasi (.env atau ~/.config/scrobbles/config.env)")
	}

	// 2. Masalah Koneksi Jaringan / Timeout
	if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "i/o timeout") {
		return fmt.Errorf("gagal terhubung ke server: koneksi ditolak atau timeout.\n-> Pastikan koneksi internet aktif dan server database dapat diakses")
	}

	// 3. Masalah Autentikasi / Token Turso Salah
	if strings.Contains(errMsg, "401") || strings.Contains(errMsg, "unauthorized") || strings.Contains(errMsg, "authentication failed") {
		return fmt.Errorf("autentikasi database gagal: token akses tidak valid.\n-> Periksa nilai DATABASE_AUTH_TOKEN di konfigurasi Anda")
	}

	// 4. Masalah Rate Limit / API Key Last.fm
	if strings.Contains(errMsg, "403") || strings.Contains(errMsg, "Forbidden") {
		return fmt.Errorf("akses ditolak oleh layanan eksternal (HTTP 403).\n-> Periksa validitas LASTFM_API_KEY Anda")
	}

	// Default fallback jika error tidak spesifik
	return err
}
