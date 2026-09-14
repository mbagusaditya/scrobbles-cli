package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Variabel ini dapat di-override saat build menggunakan:
// go build -ldflags="-X 'github.com/mbagusaditya/scrobbles-cli/cmd.Version=v1.0.0' ..."
var (
	Version   = "v1.0.0-dev"
	GitCommit = "none"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Menampilkan informasi versi aplikasi",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("scrobbles-cli %s\n", Version)
		fmt.Printf("  Commit     : %s\n", GitCommit)
		fmt.Printf("  Build Date : %s\n", BuildDate)
		fmt.Printf("  Go Version : %s (%s/%s)\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
