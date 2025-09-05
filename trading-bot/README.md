# Artemis Trading Bot

Artemis is an automated stock trading bot that executes trades based on predefined signals using the Alpaca brokerage API. The bot is designed to run on AWS Lambda and uses DynamoDB for data persistence.

## Features

- **Automated Trading**: Executes buy and sell orders based on signal dates using market orders for guaranteed execution
- **Fair Allocation**: Uses a rolling window approach to allocate funds fairly across signals
- **AWS Integration**: Designed to run on AWS Lambda with DynamoDB storage
- **Signal Management**: Comprehensive signal tracking and management
- **Paper Trading Support**: Supports both paper and live trading environments
- **Discord Notifications**: Real-time notifications for trades, errors, and account status
- **Single Table Design**: Efficient DynamoDB usage with unified table architecture
- **Multi-Daily Execution**: Runs 3 times daily for optimal signal execution timing

## Architecture

### Core Components

1. **Trading Bot** (`internal/trading_bot.go`): Main orchestrator that processes signals and executes trades
2. **DynamoDB Service** (`internal/dynamodb.go`): Handles all database operations for signals and allocation windows
3. **Alpaca Service** (`internal/alpaca_service.go`): Manages trading operations through Alpaca API
4. **Notification Service** (`pkg/notification/discord.go`): Discord notification system for trading events

### Data Models

- **Signal**: Represents a trading signal with buy/sell dates, status, and execution details
- **AllocationWindow**: Tracks the rolling window for fund allocation
- **UnifiedItem**: Single table design for efficient DynamoDB storage

## Setup

### Prerequisites

- Go 1.24.3 or later
- AWS account with DynamoDB access
- Alpaca brokerage account (paper or live)

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd Artemis/trading-bot
```

2. Install dependencies:
```bash
go mod tidy
```

3. Create DynamoDB table:
   - `artemis-data`: Unified table storing signals and allocation windows

4. Configure the application:
   - Copy `configs/config.json` and update with your credentials
   - Set your Alpaca API keys
   - Configure DynamoDB region and table names

### Configuration

The bot uses environment variables for configuration. Copy `env.example` to `.env` and fill in your values:

#### Required Environment Variables
- `ALPACA_API_KEY`: Your Alpaca API key
- `ALPACA_SECRET_KEY`: Your Alpaca secret key

#### Optional Environment Variables (with defaults)
- `DYNAMODB_REGION`: AWS region for DynamoDB (default: `us-east-1`)
- `TABLE_NAME`: DynamoDB table name (default: `artemis-data`)
- `MAX_SIGNALS_PER_WINDOW`: Maximum signals per allocation window (default: `39`)
- `WINDOW_DURATION_DAYS`: Duration of allocation window in days (default: `90`)
- `DEFAULT_ALLOCATION_AMOUNT`: Default allocation amount per signal (default: `1000.0`)
- `IS_PAPER_TRADING`: Enable paper trading (default: `true`)
- `DISCORD_WEBHOOK_URL`: Discord webhook URL for notifications (optional)

#### Example Environment File
```bash
# Copy env.example to .env and update with your values
cp env.example .env

# Edit .env file with your actual values
ALPACA_API_KEY=your_actual_api_key
ALPACA_SECRET_KEY=your_actual_secret_key
DYNAMODB_REGION=us-east-1
TABLE_NAME=artemis-data
MAX_SIGNALS_PER_WINDOW=39
WINDOW_DURATION_DAYS=90
DEFAULT_ALLOCATION_AMOUNT=1000.0
IS_PAPER_TRADING=true
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/your-webhook-url
```

## Usage

### Running the Bot

#### Local Development
```bash
# Set environment variables
export ALPACA_API_KEY=your_api_key
export ALPACA_SECRET_KEY=your_secret_key

# Run the bot
go run cmd/main.go
```

#### Using .env file
```bash
# Copy the example environment file
cp env.example .env

