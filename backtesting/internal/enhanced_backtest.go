package internal

import (
	"fmt"
	"log"
	"strings"
	"time"

	alpaca "github.com/alpacahq/alpaca-trade-api-go/v2/alpaca"
	"github.com/alpacahq/alpaca-trade-api-go/v2/marketdata"
)

// EnhancedSignalResult represents the result of enhanced backtesting
type EnhancedSignalResult struct {
	UUID            string
	Ticker          string
	BuyDate         time.Time
	SellDate        time.Time
	Strategy        string
	InitialBuyPrice float64
	FinalSellPrice  float64
	TotalProfitLoss float64
	ProfitLossPct   float64
	DaysHeld        int
	IsWin           bool
	EntryStrategy   string
	ExitStrategy    string
	MaxGain         float64
	MaxDrawdown     float64
}

// EnhancedBacktestSummary contains enhanced performance metrics
type EnhancedBacktestSummary struct {
	TotalSignals    int
	WinningSignals  int
	LosingSignals   int
	WinRate         float64
	AvgReturn       float64
	AvgHoldTime     float64 // Average hold time in days
	AvgReturnAnnual float64
	TotalReturn     float64
	BestSignal      EnhancedSignalResult
	WorstSignal     EnhancedSignalResult
	StrategyResults map[string]StrategyResult
}

// StrategyResult contains results for each strategy
type StrategyResult struct {
	StrategyName    string
	TotalSignals    int
	WinningSignals  int
	WinRate         float64
	AvgReturn       float64
	AvgHoldTime     float64 // Average hold time in days
	AvgReturnAnnual float64 // Annualized return
	TotalReturn     float64
}

func RunEnhancedBacktest(client alpaca.Client, marketdataClient marketdata.Client, signals []StockSignal, config EnhancedBacktestConfig) ([]EnhancedSignalResult, error) {
	var results []EnhancedSignalResult

	for _, signal := range signals {
		// Run basic backtest first
		basicResult, err := BacktestSignal(client, marketdataClient, signal)
		if err != nil {
			log.Printf("Warning: Error in basic backtest for signal %s (%s): %v", signal.UUID, signal.Ticker, err)
			continue
		}

		// Run enhanced strategies
		enhancedResult, err := runEnhancedStrategies(client, marketdataClient, signal, basicResult, config)
		if err != nil {
			log.Printf("Warning: Error in enhanced backtest for signal %s (%s): %v", signal.UUID, signal.Ticker, err)
			continue
		}

		results = append(results, enhancedResult...)
	}

	return results, nil
}

