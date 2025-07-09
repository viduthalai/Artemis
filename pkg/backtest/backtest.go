package backtest

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/shopspring/decimal"
	"github.com/vignesh-goutham/artemis/pkg/brokerage"
	"github.com/vignesh-goutham/artemis/pkg/engine"
	"github.com/vignesh-goutham/artemis/pkg/marketdata"
	"github.com/vignesh-goutham/artemis/pkg/signals"
)

type Backtester struct {
	signals    map[int]*signals.Signal
	marketdata marketdata.BacktestMarketData
	brokerage  brokerage.Brokerage
}

func NewBacktester(signals map[int]*signals.Signal, marketdata marketdata.BacktestMarketData) *Backtester {
	brokerageClient := brokerage.NewBacktestBrokerage(decimal.NewFromInt(5000))
	return &Backtester{
		signals:    signals,
		marketdata: marketdata,
		brokerage:  brokerageClient,
	}
}

func (b *Backtester) Run() error {
	// // Original signal-based date calculation (commented out for future use)
	earliestBuyDate := b.signals[0].BuyDateTime
	latestSellDate := b.signals[0].SellDateTime
	for _, signal := range b.signals {
		if signal.BuyDateTime.Before(earliestBuyDate) {
			earliestBuyDate = signal.BuyDateTime
		}
		if signal.SellDateTime.After(latestSellDate) {
			latestSellDate = signal.SellDateTime
		}
	}

	// Add 1 day to latest sell date to ensure we have data for the sell date
	latestSellDate = latestSellDate.AddDate(0, 0, 1)

	fmt.Printf("Fetching historical data from %s to %s\n", earliestBuyDate.Format(signals.DateLayout), latestSellDate.Format(signals.DateLayout))

	// Create a map of ticker to each day's historical data
	historicalData := make(map[string]map[time.Time]*marketdata.TickerQuote)

	for _, signal := range b.signals {
		tickerQuotes, err := b.marketdata.GetHistoricalTickerQuote(signal.Ticker, earliestBuyDate, latestSellDate)
		if err != nil {
			return err
		}
		historicalData[signal.Ticker] = tickerQuotes
	}

	// Process all signals for each day in the range
	// for date := earliestBuyDate; !date.After(latestSellDate); date = date.AddDate(0, 0, 1) {
	for date := earliestBuyDate; !date.After(latestSellDate); date = date.AddDate(0, 0, 1) {
		isMarketOpen, err := b.marketdata.IsMarketOpenOnDate(date)
		if err != nil {
			return err
		}
		if !isMarketOpen {
			fmt.Printf("Market is closed on %s, skipping\n", date.Format(signals.DateLayout))
			continue
		}

		// Add weekly deposit of $500 every Monday
		if date.Weekday() == time.Monday {
			depositAmount := decimal.NewFromInt(500)
			if err := b.brokerage.AddDeposit(depositAmount); err != nil {
				fmt.Printf("Error adding weekly deposit on %s: %v\n", date.Format(signals.DateLayout), err)
			} else {
				fmt.Printf("Added weekly deposit of $%s on %s (Monday)\n", depositAmount.StringFixed(2), date.Format(signals.DateLayout))
			}
		}

		// For each date, create a map of ticker to ticker quote
		tickerQuotes := make(map[string]*marketdata.TickerQuote)
		for ticker, quotes := range historicalData {
			// Get the quote for the current date directly
			if quote, ok := quotes[date]; ok {
				tickerQuotes[ticker] = quote
			}
		}

		eng := engine.New(
			b.signals,
			tickerQuotes,
			b.brokerage,
			date,
		)
		if err := eng.Run(); err != nil {
			return err
		}
	}

	return nil
}

