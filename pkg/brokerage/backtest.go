package brokerage

import (
	"fmt"
	"sync"

	"github.com/shopspring/decimal"
)

// Position represents a trading position with quantity and average buy price
type Position struct {
	Shares      decimal.Decimal
	AvgBuyPrice decimal.Decimal
}

// BacktestBrokerage implements the Brokerage interface for backtesting
type BacktestBrokerage struct {
	mu        sync.RWMutex
	balance   decimal.Decimal      // cash balance
	positions map[string]*Position // ticker -> position
}

// NewBacktestBrokerage creates a new backtesting brokerage with initial balance
func NewBacktestBrokerage(initialBalance decimal.Decimal) *BacktestBrokerage {
	return &BacktestBrokerage{
		balance:   initialBalance,
		positions: make(map[string]*Position),
	}
}

// GetAccountValue returns the current account value
func (b *BacktestBrokerage) GetAccountValue() (decimal.Decimal, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Calculate account value as balance + positions value
	totalValue := b.balance

	// Add value of all positions (using average buy price as current value)
	for _, position := range b.positions {
		positionValue := position.Shares.Mul(position.AvgBuyPrice)
		totalValue = totalValue.Add(positionValue)
	}

	return totalValue, nil
}

func (b *BacktestBrokerage) GetPosition(symbol string) (decimal.Decimal, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	pos, exists := b.positions[symbol]
	if !exists {
		return decimal.Zero, fmt.Errorf("no position found for symbol %s", symbol)
	}

	return pos.Shares, nil
}

// GetCashBalance returns the current cash balance
func (b *BacktestBrokerage) GetCashBalance() (decimal.Decimal, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.balance, nil
}

// ExecuteBuyOrder executes a buy order at the current market price
func (b *BacktestBrokerage) ExecuteBuyOrder(symbol string, quantity, limitPrice decimal.Decimal) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	price := limitPrice

	// Calculate the total cost of the order
	totalCost := quantity.Mul(price)

	// Check if we have enough cash
	if b.balance.LessThan(totalCost) {
		return fmt.Errorf("insufficient funds: need %s, have %s", totalCost, b.balance)
	}

	// Deduct the cost from balance (cash)
	b.balance = b.balance.Sub(totalCost)

	// Update position (add to existing or create new)
	if existingPos, exists := b.positions[symbol]; exists {
		// Calculate new average buy price
		totalShares := existingPos.Shares.Add(quantity)
		totalValue := existingPos.Shares.Mul(existingPos.AvgBuyPrice).Add(quantity.Mul(price))
		newAvgPrice := totalValue.Div(totalShares)

		existingPos.Shares = totalShares
		existingPos.AvgBuyPrice = newAvgPrice
	} else {
		// Create new position
		b.positions[symbol] = &Position{
			Shares:      quantity,
			AvgBuyPrice: price,
		}
	}

	fmt.Printf("Executed buy order for %s: %s shares at %s\n", symbol, quantity, price)
	return nil
}

// ExecuteSellOrder executes a sell order at the current market price
func (b *BacktestBrokerage) ExecuteSellOrder(symbol string, quantity, limitPrice decimal.Decimal) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Check if we have the position
	existingPos, exists := b.positions[symbol]
	if !exists {
		return fmt.Errorf("no position found for symbol %s", symbol)
	}

	// Check if we have enough shares to sell
	if existingPos.Shares.LessThan(quantity) {
		return fmt.Errorf("insufficient shares: have %s, trying to sell %s", existingPos.Shares, quantity)
	}

	// Calculate the proceeds from the sale
	proceeds := quantity.Mul(limitPrice)

	// Add proceeds to balance (cash)
	b.balance = b.balance.Add(proceeds)

	// Update position
	remainingShares := existingPos.Shares.Sub(quantity)
	if remainingShares.Equal(decimal.Zero) {
		// Remove position if no shares remaining
		delete(b.positions, symbol)
	} else {
		existingPos.Shares = remainingShares
		// Note: AvgBuyPrice remains the same for remaining shares
	}

	fmt.Printf("Executed sell order for %s: %s shares at %s, proceeds: %s\n", symbol, quantity, limitPrice, proceeds)
	return nil
}

// GetPositions returns all current positions
func (b *BacktestBrokerage) GetPositions() map[string]interface{} {
	b.mu.RLock()
	defer b.mu.RUnlock()

	positions := make(map[string]interface{})
	for symbol, pos := range b.positions {
		positions[symbol] = &Position{
			Shares:      pos.Shares,
			AvgBuyPrice: pos.AvgBuyPrice,
		}
	}
	return positions
}

// GetPositionShares returns the number of shares for a specific symbol
func (b *BacktestBrokerage) GetPositionShares(symbol string) (decimal.Decimal, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	pos, exists := b.positions[symbol]
	if !exists {
		return decimal.Zero, fmt.Errorf("no position found for symbol %s", symbol)
	}

	return pos.Shares, nil
}

// AddDeposit adds money to the account
func (b *BacktestBrokerage) AddDeposit(amount decimal.Decimal) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.balance = b.balance.Add(amount)
	// Note: accountValue should be calculated as balance + positions_value, not just balance
	// So we don't update accountValue here - it will be calculated when GetAccountValue is called

	fmt.Printf("Added deposit of $%s. New balance: $%s\n", amount.StringFixed(2), b.balance.StringFixed(2))
	return nil
}
