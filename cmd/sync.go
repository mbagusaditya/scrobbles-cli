package cmd

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mbagusaditya/scrobbles-cli/internal/service"
)

var (
	syncFrom string
	syncTo   string
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sinkronisasi riwayat scrobble dari Last.fm ke Turso",
	Long: `Mengambil scrobble musik dari Last.fm dan menyimpannya ke database Turso.
Mendukung filter rentang tanggal --from dan --to. Jika kedua flag kosong,
secara default akan mengambil data untuk hari ini penuh.`,
	RunE: runSync,
}

type spinner struct {
	message string
	stop    chan struct{}
	done    chan struct{}
	once    sync.Once
}

func startSpinner(msg string) *spinner {
	s := &spinner{
		message: msg,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}

	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

	go func() {
		defer close(s.done)
		i := 0
		for {
			select {
			case <-s.stop:
				fmt.Print("\r\033[K")
				return
			default:
				fmt.Printf("\r%s %s", frames[i%len(frames)], s.message)
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()

	return s
}

func (s *spinner) stopSpinner() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		close(s.stop)
		<-s.done
	})
}

func init() {
	rootCmd.AddCommand(syncCmd)

	syncCmd.Flags().StringVar(&syncFrom, "from", "", "Batas awal pengambilan data (format: YYYY-MM-DD)")
	syncCmd.Flags().StringVar(&syncTo, "to", "", "Batas akhir pengambilan data (format: YYYY-MM-DD)")
}

func runSync(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	now := time.Now()

	var fromTime, toTime time.Time

	// 1. Logika penentuan rentang waktu
	if syncFrom == "" && syncTo == "" {
		// Kasus: Kedua flag kosong -> satu hari ini penuh
		fromTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
		toTime = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.Local)
	} else {
		// Parsing --from jika disediakan
		if syncFrom != "" {
			t, err := time.ParseInLocation("2006-01-02", syncFrom, time.Local)
			if err != nil {
				return fmt.Errorf("format flag --from tidak valid (gunakan YYYY-MM-DD): %w", err)
			}
			fromTime = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
		}

		// Parsing --to jika disediakan
		if syncTo != "" {
			t, err := time.ParseInLocation("2006-01-02", syncTo, time.Local)
			if err != nil {
				return fmt.Errorf("format flag --to tidak valid (gunakan YYYY-MM-DD): %w", err)
			}
			toTime = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.Local)
		}

		// Jika hanya --from yang diisi, --to otomatis ke akhir hari ini
		if syncFrom != "" && syncTo == "" {
			toTime = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.Local)
		}
		// Jika hanya --to yang diisi, fromTime dibiarkan time.Time{} (zero value)
		// agar service mengambil dari scrobble terakhir yang ada di Turso.
	}

	// 2. Validasi rentang tanggal jika keduanya terisi
	if !fromTime.IsZero() && !toTime.IsZero() && fromTime.After(toTime) {
		return fmt.Errorf("rentang tanggal tidak valid: --from tidak boleh lebih baru dari --to")
	}

	// 3. Tampilkan informasi rentang waktu sinkronisasi ke user
	fmt.Println("Memulai sinkronisasi scrobble...")
	if fromTime.IsZero() {
		fmt.Printf("Rentang: [Scrobble terakhir di DB] s.d. %s\n\n", toTime.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("Rentang: %s s.d. %s\n\n", fromTime.Format("2006-01-02 15:04:05"), toTime.Format("2006-01-02 15:04:05"))
	}

	// Nyalakan loading spinner awal
	sp := startSpinner("Menghubungi Last.fm API...")
	defer func() {
		sp.stopSpinner()
	}()

	onProgress := func(p service.PageProgress) {
		sp.stopSpinner()

		fmt.Printf("[Halaman %d/%d] Diambil: %d | Tersimpan: %d | Dilewati: %d\n",
			p.Page, p.TotalPages, p.Fetched, p.Inserted, p.Skipped)

		if p.Page < p.TotalPages {
			sp = startSpinner(fmt.Sprintf("Mengambil halaman %d/%d...", p.Page+1, p.TotalPages))
		}
	}

	result, err := app.scrobbleService.FetchRange(ctx, fromTime, toTime, onProgress)
	sp.stopSpinner() // Sekarang aman dipanggil kembali berkat sync.Once

	if err != nil {
		// ... error handling
		if result != nil && (result.Inserted > 0 || result.Skipped > 0) {
			fmt.Printf("\n⚠️  Proses terhenti karena kesalahan: %v\n", err)
			fmt.Printf("Sebagian data berhasil diamankan -> Tersimpan: %d, Dilewati: %d\n", result.Inserted, result.Skipped)
		}
		return err
	}

	fmt.Println("\nSinkronisasi selesai!")
	fmt.Printf("Total Diambil : %d\n", result.Fetched)
	fmt.Printf("Total Tersimpan: %d\n", result.Inserted)
	fmt.Printf("Total Duplikat : %d\n", result.Skipped)

	return nil
}
