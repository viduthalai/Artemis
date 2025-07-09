package artemis

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "artemis",
	Short: "Artemis is a trading bot and backtesting framework",
	Long: `Artemis is a comprehensive trading bot and backtesting framework that helps you
test and deploy trading strategies. It supports both stock and options trading,
with features for historical data analysis and strategy optimization.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
