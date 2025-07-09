package artemis

import (
	"github.com/spf13/cobra"
)

var backtestCmd = &cobra.Command{
	Use:   "backtest",
	Short: "Run backtests for trading strategies",
	Long: `Run backtests for various trading strategies. This command allows you to
test your trading strategies against historical data to evaluate their performance.`,
}

func init() {
	rootCmd.AddCommand(backtestCmd)
}
