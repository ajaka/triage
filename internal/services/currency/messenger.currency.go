package currency

import (
	"context"
	"time"

	"github.com/ajaka/triage/internal/errs"
)

type CurrencyServiceMessenger struct {
	messageChan chan CurrencyRequest
}

type CurrencyResponse struct {
	Rate       float64
	DollarRate float64
	Err        error
}

type CurrencyRequest struct {
	from     string
	to       string
	respChan chan CurrencyResponse
}

func (c *CurrencyServiceMessenger) MakeCurrencyRequest(ctx context.Context, from string, to string) (*CurrencyResponse, error) {
	respChan := make(chan CurrencyResponse, 1)

	req := CurrencyRequest{
		from:     from,
		to:       to,
		respChan: respChan,
	}

	select {
	case c.messageChan <- req:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Second):
		return nil, errs.ERR_CURRENCY_SERVICE_UNRESPONSIVE
	}

	select {
	case resp := <-respChan:
		return &resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Second):
		return nil, errs.ERR_CURRENCY_SERVICE_UNRESPONSIVE
	}
}