func runEnhancedStrategies(client alpaca.Client, marketdataClient marketdata.Client, signal StockSignal, basicResult SignalResult, config EnhancedBacktestConfig) ([]EnhancedSignalResult, error) {
	var results []EnhancedSignalResult

	// Strategy 1: Basic buy and hold (for comparison)
	basicEnhanced := EnhancedSignalResult{
		UUID:            signal.UUID + "_basic",
		Ticker:          signal.Ticker,
		BuyDate:         signal.BuyDate,
		SellDate:        signal.SellDate,
		Strategy:        "Basic Buy & Hold",
		InitialBuyPrice: basicResult.BuyPrice,
		FinalSellPrice:  basicResult.SellPrice,
		TotalProfitLoss: basicResult.ProfitLoss,
		ProfitLossPct:   basicResult.ProfitLossPct,
		DaysHeld:        basicResult.DaysHeld,
		IsWin:           basicResult.IsWin,
		EntryStrategy:   "100% at buy date",
		ExitStrategy:    "Sell at sell date",
		MaxGain:         basicResult.ProfitLossPct,
		MaxDrawdown:     basicResult.ProfitLossPct,
	}
	results = append(results, basicEnhanced)

	// Strategy 2: Staggered entry
	if config.StaggerEntry {
		staggeredResult, err := runStaggeredEntryStrategy(client, marketdataClient, signal, config)
		if err != nil {
			log.Printf("Warning: Error in staggered entry strategy for %s: %v", signal.Ticker, err)
		} else {
			results = append(results, staggeredResult)
		}
	}

	// Strategy 3: Take profit with trailing stop
	if config.TakeProfitPct > 0 {
		trailingResult, err := runTrailingStopStrategy(client, marketdataClient, signal, config)
		if err != nil {
			log.Printf("Warning: Error in trailing stop strategy for %s: %v", signal.Ticker, err)
		} else {
			results = append(results, trailingResult)
		}
	}

	// Strategy 4: Simple trailing stop (no take profit trigger)
	simpleTrailingResult, err := runSimpleTrailingStopStrategy(client, marketdataClient, signal, config)
	if err != nil {
		log.Printf("Warning: Error in simple trailing stop strategy for %s: %v", signal.Ticker, err)
	} else {
		results = append(results, simpleTrailingResult)
	}

	// Strategy 5: Staggered entry + trailing stop
	if config.StaggerEntry {
		staggeredTrailingResult, err := runStaggeredTrailingStopStrategy(client, marketdataClient, signal, config)
		if err != nil {
			log.Printf("Warning: Error in staggered trailing stop strategy for %s: %v", signal.Ticker, err)
		} else {
			results = append(results, staggeredTrailingResult)
		}
	}

	return results, nil
}

func runStaggeredEntryStrategy(client alpaca.Client, marketdataClient marketdata.Client, signal StockSignal, config EnhancedBacktestConfig) (EnhancedSignalResult, error) {
	// Get initial buy price (80% entry)
	initialBuyPrice, err := GetClosingPrice(marketdataClient, signal.Ticker, signal.BuyDate)
	if err != nil {
		return EnhancedSignalResult{}, err
	}

	// Get daily prices for the first week to detect dips
	dailyPrices, err := getDailyPrices(marketdataClient, signal.Ticker, signal.BuyDate, signal.BuyDate.AddDate(0, 0, 7))
	if err != nil {
		// If we can't get data, use the initial buy price
		dailyPrices = []float64{initialBuyPrice}
	}

	// Detect dip using realistic indicators
	dipBuyPrice, dipStrategy := detectDip(initialBuyPrice, dailyPrices, config.StaggerPercent)

	// Calculate weighted average buy price
	weightedBuyPrice := (initialBuyPrice * config.StaggerPercent) + (dipBuyPrice * (1 - config.StaggerPercent))

	// Get final sell price
	finalSellPrice, err := GetClosingPrice(marketdataClient, signal.Ticker, signal.SellDate)
	if err != nil {
		return EnhancedSignalResult{}, err
	}

	// Calculate profit/loss
	profitLoss := finalSellPrice - weightedBuyPrice
	profitLossPct := (profitLoss / weightedBuyPrice) * 100
	daysHeld := int(signal.SellDate.Sub(signal.BuyDate).Hours() / 24)

	return EnhancedSignalResult{
		UUID:            signal.UUID + "_staggered",
		Ticker:          signal.Ticker,
		BuyDate:         signal.BuyDate,
		SellDate:        signal.SellDate,
		Strategy:        "Staggered Entry",
		InitialBuyPrice: weightedBuyPrice,
		FinalSellPrice:  finalSellPrice,
		TotalProfitLoss: profitLoss,
		ProfitLossPct:   profitLossPct,
		DaysHeld:        daysHeld,
		IsWin:           profitLoss > 0,
		EntryStrategy:   fmt.Sprintf("%.0f%% at buy date, %.0f%% %s", config.StaggerPercent*100, (1-config.StaggerPercent)*100, dipStrategy),
		ExitStrategy:    "Sell at sell date",
		MaxGain:         profitLossPct,
		MaxDrawdown:     profitLossPct,
	}, nil
}

