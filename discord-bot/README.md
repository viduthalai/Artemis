# Artemis Discord Bot

This is a Discord bot that provides a modal form interface for adding trading signals to the Artemis trading system. The bot runs as an AWS Lambda function and integrates with Discord's interaction system.

## Features

- **Slash Commands**: `/addsignal` - Opens a modal form for signal input
- **Modal Forms**: User-friendly form with fields for ticker, buy date, and sell date
- **Input Validation**: Validates date formats and ensures buy date is before sell date
- **DynamoDB Integration**: Saves signals to the same DynamoDB table used by the trading bot
- **Discord Interactions**: Handles Discord's interaction protocol including PING/PONG

## Discord Bot Setup

### 1. Create a Discord Application

1. Go to [Discord Developer Portal](https://discord.com/developers/applications)
2. Click "New Application" and give it a name (e.g., "Artemis Bot")
3. Go to the "Bot" section and create a bot
4. Copy the bot token for later use

### 2. Create Slash Command

You need to register the `/addsignal` command with Discord. Use this curl command:

```bash
curl -X POST \
  -H "Authorization: Bot YOUR_BOT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "addsignal",
    "description": "Add a new trading signal",
    "type": 1
  }' \
  https://discord.com/api/v10/applications/YOUR_APPLICATION_ID/commands
```

Replace:
- `YOUR_BOT_TOKEN` with your bot token
- `YOUR_APPLICATION_ID` with your application ID

### 3. Invite Bot to Server

Use this URL (replace `YOUR_APPLICATION_ID`):
```
https://discord.com/api/oauth2/authorize?client_id=YOUR_APPLICATION_ID&permissions=2048&scope=bot%20applications.commands
```

## AWS Lambda Setup

### 1. Deploy the Lambda Function

1. Build the binary for AWS Lambda Go runtime:
```bash
# Using the Makefile (recommended)
make deploy-discord

# Or manually:
cd discord-bot
GOOS=linux GOARCH=amd64 go build -o bootstrap ./cmd
zip artemis-discord-bot.zip bootstrap
```

**Note**: The binary must be named `bootstrap` for AWS Lambda's Go runtime.

3. Configure environment variables:
   - `DYNAMODB_REGION`: AWS region (default: us-east-1)
   - `TABLE_NAME`: DynamoDB table name (default: artemis-data)
   - `DISCORD_PUBLIC_KEY`: Your Discord application's public key (required)

### 2. Create Function URL

1. In the Lambda console, go to your function
2. Click "Configuration" → "Function URL"
3. Create a function URL with "NONE" auth type
4. Copy the function URL

### 3. Configure Discord Interactions Endpoint

1. Go to your Discord application in the Developer Portal
2. Go to "General Information"
3. Set the "Interactions Endpoint URL" to your Lambda function URL
4. Save changes

## Usage

1. In your Discord server, type `/addsignal`
2. A modal form will appear with fields:
   - **Stock Ticker**: Enter the stock symbol (e.g., AAPL)
   - **Buy Date**: Enter the buy date in YYYY-MM-DD format
   - **Sell Date**: Enter the sell date in YYYY-MM-DD format
3. Submit the form
4. The bot will validate the input and save the signal to DynamoDB
5. You'll receive a confirmation message with the signal details

## Discord Interaction Types

The bot handles these Discord interaction types:

- **Type 1 (PING)**: Responds with PONG for Discord's health checks
- **Type 2 (APPLICATION_COMMAND)**: Handles slash commands like `/addsignal`
- **Type 5 (MODAL_SUBMIT)**: Processes modal form submissions

## Response Types

- **Type 1 (PONG)**: Response to Discord's ping
- **Type 4 (CHANNEL_MESSAGE_WITH_SOURCE)**: Sends a message to the channel
- **Type 9 (MODAL)**: Shows a modal form to the user

## Error Handling

The bot provides user-friendly error messages for:
- Missing required fields
- Invalid date formats
- Buy date after sell date
- Database connection issues

## Security

- **Discord Signature Verification**: All requests are verified using Discord's Ed25519 signature verification
- **Timestamp Validation**: Requests must be within 5 minutes to prevent replay attacks
- **Input Validation**: The bot validates all input data and date formats
- **Environment Variables**: Uses environment variables for configuration
- **AWS IAM Integration**: Integrates with AWS IAM for DynamoDB access
- **No Sensitive Logging**: No sensitive data is logged

### Discord Public Key

You need to get your Discord application's public key:

1. Go to your Discord application in the Developer Portal
2. Go to "General Information"
3. Copy the "Public Key" value
4. Set it as the `DISCORD_PUBLIC_KEY` environment variable

## Development

To run locally for testing:

```bash
# Set environment variables
export DYNAMODB_REGION=us-east-1
export TABLE_NAME=artemis-data
export DISCORD_PUBLIC_KEY=your_discord_public_key_here

# Run the bot
go run ./cmd
```

## Integration with Trading Bot

The Discord bot saves signals to the same DynamoDB table used by the Artemis trading bot. When the trading bot runs, it will:

1. Load all signals from DynamoDB
2. Process signals based on their status (PENDING, BOUGHT)
3. Execute trades according to the buy/sell dates
4. Update signal status and save results back to DynamoDB

This creates a complete workflow from signal input via Discord to automated trading execution.
