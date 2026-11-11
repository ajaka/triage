package currency

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/ajaka/triage/internal/configs"
	"github.com/ajaka/triage/internal/errs"
	"github.com/ajaka/triage/internal/models"
	"github.com/ajaka/triage/internal/repository"
)

const BASE = "USDT"

var ALLOWED_PAIRS = []string{"EURUSDT", "GBPUSDT"}

type CurrencyUpdateMessage struct {
	code  string
	price float64
}

type CurrencyService struct {
	repo   *repository.Repository
	store  map[string]models.Currency
	logger *slog.Logger
	env    *configs.Env
}

func (c *CurrencyService) Init(ctx context.Context, CurrencyRequestChan chan CurrencyRequest) error {
	c.logger.Info("Service starting")
	currencies, err := c.repo.GetActiveCurrencies(ctx)
	if err != nil {
		return err
	}
	allowed, err := c.checkValidityAndBuildAllowedCurrencies(ctx, &currencies)
	if err != nil {
		return err
	}

	var symbols []string

	for k, _ := range c.store {
		symbol := k + BASE
		if _, ok := allowed[symbol]; ok {
			symbols = append(symbols, strings.ToLower(symbol))
		} else {
			delete(c.store, k)
		}
	}
	if len(symbols) == 0 {
		return errs.ERR_EMPTY_CURRENCY_LIST
	}
	currencyUpdateChan := make(chan CurrencyUpdateMessage, 1)

	err = c.HandleWebsocketConn(ctx, symbols, currencyUpdateChan)
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				if err := c.Shutdown(ctx); err != nil {
					c.logger.Error("An Error occured while flsuhing currency to database, shutting down anyway", "error", err.Error())
				}
				return
			case msg := <-currencyUpdateChan:
				if value, ok := c.store[msg.code]; ok {
					value.LastKnownRateToDollars = msg.price
					c.store[msg.code] = value
				} else {
					c.logger.Warn("Received Invalid currency from the worker, dropping and moving on", "msg", msg)
				}

			case msg := <-CurrencyRequestChan:
				resp := CurrencyResponse{
					Rate:       0,
					DollarRate: 0,
					Err:        nil,
				}
				from, ok := c.store[msg.from]
				if !ok {
					resp.Err = errs.ERR_UNSUPPORTED_CURRENCY
					msg.respChan <- resp
					continue
				}
				to, ok := c.store[msg.to]
				if !ok {
					resp.Err = errs.ERR_UNSUPPORTED_CURRENCY
					msg.respChan <- resp
					continue
				}
				resp.Rate = from.LastKnownRateToDollars / to.LastKnownRateToDollars
				resp.DollarRate = from.LastKnownRateToDollars
				msg.respChan <- resp
			}

		}
	}()
	return nil
}

func (c *CurrencyService) Shutdown(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	var currencies []models.Currency
	for _, v := range c.store {
		currencies = append(currencies, v)
	}
	return c.repo.UpdateCurrencies(ctx, currencies)
}

func InitCurrencyService(ctx context.Context, repo *repository.Repository, env *configs.Env, logger *slog.Logger) (*CurrencyServiceMessenger, error) {
	logger = logger.With("service", "currency_service")
	service := CurrencyService{
		repo:   repo,
		logger: logger,
		store:  make(map[string]models.Currency),
		env:    env,
	}

	currencyRequestChan := make(chan CurrencyRequest, 100)
	if err := service.Init(ctx, currencyRequestChan); err != nil {
		return nil, err
	}

	messenger := CurrencyServiceMessenger{
		messageChan: currencyRequestChan,
	}
	service.logger.Info("Currency Service is life")
	return &messenger, nil
}