func runTrailingStopStrategy(client alpaca.Client, marketdataClient marketdata.Client, signal StockSignal, config EnhancedBacktestConfig) (EnhancedSignalResult, error) {
	// Get initial buy price
	initialBuyPrice, err := GetClosingPrice(marketdataClient, signal.Ticker, signal.BuyDate)
	if err != nil {
		return EnhancedSignalResult{}, err
	}

	// Get daily prices for the entire period
	dailyPrices, err := getDailyPrices(marketdataClient, signal.Ticker, signal.BuyDate, signal.SellDate)
	if err != nil {
		return EnhancedSignalResult{}, err
	}

	// Simulate the strategy
	finalSellPrice, exitStrategy, maxGain, maxDrawdown := simulateTrailingStop(
		initialBuyPrice, dailyPrices, config.TakeProfitPct, config.TrailingStopPct)

	// Calculate profit/loss
	profitLoss := finalSellPrice - initialBuyPrice
	profitLossPct := (profitLoss / initialBuyPrice) * 100
	daysHeld := int(signal.SellDate.Sub(signal.BuyDate).Hours() / 24)

	return EnhancedSignalResult{
		UUID:            signal.UUID + "_trailing",
		Ticker:          signal.Ticker,
		BuyDate:         signal.BuyDate,
		SellDate:        signal.SellDate,
		Strategy:        "Take Profit + Trailing Stop",
		InitialBuyPrice: initialBuyPrice,
		FinalSellPrice:  finalSellPrice,
		TotalProfitLoss: profitLoss,
		ProfitLossPct:   profitLossPct,
		DaysHeld:        daysHeld,
		IsWin:           profitLoss > 0,
		EntryStrategy:   "100% at buy date",
		ExitStrategy:    exitStrategy,
		MaxGain:         maxGain,
		MaxDrawdown:     maxDrawdown,
	}, nil
}

func runSimpleTrailingStopStrategy(client alpaca.Client, marketdataClient marketdata.Client, signal StockSignal, config EnhancedBacktestConfig) (EnhancedSignalResult, error) {
	// Get initial buy price
	initialBuyPrice, err := GetClosingPrice(marketdataClient, signal.Ticker, signal.BuyDate)
	if err != nil {
		return EnhancedSignalResult{}, err
	}

	// Get daily prices for the entire period
	dailyPrices, err := getDailyPrices(marketdataClient, signal.Ticker, signal.BuyDate, signal.SellDate)
	if err != nil {
		return EnhancedSignalResult{}, err
	}

	// Simulate simple trailing stop (no take profit trigger)
	finalSellPrice, exitStrategy, maxGain, maxDrawdown := simulateSimpleTrailingStop(
		initialBuyPrice, dailyPrices, config.TrailingStopPct)

	// Calculate profit/loss
	profitLoss := finalSellPrice - initialBuyPrice
	profitLossPct := (profitLoss / initialBuyPrice) * 100
	daysHeld := int(signal.SellDate.Sub(signal.BuyDate).Hours() / 24)

	return EnhancedSignalResult{
		UUID:            signal.UUID + "_simple_trailing",
		Ticker:          signal.Ticker,
		BuyDate:         signal.BuyDate,
		SellDate:        signal.SellDate,
		Strategy:        "Simple Trailing Stop",
		InitialBuyPrice: initialBuyPrice,
		FinalSellPrice:  finalSellPrice,
		TotalProfitLoss: profitLoss,
		ProfitLossPct:   profitLossPct,
		DaysHeld:        daysHeld,
		IsWin:           profitLoss > 0,
		EntryStrategy:   "100% at buy date",
		ExitStrategy:    exitStrategy,
		MaxGain:         maxGain,
		MaxDrawdown:     maxDrawdown,
	}, nil
}

