package internal

import (
	"fmt"
	"log"
	"time"

	alpaca "github.com/alpacahq/alpaca-trade-api-go/v2/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v2/marketdata"
)

// StockSignal represents a single stock signal
type StockSignal struct {
	UUID     string
	Ticker   string
	BuyDate  time.Time
	SellDate time.Time
}

// SignalResult represents the result of backtesting a signal
type SignalResult struct {
	UUID          string
	Ticker        string
	BuyDate       time.Time
	SellDate      time.Time
	BuyPrice      float64
	SellPrice     float64
	ProfitLoss    float64
	ProfitLossPct float64
	DaysHeld      int
	IsWin         bool
}

// BacktestSummary contains overall performance metrics
type BacktestSummary struct {
	TotalSignals         int
	WinningSignals       int
	LosingSignals        int
	WinRate              float64
	AvgReturn            float64
	AvgHoldTime          float64 // Average hold time in days
	AvgReturnAnnual      float64
	TotalReturn          float64
	MaxConcurrentSignals int // Maximum number of concurrent signals
	BestSignal           SignalResult
	WorstSignal          SignalResult
}

// EnhancedBacktestConfig contains configuration for enhanced backtesting
type EnhancedBacktestConfig struct {
	StaggerEntry    bool
	StaggerPercent  float64 // Percentage for initial entry (e.g., 0.8 for 80%)
	TakeProfitPct   float64 // Take profit percentage (e.g., 0.15 for 15%)
	TrailingStopPct float64 // Trailing stop percentage (e.g., 0.15 for 15%)
}

// getClosingPrice retrieves the closing price for a given ticker and date
func GetClosingPrice(marketdataClient marketdata.Client, ticker string, date time.Time) (float64, error) {
	maxAttempts := 10
	currentDate := date

	for attempt := 0; attempt < maxAttempts; attempt++ {
		dateStr := currentDate.Format("2006-01-02")
		params := marketdata.GetBarsParams{
			Start:     currentDate,
			End:       currentDate.Add(24 * time.Hour),
			TimeFrame: marketdata.OneDay,
		}
		bars, err := marketdataClient.GetBars(ticker, params)
		if err != nil {
			currentDate = currentDate.AddDate(0, 0, 1)
			continue
		}
		if len(bars) > 0 {
			closingPrice := bars[0].Close
			if attempt > 0 {
				log.Printf("Found data for %s on %s (original date: %s)", ticker, dateStr, date.Format("2006-01-02"))
			}
			return closingPrice, nil
		}
		currentDate = currentDate.AddDate(0, 0, 1)
	}
	return 0, fmt.Errorf("no data found for %s after trying %d days starting from %s", ticker, maxAttempts, date.Format("2006-01-02"))
}

// backtestSignal performs backtesting on a single signal
func BacktestSignal(client alpaca.Client, marketdataClient marketdata.Client, signal StockSignal) (SignalResult, error) {
	// Get buy price
	buyPrice, err := GetClosingPrice(marketdataClient, signal.Ticker, signal.BuyDate)
	if err != nil {
		return SignalResult{}, fmt.Errorf("error getting buy price: %v", err)
	}

	// Get sell price
	sellPrice, err := GetClosingPrice(marketdataClient, signal.Ticker, signal.SellDate)
	if err != nil {
		return SignalResult{}, fmt.Errorf("error getting sell price: %v", err)
	}

	// Calculate profit/loss
	profitLoss := sellPrice - buyPrice
	profitLossPct := (profitLoss / buyPrice) * 100
	daysHeld := int(signal.SellDate.Sub(signal.BuyDate).Hours() / 24)

	return SignalResult{
		UUID:          signal.UUID,
		Ticker:        signal.Ticker,
		BuyDate:       signal.BuyDate,
		SellDate:      signal.SellDate,
		BuyPrice:      buyPrice,
		SellPrice:     sellPrice,
		ProfitLoss:    profitLoss,
		ProfitLossPct: profitLossPct,
		DaysHeld:      daysHeld,
		IsWin:         profitLoss > 0,
	}, nil
}
