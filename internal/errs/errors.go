package errs

import (
	"errors"
)

var (
	ERR_UNSUPPORTED_CURRENCY          error = errors.New("The provided currency is not supported")
	ERR_EMPTY_CURRENCY_LIST           error = errors.New("The allowed currency list is empty")
	ERR_CURRENCY_SERVICE_UNRESPONSIVE error = errors.New("Currency Service is currently unresponsive")
	ERR_BINANCE_BAD_RESPONSE          error = errors.New("Binance returned a non ok response")
)
