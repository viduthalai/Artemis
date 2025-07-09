package engine

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/vignesh-goutham/artemis/pkg/brokerage"
	"github.com/vignesh-goutham/artemis/pkg/marketdata"
	"github.com/vignesh-goutham/artemis/pkg/signals"
)

type Engine struct {
	signals     map[int]*signals.Signal
	tickerData  map[string]*marketdata.TickerQuote
	brokerage   brokerage.Brokerage
	currentDate time.Time
}

func New(sigs map[int]*signals.Signal, tickerData map[string]*marketdata.TickerQuote, brokerage brokerage.Brokerage, currentDate time.Time) *Engine {
	return &Engine{
		signals:     sigs,
		tickerData:  tickerData,
		brokerage:   brokerage,
		currentDate: currentDate,
	}
}

func (e *Engine) GetSignals() map[int]*signals.Signal {
	return e.signals
}

func (e *Engine) Run() error {
	for _, signal := range e.signals {

		// If the signal is pending and the current date is after the buy date, buy the stock
		if signal.Status == signals.StatusPending && (e.currentDate.After(signal.BuyDateTime) || e.currentDate.Equal(signal.BuyDateTime)) {
			fmt.Printf("Buy conditions met for %s at %s\n", signal.Ticker, e.currentDate.Format(signals.DateLayout))
			if _, ok := e.tickerData[signal.Ticker]; !ok {
				fmt.Printf("No ticker quote found for %s at %s, skipping\n", signal.Ticker, e.currentDate.Format(signals.DateLayout))
				continue
			}
			err := e.ExecuteBuy(signal)
			if err != nil {
				fmt.Printf("Error executing buy for %s: %s\n", signal.Ticker, err)
				continue
			}
		} else if signal.Status == signals.StatusActive {
			if e.currentDate.After(signal.SellDateTime) || e.currentDate.Equal(signal.SellDateTime) {
				fmt.Printf("Sell conditions met for %s at %s\n", signal.Ticker, e.currentDate.Format(signals.DateLayout))
				if _, ok := e.tickerData[signal.Ticker]; !ok {
					fmt.Printf("No ticker quote found for %s at %s, skipping\n", signal.Ticker, e.currentDate.Format(signals.DateLayout))
					continue
				}
				err := e.ExecuteSell(signal)
				if err != nil {
					fmt.Printf("Error executing sell for %s: %s\n", signal.Ticker, err)
					continue
				}
			} else if e.CheckDipBuyConditions(signal) {
				fmt.Printf("Dip buy conditions met for %s at %s\n", signal.Ticker, e.currentDate.Format(signals.DateLayout))
				err := e.ExecuteDipBuy(signal)
				if err != nil {
					fmt.Printf("Error executing dip buy for %s: %s\n", signal.Ticker, err)
					continue
				}
			}
		}
	}
	e.UpdateSignalsHighPrice()
	return nil
}

func (e *Engine) ExecuteBuy(signal *signals.Signal) error {
	tickerPrice := e.tickerData[signal.Ticker].Price

	// Use the new allocation calculation method
	allocationAmount, err := e.CalculateAllocationAmount(signal)
	if err != nil {
		return fmt.Errorf("failed to calculate allocation amount: %v", err)
	}

	quantity := allocationAmount.Div(tickerPrice)
	err = e.brokerage.ExecuteBuyOrder(signal.Ticker, quantity, tickerPrice)
	if err != nil {
		return err
	}

	signal.Status = signals.StatusActive
	signal.InitialTrade = &signals.Trade{
		BuyDateTime: e.currentDate,
		BuyPrice:    tickerPrice,
		Cost:        allocationAmount,
		Quantity:    quantity,
	}
	return nil
}

func (e *Engine) ExecuteDipBuy(signal *signals.Signal) error {
	tickerPrice := e.tickerData[signal.Ticker].Price

	// Use the new allocation calculation method (should return initial trade amount for dip buys)
	allocationAmount, err := e.CalculateAllocationAmount(signal)
	if err != nil {
		return fmt.Errorf("failed to calculate allocation amount for dip buy: %v", err)
	}

	quantity := allocationAmount.Div(tickerPrice)
	err = e.brokerage.ExecuteBuyOrder(signal.Ticker, quantity, tickerPrice)
	if err != nil {
		return err
	}

	// Create dip trade
	dipTrade := &signals.Trade{
		BuyDateTime: e.currentDate,
		BuyPrice:    tickerPrice,
		Cost:        allocationAmount,
		Quantity:    quantity,
	}

	// Add dip trade to signal
	signal.DipTrades = append(signal.DipTrades, dipTrade)
	return nil
}

