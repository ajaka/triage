# triage

A banking backend written in Go.

## Overview

`triage` is a Go banking application. Its still a WIP but certain modules have been implemented including **currency service**, which maintains live exchange rates and serves on-the-fly currency conversion requests.

## Project structure

```
cmd/api/          # Application entry point
internal/         # Private application code
  configs/        # Environment and database configuration
  errs/           # Sentinel errors
  middlewares/    # HTTP middleware
  models/         # Database models
  repository/     # Data access layer (pgx)
  services/       # Business services
    currency/     # Currency service
migrations/       # SQL migrations (up/down)
scripts/          # Helper scripts
design/           # Architecture and design documents
.env.example      # Example environment variables
```

## Configuration

Copy `.env.example` to `.env` and fill in the values:

```
DATABASE_URL=postgres://username:password@localhost:5432/triage
PORT=:5000
BASE_URL_FOR_LISTING_AVAILABLE_SPOTS=https://api.binance.com/api/v3/exchangeInfo
BASE_URL_FOR_LIVE_FEED=wss://stream.binance.com:9443/stream
BINANCE_ALLOWED_TERM=TRADING
BINANCE_QUERY_NAME_FOR_LISTING_SPOTS=symbols
BINANCE_QUERY_NAME_FOR_LIVE_FEED=streams
ORGANIZATION_SECRET_ENCRYPTION_KEY=...
EMAIL_JWT_SECRET_KEY=...
```

## Running

Run the migration using a tool of your choice — `golang-migrate` is recommended.

```bash

# Run the server
go run ./cmd/api
```

## Design documentation

Architecture and design documents live in the [`design/`](design) folder..

## Testing

```bash
go test ./...

go test ./... -race

```

## License

MIT
