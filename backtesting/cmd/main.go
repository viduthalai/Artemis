package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"time"

	alpaca "github.com/alpacahq/alpaca-trade-api-go/v2/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v2/marketdata"
	"github.com/google/uuid"

	"github.com/vignesh-goutham/artemis/backtesting/internal"
)

func main() {
	// Initialize Alpaca trading client with v2 API
	client := alpaca.NewClient(alpaca.ClientOpts{
		ApiKey:    os.Getenv("ALPACA_API_KEY"),
		ApiSecret: os.Getenv("ALPACA_SECRET_KEY"),
		BaseURL:   "https://paper-api.alpaca.markets",
	})

	// Initialize Alpaca marketdata client with v2 API
	marketdataClient := marketdata.NewClient(marketdata.ClientOpts{
		ApiKey:    os.Getenv("ALPACA_API_KEY"),
		ApiSecret: os.Getenv("ALPACA_SECRET_KEY"),
		BaseURL:   "https://data.alpaca.markets",
	})

	// Load stock signals from CSV
	signals, err := loadSignalsFromCSV("data/example/stock_signals.csv")
	if err != nil {
		log.Fatalf("Error loading signals: %v", err)
	}

	fmt.Printf("Loaded %d stock signals\n", len(signals))

	// Run basic backtest
	fmt.Println("\n=== RUNNING BASIC BACKTEST ===")
	results, err := runBacktest(client, marketdataClient, signals)
	if err != nil {
		log.Fatalf("Error running backtest: %v", err)
	}

	// Calculate summary
	summary := calculateSummary(results)

	// Print results
	printResults(results, summary)

	// Run enhanced backtest
	fmt.Println("\n=== RUNNING ENHANCED BACKTEST ===")
	enhancedConfig := internal.EnhancedBacktestConfig{
		StaggerEntry:    true,
		StaggerPercent:  0.8,  // 80% initial entry
		TakeProfitPct:   0.15, // 15% take profit
		TrailingStopPct: 0.15, // 15% trailing stop
	}

	enhancedResults, err := internal.RunEnhancedBacktest(client, marketdataClient, signals, enhancedConfig)
	if err != nil {
		log.Fatalf("Error running enhanced backtest: %v", err)
	}

	// Calculate enhanced summary
	enhancedSummary := internal.CalculateEnhancedSummary(enhancedResults)

	// Print enhanced results
	internal.PrintEnhancedResults(enhancedResults, enhancedSummary)
}

func loadSignalsFromCSV(filename string) ([]internal.StockSignal, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var signals []internal.StockSignal
	for i, record := range records {
		if i == 0 { // Skip header
			continue
		}

		if len(record) < 3 {
			continue
		}

		buyDate, err := time.Parse("2006-01-02", record[1])
		if err != nil {
			log.Printf("Warning: Invalid buy date for signal %d: %s", i, record[1])
			continue
		}

		sellDate, err := time.Parse("2006-01-02", record[2])
		if err != nil {
			log.Printf("Warning: Invalid sell date for signal %d: %s", i, record[2])
			continue
		}

		signal := internal.StockSignal{
			UUID:     uuid.New().String(),
			Ticker:   record[0],
			BuyDate:  buyDate,
			SellDate: sellDate,
		}
		signals = append(signals, signal)
	}

	return signals, nil
}