func (e *Engine) ExecuteSell(signal *signals.Signal) error {
	tickerPrice := e.tickerData[signal.Ticker].Price

	totalQuantity := signal.InitialTrade.Quantity

	// Add dip trade quantities
	for _, dipTrade := range signal.DipTrades {
		if dipTrade != nil {
			totalQuantity = totalQuantity.Add(dipTrade.Quantity)
		}
	}

	// Validate that we have shares to sell
	if totalQuantity.Equal(decimal.Zero) {
		return fmt.Errorf("no shares to sell for signal %s", signal.Ticker)
	}

	// Validate that the position exists in the account
	accountPosition, err := e.brokerage.GetPosition(signal.Ticker)
	if err != nil {
		return fmt.Errorf("position not found in account for %s: %v", signal.Ticker, err)
	}

	// Validate that we have enough shares in the account
	if accountPosition.LessThan(totalQuantity) {
		return fmt.Errorf("insufficient shares in account for %s: have %s, need %s",
			signal.Ticker, accountPosition, totalQuantity)
	}

	err = e.brokerage.ExecuteSellOrder(signal.Ticker, totalQuantity, tickerPrice)
	if err != nil {
		return err
	}

	// Update initial trade with its own proceeds based on its quantity
	if signal.InitialTrade != nil {
		initialProceeds := signal.InitialTrade.Quantity.Mul(tickerPrice)
		signal.InitialTrade.SellDateTime = e.currentDate
		signal.InitialTrade.SellPrice = tickerPrice
		signal.InitialTrade.Proceeds = initialProceeds
		signal.InitialTrade.ProfitLoss = initialProceeds.Sub(signal.InitialTrade.Cost)
	}

	// Update dip trades with their own proceeds based on their quantities
	for _, dipTrade := range signal.DipTrades {
		if dipTrade != nil {
			dipProceeds := dipTrade.Quantity.Mul(tickerPrice)
			dipTrade.SellDateTime = e.currentDate
			dipTrade.SellPrice = tickerPrice
			dipTrade.Proceeds = dipProceeds
			dipTrade.ProfitLoss = dipProceeds.Sub(dipTrade.Cost)
		}
	}

	signal.Status = signals.StatusSold
	return nil
}

// CheckDipBuyConditions checks if dip buy conditions are met for a signal
func (e *Engine) CheckDipBuyConditions(signal *signals.Signal) bool {
	// Get current ticker data
	tickerQuote, exists := e.tickerData[signal.Ticker]
	if !exists {
		fmt.Printf("No ticker data available for %s, skipping dip buy check\n", signal.Ticker)
		return false
	}

	currentPrice := tickerQuote.Price
	sma20 := tickerQuote.SMA20
	high20 := signal.HighPrice

	// Condition 1: Current price is below 20-SMA
	if currentPrice.GreaterThanOrEqual(sma20) {
		fmt.Printf("Dip buy condition failed for %s: Current price ($%s) is not below 20-SMA ($%s)\n",
			signal.Ticker, currentPrice.StringFixed(2), sma20.StringFixed(2))
		return false
	}

	// Condition 2: Current price has dropped 10% from 20-day high
	dropPercentage := high20.Sub(currentPrice).Div(high20).Mul(decimal.NewFromInt(100))
	if dropPercentage.LessThan(decimal.NewFromInt(10)) {
		fmt.Printf("Dip buy condition failed for %s: Price drop ($%s) is only %s%% from 20-day high ($%s), need 10%%\n",
			signal.Ticker, high20.Sub(currentPrice).StringFixed(2), dropPercentage.StringFixed(2), high20.StringFixed(2))
		return false
	}

	// Condition 3: SMA-20 is above the initial buy price, indicating overall uptrend
	if sma20.LessThan(signal.InitialTrade.BuyPrice) {
		fmt.Printf("Dip buy condition failed for %s: 20-SMA ($%s) is less than initial buy price ($%s)\n",
			signal.Ticker, sma20.StringFixed(2), signal.InitialTrade.BuyPrice.StringFixed(2))
		return false
	}

	// Condition 4: At least 5 days have passed since the last buy
	lastBuyDate := signal.BuyDateTime
	if signal.InitialTrade != nil {
		lastBuyDate = signal.InitialTrade.BuyDateTime
	}

	// Check if there are any dip trades and get the latest one
	for _, dipTrade := range signal.DipTrades {
		if dipTrade != nil && dipTrade.BuyDateTime.After(lastBuyDate) {
			lastBuyDate = dipTrade.BuyDateTime
		}
	}

	daysSinceLastBuy := e.currentDate.Sub(lastBuyDate).Hours() / 24
	if daysSinceLastBuy < 5 {
		fmt.Printf("Dip buy condition failed for %s: Only %.1f days since last buy, need at least 5 days\n",
			signal.Ticker, daysSinceLastBuy)
		return false
	}

	// Condition 5: Current price is below initial buy price
	if currentPrice.GreaterThanOrEqual(signal.InitialTrade.BuyPrice) {
		fmt.Printf("Dip buy condition failed for %s: Current price ($%s) is not below initial buy price ($%s)\n",
			signal.Ticker, currentPrice.StringFixed(2), signal.InitialTrade.BuyPrice.StringFixed(2))
		return false
	}

	// Condition 6: Maximum of 3 additional dip buys only
	dipBuyCount := len(signal.DipTrades)
	if dipBuyCount >= 2 {
		fmt.Printf("Dip buy condition failed for %s: Already have %d dip buys, maximum is 3\n",
			signal.Ticker, dipBuyCount)
		return false
	}

	fmt.Printf("All dip buy conditions met for %s:\n", signal.Ticker)
	fmt.Printf("  - Current price ($%s) is below 20-SMA ($%s)\n", currentPrice.StringFixed(2), sma20.StringFixed(2))
	fmt.Printf("  - Price dropped %s%% from 20-day high ($%s)\n", dropPercentage.StringFixed(2), high20.StringFixed(2))
	fmt.Printf("  - %.1f days since last buy\n", daysSinceLastBuy)
	fmt.Printf("  - Current price ($%s) is below initial buy price ($%s)\n", currentPrice.StringFixed(2), signal.InitialTrade.BuyPrice.StringFixed(2))
	fmt.Printf("  - Current dip buy count: %d\n", dipBuyCount)

	return true
}

