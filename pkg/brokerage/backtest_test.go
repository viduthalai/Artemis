package brokerage

import (
	"sync"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBacktestBrokerage(t *testing.T) {
	tests := []struct {
		name            string
		initialBalance  decimal.Decimal
		expectedBalance decimal.Decimal
	}{
		{
			name:            "zero initial balance",
			initialBalance:  decimal.Zero,
			expectedBalance: decimal.Zero,
		},
		{
			name:            "positive initial balance",
			initialBalance:  decimal.NewFromFloat(10000.50),
			expectedBalance: decimal.NewFromFloat(10000.50),
		},
		{
			name:            "large initial balance",
			initialBalance:  decimal.NewFromFloat(1000000.00),
			expectedBalance: decimal.NewFromFloat(1000000.00),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brokerage := NewBacktestBrokerage(tt.initialBalance)

			assert.NotNil(t, brokerage)
			assert.Equal(t, tt.expectedBalance, brokerage.balance)
			assert.NotNil(t, brokerage.positions)
			assert.Empty(t, brokerage.positions)
		})
	}
}

func TestGetCashBalance(t *testing.T) {
	tests := []struct {
		name            string
		initialBalance  decimal.Decimal
		expectedBalance decimal.Decimal
	}{
		{
			name:            "zero balance",
			initialBalance:  decimal.Zero,
			expectedBalance: decimal.Zero,
		},
		{
			name:            "positive balance",
			initialBalance:  decimal.NewFromFloat(5000.25),
			expectedBalance: decimal.NewFromFloat(5000.25),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brokerage := NewBacktestBrokerage(tt.initialBalance)

			balance, err := brokerage.GetCashBalance()

			require.NoError(t, err)
			assert.Equal(t, tt.expectedBalance, balance)
		})
	}
}

func TestGetAccountValue(t *testing.T) {
	tests := []struct {
		name           string
		initialBalance decimal.Decimal
		positions      map[string]*Position
		expectedValue  decimal.Decimal
	}{
		{
			name:           "no positions",
			initialBalance: decimal.NewFromFloat(10000.00),
			positions:      map[string]*Position{},
			expectedValue:  decimal.NewFromFloat(10000.00),
		},
		{
			name:           "single position",
			initialBalance: decimal.NewFromFloat(5000.00),
			positions: map[string]*Position{
				"AAPL": {
					Shares:      decimal.NewFromFloat(10.0),
					AvgBuyPrice: decimal.NewFromFloat(150.00),
				},
			},
			expectedValue: decimal.NewFromFloat(6500.00), // 5000 + (10 * 150)
		},
		{
			name:           "multiple positions",
			initialBalance: decimal.NewFromFloat(1000.00),
			positions: map[string]*Position{
				"AAPL": {
					Shares:      decimal.NewFromFloat(5.0),
					AvgBuyPrice: decimal.NewFromFloat(150.00),
				},
				"GOOGL": {
					Shares:      decimal.NewFromFloat(2.0),
					AvgBuyPrice: decimal.NewFromFloat(2500.00),
				},
			},
			expectedValue: decimal.NewFromFloat(6750.00), // 1000 + (5 * 150) + (2 * 2500)
		},
		{
			name:           "zero balance with positions",
			initialBalance: decimal.Zero,
			positions: map[string]*Position{
				"TSLA": {
					Shares:      decimal.NewFromFloat(1.0),
					AvgBuyPrice: decimal.NewFromFloat(800.00),
				},
			},
			expectedValue: decimal.NewFromFloat(800.00),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brokerage := NewBacktestBrokerage(tt.initialBalance)
			brokerage.positions = tt.positions

			value, err := brokerage.GetAccountValue()

			require.NoError(t, err)
			// Use a tolerance for floating point comparison
			diff := value.Sub(tt.expectedValue).Abs()
			assert.True(t, diff.LessThanOrEqual(decimal.NewFromFloat(0.01)),
				"Expected value around %v, got %v", tt.expectedValue, value)
		})
	}
}

