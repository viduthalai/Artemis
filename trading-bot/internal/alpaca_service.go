package internal

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	alpaca "github.com/alpacahq/alpaca-trade-api-go/v2/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v2/marketdata"
	"github.com/shopspring/decimal"
)

// AlpacaService handles all Alpaca trading operations
type AlpacaService struct {
	client     alpaca.Client
	marketData marketdata.Client
	config     *Config
}

// NewAlpacaService creates a new Alpaca service instance
func NewAlpacaService(config *Config) (*AlpacaService, error) {
	// Determine URLs based on paper trading flag
	var tradingBaseURL, marketDataBaseURL string

	if config.IsPaperTrading {
		tradingBaseURL = "https://paper-api.alpaca.markets"
		marketDataBaseURL = "https://data.alpaca.markets"
	} else {
		tradingBaseURL = "https://api.alpaca.markets"
		marketDataBaseURL = "https://data.alpaca.markets"
	}

	client := alpaca.NewClient(alpaca.ClientOpts{
		ApiKey:    config.AlpacaAPIKey,
		ApiSecret: config.AlpacaSecretKey,
		BaseURL:   tradingBaseURL,
	})

	marketDataClient := marketdata.NewClient(marketdata.ClientOpts{
		ApiKey:    config.AlpacaAPIKey,
		ApiSecret: config.AlpacaSecretKey,
		BaseURL:   marketDataBaseURL,
	})

	return &AlpacaService{
		client:     client,
		marketData: marketDataClient,
		config:     config,
	}, nil
}

// GetAccountValue retrieves the current account value
func (a *AlpacaService) GetAccountValue(ctx context.Context) (float64, error) {
	account, err := a.client.GetAccount()
	if err != nil {
		return 0, fmt.Errorf("failed to get account: %w", err)
	}

	// Convert from decimal to float64
	value, _ := account.PortfolioValue.Float64()
	return value, nil
}

// GetCashBalance retrieves the current cash balance
func (a *AlpacaService) GetCashBalance(ctx context.Context) (float64, error) {
	account, err := a.client.GetAccount()
	if err != nil {
		return 0, fmt.Errorf("failed to get account: %w", err)
	}

	// Convert from decimal to float64
	cash, _ := account.Cash.Float64()
	return cash, nil
}

// GetCurrentPrice retrieves the current ask price for a ticker (for buying)
func (a *AlpacaService) GetCurrentPrice(ctx context.Context, ticker string) (float64, error) {
	// Get the latest quote
	quote, err := a.marketData.GetLatestQuote(ticker)
	if err != nil {
		return 0, fmt.Errorf("failed to get latest quote for %s: %w", ticker, err)
	}

	// Use ask price as current price (what we'd pay to buy)
	return quote.AskPrice, nil
}

// GetBidPrice retrieves the current bid price for a ticker (for selling)
func (a *AlpacaService) GetBidPrice(ctx context.Context, ticker string) (float64, error) {
	// Get the latest quote
	quote, err := a.marketData.GetLatestQuote(ticker)
	if err != nil {
		return 0, fmt.Errorf("failed to get latest quote for %s: %w", ticker, err)
	}

	// Use bid price as current price (what we'd receive to sell)
	return quote.BidPrice, nil
}

// IsFractionable checks if a ticker supports fractional shares
func (a *AlpacaService) IsFractionable(ctx context.Context, ticker string) (bool, error) {
	asset, err := a.client.GetAsset(ticker)
	if err != nil {
		return false, fmt.Errorf("failed to get asset info for %s: %w", ticker, err)
	}
	return asset.Fractionable, nil
}

