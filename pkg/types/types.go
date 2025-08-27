package types

import (
	"time"

	"github.com/google/uuid"
)

// SignalStatus represents the current status of a trading signal
type SignalStatus string

const (
	SignalStatusPending   SignalStatus = "PENDING"
	SignalStatusBought    SignalStatus = "BOUGHT"
	SignalStatusCompleted SignalStatus = "COMPLETED"
)

// ItemType represents the type of item in the unified table
type ItemType string

const (
	ItemTypeSignal     ItemType = "SIGNAL"
	ItemTypeAllocation ItemType = "ALLOCATION"
)

// UnifiedItem represents a single item in the unified DynamoDB table
type UnifiedItem struct {
	PK        string    `json:"pk" dynamodbav:"pk"`     // Partition key
	SK        string    `json:"sk" dynamodbav:"sk"`     // Sort key
	Type      ItemType  `json:"type" dynamodbav:"type"` // Item type
	Data      string    `json:"data" dynamodbav:"data"` // JSON data
	CreatedAt time.Time `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt time.Time `json:"updated_at" dynamodbav:"updated_at"`
}

// Signal represents a trading signal
type Signal struct {
	UUID      uuid.UUID    `json:"uuid"`
	Ticker    string       `json:"ticker"`
	BuyDate   time.Time    `json:"buy_date"`
	SellDate  time.Time    `json:"sell_date"`
	Status    SignalStatus `json:"status"`
	NumStocks float64      `json:"num_stocks"`
	BuyPrice  float64      `json:"buy_price"`
	SellPrice float64      `json:"sell_price"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// AllocationWindow represents the rolling window for signal allocation
type AllocationWindow struct {
	WindowStartDate      time.Time `json:"window_start_date"`
	WindowEndDate        time.Time `json:"window_end_date"`
	AccountValue         float64   `json:"account_value"`
	AllocationPerSignal  float64   `json:"allocation_per_signal"`
	TotalSignalsInWindow int       `json:"total_signals_in_window"`
	UpdatedAt            time.Time `json:"updated_at"`
}
