package internal

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/vignesh-goutham/artemis/pkg/dynamodb"
	"github.com/vignesh-goutham/artemis/pkg/types"
	"github.com/vignesh-goutham/artemis/trading-bot/pkg/notification"
)

// TradingBot orchestrates the trading operations
type TradingBot struct {
	config              *Config
	dbService           *dynamodb.Service
	alpacaService       *AlpacaService
	notificationService *notification.DiscordNotificationService
	signals             []types.Signal
	signalsToDelete     []types.Signal // Track signals that need to be deleted
	allocationWindow    *types.AllocationWindow
	errorCount          int
	processedCount      int
}

// NewTradingBot creates a new trading bot instance
func NewTradingBot(config *Config) (*TradingBot, error) {
	dbService, err := dynamodb.NewService(config.DynamoDBRegion, config.TableName)
	if err != nil {
		return nil, fmt.Errorf("failed to create DynamoDB service: %w", err)
	}

	alpacaService, err := NewAlpacaService(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Alpaca service: %w", err)
	}

	notificationService := notification.NewDiscordNotificationService(config.DiscordWebhookURL)

	return &TradingBot{
		config:              config,
		dbService:           dbService,
		alpacaService:       alpacaService,
		notificationService: notificationService,
		signals:             []types.Signal{},
		signalsToDelete:     []types.Signal{},
		allocationWindow:    nil,
		errorCount:          0,
		processedCount:      0,
	}, nil
}

// Run executes the main trading bot logic
func (tb *TradingBot) Run(ctx context.Context) error {
	log.Println("Starting Artemis Trading Bot...")

	// Check if market is open
	// isOpen, err := tb.alpacaService.IsMarketOpen(ctx)
	// if err != nil {
	// 	log.Printf("Warning: Could not check market status: %v", err)
	// 	tb.notificationService.NotifyError("Market Status Check", "Could not check market status", err.Error())
	// } else if !isOpen {
	// 	log.Println("Market is closed. Skipping trading operations.")
	// 	tb.notificationService.NotifyMarketClosed()
	// 	return nil
	// }

	// Load all data from DynamoDB into memory
	err := tb.loadData(ctx)
	if err != nil {
		tb.notificationService.NotifyError("Data Load", "Failed to load data from DynamoDB", err.Error())
		return fmt.Errorf("failed to load data: %w", err)
	}

	// Update allocation window if needed
	err = tb.updateAllocationWindow(ctx)
	if err != nil {
		log.Printf("Warning: Failed to update allocation window: %v", err)
		tb.notificationService.NotifyError("Allocation Window", "Failed to update allocation window", err.Error())
		return err
	}

	// Get current allocation per signal
	allocationPerSignal, err := tb.getAllocationPerSignal()
	if err != nil {
		log.Printf("Warning: Failed to get allocation per signal: %v", err)
		allocationPerSignal = tb.config.DefaultAllocationAmount
	}

	log.Printf("Found %d active signals", len(tb.signals))

	// Process each signal
	for i := range tb.signals {
		err := tb.processSignal(ctx, &tb.signals[i], allocationPerSignal)
		if err != nil {
			log.Printf("Error processing signal %s: %v", tb.signals[i].UUID, err)
			tb.errorCount++
			tb.notificationService.NotifyError("Signal Processing", fmt.Sprintf("Error processing signal %s", tb.signals[i].UUID), err.Error())
			continue
		}
		tb.processedCount++
	}

	// Save all changes back to DynamoDB
	err = tb.saveData(ctx)
	if err != nil {
		tb.notificationService.NotifyError("Data Save", "Failed to save data to DynamoDB", err.Error())
		return fmt.Errorf("failed to save data: %w", err)
	}

	// Send bot completion notification with account summary (only notification with @everyone)
	accountValue, _ := tb.alpacaService.GetAccountValue(ctx)
	cashBalance, _ := tb.alpacaService.GetCashBalance(ctx)
	tb.notificationService.NotifyBotComplete(tb.processedCount, tb.errorCount, accountValue, cashBalance, len(tb.signals))

	log.Println("Trading bot run completed")
	return nil
}

