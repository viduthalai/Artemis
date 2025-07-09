package signals

import (
	"time"

	"github.com/shopspring/decimal"
)

type Status string

const (
	StatusActive  Status = "ACTIVE"
	StatusPending Status = "PENDING"
	StatusSold    Status = "SOLD"

	DateLayout = "2006-01-02"
)

type Trade struct {
	BuyDateTime  time.Time
	SellDateTime time.Time
	BuyPrice     decimal.Decimal
	SellPrice    decimal.Decimal
	Cost         decimal.Decimal
	Proceeds     decimal.Decimal
	ProfitLoss   decimal.Decimal
	Quantity     decimal.Decimal
}

type Signal struct {
	ID                   int
	Ticker               string
	BuyDateTime          time.Time
	SellDateTime         time.Time
	AllocationPercentage decimal.Decimal
	Status               Status
	InitialTrade         *Trade
	DipTrades            []*Trade
	HighPrice            decimal.Decimal
}

func (s *Signal) IsActive() bool {
	return s.Status == StatusActive
}

func (s *Signal) IsPending() bool {
	return s.Status == StatusPending
}

func (s *Signal) IsSold() bool {
	return s.Status == StatusSold
}

func (s *Signal) GetTotalCost() decimal.Decimal {
	totalCost := s.InitialTrade.Cost
	for _, trade := range s.DipTrades {
		totalCost = totalCost.Add(trade.Cost)
	}
	return totalCost
}