func runStaggeredTrailingStopStrategy(client alpaca.Client, marketdataClient marketdata.Client, signal StockSignal, config EnhancedBacktestConfig) (EnhancedSignalResult, error) {
	// Get initial buy price (80% entry)
	initialBuyPrice, err := GetClosingPrice(marketdataClient, signal.Ticker, signal.BuyDate)
	if err != nil {
		return EnhancedSignalResult{}, err
	}

	// Find the lowest price in the first week for the 20% dip entry
	dipBuyPrice, err := findLowestPriceInPeriod(marketdataClient, signal.Ticker, signal.BuyDate, signal.BuyDate.AddDate(0, 0, 7))
	if err != nil {
		// If we can't find a dip, use the initial buy price
		dipBuyPrice = initialBuyPrice
	}

	// Calculate weighted average buy price
	weightedBuyPrice := (initialBuyPrice * config.StaggerPercent) + (dipBuyPrice * (1 - config.StaggerPercent))

	// Get daily prices for the entire period
	dailyPrices, err := getDailyPrices(marketdataClient, signal.Ticker, signal.BuyDate, signal.SellDate)
	if err != nil {
		return EnhancedSignalResult{}, err
	}

	// Simulate simple trailing stop with weighted buy price
	finalSellPrice, exitStrategy, maxGain, maxDrawdown := simulateSimpleTrailingStop(
		weightedBuyPrice, dailyPrices, config.TrailingStopPct)

	// Calculate profit/loss
	profitLoss := finalSellPrice - weightedBuyPrice
	profitLossPct := (profitLoss / weightedBuyPrice) * 100
	daysHeld := int(signal.SellDate.Sub(signal.BuyDate).Hours() / 24)

	return EnhancedSignalResult{
		UUID:            signal.UUID + "_staggered_trailing",
		Ticker:          signal.Ticker,
		BuyDate:         signal.BuyDate,
		SellDate:        signal.SellDate,
		Strategy:        "Staggered Entry + Trailing Stop",
		InitialBuyPrice: weightedBuyPrice,
		FinalSellPrice:  finalSellPrice,
		TotalProfitLoss: profitLoss,
		ProfitLossPct:   profitLossPct,
		DaysHeld:        daysHeld,
		IsWin:           profitLoss > 0,
		EntryStrategy:   fmt.Sprintf("%.0f%% at buy date, %.0f%% on dips", config.StaggerPercent*100, (1-config.StaggerPercent)*100),
		ExitStrategy:    exitStrategy,
		MaxGain:         maxGain,
		MaxDrawdown:     maxDrawdown,
	}, nil
}

func findLowestPriceInPeriod(marketdataClient marketdata.Client, ticker string, startDate, endDate time.Time) (float64, error) {
	// Try to find data within the period, expanding the search if needed
	maxAttempts := 10
	currentStartDate := startDate

	for attempt := 0; attempt < maxAttempts; attempt++ {
		params := marketdata.GetBarsParams{
			Start:     currentStartDate,
			End:       endDate,
			TimeFrame: marketdata.OneDay,
		}

		bars, err := marketdataClient.GetBars(ticker, params)
		if err != nil {
			// If there's an API error, try starting from the next day
			currentStartDate = currentStartDate.AddDate(0, 0, 1)
			continue
		}

		if len(bars) > 0 {
			// Found data, find the lowest price
			lowestPrice := bars[0].Low
			for _, bar := range bars {
				if bar.Low < lowestPrice {
					lowestPrice = bar.Low
				}
			}

			if attempt > 0 {
				log.Printf("Found data for %s in period starting from %s (original start: %s)", ticker, currentStartDate.Format("2006-01-02"), startDate.Format("2006-01-02"))
			}
			return lowestPrice, nil
		}

		// No data found, try starting from the next day
		currentStartDate = currentStartDate.AddDate(0, 0, 1)
	}

	return 0, fmt.Errorf("no data found for %s in period after trying %d days starting from %s", ticker, maxAttempts, startDate.Format("2006-01-02"))
}