func runBacktest(client alpaca.Client, marketdataClient marketdata.Client, signals []internal.StockSignal) ([]internal.SignalResult, error) {
	var results []internal.SignalResult

	for _, signal := range signals {
		result, err := internal.BacktestSignal(client, marketdataClient, signal)
		if err != nil {
			log.Printf("Warning: Error backtesting signal %s (%s): %v", signal.UUID, signal.Ticker, err)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

func calculateSummary(results []internal.SignalResult) internal.BacktestSummary {
	if len(results) == 0 {
		return internal.BacktestSummary{}
	}

	var totalReturn, totalDays float64
	var winningSignals, losingSignals int
	var bestSignal, worstSignal internal.SignalResult

	for i, result := range results {
		totalReturn += result.ProfitLossPct
		totalDays += float64(result.DaysHeld)

		if result.IsWin {
			winningSignals++
		} else {
			losingSignals++
		}

		// Track best and worst signals
		if i == 0 {
			bestSignal = result
			worstSignal = result
		} else {
			if result.ProfitLossPct > bestSignal.ProfitLossPct {
				bestSignal = result
			}
			if result.ProfitLossPct < worstSignal.ProfitLossPct {
				worstSignal = result
			}
		}
	}

	totalSignals := len(results)
	winRate := float64(winningSignals) / float64(totalSignals) * 100
	avgReturn := totalReturn / float64(totalSignals)
	avgHoldTime := totalDays / float64(totalSignals)

	// Calculate annualized return using average hold time
	var avgReturnAnnual float64
	if avgHoldTime > 0 {
		avgReturnAnnual = avgReturn * (365 / avgHoldTime)
	}

	// Calculate maximum concurrent signals
	maxConcurrentSignals := calculateMaxConcurrentSignals(results)

	return internal.BacktestSummary{
		TotalSignals:         totalSignals,
		WinningSignals:       winningSignals,
		LosingSignals:        losingSignals,
		WinRate:              winRate,
		AvgReturn:            avgReturn,
		AvgHoldTime:          avgHoldTime,
		AvgReturnAnnual:      avgReturnAnnual,
		TotalReturn:          totalReturn,
		MaxConcurrentSignals: maxConcurrentSignals,
		BestSignal:           bestSignal,
		WorstSignal:          worstSignal,
	}
}

func calculateMaxConcurrentSignals(results []internal.SignalResult) int {
	if len(results) == 0 {
		return 0
	}

	// Create a timeline of signal start and end dates
	type event struct {
		date  time.Time
		start bool // true for start, false for end
	}

	var events []event

	// Add start and end events for each signal
	for _, result := range results {
		events = append(events, event{date: result.BuyDate, start: true})
		events = append(events, event{date: result.SellDate, start: false})
	}

	// Sort events by date
	for i := 0; i < len(events)-1; i++ {
		for j := 0; j < len(events)-i-1; j++ {
			if events[j].date.After(events[j+1].date) {
				events[j], events[j+1] = events[j+1], events[j]
			}
		}
	}

	// Count concurrent signals
	currentSignals := 0
	maxConcurrent := 0

	for _, event := range events {
		if event.start {
			currentSignals++
			if currentSignals > maxConcurrent {
				maxConcurrent = currentSignals
			}
		} else {
			currentSignals--
		}
	}

	return maxConcurrent
}

func printResults(results []internal.SignalResult, summary internal.BacktestSummary) {
	fmt.Println("\n=== BACKTEST RESULTS ===")
	fmt.Printf("Total Signals: %d\n", summary.TotalSignals)
	fmt.Printf("Winning Signals: %d\n", summary.WinningSignals)
	fmt.Printf("Losing Signals: %d\n", summary.LosingSignals)
	fmt.Printf("Win Rate: %.2f%%\n", summary.WinRate)
	fmt.Printf("Average Return: %.2f%%\n", summary.AvgReturn)
	fmt.Printf("Average Hold Time: %.1f days\n", summary.AvgHoldTime)
	fmt.Printf("Average Return (Annualized): %.2f%%\n", summary.AvgReturnAnnual)
	fmt.Printf("Total Return: %.2f%%\n", summary.TotalReturn)
	fmt.Printf("Max Concurrent Signals: %d\n", summary.MaxConcurrentSignals)

	fmt.Println("\n=== BEST SIGNAL ===")
	printSignalResult(summary.BestSignal)

	fmt.Println("\n=== WORST SIGNAL ===")
	printSignalResult(summary.WorstSignal)

	fmt.Println("\n=== ALL SIGNALS ===")
	for _, result := range results {
		printSignalResult(result)
	}
}

func printSignalResult(result internal.SignalResult) {
	status := "LOSS"
	if result.IsWin {
		status = "WIN"
	}

	fmt.Printf("%s | %s | Buy: %s (%.2f) | Sell: %s (%.2f) | P/L: %.2f%% | Days: %d\n",
		status,
		result.Ticker,
		result.BuyDate.Format("2006-01-02"),
		result.BuyPrice,
		result.SellDate.Format("2006-01-02"),
		result.SellPrice,
		result.ProfitLossPct,
		result.DaysHeld)
}