// processSignal handles a single signal based on its status
func (tb *TradingBot) processSignal(ctx context.Context, signal *types.Signal, allocationPerSignal float64) error {
	currentDate := time.Now().UTC().Truncate(24 * time.Hour)
	oldStatus := signal.Status

	switch signal.Status {
	case types.SignalStatusPending:
		err := tb.processPendingSignal(ctx, signal, allocationPerSignal, currentDate)
		if err != nil {
			return err
		}
		// If status changed, track the old state for deletion
		if signal.Status != oldStatus {
			// Create a copy of the signal with the old status for deletion
			oldSignal := *signal
			oldSignal.Status = oldStatus
			tb.signalsToDelete = append(tb.signalsToDelete, oldSignal)
			log.Printf("Signal %s status changed from %s to %s, will delete old record", signal.UUID, oldStatus, signal.Status)
		}
		return nil
	case types.SignalStatusBought:
		err := tb.processBoughtSignal(ctx, signal, currentDate)
		if err != nil {
			return err
		}
		// If status changed to completed, track for deletion
		if signal.Status == types.SignalStatusCompleted {
			// Create a copy of the signal with the current status for deletion
			signalToDelete := *signal
			tb.signalsToDelete = append(tb.signalsToDelete, signalToDelete)
			log.Printf("Signal %s completed, will delete from database", signal.UUID)
		}
		return nil
	default:
		log.Printf("Unknown signal status: %s for signal %s", signal.Status, signal.UUID)
		return nil
	}
}

// processPendingSignal handles signals that are pending execution
func (tb *TradingBot) processPendingSignal(ctx context.Context, signal *types.Signal, allocationPerSignal float64, currentDate time.Time) error {
	buyDate := signal.BuyDate.UTC().Truncate(24 * time.Hour)

	if currentDate.Before(buyDate) {
		log.Printf("Signal %s buy date %s is in the future, skipping", signal.UUID, buyDate.Format("2006-01-02"))
		return nil
	}

	log.Printf("Processing pending signal %s for %s", signal.UUID, signal.Ticker)

	// Execute buy order
	order, err := tb.alpacaService.BuyStock(ctx, signal.Ticker, allocationPerSignal)
	if err != nil {
		return fmt.Errorf("failed to buy stock for signal %s: %w", signal.UUID, err)
	}

	// Get order details
	shares, _ := order.Qty.Float64()

	// For market orders, get the actual execution price from the order
	// Market orders execute immediately, so the filled average price should be available
	var executionPrice float64
	if order.FilledAvgPrice != nil {
		executionPrice, _ = order.FilledAvgPrice.Float64()
	} else {
		// Fallback to current price if filled average price is not available yet
		executionPrice, err = tb.alpacaService.GetCurrentPrice(ctx, signal.Ticker)
		if err != nil {
			log.Printf("Warning: Could not get execution price for %s: %v", signal.Ticker, err)
			executionPrice = 0
		}
	}

	if shares > 0 {
		signal.NumStocks = shares
		signal.BuyPrice = executionPrice
		signal.Status = types.SignalStatusBought
		signal.UpdatedAt = time.Now()

		// Signal is already updated in memory, will be saved at the end

		// Send Discord notification
		tb.notificationService.NotifySignalBought(signal.Ticker, shares, executionPrice, signal.BuyDate, signal.SellDate)

		log.Printf("Successfully placed market buy order for %f shares of %s at $%.2f for signal %s",
			shares, signal.Ticker, executionPrice, signal.UUID)
	} else {
		log.Printf("Warning: Could not determine shares from order for signal %s", signal.UUID)
	}

	return nil
}

// processBoughtSignal handles signals that have been bought and need to be sold
func (tb *TradingBot) processBoughtSignal(ctx context.Context, signal *types.Signal, currentDate time.Time) error {
	sellDate := signal.SellDate.UTC().Truncate(24 * time.Hour)

	if currentDate.Before(sellDate) {
		log.Printf("Signal %s sell date %s is in the future, skipping", signal.UUID, sellDate.Format("2006-01-02"))
		return nil
	}

	log.Printf("Processing bought signal %s for %s", signal.UUID, signal.Ticker)

	// Execute sell order
	order, err := tb.alpacaService.SellStock(ctx, signal.Ticker, signal.NumStocks)
	if err != nil {
		return fmt.Errorf("failed to sell stock for signal %s: %w", signal.UUID, err)
	}

	// Get order details - for market orders, get the actual execution price from the order
	var executionPrice float64
	if order.FilledAvgPrice != nil {
		executionPrice, _ = order.FilledAvgPrice.Float64()
	} else {
		// Fallback to current price if filled average price is not available yet
		executionPrice, err = tb.alpacaService.GetCurrentPrice(ctx, signal.Ticker)
		if err != nil {
			log.Printf("Warning: Could not get execution price for %s: %v", signal.Ticker, err)
			executionPrice = 0
		}
	}

	// Calculate profit/loss
	var profitLoss, profitLossPct float64
	if signal.BuyPrice > 0 && executionPrice > 0 {
		profitLoss = (executionPrice - signal.BuyPrice) * signal.NumStocks
		profitLossPct = ((executionPrice - signal.BuyPrice) / signal.BuyPrice) * 100
	}

	// Calculate duration
	duration := int(currentDate.Sub(signal.BuyDate).Hours() / 24)

	// Send Discord notification
	tb.notificationService.NotifySignalSold(signal.Ticker, signal.NumStocks, executionPrice, signal.BuyPrice, profitLoss, profitLossPct, duration)

	// Log the trade result
	log.Printf("Trade completed - Signal: %s, Ticker: %s, P&L: $%.2f (%.2f%%), Duration: %d days",
		signal.UUID, signal.Ticker, profitLoss, profitLossPct, duration)

	// Mark signal for deletion (will be removed when saving)
	signal.Status = types.SignalStatusCompleted

	return nil
}