func TestAddDeposit(t *testing.T) {
	tests := []struct {
		name            string
		initialBalance  decimal.Decimal
		depositAmount   decimal.Decimal
		expectedBalance decimal.Decimal
	}{
		{
			name:            "add to zero balance",
			initialBalance:  decimal.Zero,
			depositAmount:   decimal.NewFromFloat(1000.00),
			expectedBalance: decimal.NewFromFloat(1000.00),
		},
		{
			name:            "add to existing balance",
			initialBalance:  decimal.NewFromFloat(5000.00),
			depositAmount:   decimal.NewFromFloat(2500.50),
			expectedBalance: decimal.NewFromFloat(7500.50),
		},
		{
			name:            "add zero amount",
			initialBalance:  decimal.NewFromFloat(1000.00),
			depositAmount:   decimal.Zero,
			expectedBalance: decimal.NewFromFloat(1000.00),
		},
		{
			name:            "add large amount",
			initialBalance:  decimal.NewFromFloat(100.00),
			depositAmount:   decimal.NewFromFloat(999999.99),
			expectedBalance: decimal.NewFromFloat(1000099.99),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brokerage := NewBacktestBrokerage(tt.initialBalance)

			err := brokerage.AddDeposit(tt.depositAmount)

			require.NoError(t, err)
			balance, err := brokerage.GetCashBalance()
			require.NoError(t, err)
			// Use a tolerance for floating point comparison
			diff := balance.Sub(tt.expectedBalance).Abs()
			assert.True(t, diff.LessThanOrEqual(decimal.NewFromFloat(0.01)),
				"Expected balance around %v, got %v", tt.expectedBalance, balance)
		})
	}
}

func TestExecuteBuyOrder(t *testing.T) {
	tests := []struct {
		name             string
		initialBalance   decimal.Decimal
		symbol           string
		quantity         decimal.Decimal
		price            decimal.Decimal
		expectError      bool
		errorContains    string
		expectedBalance  decimal.Decimal
		expectedShares   decimal.Decimal
		expectedAvgPrice decimal.Decimal
	}{
		{
			name:             "successful first buy",
			initialBalance:   decimal.NewFromFloat(10000.00),
			symbol:           "AAPL",
			quantity:         decimal.NewFromFloat(10.0),
			price:            decimal.NewFromFloat(150.00),
			expectError:      false,
			expectedBalance:  decimal.NewFromFloat(8500.00), // 10000 - (10 * 150)
			expectedShares:   decimal.NewFromFloat(10.0),
			expectedAvgPrice: decimal.NewFromFloat(150.00),
		},
		{
			name:           "insufficient funds",
			initialBalance: decimal.NewFromFloat(1000.00),
			symbol:         "GOOGL",
			quantity:       decimal.NewFromFloat(1.0),
			price:          decimal.NewFromFloat(2500.00),
			expectError:    true,
			errorContains:  "insufficient funds",
		},
		{
			name:             "add to existing position",
			initialBalance:   decimal.NewFromFloat(10000.00),
			symbol:           "AAPL",
			quantity:         decimal.NewFromFloat(5.0),
			price:            decimal.NewFromFloat(160.00),
			expectError:      false,
			expectedBalance:  decimal.NewFromFloat(7700.00), // 10000 - (10 * 150) - (5 * 160)
			expectedShares:   decimal.NewFromFloat(15.0),    // 10 + 5
			expectedAvgPrice: decimal.NewFromFloat(153.33),  // (10*150 + 5*160) / 15
		},
		{
			name:             "buy with exact balance",
			initialBalance:   decimal.NewFromFloat(1500.00),
			symbol:           "TSLA",
			quantity:         decimal.NewFromFloat(2.0),
			price:            decimal.NewFromFloat(750.00),
			expectError:      false,
			expectedBalance:  decimal.Zero,
			expectedShares:   decimal.NewFromFloat(2.0),
			expectedAvgPrice: decimal.NewFromFloat(750.00),
		},
		{
			name:             "buy zero shares",
			initialBalance:   decimal.NewFromFloat(1000.00),
			symbol:           "AAPL",
			quantity:         decimal.Zero,
			price:            decimal.NewFromFloat(150.00),
			expectError:      false,
			expectedBalance:  decimal.NewFromFloat(1000.00),
			expectedShares:   decimal.Zero,
			expectedAvgPrice: decimal.NewFromFloat(150.00),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brokerage := NewBacktestBrokerage(tt.initialBalance)

			// If this is an "add to existing position" test, create the initial position
			if tt.name == "add to existing position" {
				err := brokerage.ExecuteBuyOrder("AAPL", decimal.NewFromFloat(10.0), decimal.NewFromFloat(150.00))
				require.NoError(t, err)
			}

			err := brokerage.ExecuteBuyOrder(tt.symbol, tt.quantity, tt.price)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)

				// Check balance
				balance, err := brokerage.GetCashBalance()
				require.NoError(t, err)
				diff := balance.Sub(tt.expectedBalance).Abs()
				assert.True(t, diff.LessThanOrEqual(decimal.NewFromFloat(0.01)),
					"Expected balance around %v, got %v", tt.expectedBalance, balance)

				// Check position
				shares, err := brokerage.GetPosition(tt.symbol)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedShares, shares)

				// Check average price
				positions := brokerage.GetPositions()
				if pos, exists := positions[tt.symbol]; exists {
					position := pos.(*Position)
					// Use a tolerance for floating point comparison
					diff := position.AvgBuyPrice.Sub(tt.expectedAvgPrice).Abs()
					assert.True(t, diff.LessThanOrEqual(decimal.NewFromFloat(0.01)),
						"Expected average price around %v, got %v", tt.expectedAvgPrice, position.AvgBuyPrice)
				}
			}
		})
	}
}