func getDailyPrices(marketdataClient marketdata.Client, ticker string, startDate, endDate time.Time) ([]float64, error) {
	// Try to find data within the period, expanding the search if needed
	maxAttempts := 10
	currentStartDate := startDate

	for attempt := 0; attempt < maxAttempts; attempt++ {
		params := marketdata.GetBarsParams{
			Start:     currentStartDate,
			End:       endDate,
			TimeFrame: marketdata.OneDay,
		}

		bars, err := marketdataClient.GetBars(ticker, params)
		if err != nil {
			// If there's an API error, try starting from the next day
			currentStartDate = currentStartDate.AddDate(0, 0, 1)
			continue
		}

		if len(bars) > 0 {
			// Found data, extract closing prices
			var prices []float64
			for _, bar := range bars {
				prices = append(prices, bar.Close)
			}

			if attempt > 0 {
				log.Printf("Found data for %s in period starting from %s (original start: %s)", ticker, currentStartDate.Format("2006-01-02"), startDate.Format("2006-01-02"))
			}
			return prices, nil
		}

		// No data found, try starting from the next day
		currentStartDate = currentStartDate.AddDate(0, 0, 1)
	}

	return nil, fmt.Errorf("no data found for %s in period after trying %d days starting from %s", ticker, maxAttempts, startDate.Format("2006-01-02"))
}

func simulateTrailingStop(initialPrice float64, dailyPrices []float64, takeProfitPct, trailingStopPct float64) (float64, string, float64, float64) {
	if len(dailyPrices) == 0 {
		return initialPrice, "No data", 0, 0
	}

	takeProfitTarget := initialPrice * (1 + takeProfitPct)
	trailingStopLevel := initialPrice * (1 - trailingStopPct)
	highestPrice := initialPrice
	takeProfitHit := false

	var maxGain, maxDrawdown float64
	var exitStrategy string
	finalPrice := initialPrice

	for i, price := range dailyPrices {
		// Calculate current gain/loss
		currentGain := (price - initialPrice) / initialPrice * 100

		// Track max gain and drawdown
		if currentGain > maxGain {
			maxGain = currentGain
		}
		if currentGain < maxDrawdown {
			maxDrawdown = currentGain
		}

		// Check if we hit take profit target
		if price >= takeProfitTarget {
			takeProfitHit = true
			exitStrategy = "Take profit triggered, trailing stop active"
		}

		// Update trailing stop if price makes a new high
		if price > highestPrice {
			highestPrice = price
			trailingStopLevel = price * (1 - trailingStopPct)
		}

		// Check if we hit trailing stop
		if price <= trailingStopLevel {
			finalPrice = price
			if takeProfitHit {
				exitStrategy = "Trailing stop hit (after take profit)"
			} else {
				exitStrategy = "Trailing stop hit (before take profit)"
			}
			break
		}

		// If we reach the end, sell at the last price
		if i == len(dailyPrices)-1 {
			finalPrice = price
			if exitStrategy == "" {
				exitStrategy = "Held until sell date"
			}
		}
	}

	return finalPrice, exitStrategy, maxGain, maxDrawdown
}

func simulateSimpleTrailingStop(initialPrice float64, dailyPrices []float64, trailingStopPct float64) (float64, string, float64, float64) {
	if len(dailyPrices) == 0 {
		return initialPrice, "No data", 0, 0
	}

	// Start trailing stop at initial price (15% below initial price)
	trailingStopLevel := initialPrice * (1 - trailingStopPct)
	highestPrice := initialPrice

	var maxGain, maxDrawdown float64
	var exitStrategy string
	finalPrice := initialPrice

	for i, price := range dailyPrices {
		// Calculate current gain/loss
		currentGain := (price - initialPrice) / initialPrice * 100

		// Track max gain and drawdown
		if currentGain > maxGain {
			maxGain = currentGain
		}
		if currentGain < maxDrawdown {
			maxDrawdown = currentGain
		}

		// Check if we hit trailing stop
		if price <= trailingStopLevel {
			finalPrice = price
			exitStrategy = "Trailing stop hit"
			break
		}

		// Update trailing stop if price makes a new high
		if price > highestPrice {
			highestPrice = price
			trailingStopLevel = price * (1 - trailingStopPct)
		}

		// If we reach the end, sell at the last price
		if i == len(dailyPrices)-1 {
			finalPrice = price
			if exitStrategy == "" {
				exitStrategy = "Held until sell date"
			}
		}
	}

	return finalPrice, exitStrategy, maxGain, maxDrawdown
}