// updateAllocationWindow updates the allocation window if needed
func (tb *TradingBot) updateAllocationWindow(ctx context.Context) error {
	currentDate := time.Now().UTC().Truncate(24 * time.Hour)

	// If no window exists or current window has expired, create/update it
	if tb.allocationWindow == nil || currentDate.After(tb.allocationWindow.WindowEndDate) {
		// Get account value
		accountValue, err := tb.alpacaService.GetAccountValue(ctx)
		if err != nil {
			return fmt.Errorf("failed to get account value: %w", err)
		}

		// Calculate new window dates
		windowStartDate := currentDate
		windowEndDate := currentDate.AddDate(0, 0, tb.config.WindowDurationDays)

		// Calculate allocation per signal based on max expected signals
		allocationPerSignal := accountValue / float64(tb.config.MaxSignalsPerWindow)

		tb.allocationWindow = &types.AllocationWindow{
			WindowStartDate:      windowStartDate,
			WindowEndDate:        windowEndDate,
			AccountValue:         accountValue,
			AllocationPerSignal:  allocationPerSignal,
			TotalSignalsInWindow: tb.config.MaxSignalsPerWindow,
			UpdatedAt:            time.Now(),
		}

		log.Printf("Updated allocation window: $%.2f per signal for max %d signals",
			allocationPerSignal, tb.config.MaxSignalsPerWindow)
	}

	return nil
}

// loadData loads all data from DynamoDB into memory
func (tb *TradingBot) loadData(ctx context.Context) error {
	signals, allocationWindow, err := tb.dbService.LoadAllData(ctx)
	if err != nil {
		return fmt.Errorf("failed to load data from DynamoDB: %w", err)
	}

	// Filter to only active signals (exclude completed signals)
	var activeSignals []types.Signal
	for _, signal := range signals {
		if signal.Status == types.SignalStatusPending || signal.Status == types.SignalStatusBought {
			activeSignals = append(activeSignals, signal)
		} else {
			log.Printf("Skipping signal %s with status %s", signal.UUID, signal.Status)
		}
	}

	tb.signals = activeSignals
	tb.signalsToDelete = []types.Signal{} // Clear signals to delete at start of each run
	tb.allocationWindow = allocationWindow

	log.Printf("Loaded %d active signals and allocation window", len(activeSignals))
	return nil
}

// saveData saves all data back to DynamoDB
func (tb *TradingBot) saveData(ctx context.Context) error {
	// Filter out completed signals (they should be removed from the database)
	var activeSignals []types.Signal
	for _, signal := range tb.signals {
		if signal.Status != types.SignalStatusCompleted {
			activeSignals = append(activeSignals, signal)
		}
	}

	err := tb.dbService.SaveAllData(ctx, activeSignals, tb.signalsToDelete, tb.allocationWindow)
	if err != nil {
		return fmt.Errorf("failed to save data to DynamoDB: %w", err)
	}

	log.Printf("Saved %d active signals, deleted %d signals, and allocation window to DynamoDB", len(activeSignals), len(tb.signalsToDelete))
	return nil
}

// getAllocationPerSignal gets the current allocation per signal from memory
func (tb *TradingBot) getAllocationPerSignal() (float64, error) {
	if tb.allocationWindow == nil {
		return 0, fmt.Errorf("no allocation window found")
	}

	return tb.allocationWindow.AllocationPerSignal, nil
}