func TestExecuteSellOrder(t *testing.T) {
	tests := []struct {
		name                string
		initialBalance      decimal.Decimal
		initialPosition     *Position
		symbol              string
		quantity            decimal.Decimal
		price               decimal.Decimal
		expectError         bool
		errorContains       string
		expectedBalance     decimal.Decimal
		expectedShares      decimal.Decimal
		positionShouldExist bool
	}{
		{
			name:           "successful partial sell",
			initialBalance: decimal.NewFromFloat(1000.00),
			initialPosition: &Position{
				Shares:      decimal.NewFromFloat(10.0),
				AvgBuyPrice: decimal.NewFromFloat(150.00),
			},
			symbol:              "AAPL",
			quantity:            decimal.NewFromFloat(5.0),
			price:               decimal.NewFromFloat(160.00),
			expectError:         false,
			expectedBalance:     decimal.NewFromFloat(1800.00), // 1000 + (5 * 160)
			expectedShares:      decimal.NewFromFloat(5.0),
			positionShouldExist: true,
		},
		{
			name:           "successful full sell",
			initialBalance: decimal.NewFromFloat(1000.00),
			initialPosition: &Position{
				Shares:      decimal.NewFromFloat(10.0),
				AvgBuyPrice: decimal.NewFromFloat(150.00),
			},
			symbol:              "AAPL",
			quantity:            decimal.NewFromFloat(10.0),
			price:               decimal.NewFromFloat(160.00),
			expectError:         false,
			expectedBalance:     decimal.NewFromFloat(2600.00), // 1000 + (10 * 160)
			expectedShares:      decimal.Zero,
			positionShouldExist: false,
		},
		{
			name:            "no position exists",
			initialBalance:  decimal.NewFromFloat(1000.00),
			initialPosition: nil,
			symbol:          "AAPL",
			quantity:        decimal.NewFromFloat(5.0),
			price:           decimal.NewFromFloat(160.00),
			expectError:     true,
			errorContains:   "no position found",
		},
		{
			name:           "insufficient shares",
			initialBalance: decimal.NewFromFloat(1000.00),
			initialPosition: &Position{
				Shares:      decimal.NewFromFloat(5.0),
				AvgBuyPrice: decimal.NewFromFloat(150.00),
			},
			symbol:        "AAPL",
			quantity:      decimal.NewFromFloat(10.0),
			price:         decimal.NewFromFloat(160.00),
			expectError:   true,
			errorContains: "insufficient shares",
		},
		{
			name:           "sell at loss",
			initialBalance: decimal.NewFromFloat(1000.00),
			initialPosition: &Position{
				Shares:      decimal.NewFromFloat(10.0),
				AvgBuyPrice: decimal.NewFromFloat(150.00),
			},
			symbol:              "AAPL",
			quantity:            decimal.NewFromFloat(5.0),
			price:               decimal.NewFromFloat(140.00),
			expectError:         false,
			expectedBalance:     decimal.NewFromFloat(1700.00), // 1000 + (5 * 140)
			expectedShares:      decimal.NewFromFloat(5.0),
			positionShouldExist: true,
		},
		{
			name:           "sell zero shares",
			initialBalance: decimal.NewFromFloat(1000.00),
			initialPosition: &Position{
				Shares:      decimal.NewFromFloat(10.0),
				AvgBuyPrice: decimal.NewFromFloat(150.00),
			},
			symbol:              "AAPL",
			quantity:            decimal.NewFromFloat(0.0),
			price:               decimal.NewFromFloat(160.00),
			expectError:         false,
			expectedBalance:     decimal.NewFromFloat(1000.00),
			expectedShares:      decimal.NewFromFloat(10.0),
			positionShouldExist: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brokerage := NewBacktestBrokerage(tt.initialBalance)

			// Set up initial position if provided
			if tt.initialPosition != nil {
				brokerage.positions[tt.symbol] = tt.initialPosition
			}

			err := brokerage.ExecuteSellOrder(tt.symbol, tt.quantity, tt.price)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)

				// Check balance
				balance, err := brokerage.GetCashBalance()
				require.NoError(t, err)
				diff := balance.Sub(tt.expectedBalance).Abs()
				assert.True(t, diff.LessThanOrEqual(decimal.NewFromFloat(0.01)),
					"Expected balance around %v, got %v", tt.expectedBalance, balance)

				// Check position
				shares, err := brokerage.GetPosition(tt.symbol)
				if tt.positionShouldExist {
					require.NoError(t, err)
					// Use tolerance for zero shares comparison
					if tt.expectedShares.Equal(decimal.Zero) {
						diff := shares.Sub(tt.expectedShares).Abs()
						assert.True(t, diff.LessThanOrEqual(decimal.NewFromFloat(0.01)),
							"Expected shares around %v, got %v", tt.expectedShares, shares)
					} else {
						assert.Equal(t, tt.expectedShares, shares)
					}
				} else {
					require.Error(t, err)
					assert.Contains(t, err.Error(), "no position found")
				}
			}
		})
	}
}