// PrintResults prints the backtest results in a table format
func (b *Backtester) PrintResults() {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("BACKTEST RESULTS")

	// Create table
	table := tablewriter.NewWriter(os.Stdout)
	table.Header("Ticker", "Signal Buy Date", "Actual Buy Date", "Sell Date", "Buy Price", "Sell Price", "Investment", "Sold Amount", "P/L Amount", "P/L %", "Trade Type")

	totalInvestment := decimal.Zero
	totalSoldAmount := decimal.Zero
	totalPL := decimal.Zero

	// Collect all trades for sorting
	type TradeRow struct {
		Ticker          string
		SignalBuyDate   string
		ActualBuyDate   string
		SellDate        string
		BuyPrice        string
		SellPrice       string
		Investment      string
		SoldAmount      string
		PLAmount        string
		PLPercentage    decimal.Decimal
		TradeType       string
		InvestmentValue decimal.Decimal
		SoldAmountValue decimal.Decimal
		PLAmountValue   decimal.Decimal
	}

	var allTrades []TradeRow

	for _, signal := range b.signals {
		if signal.Status == signals.StatusSold && signal.InitialTrade != nil {
			// Add initial trade row
			buyPrice := signal.InitialTrade.BuyPrice
			sellPrice := signal.InitialTrade.SellPrice
			investment := signal.InitialTrade.Cost
			soldAmount := signal.InitialTrade.Proceeds
			plAmount := signal.InitialTrade.ProfitLoss

			// Calculate P/L percentage
			plPercentage := decimal.Zero
			if !investment.Equal(decimal.Zero) {
				plPercentage = plAmount.Div(investment).Mul(decimal.NewFromInt(100))
			}

			allTrades = append(allTrades, TradeRow{
				Ticker:          signal.Ticker,
				SignalBuyDate:   signal.BuyDateTime.Format("2006-01-02"),
				ActualBuyDate:   signal.InitialTrade.BuyDateTime.Format("2006-01-02"),
				SellDate:        signal.SellDateTime.Format("2006-01-02"),
				BuyPrice:        "$" + buyPrice.StringFixed(2),
				SellPrice:       "$" + sellPrice.StringFixed(2),
				Investment:      "$" + investment.StringFixed(2),
				SoldAmount:      "$" + soldAmount.StringFixed(2),
				PLAmount:        "$" + plAmount.StringFixed(2),
				PLPercentage:    plPercentage,
				TradeType:       "Initial",
				InvestmentValue: investment,
				SoldAmountValue: soldAmount,
				PLAmountValue:   plAmount,
			})

			// Add dip buy rows
			for i, dipTrade := range signal.DipTrades {
				if dipTrade != nil {
					dipBuyPrice := dipTrade.BuyPrice
					dipSellPrice := dipTrade.SellPrice
					dipInvestment := dipTrade.Cost
					dipSoldAmount := dipTrade.Proceeds
					dipPLAmount := dipTrade.ProfitLoss

					// Calculate P/L percentage for dip trade
					dipPLPercentage := decimal.Zero
					if !dipInvestment.Equal(decimal.Zero) {
						dipPLPercentage = dipPLAmount.Div(dipInvestment).Mul(decimal.NewFromInt(100))
					}

					allTrades = append(allTrades, TradeRow{
						Ticker:          signal.Ticker,
						SignalBuyDate:   signal.BuyDateTime.Format("2006-01-02"),
						ActualBuyDate:   dipTrade.BuyDateTime.Format("2006-01-02"),
						SellDate:        dipTrade.SellDateTime.Format("2006-01-02"),
						BuyPrice:        "$" + dipBuyPrice.StringFixed(2),
						SellPrice:       "$" + dipSellPrice.StringFixed(2),
						Investment:      "$" + dipInvestment.StringFixed(2),
						SoldAmount:      "$" + dipSoldAmount.StringFixed(2),
						PLAmount:        "$" + dipPLAmount.StringFixed(2),
						PLPercentage:    dipPLPercentage,
						TradeType:       fmt.Sprintf("Dip Buy %d", i+1),
						InvestmentValue: dipInvestment,
						SoldAmountValue: dipSoldAmount,
						PLAmountValue:   dipPLAmount,
					})
				}
			}
		}
	}

	// Sort trades by P/L percentage in descending order (least profitable to most profitable)
	for i := 0; i < len(allTrades); i++ {
		for j := i + 1; j < len(allTrades); j++ {
			if allTrades[i].PLPercentage.LessThan(allTrades[j].PLPercentage) {
				allTrades[i], allTrades[j] = allTrades[j], allTrades[i]
			}
		}
	}

	// Add sorted trades to table and accumulate totals
	for _, trade := range allTrades {
		table.Append(trade.Ticker,
			trade.SignalBuyDate,
			trade.ActualBuyDate,
			trade.SellDate,
			trade.BuyPrice,
			trade.SellPrice,
			trade.Investment,
			trade.SoldAmount,
			trade.PLAmount,
			trade.PLPercentage.StringFixed(2)+"%",
			trade.TradeType)

		// Accumulate totals
		totalInvestment = totalInvestment.Add(trade.InvestmentValue)
		totalSoldAmount = totalSoldAmount.Add(trade.SoldAmountValue)
		totalPL = totalPL.Add(trade.PLAmountValue)
	}

	table.Render()
	fmt.Println(strings.Repeat("=", 80))

	// Print table totals
	fmt.Println("\nTABLE TOTALS:")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("Total Investment: $%s\n", totalInvestment.StringFixed(2))
	fmt.Printf("Total Sold Amount: $%s\n", totalSoldAmount.StringFixed(2))
	fmt.Printf("Total P/L from Trades: $%s\n", totalPL.StringFixed(2))

	// Calculate total P/L percentage from table
	totalPLPercentage := decimal.Zero
	if !totalInvestment.Equal(decimal.Zero) {
		totalPLPercentage = totalPL.Div(totalInvestment).Mul(decimal.NewFromInt(100))
	}
	fmt.Printf("Total P/L %% from Trades: %s%%\n", totalPLPercentage.StringFixed(2))

	// Print P/L Summary
	fmt.Println("\nPROFIT/LOSS SUMMARY:")
	fmt.Println(strings.Repeat("-", 40))

	// Calculate total deposits
	initialDeposit := decimal.NewFromInt(5000) // $100k initial deposit
	weeklyDeposit := decimal.NewFromInt(500)   // $1k weekly deposit

	// Calculate number of weeks between first and last trade
	var firstTradeDate, lastTradeDate time.Time
	for _, signal := range b.signals {
		if signal.InitialTrade != nil {
			if firstTradeDate.IsZero() || signal.InitialTrade.BuyDateTime.Before(firstTradeDate) {
				firstTradeDate = signal.InitialTrade.BuyDateTime
			}
			if signal.InitialTrade.SellDateTime.After(lastTradeDate) {
				lastTradeDate = signal.InitialTrade.SellDateTime
			}
		}
		for _, dipTrade := range signal.DipTrades {
			if dipTrade != nil {
				if dipTrade.SellDateTime.After(lastTradeDate) {
					lastTradeDate = dipTrade.SellDateTime
				}
			}
		}
	}

	weeks := int(lastTradeDate.Sub(firstTradeDate).Hours() / (24 * 7))
	totalDeposits := initialDeposit.Add(weeklyDeposit.Mul(decimal.NewFromInt(int64(weeks))))

	// Get final account value
	finalAccountValue, err := b.brokerage.GetAccountValue()
	if err != nil {
		fmt.Printf("Error getting final account value: %v\n", err)
		return
	}

	// Calculate absolute and percentage P/L
	absolutePL := finalAccountValue.Sub(totalDeposits)
	percentagePL := absolutePL.Div(totalDeposits).Mul(decimal.NewFromInt(100))

	fmt.Printf("Initial Deposit: $%s\n", initialDeposit.StringFixed(2))
	fmt.Printf("Weekly Deposits: $%s x %d weeks = $%s\n",
		weeklyDeposit.StringFixed(2),
		weeks,
		weeklyDeposit.Mul(decimal.NewFromInt(int64(weeks))).StringFixed(2))
	fmt.Printf("Total Deposits: $%s\n", totalDeposits.StringFixed(2))
	fmt.Printf("Final Account Value: $%s\n", finalAccountValue.StringFixed(2))
	fmt.Printf("Total P/L: $%s (%.2f%%)\n", absolutePL.StringFixed(2), percentagePL.InexactFloat64())

	// Print pending positions
	fmt.Println("\nPENDING POSITIONS:")
	fmt.Println(strings.Repeat("-", 40))
	pendingCount := 0
	for _, signal := range b.signals {
		if signal.Status == signals.StatusPending {
			fmt.Printf("Ticker: %s | Signal Buy Date: %s | Sell Date: %s | Allocation: %s%%\n",
				signal.Ticker,
				signal.BuyDateTime.Format("2006-01-02"),
				signal.SellDateTime.Format("2006-01-02"),
				signal.AllocationPercentage.StringFixed(2))
			pendingCount++
		}
	}
	if pendingCount == 0 {
		fmt.Println("No pending positions")
	}

	// Print active positions (positions that have been bought but not sold)
	fmt.Println("\nACTIVE POSITIONS:")
	fmt.Println(strings.Repeat("-", 40))
	activeCount := 0
	for _, signal := range b.signals {
		if signal.Status == signals.StatusActive {
			fmt.Printf("Ticker: %s | Buy Date: %s | Sell Date: %s | Allocation: %s%%\n",
				signal.Ticker,
				signal.BuyDateTime.Format("2006-01-02"),
				signal.SellDateTime.Format("2006-01-02"),
				signal.AllocationPercentage.StringFixed(2))
			activeCount++
		}
	}
	if activeCount == 0 {
		fmt.Println("No active positions")
	}

	// Print current positions in brokerage account
	fmt.Println("\nCURRENT POSITIONS IN ACCOUNT:")
	fmt.Println(strings.Repeat("-", 40))
	positions := b.brokerage.GetPositions()
	if len(positions) == 0 {
		fmt.Println("No positions in account")
	} else {
		for symbol, positionInterface := range positions {
			if position, ok := positionInterface.(*brokerage.Position); ok {
				fmt.Printf("Ticker: %s | Shares: %s | Avg Buy Price: $%s | Position Value: $%s\n",
					symbol,
					position.Shares.StringFixed(4),
					position.AvgBuyPrice.StringFixed(2),
					position.Shares.Mul(position.AvgBuyPrice).StringFixed(2))
			}
		}
	}
	fmt.Println(strings.Repeat("=", 80))
}