// detectDip implements realistic dip detection strategies
func detectDip(initialPrice float64, dailyPrices []float64, staggerPercent float64) (float64, string) {
	if len(dailyPrices) == 0 {
		return initialPrice, "no dip detected (no data)"
	}

	// Strategy 1: RSI-like oversold detection (simplified)
	// Look for price drops of 5%+ from initial price
	oversoldThreshold := initialPrice * 0.95 // 5% drop
	for _, price := range dailyPrices {
		if price <= oversoldThreshold {
			return price, "RSI oversold (5% drop)"
		}
	}

	// Strategy 2: Moving average crossover
	// If we have at least 3 days of data, check for MA crossover
	if len(dailyPrices) >= 3 {
		// Simple 2-day moving average
		ma2 := (dailyPrices[0] + dailyPrices[1]) / 2
		if dailyPrices[2] < ma2 && dailyPrices[2] < initialPrice {
			return dailyPrices[2], "MA crossover"
		}
	}

	// Strategy 3: Volume spike simulation (price volatility)
	// Look for significant price swings (3%+ daily moves)
	for i := 1; i < len(dailyPrices); i++ {
		dailyChange := (dailyPrices[i] - dailyPrices[i-1]) / dailyPrices[i-1]
		if dailyChange < -0.03 { // 3% daily drop
			return dailyPrices[i], "volatility spike (3% drop)"
		}
	}

	// Strategy 4: Support level break
	// Look for price breaking below 2% of initial price
	supportLevel := initialPrice * 0.98
	for _, price := range dailyPrices {
		if price <= supportLevel {
			return price, "support break (2% drop)"
		}
	}

	// Strategy 5: Momentum reversal
	// Look for consecutive down days
	if len(dailyPrices) >= 2 {
		if dailyPrices[1] < dailyPrices[0] && dailyPrices[1] < initialPrice {
			return dailyPrices[1], "momentum reversal"
		}
	}

	// If no dip detected, use the lowest price in the period
	lowestPrice := dailyPrices[0]
	for _, price := range dailyPrices {
		if price < lowestPrice {
			lowestPrice = price
		}
	}

	// Only use the dip if it's at least 1% lower than initial price
	if lowestPrice < initialPrice*0.99 {
		return lowestPrice, "lowest price (1%+ drop)"
	}

	// No significant dip found
	return initialPrice, "no dip detected"
}

