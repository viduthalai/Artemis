package brokerage

import "github.com/shopspring/decimal"

type AccountInfo struct {
	TotalAccountValue decimal.Decimal
	Balance           decimal.Decimal
}

// Brokerage defines the interface for the brokerage service
type Brokerage interface {
	// GetCashBalance gets the cash balance
	GetCashBalance() (decimal.Decimal, error)

	// GetPosition gets the position for a symbol
	GetPosition(symbol string) (decimal.Decimal, error)

	// GetPositions gets all current positions
	GetPositions() map[string]interface{}

	// GetAccountValue gets the account value
	GetAccountValue() (decimal.Decimal, error)

	// AddDeposit adds money to the account
	AddDeposit(amount decimal.Decimal) error

	// ExecuteBuyOrder executes a buy order
	ExecuteBuyOrder(symbol string, quantity, limitPrice decimal.Decimal) error

	// ExecuteSellOrder executes a sell order
	ExecuteSellOrder(symbol string, quantity, limitPrice decimal.Decimal) error
}