# Edit .env with your actual values
# Then run the bot
go run cmd/main.go
```





### Signal Processing

The bot automatically processes signals based on their status:

1. **PENDING**: If current date >= buy date, calculates allocation and buys the stock using market orders
2. **BOUGHT**: If current date >= sell date, sells the stock using market orders and calculates P&L

### Execution Strategy

The bot runs **3 times daily** for optimal signal execution:

- **10:00 AM EST**: Morning execution - handles both new buys and sell orders
- **12:30 PM EST**: Mid-day execution - handles new buy signals added during morning
- **2:30 PM EST**: Afternoon execution - handles new buy signals added during lunch/mid-day

**Market Orders**: All orders use market orders for guaranteed execution, ensuring no missed trades due to price movement or volatility.

### Allocation Strategy

The bot uses a rolling 90-day window approach:

- Divides account value by the number of signals in the window (max 39)
- Updates allocation window when it expires
- Ensures fair distribution of funds across signals

## AWS Lambda Deployment

### Lambda Function

The bot is designed to run as an AWS Lambda function triggered 3 times daily via EventBridge Scheduler:
- **10:00 AM EST**: Morning execution
- **12:30 PM EST**: Mid-day execution  
- **2:30 PM EST**: Afternoon execution

### DynamoDB Table

#### Unified Table (`artemis-data`)
- Partition Key: `pk` (String) - "SIGNAL#PENDING", "SIGNAL#BOUGHT", "ALLOCATION#CURRENT"
- Sort Key: `sk` (String) - Signal UUID or "WINDOW"
- Attributes: type, data (JSON), created_at, updated_at

### Discord Notifications

The bot sends real-time notifications to Discord for:
- **Bot Start/Complete**: When the bot begins and finishes processing
- **Signal Bought**: Details of executed buy orders with actual fill prices
- **Signal Sold**: Trade completion with profit/loss information and actual fill prices
- **Account Status**: Current account value, cash balance, and active signals
- **Errors**: Detailed error notifications with context

To enable Discord notifications:
1. Create a Discord webhook in your server
2. Set the `DISCORD_WEBHOOK_URL` environment variable
3. The bot will automatically send notifications for all events

## Execution Strategy & Cost Analysis

### Why Market Orders?

The bot uses **market orders** for all trades to ensure:
- **Guaranteed Execution**: No missed trades due to price movement
- **Immediate Fills**: Orders execute at current market price
- **No End-of-Day Cancellations**: Market orders fill immediately
- **Perfect for Volatile Markets**: Ideal for earnings/news days

### Why 3x Daily Execution?

Running 3 times daily provides:
- **Multiple Execution Windows**: Catch signals added throughout the day
- **Mid-Day Signal Support**: Execute signals added during lunch/mid-day
- **Risk Management**: Multiple opportunities to enter positions
- **Consistent Sell Timing**: All sell orders execute in the morning run

### AWS Cost Analysis

**Monthly Costs (us-east-2):**
- **1x daily**: $0.000175/month
- **2x daily**: $0.000349/month  
- **3x daily**: $0.000524/month

**Annual Cost**: Less than $0.01/year regardless of execution frequency.

The cost is negligible, allowing focus on execution quality rather than cost optimization.

## Development

### Project Structure

```
trading-bot/
├── cmd/
│   └── main.go              # Application entry point
├── internal/
│   ├── types.go             # Data models and types
│   ├── dynamodb.go          # DynamoDB operations
│   ├── alpaca_service.go    # Alpaca trading API
│   └── trading_bot.go       # Main trading logic
├── pkg/
│   ├── signal_manager.go    # Signal management utilities
│   └── notification/
│       ├── discord.go       # Discord notification service
│       └── discord_test.go  # Notification tests
├── configs/
│   └── config.json          # Configuration file
└── README.md
```

### Testing

```bash
go test ./...
```

### Building

```bash
go build -o artemis-bot cmd/main.go
```

## Monitoring and Logging

The bot provides comprehensive logging for:
- Signal processing status across all 3 daily runs
- Market order execution details with actual fill prices
- Allocation window updates
- Error conditions and retry logic
- Execution timing and performance metrics

## Security Considerations

- Store API keys securely (use AWS Secrets Manager or environment variables)
- Use IAM roles with minimal required permissions
- Enable CloudTrail for audit logging
- Use VPC for Lambda if required

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

[Add your license information here]
