# Artemis - Multi-Goal Repository

This repository contains multiple projects and tools. Currently, it includes:

## 📊 Backtesting Project

A comprehensive stock signal backtesting system that analyzes trading strategies using historical market data from Alpaca.

### Features

- **Basic Backtesting**: Simple buy-and-hold strategy analysis
- **Enhanced Strategies**: Multiple advanced trading strategies including:
  - Staggered Entry (80% at buy date, 20% during dips)
  - Take Profit with Trailing Stop (15% take profit, 15% trailing stop)
  - Simple Trailing Stop (15% trailing stop only)
  - Staggered Entry + Trailing Stop (combination strategy)
- **Performance Metrics**: Win rate, average returns, hold times, and annualized returns
- **Data Fallback**: Intelligent handling of missing historical data
- **Comprehensive Reporting**: Detailed results for each strategy

### Project Structure

```
Artemis/
├── backtesting/                 # Backtesting project
│   ├── cmd/                    # Command-line application
│   │   └── main.go            # Main entry point
│   ├── internal/               # Internal packages
│   │   ├── types.go           # Shared types and basic functions
│   │   └── enhanced_backtest.go # Enhanced strategy implementations
│   ├── data/                   # Data files
│   │   └── example/           # Example CSV files
│   ├── configs/                # Configuration files (future)
│   └── pkg/                    # Public packages (future)
├── go.mod                      # Go module file
├── go.sum                      # Go dependencies
└── README.md                   # This file
```

### Setup

1. **Install Dependencies**:
   ```bash
   make deps
   ```

2. **Set Environment Variables**:
   ```bash
   export ALPACA_API_KEY="your_api_key"
   export ALPACA_SECRET_KEY="your_secret_key"
   ```

3. **Build and Run** (using Makefile):
   ```bash
   # Build the application
   make build
   
   # Or build and run in one command
   make run
   
   # Or just run (if already built)
   make run-only
   ```

4. **Alternative Manual Build**:
   ```bash
   cd backtesting
   go build -o artemis-backtest ./cmd
   ./artemis-backtest
   ```

### CSV Input Format

The application reads stock signals from `backtesting/data/example/stock_signals.csv`:

```csv
ticker,buydate,selldate
AAPL,2024-01-15,2024-02-15
GOOGL,2024-01-20,2024-02-20
```

### Output

The application provides comprehensive backtesting results including:

- **Basic Backtest Results**: Simple buy-and-hold performance
- **Enhanced Strategy Comparison**: Performance of all 5 strategies
- **Individual Signal Results**: Detailed results for each signal
- **Performance Metrics**: Win rates, average returns, hold times, and annualized returns

### Strategy Details

1. **Basic Buy & Hold**: Traditional buy at signal date, sell at target date
2. **Staggered Entry**: 80% position at buy date, 20% during first week dips
3. **Take Profit + Trailing Stop**: Take profit at 15% gain, then 15% trailing stop
4. **Simple Trailing Stop**: 15% trailing stop from entry
5. **Staggered Entry + Trailing Stop**: Combines staggered entry with trailing stop

### Dependencies

- `github.com/alpacahq/alpaca-trade-api-go/v2` - Alpaca trading API
- `github.com/google/uuid` - UUID generation

### Makefile Commands

The repository includes a comprehensive Makefile for easy development and deployment:

```bash
# Show all available commands
make help

# Development workflow
make deps          # Download and tidy dependencies
make fmt           # Format Go code
make vet           # Run go vet
make build         # Build the application
make run           # Build and run
make test          # Run tests
make clean         # Clean build artifacts

# Cross-platform builds
make build-linux   # Build for Linux
make build-darwin  # Build for macOS
make build-windows # Build for Windows
make build-all     # Build for all platforms

# Data and environment
make validate-data # Validate CSV data format
make check-env     # Check environment variables

# Development shortcuts
make dev           # Full development workflow (deps, fmt, vet, build)
make full-test     # Complete test workflow
```

### Future Projects

This repository is designed to accommodate additional projects. Each new project should follow the same structure:

```
project-name/
├── cmd/           # Command-line applications
├── internal/      # Internal packages
├── data/          # Data files
├── configs/       # Configuration files
└── pkg/           # Public packages
```

---

*This repository is designed for educational and research purposes. Always verify results and consider transaction costs in real trading scenarios.*