func TestGetPosition(t *testing.T) {
	tests := []struct {
		name           string
		positions      map[string]*Position
		symbol         string
		expectError    bool
		expectedShares decimal.Decimal
	}{
		{
			name: "existing position",
			positions: map[string]*Position{
				"AAPL": {
					Shares:      decimal.NewFromFloat(10.0),
					AvgBuyPrice: decimal.NewFromFloat(150.00),
				},
			},
			symbol:         "AAPL",
			expectError:    false,
			expectedShares: decimal.NewFromFloat(10.0),
		},
		{
			name:           "non-existent position",
			positions:      map[string]*Position{},
			symbol:         "GOOGL",
			expectError:    true,
			expectedShares: decimal.Zero,
		},
		{
			name: "zero shares position",
			positions: map[string]*Position{
				"TSLA": {
					Shares:      decimal.Zero,
					AvgBuyPrice: decimal.NewFromFloat(800.00),
				},
			},
			symbol:         "TSLA",
			expectError:    false,
			expectedShares: decimal.Zero,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brokerage := NewBacktestBrokerage(decimal.NewFromFloat(10000.00))
			brokerage.positions = tt.positions

			shares, err := brokerage.GetPosition(tt.symbol)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedShares, shares)
			}
		})
	}
}

func TestGetPositions(t *testing.T) {
	tests := []struct {
		name          string
		positions     map[string]*Position
		expectedCount int
	}{
		{
			name:          "no positions",
			positions:     map[string]*Position{},
			expectedCount: 0,
		},
		{
			name: "single position",
			positions: map[string]*Position{
				"AAPL": {
					Shares:      decimal.NewFromFloat(10.0),
					AvgBuyPrice: decimal.NewFromFloat(150.00),
				},
			},
			expectedCount: 1,
		},
		{
			name: "multiple positions",
			positions: map[string]*Position{
				"AAPL": {
					Shares:      decimal.NewFromFloat(10.0),
					AvgBuyPrice: decimal.NewFromFloat(150.00),
				},
				"GOOGL": {
					Shares:      decimal.NewFromFloat(5.0),
					AvgBuyPrice: decimal.NewFromFloat(2500.00),
				},
				"TSLA": {
					Shares:      decimal.NewFromFloat(2.0),
					AvgBuyPrice: decimal.NewFromFloat(800.00),
				},
			},
			expectedCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brokerage := NewBacktestBrokerage(decimal.NewFromFloat(10000.00))
			brokerage.positions = tt.positions

			positions := brokerage.GetPositions()

			assert.Len(t, positions, tt.expectedCount)

			// Verify positions are copied correctly
			for symbol, expectedPos := range tt.positions {
				if pos, exists := positions[symbol]; exists {
					position := pos.(*Position)
					assert.Equal(t, expectedPos.Shares, position.Shares)
					assert.Equal(t, expectedPos.AvgBuyPrice, position.AvgBuyPrice)
				} else {
					t.Errorf("Expected position for %s not found", symbol)
				}
			}
		})
	}
}

