package alpaca

import (
	"fmt"
	"os"
	"time"

	alpacadata "github.com/alpacahq/alpaca-trade-api-go/v3/marketdata"
	"github.com/shopspring/decimal"
	"github.com/vignesh-goutham/artemis/pkg/marketdata"
)

// AlpacaMarketData implements the BacktestMarketData interface for historical data
type AlpacaMarketData struct {
	client *alpacadata.Client
}

// NewAlpacaMarketData creates a new Alpaca market data client for historical data
func NewAlpacaMarketData() (*AlpacaMarketData, error) {
	apiKey := os.Getenv("ALPACA_API_KEY")
	secretKey := os.Getenv("ALPACA_SECRET_KEY")

	if apiKey == "" || secretKey == "" {
		return nil, fmt.Errorf("ALPACA_API_KEY and ALPACA_SECRET_KEY must be set")
	}

	client := alpacadata.NewClient(alpacadata.ClientOpts{
		APIKey:    apiKey,
		APISecret: secretKey,
	})

	return &AlpacaMarketData{
		client: client,
	}, nil
}

// GetHistoricalTickerQuote gets historical ticker quotes for a given symbol and date range
func (a *AlpacaMarketData) GetHistoricalTickerQuote(symbol string, from, to time.Time) (map[time.Time]*marketdata.TickerQuote, error) {
	// Extend the date range by 30 days to calculate SMA20
	smaFrom := from.AddDate(0, 0, -30)

	// Get historical bars
	bars, err := a.client.GetBars(symbol, alpacadata.GetBarsRequest{
		Start:     smaFrom,
		End:       to,
		TimeFrame: alpacadata.OneDay,
	})
	if err != nil {
		return nil, fmt.Errorf("error getting historical bars for %s: %w", symbol, err)
	}

	result := make(map[time.Time]*marketdata.TickerQuote)

	// Process each bar
	for i, bar := range bars {
		// Convert timestamp to time.Time and truncate to day
		date := bar.Timestamp.Truncate(24 * time.Hour)

		// Only include data points within the requested range
		if date.Before(from) || date.After(to) {
			continue
		}

		// Calculate SMA20 using the last 20 days of data
		sma20 := decimal.Zero

		if i >= 19 { // Need at least 20 data points for SMA20
			var sum decimal.Decimal

			// Calculate SMA20 from the last 20 days
			for j := i - 19; j <= i; j++ {
				if j >= 0 && j < len(bars) {
					price := decimal.NewFromFloat(bars[j].Close)
					sum = sum.Add(price)
				}
			}

			if sum.GreaterThan(decimal.Zero) {
				sma20 = sum.Div(decimal.NewFromInt(20))
			}
		}

		// Create TickerQuote
		quote := &marketdata.TickerQuote{
			Price: decimal.NewFromFloat(bar.Close),
			TechnicalIndicator: marketdata.TechnicalIndicator{
				SMA20: sma20,
			},
		}

		result[date] = quote
	}

	return a.cleanQuotes(result), nil
}

// IsMarketOpenOnDate checks if the market is open on a given date
func (a *AlpacaMarketData) IsMarketOpenOnDate(date time.Time) (bool, error) {
	// First, check if the date is a weekend (Saturday = 6, Sunday = 0)
	weekday := date.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return false, nil
	}

	// Use a simple approach: try to get OHLC data for the specific date
	// If we get data, the market was open; if we get no data, it was closed
	from := date.Truncate(24 * time.Hour)
	to := from.Add(24 * time.Hour)

	// Use a simple ticker like SPY (S&P 500 ETF) which is very liquid and always trades when market is open
	bars, err := a.client.GetBars("SPY", alpacadata.GetBarsRequest{
		Start:     from,
		End:       to,
		TimeFrame: alpacadata.OneDay,
	})
	if err != nil {
		return false, fmt.Errorf("error checking market data for date %s: %w", date.Format("2006-01-02"), err)
	}

	return len(bars) > 0, nil
}

// cleanQuotes ensures all dates are normalized to UTC midnight
func (a *AlpacaMarketData) cleanQuotes(quotes map[time.Time]*marketdata.TickerQuote) map[time.Time]*marketdata.TickerQuote {
	cleanedQuotes := make(map[time.Time]*marketdata.TickerQuote)

	for date, quote := range quotes {
		year, month, day := date.Date()
		cleanedDate := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		cleanedQuotes[cleanedDate] = quote
	}

	return cleanedQuotes
}
