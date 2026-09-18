package cmd

import (
	"fmt"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mbagusaditya/scrobbles-cli/internal/service"
)

var (
	syncFrom  string
	syncTo    string
	syncDay   bool
	syncWeek  bool
	syncMonth bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sinkronisasi riwayat scrobble dari Last.fm ke Turso",
	Long: `Mengambil scrobble musik dari Last.fm dan menyimpannya ke database Turso.
Mendukung filter rentang tanggal --from dan --to, serta preset --day, --week, dan --month.
Jika tanpa flag, secara default akan mengambil data untuk hari ini penuh.`,
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
	syncCmd.Flags().BoolVar(&syncDay, "day", false, "Sync data hari ini penuh")
	syncCmd.Flags().BoolVar(&syncWeek, "week", false, "Sync data 7 hari terakhir")
	syncCmd.Flags().BoolVar(&syncMonth, "month", false, "Sync data bulan berjalan (sejak tgl 1)")
}

func runSync(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	now := time.Now()

	// 1. Validasi mutual exclusion untuk preset flags
	presetCount := 0
	if syncDay {
		presetCount++
	}
	if syncWeek {
		presetCount++
	}
	if syncMonth {
		presetCount++
	}

	if presetCount > 1 {
		return fmt.Errorf("pilih salah satu preset saja: --day, --week, atau --month")
	}

	if presetCount == 1 && (syncFrom != "" || syncTo != "") {
		return fmt.Errorf("flag preset (--day/--week/--month) tidak dapat digabung dengan --from atau --to")
	}

	// 2. Logika penentuan rentang waktu
	var fromTime, toTime time.Time
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	todayEnd := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.Local)

	switch {
	case syncDay:
		fromTime = todayStart
		toTime = todayEnd

	case syncWeek:
		// Rolling 7 hari ke belakang sampai akhir hari ini
		fromTime = todayStart.AddDate(0, 0, -6)
		toTime = todayEnd

	case syncMonth:
		// Sejak tanggal 1 bulan berjalan
		fromTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		toTime = todayEnd

	case syncFrom == "" && syncTo == "":
		// Default: jika semua flag kosong, sync hari ini penuh
		fromTime = todayStart
		toTime = todayEnd

	default:
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
			toTime = todayEnd
		}
		// Jika hanya --to yang diisi, fromTime tetap zero value
		// agar service mengambil dari scrobble terakhir di DB.
	}

	// 3. Validasi rentang tanggal jika keduanya terisi
	if !fromTime.IsZero() && !toTime.IsZero() && fromTime.After(toTime) {
		return fmt.Errorf("rentang tanggal tidak valid: --from tidak boleh lebih baru dari --to")
	}

	// 4. Tampilkan informasi rentang waktu sinkronisasi ke user
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
	sp.stopSpinner()

	if err != nil {
		if result != nil && (result.Inserted > 0 || result.Skipped > 0) {
			fmt.Printf("\n⚠️  Proses terhenti karena kesalahan:\n")
			fmt.Printf("Sebagian data berhasil diamankan -> Tersimpan: %d, Dilewati: %d\n", result.Inserted, result.Skipped)
		}
		return humanizeError(err)
	}

	fmt.Println("\nSinkronisasi selesai!")
	fmt.Printf("Total Diambil   : %s\n", formatNumber(result.Fetched))
	fmt.Printf("Total Tersimpan : %s\n", formatNumber(result.Inserted))
	fmt.Printf("Total Duplikat  : %s\n", formatNumber(result.Skipped))

	return nil
}