func TestGetPositionShares(t *testing.T) {
	tests := []struct {
		name           string
		positions      map[string]*Position
		symbol         string
		expectError    bool
		expectedShares decimal.Decimal
	}{
		{
			name: "existing position",
			positions: map[string]*Position{
				"AAPL": {
					Shares:      decimal.NewFromFloat(10.0),
					AvgBuyPrice: decimal.NewFromFloat(150.00),
				},
			},
			symbol:         "AAPL",
			expectError:    false,
			expectedShares: decimal.NewFromFloat(10.0),
		},
		{
			name:           "non-existent position",
			positions:      map[string]*Position{},
			symbol:         "GOOGL",
			expectError:    true,
			expectedShares: decimal.Zero,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brokerage := NewBacktestBrokerage(decimal.NewFromFloat(10000.00))
			brokerage.positions = tt.positions

			shares, err := brokerage.GetPositionShares(tt.symbol)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "no position found")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedShares, shares)
			}
		})
	}
}

func TestConcurrentOperations(t *testing.T) {
	brokerage := NewBacktestBrokerage(decimal.NewFromFloat(10000.00))

	// Test concurrent deposits
	t.Run("concurrent deposits", func(t *testing.T) {
		numGoroutines := 10
		depositAmount := decimal.NewFromFloat(100.00)

		var wg sync.WaitGroup
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := brokerage.AddDeposit(depositAmount)
				require.NoError(t, err)
			}()
		}
		wg.Wait()

		expectedBalance := decimal.NewFromFloat(10000.00).Add(depositAmount.Mul(decimal.NewFromInt(int64(numGoroutines))))
		balance, err := brokerage.GetCashBalance()
		require.NoError(t, err)
		assert.Equal(t, expectedBalance, balance)
	})

	// Test concurrent buy orders
	t.Run("concurrent buy orders", func(t *testing.T) {
		numGoroutines := 5
		quantity := decimal.NewFromFloat(1.0)
		price := decimal.NewFromFloat(100.00)

		var wg sync.WaitGroup
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := brokerage.ExecuteBuyOrder("AAPL", quantity, price)
				require.NoError(t, err)
			}()
		}
		wg.Wait()

		expectedShares := decimal.NewFromFloat(float64(numGoroutines))
		shares, err := brokerage.GetPosition("AAPL")
		require.NoError(t, err)
		assert.Equal(t, expectedShares, shares)
	})
}

func TestEdgeCases(t *testing.T) {
	t.Run("very large numbers", func(t *testing.T) {
		brokerage := NewBacktestBrokerage(decimal.NewFromFloat(1000000000.00))

		// Buy with very large quantity and price
		err := brokerage.ExecuteBuyOrder("AAPL", decimal.NewFromFloat(1000000.0), decimal.NewFromFloat(1000.00))
		require.NoError(t, err)

		shares, err := brokerage.GetPosition("AAPL")
		require.NoError(t, err)
		assert.Equal(t, decimal.NewFromFloat(1000000.0), shares)
	})

	t.Run("very small numbers", func(t *testing.T) {
		brokerage := NewBacktestBrokerage(decimal.NewFromFloat(1000.00))

		// Buy with very small quantity and price
		err := brokerage.ExecuteBuyOrder("AAPL", decimal.NewFromFloat(0.0001), decimal.NewFromFloat(0.01))
		require.NoError(t, err)

		shares, err := brokerage.GetPosition("AAPL")
		require.NoError(t, err)
		assert.Equal(t, decimal.NewFromFloat(0.0001), shares)
	})

	t.Run("empty symbol", func(t *testing.T) {
		brokerage := NewBacktestBrokerage(decimal.NewFromFloat(1000.00))

		err := brokerage.ExecuteBuyOrder("", decimal.NewFromFloat(1.0), decimal.NewFromFloat(100.00))
		require.NoError(t, err)

		shares, err := brokerage.GetPosition("")
		require.NoError(t, err)
		assert.Equal(t, decimal.NewFromFloat(1.0), shares)
	})
}

func TestPositionStruct(t *testing.T) {
	t.Run("position creation", func(t *testing.T) {
		shares := decimal.NewFromFloat(10.0)
		avgPrice := decimal.NewFromFloat(150.00)

		position := &Position{
			Shares:      shares,
			AvgBuyPrice: avgPrice,
		}

		assert.Equal(t, shares, position.Shares)
		assert.Equal(t, avgPrice, position.AvgBuyPrice)
	})

	t.Run("position value calculation", func(t *testing.T) {
		position := &Position{
			Shares:      decimal.NewFromFloat(5.0),
			AvgBuyPrice: decimal.NewFromFloat(200.00),
		}

		expectedValue := decimal.NewFromFloat(1000.00) // 5 * 200
		actualValue := position.Shares.Mul(position.AvgBuyPrice)

		diff := actualValue.Sub(expectedValue).Abs()
		assert.True(t, diff.LessThanOrEqual(decimal.NewFromFloat(0.01)),
			"Expected value around %v, got %v", expectedValue, actualValue)
	})
}
