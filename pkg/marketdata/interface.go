package marketdata

import (
	"time"

	"github.com/shopspring/decimal"
)

type TickerQuote struct {
	// Price is the current price of the stock
	Price decimal.Decimal

	// TechnicalIndicator is the technical indicator of the stock
	TechnicalIndicator
}

// TechnicalIndicator is the technical indicators of the stock
type TechnicalIndicator struct {
	// SMA20 is the 20-day simple moving average of the stock
	SMA20 decimal.Decimal
}

// LiveMarketData is the interface for the live market data
type LiveMarketData interface {
	// GetTickerQuote gets the ticker quote for a given symbol
	GetTickerQuote(symbol string) (*TickerQuote, error)

	// BatchGetTickerQuotes gets the ticker quotes for a given symbols and date
	BatchGetTickerQuotes(symbols []string) (map[string]*TickerQuote, error)
}

// BacktestMarketData is the interface for the backtest market data
type BacktestMarketData interface {
	// GetHistoricalTickerQuote gets the ticker quotes for a given symbol and date range
	GetHistoricalTickerQuote(symbol string, from, to time.Time) (map[time.Time]*TickerQuote, error)

	// IsMarketOpenOnDate checks if the market is open on a given date
	IsMarketOpenOnDate(date time.Time) (bool, error)
}