func (e *Engine) UpdateSignalsHighPrice() {
	for _, signal := range e.signals {
		tickerQuote, exists := e.tickerData[signal.Ticker]
		if !exists {
			fmt.Printf("No ticker data available for %s, skipping high price update\n", signal.Ticker)
			continue
		}

		if tickerQuote.Price.GreaterThan(signal.HighPrice) {
			signal.HighPrice = tickerQuote.Price
		}
	}
}

// CalculateAllocationAmount calculates the allocation amount for a signal based on current account value and signal status
func (e *Engine) CalculateAllocationAmount(signal *signals.Signal) (decimal.Decimal, error) {
	// For dip buys, always use the initial trade amount
	if signal.Status == signals.StatusActive && signal.InitialTrade != nil {
		// Check if this is a dip buy scenario by looking at the current date
		// If we're past the initial buy date and the signal is active, this could be a dip buy
		if e.currentDate.After(signal.InitialTrade.BuyDateTime) {
			return signal.InitialTrade.Cost, nil
		}
	}

	// Get all non-pending signals (active and sold ones)
	var liveSignals []*signals.Signal
	for _, s := range e.signals {
		if s.Status == signals.StatusPending || s.Status == signals.StatusActive {
			liveSignals = append(liveSignals, s)
		}
	}

	// Calculate total percentage sum
	totalPercentage := decimal.Zero
	for _, s := range liveSignals {
		totalPercentage = totalPercentage.Add(s.AllocationPercentage)
	}

	// Get total account value (cash + stocks)
	accountValue, err := e.brokerage.GetAccountValue()
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to get account value: %v", err)
	}

	var allocationAmount decimal.Decimal
	var normalizedPercentage decimal.Decimal

	if totalPercentage.LessThanOrEqual(decimal.NewFromInt(100)) {
		// If sum <= 100%, use the signal's allocation percentage directly
		normalizedPercentage = signal.AllocationPercentage
		allocationAmount = normalizedPercentage.Div(decimal.NewFromInt(100)).Mul(accountValue)
	} else {
		// If sum > 100%, normalize the percentage to 100%
		normalizedPercentage = signal.AllocationPercentage.Div(totalPercentage).Mul(decimal.NewFromInt(100))
		allocationAmount = normalizedPercentage.Div(decimal.NewFromInt(100)).Mul(accountValue)
	}

	fmt.Printf("Allocation calculation for %s:\n", signal.Ticker)
	fmt.Printf("  - Signal allocation percentage: %s%%\n", signal.AllocationPercentage.StringFixed(2))
	fmt.Printf("  - Total non-pending signals percentage: %s%%\n", totalPercentage.StringFixed(2))
	fmt.Printf("  - Account value: $%s\n", accountValue.StringFixed(2))
	fmt.Printf("  - Normalized percentage: %s%%\n", normalizedPercentage.StringFixed(2))
	fmt.Printf("  - Calculated allocation amount: $%s\n", allocationAmount.StringFixed(2))

	return allocationAmount, nil
}