func CalculateEnhancedSummary(results []EnhancedSignalResult) EnhancedBacktestSummary {
	if len(results) == 0 {
		return EnhancedBacktestSummary{}
	}

	var totalReturn, totalDays float64
	var winningSignals, losingSignals int
	var bestSignal, worstSignal EnhancedSignalResult
	strategyResults := make(map[string]StrategyResult)

	for i, result := range results {
		totalReturn += result.ProfitLossPct
		totalDays += float64(result.DaysHeld)

		if result.IsWin {
			winningSignals++
		} else {
			losingSignals++
		}

		// Track best and worst signals
		if i == 0 {
			bestSignal = result
			worstSignal = result
		} else {
			if result.ProfitLossPct > bestSignal.ProfitLossPct {
				bestSignal = result
			}
			if result.ProfitLossPct < worstSignal.ProfitLossPct {
				worstSignal = result
			}
		}

		// Track strategy results
		if sr, exists := strategyResults[result.Strategy]; exists {
			sr.TotalSignals++
			if result.IsWin {
				sr.WinningSignals++
			}
			sr.TotalReturn += result.ProfitLossPct
			sr.AvgReturn = sr.TotalReturn / float64(sr.TotalSignals)
			sr.WinRate = float64(sr.WinningSignals) / float64(sr.TotalSignals) * 100

			// Track hold time for annualized calculation
			sr.AvgHoldTime = (sr.AvgHoldTime*float64(sr.TotalSignals-1) + float64(result.DaysHeld)) / float64(sr.TotalSignals)

			// Calculate annualized return
			if sr.AvgHoldTime > 0 {
				sr.AvgReturnAnnual = sr.AvgReturn * (365 / sr.AvgHoldTime)
			}

			strategyResults[result.Strategy] = sr
		} else {
			sr := StrategyResult{
				StrategyName: result.Strategy,
				TotalSignals: 1,
				TotalReturn:  result.ProfitLossPct,
				AvgReturn:    result.ProfitLossPct,
				AvgHoldTime:  float64(result.DaysHeld),
			}
			if result.IsWin {
				sr.WinningSignals = 1
				sr.WinRate = 100
			}

			// Calculate annualized return for first signal
			if sr.AvgHoldTime > 0 {
				sr.AvgReturnAnnual = sr.AvgReturn * (365 / sr.AvgHoldTime)
			}

			strategyResults[result.Strategy] = sr
		}
	}

	totalSignals := len(results)
	winRate := float64(winningSignals) / float64(totalSignals) * 100
	avgReturn := totalReturn / float64(totalSignals)
	avgHoldTime := totalDays / float64(totalSignals)

	// Calculate annualized return using average hold time
	var avgReturnAnnual float64
	if avgHoldTime > 0 {
		avgReturnAnnual = avgReturn * (365 / avgHoldTime)
	}

	return EnhancedBacktestSummary{
		TotalSignals:    totalSignals,
		WinningSignals:  winningSignals,
		LosingSignals:   losingSignals,
		WinRate:         winRate,
		AvgReturn:       avgReturn,
		AvgHoldTime:     avgHoldTime,
		AvgReturnAnnual: avgReturnAnnual,
		TotalReturn:     totalReturn,
		BestSignal:      bestSignal,
		WorstSignal:     worstSignal,
		StrategyResults: strategyResults,
	}
}

func PrintEnhancedResults(results []EnhancedSignalResult, summary EnhancedBacktestSummary) {
	fmt.Println("\n=== ENHANCED BACKTEST RESULTS ===")
	fmt.Printf("Total Signals: %d\n", summary.TotalSignals)
	fmt.Printf("Winning Signals: %d\n", summary.WinningSignals)
	fmt.Printf("Losing Signals: %d\n", summary.LosingSignals)
	fmt.Printf("Win Rate: %.2f%%\n", summary.WinRate)
	fmt.Printf("Average Return: %.2f%%\n", summary.AvgReturn)
	fmt.Printf("Average Hold Time: %.1f days\n", summary.AvgHoldTime)
	fmt.Printf("Average Return (Annualized): %.2f%%\n", summary.AvgReturnAnnual)
	fmt.Printf("Total Return: %.2f%%\n", summary.TotalReturn)

	fmt.Println("\n=== STRATEGY COMPARISON ===")
	for strategyName, result := range summary.StrategyResults {
		fmt.Printf("\nStrategy: %s\n", strategyName)
		fmt.Printf("  Total Signals: %d\n", result.TotalSignals)
		fmt.Printf("  Win Rate: %.2f%%\n", result.WinRate)
		fmt.Printf("  Average Return: %.2f%%\n", result.AvgReturn)
		fmt.Printf("  Average Hold Time: %.1f days\n", result.AvgHoldTime)
		fmt.Printf("  Average Return (Annualized): %.2f%%\n", result.AvgReturnAnnual)
		fmt.Printf("  Total Return: %.2f%%\n", result.TotalReturn)
	}

	fmt.Println("\n=== BEST SIGNAL ===")
	printEnhancedSignalResult(summary.BestSignal)

	fmt.Println("\n=== WORST SIGNAL ===")
	printEnhancedSignalResult(summary.WorstSignal)

	fmt.Println("\n=== ALL SIGNALS (Sorted by P/L %) ===")
	printEnhancedSignalsTable(results)
}

