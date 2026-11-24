package cmd

import (
	"fmt"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
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

	fromTime, toTime, _, err := parsePeriodRange(syncFrom, syncTo, syncDay, syncWeek, syncMonth, false)
	if err != nil {
		return err
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	todayEnd := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.Local)

	if fromTime.IsZero() && toTime.IsZero() {
		// Default tanpa flag: sync hari ini penuh
		fromTime = todayStart
		toTime = todayEnd
	} else if !fromTime.IsZero() && toTime.IsZero() {
		// Hanya --from diisi: otomatis tarik sampai akhir hari ini
		toTime = todayEnd
	}

	scrobbleSvc, err := app.ScrobbleService()
	if err != nil {
		return humanizeError(err)
	}

	fmt.Println("Memulai sinkronisasi scrobble...")
	if fromTime.IsZero() {
		fmt.Printf("Rentang: [Scrobble terakhir di DB] s.d. %s\n\n", toTime.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("Rentang: %s s.d. %s\n\n", fromTime.Format("2006-01-02 15:04:05"), toTime.Format("2006-01-02 15:04:05"))
	}

	sp := startSpinner("Menghubungi Last.fm API...")
	defer func() {
		sp.stopSpinner()
	}()

	onProgress := func(p model.PageProgress) {
		sp.stopSpinner()

		fmt.Printf("[Halaman %d/%d] Diambil: %d | Tersimpan: %d | Dilewati: %d\n",
			p.Page, p.TotalPages, p.Fetched, p.Inserted, p.Skipped)

		if p.Page < p.TotalPages {
			sp = startSpinner(fmt.Sprintf("Mengambil halaman %d/%d...", p.Page+1, p.TotalPages))
		}
	}

	result, err := scrobbleSvc.FetchRange(ctx, fromTime, toTime, onProgress)
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