// BuyStock executes a buy order for the specified ticker and allocation
func (a *AlpacaService) BuyStock(ctx context.Context, ticker string, allocation float64) (*alpaca.Order, error) {
	// Calculate the number of shares based on allocation and current price
	currentPrice, err := a.GetCurrentPrice(ctx, ticker)
	if err != nil {
		return nil, fmt.Errorf("failed to get current price for %s: %w", ticker, err)
	}

	// Calculate shares based on allocation amount
	shares := allocation / currentPrice
	if shares <= 0 {
		return nil, fmt.Errorf("allocation amount %.2f is too small for current price %.2f", allocation, currentPrice)
	}

	// Check if the ticker supports fractional shares
	isFractionable, err := a.IsFractionable(ctx, ticker)
	if err != nil {
		log.Printf("Warning: Could not check if %s supports fractional shares, assuming it does: %v", ticker, err)
		isFractionable = true // Default to fractional shares if we can't check
	}

	// If not fractionable, round down to whole number of shares
	if !isFractionable {
		shares = math.Floor(shares)
		if shares <= 0 {
			return nil, fmt.Errorf("allocation amount %.2f results in 0 shares for non-fractionable ticker %s at price %.2f", allocation, ticker, currentPrice)
		}
		log.Printf("Rounded down to %f whole shares for non-fractionable ticker %s", shares, ticker)
	}

	// Create the buy order with limit price at 99% of ask price
	qty := decimal.NewFromFloat(shares)
	// Round limit price to 2 decimal places to meet Alpaca's pricing requirements
	limitPriceValue := math.Round(currentPrice*0.99*100) / 100
	limitPrice := decimal.NewFromFloat(limitPriceValue)
	orderRequest := alpaca.PlaceOrderRequest{
		AssetKey:    &ticker,
		Qty:         &qty,
		Side:        alpaca.Buy,
		Type:        alpaca.Limit,
		TimeInForce: alpaca.Day,
		LimitPrice:  &limitPrice,
	}

	order, err := a.client.PlaceOrder(orderRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to place buy order for %s: %w", ticker, err)
	}

	log.Printf("Placed buy order for %s: %f shares at limit price $%.2f", ticker, shares, limitPriceValue)
	return order, nil
}

// SellStock executes a sell order for the specified ticker and quantity
func (a *AlpacaService) SellStock(ctx context.Context, ticker string, quantity float64) (*alpaca.Order, error) {
	// Check if we have enough shares to sell
	currentPosition, err := a.GetPosition(ctx, ticker)
	if err != nil {
		return nil, fmt.Errorf("failed to get position for %s: %w", ticker, err)
	}

	if currentPosition < quantity {
		return nil, fmt.Errorf("insufficient shares to sell: have %.2f, trying to sell %.2f", currentPosition, quantity)
	}

	// Get current bid price for limit order
	bidPrice, err := a.GetBidPrice(ctx, ticker)
	if err != nil {
		return nil, fmt.Errorf("failed to get bid price for %s: %w", ticker, err)
	}

	// Create the sell order with limit price at 101% of bid price
	qty := decimal.NewFromFloat(quantity)
	// Round limit price to 2 decimal places to meet Alpaca's pricing requirements
	limitPriceValue := math.Round(bidPrice*1.01*100) / 100
	limitPrice := decimal.NewFromFloat(limitPriceValue)
	orderRequest := alpaca.PlaceOrderRequest{
		AssetKey:    &ticker,
		Qty:         &qty,
		Side:        alpaca.Sell,
		Type:        alpaca.Limit,
		TimeInForce: alpaca.GTC, // Good Till Canceled - ensures the order stays active until filled or manually canceled
		LimitPrice:  &limitPrice,
	}

	order, err := a.client.PlaceOrder(orderRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to place sell order for %s: %w", ticker, err)
	}

	log.Printf("Placed sell order for %s: %f shares at limit price $%.2f", ticker, quantity, bidPrice*1.05)
	return order, nil
}

// GetOrderStatus retrieves the status of an order
func (a *AlpacaService) GetOrderStatus(ctx context.Context, orderID string) (*alpaca.Order, error) {
	order, err := a.client.GetOrder(orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order status: %w", err)
	}

	return order, nil
}

// IsMarketOpen checks if the market is currently open
func (a *AlpacaService) IsMarketOpen(ctx context.Context) (bool, error) {
	clock, err := a.client.GetClock()
	if err != nil {
		return false, fmt.Errorf("failed to get market clock: %w", err)
	}

	return clock.IsOpen, nil
}

// GetNextMarketOpen gets the next market open time
func (a *AlpacaService) GetNextMarketOpen(ctx context.Context) (time.Time, error) {
	clock, err := a.client.GetClock()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get market clock: %w", err)
	}

	return clock.NextOpen, nil
}

// GetPosition retrieves the current position for a ticker
func (a *AlpacaService) GetPosition(ctx context.Context, ticker string) (float64, error) {
	position, err := a.client.GetPosition(ticker)
	if err != nil {
		return 0, fmt.Errorf("failed to get position for %s: %w", ticker, err)
	}

	// Convert from decimal to float64
	qty, _ := position.Qty.Float64()
	return qty, nil
}