func printEnhancedSignalResult(result EnhancedSignalResult) {
	status := "LOSS"
	if result.IsWin {
		status = "WIN"
	}

	fmt.Printf("%s | %s | %s | Buy: %s (%.2f) | Sell: %s (%.2f) | P/L: %.2f%% | Days: %d | Max Gain: %.2f%% | Max DD: %.2f%%\n",
		status,
		result.Ticker,
		result.Strategy,
		result.BuyDate.Format("2006-01-02"),
		result.InitialBuyPrice,
		result.SellDate.Format("2006-01-02"),
		result.FinalSellPrice,
		result.ProfitLossPct,
		result.DaysHeld,
		result.MaxGain,
		result.MaxDrawdown)
}

func printEnhancedSignalsTable(results []EnhancedSignalResult) {
	// Sort results by profit/loss percentage (highest first)
	sortedResults := make([]EnhancedSignalResult, len(results))
	copy(sortedResults, results)

	// Simple bubble sort for sorting by P/L percentage (highest first)
	for i := 0; i < len(sortedResults)-1; i++ {
		for j := 0; j < len(sortedResults)-i-1; j++ {
			if sortedResults[j].ProfitLossPct < sortedResults[j+1].ProfitLossPct {
				sortedResults[j], sortedResults[j+1] = sortedResults[j+1], sortedResults[j]
			}
		}
	}

	// Print table header
	fmt.Printf("%-6s | %-8s | %-25s | %-12s | %-12s | %-8s | %-4s | %-8s | %-8s | %-15s\n",
		"Status", "Ticker", "Strategy", "Buy Date", "Sell Date", "P/L %", "Days", "Max Gain", "Max DD", "Exit Strategy")
	fmt.Println(strings.Repeat("-", 120))

	// Print table rows
	for _, result := range sortedResults {
		status := "LOSS"
		if result.IsWin {
			status = "WIN"
		}

		// Truncate strategy name if too long
		strategyName := result.Strategy
		if len(strategyName) > 24 {
			strategyName = strategyName[:21] + "..."
		}

		// Truncate exit strategy if too long
		exitStrategy := result.ExitStrategy
		if len(exitStrategy) > 14 {
			exitStrategy = exitStrategy[:11] + "..."
		}

		fmt.Printf("%-6s | %-8s | %-25s | %-12s | %-12s | %-8.2f | %-4d | %-8.2f | %-8.2f | %-15s\n",
			status,
			result.Ticker,
			strategyName,
			result.BuyDate.Format("2006-01-02"),
			result.SellDate.Format("2006-01-02"),
			result.ProfitLossPct,
			result.DaysHeld,
			result.MaxGain,
			result.MaxDrawdown,
			exitStrategy)
	}

	// Print summary statistics
	fmt.Println(strings.Repeat("-", 120))

	// Count wins and losses
	wins, losses := 0, 0
	for _, result := range sortedResults {
		if result.IsWin {
			wins++
		} else {
			losses++
		}
	}

	fmt.Printf("Summary: %d total signals | %d wins (%.1f%%) | %d losses (%.1f%%)\n",
		len(sortedResults), wins, float64(wins)/float64(len(sortedResults))*100,
		losses, float64(losses)/float64(len(sortedResults))*100)
}
