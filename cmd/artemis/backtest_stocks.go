package artemis

import (
	"encoding/csv"
	"fmt"
	"os"
	"time"

	"github.com/shopspring/decimal"
	"github.com/spf13/cobra"
	"github.com/vignesh-goutham/artemis/pkg/alpaca"
	"github.com/vignesh-goutham/artemis/pkg/backtest"
	"github.com/vignesh-goutham/artemis/pkg/signals"
)

var (
	csvPath string
)

var backtestStocksCmd = &cobra.Command{
	Use:   "stocks",
	Short: "Run backtests for stock trading strategies",
	Long: `Run backtests for stock trading strategies using trade signals from a CSV file.
The CSV file should contain trade signals with the following columns:
- ticker
- buy_date
- sell_date
- status
- allocation_percentage

Example:
  artemis backtest stocks --csv path/to/trade_signals.csv`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Verify CSV file exists
		if _, err := os.Stat(csvPath); os.IsNotExist(err) {
			return fmt.Errorf("CSV file does not exist: %s", csvPath)
		}

		// Open and read CSV file
		file, err := os.Open(csvPath)
		if err != nil {
			return fmt.Errorf("error opening CSV file: %v", err)
		}
		defer file.Close()

		reader := csv.NewReader(file)

		// Read header
		header, err := reader.Read()
		if err != nil {
			return fmt.Errorf("error reading CSV header: %v", err)
		}

		// Create map of column indices
		columnMap := make(map[string]int)
		for i, col := range header {
			columnMap[col] = i
		}

		// Read and parse all records
		records, err := reader.ReadAll()
		if err != nil {
			return fmt.Errorf("error reading CSV records: %v", err)
		}

		signalList := make(map[int]*signals.Signal)
		for id, record := range records {
			buyDate, err := time.Parse("2006-01-02", record[columnMap["buy_date"]])
			if err != nil {
				return fmt.Errorf("error parsing buy_date: %v", err)
			}

			sellDate, err := time.Parse("2006-01-02", record[columnMap["sell_date"]])
			if err != nil {
				return fmt.Errorf("error parsing sell_date: %v", err)
			}

			allocation, err := decimal.NewFromString(record[columnMap["allocation_percentage"]])
			if err != nil {
				return fmt.Errorf("error parsing allocation_percentage: %v", err)
			}

			signalList[id] = &signals.Signal{
				ID:                   id,
				Ticker:               record[columnMap["ticker"]],
				BuyDateTime:          buyDate,
				SellDateTime:         sellDate,
				AllocationPercentage: allocation,
				Status:               signals.StatusPending,
			}
		}

		fmt.Printf("Successfully loaded %d trade signals from %s\n", len(signalList), csvPath)
		for _, signal := range signalList {
			fmt.Printf("\tTicker: %s\n", signal.Ticker)
			fmt.Printf("\tBuy Date: %s\n", signal.BuyDateTime.Format(signals.DateLayout))
			fmt.Printf("\tSell Date: %s\n", signal.SellDateTime.Format(signals.DateLayout))
			fmt.Printf("\tAllocation: %s%%\n", signal.AllocationPercentage)
			fmt.Printf("\tStatus: %s\n\n", signal.Status)
		}

		alpacaClient, err := alpaca.NewAlpacaMarketData()
		if err != nil {
			return fmt.Errorf("error creating alpaca client: %v", err)
		}

		backtester := backtest.NewBacktester(signalList, alpacaClient)
		if err := backtester.Run(); err != nil {
			return fmt.Errorf("error running backtest: %v", err)
		}

		// Print the backtest results
		backtester.PrintResults()

		return nil
	},
}

func init() {
	backtestCmd.AddCommand(backtestStocksCmd)

	// Add flags
	backtestStocksCmd.Flags().StringVar(&csvPath, "csv", "", "Path to CSV file containing trade signals")

	// Mark required flags
	backtestStocksCmd.MarkFlagRequired("csv")
}
