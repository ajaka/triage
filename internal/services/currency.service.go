package services

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ajaka/triage/internal/errs"
	"github.com/ajaka/triage/internal/models"
	"github.com/ajaka/triage/internal/repository"
	"github.com/gorilla/websocket"
)

const (
	BASE_URL_FOR_LISTING_AVAILABLE_SPOTS = "https://api.binance.com/api/v3/exchangeInfo"
	BASE_URL_FOR_LIVE_FEED               = "wss://stream.binance.com:9443/stream"
	BASE                                 = "USDT"
	BINANCE_ALLOWED_TERM                 = "TRADING"
	BINANCE_QUERY_NAME_FOR_LISTING_SPOTS = "symbols"
	BINANCE_QUERY_NAME_FOR_LIVE_FEED     = "streams"
)

var ALLOWED_PAIRS = []string{"EURUSDT", "GBPUSDT"}

type CurrencyUpdateMessage struct {
	code  string
	price float64
}

type BinanceResponse struct {
	Symbols []SimpleSymbol `json:"symbols"`
}

type SimpleSymbol struct {
	Symbol string `json:"symbol"`
	Status string `json:"status"`
}

type BinanceStreamMessage struct {
	Stream string       `json:"stream"`
	Data   BinanceTrade `json:"data"`
}

type BinanceTrade struct {
	Symbol string `json:"s"`
	Price  string `json:"p"`
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

type CurrencyServiceMessenger struct {
	messageChan chan CurrencyRequest
}

func (c *CurrencyServiceMessenger) MakeCurrencyRequest(from string, to string) (*CurrencyResponse, error) {
	respChan := make(chan CurrencyResponse, 1)

	req := CurrencyRequest{
		from:     from,
		to:       to,
		respChan: respChan,
	}

	select {
	case c.messageChan <- req:
	case <-time.After(5 * time.Second):
		return nil, errs.ERR_CURRENCY_SERVICE_UNRESPONSIVE
	}
	select {
	case resp := <-respChan:
		return &resp, nil
	case <-time.After(5 * time.Second):
		return nil, errs.ERR_CURRENCY_SERVICE_UNRESPONSIVE
	}
}

type CurrencyService struct {
	repo   *repository.Repository
	store  map[string]models.Currency
	logger *slog.Logger
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

func (c *CurrencyService) checkValidityAndBuildAllowedCurrencies(ctx context.Context, data *[]models.Currency) (map[string]struct{}, error) {
	initiallyAllowed := make(map[string]struct{})
	for _, symbol := range ALLOWED_PAIRS {
		initiallyAllowed[symbol] = struct{}{}
	}
	var symbols []string

	for _, p := range *data {
		cap := strings.ToUpper(p.Code)
		symbol := cap + BASE
		if _, ok := initiallyAllowed[symbol]; ok {
			symbols = append(symbols, symbol)
			c.store[p.Code] = p
		}
	}
	newUrl, err := c.encode(BASE_URL_FOR_LISTING_AVAILABLE_SPOTS, BINANCE_QUERY_NAME_FOR_LISTING_SPOTS, symbols)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	client := http.Client{}
	req, err := http.NewRequestWithContext(ctx, "GET", newUrl, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var masterData BinanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&masterData); err != nil {
		c.logger.Error("Error parsing master config:", "error", err.Error())
		return nil, err
	}
	allowed := make(map[string]struct{})
	for _, s := range masterData.Symbols {
		if s.Status == BINANCE_ALLOWED_TERM {
			allowed[s.Symbol] = struct{}{}
		}
	}
	return allowed, nil
}

func (c *CurrencyService) encode(new_url string, query string, values []string) (string, error) {
	newUrl, err := url.Parse(new_url)
	if err != nil {
		return "", err
	}

	bytes, err := json.Marshal(values)
	if err != nil {
		c.logger.Info("Error marshaling symbols:", "error", err.Error())
		return "", err
	}

	encodedString := string(bytes)

	queries := newUrl.Query()
	queries.Set(query, encodedString)

	newUrl.RawQuery = queries.Encode()

	return newUrl.String(), nil
}

func (c *CurrencyService) HandleWebsocketConn(ctx context.Context, allowedSymbols []string, currencyUpdateChan chan CurrencyUpdateMessage) error {
	newUrl, err := url.Parse(BASE_URL_FOR_LIVE_FEED)
	if err != nil {
		return err
	}
	queries := newUrl.Query()
	streams := ""
	for _, s := range allowedSymbols {
		streams += s + "@trade" + "/"
	}
	streams, _ = strings.CutSuffix(streams, "/")

	queries.Set(BINANCE_QUERY_NAME_FOR_LIVE_FEED, streams)
	newUrl.RawQuery = queries.Encode()
	c.logger.Info("ws", "query", newUrl.String())

	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	initialConn, err := c.connect(ctx, dialer, newUrl.String())
	if err != nil {
		return err
	}

	var conn atomic.Pointer[websocket.Conn]
	conn.Store(initialConn)

	go func() {
		msgChan := make(chan []byte, 1)
		errChan := make(chan error, 1)

		go func() {
			defer func() {
				if current := conn.Load(); current != nil {
					current.Close()
				}
			}()
			errorsInRow := 0
			for {
				current := conn.Load()
				if current == nil {
					return
				}

				_, msg, err := current.ReadMessage()
				if err != nil {
					var closeErr *websocket.CloseError
					var netErr net.Error
					switch {
					case errors.Is(err, net.ErrClosed):
						return
					case errors.As(err, &closeErr):
						c.logger.Info("connection closed by server, reconnecing...", "code", closeErr.Code)
					case errors.As(err, &netErr):
						c.logger.Warn("network error, reconnecting...", "error", err)
					default:
						c.logger.Warn("An Error occured while reading from websocket", "error", err.Error())
						errorsInRow++
						if errorsInRow > 3 {
							c.logger.Error("An Unknown error has occured 3 times in a row. Exiting now.", "error", err.Error())
							return
						}
						continue
					}

					errorsInRow = 0
					newConn, err := c.connect(ctx, dialer, newUrl.String())
					if err != nil {
						errChan <- err
						return
					}
					c.logger.Info("Reconnection succeded")
					if old := conn.Swap(newConn); old != nil {
						old.Close()
					}
					continue
				} else {
					errorsInRow = 0
				}

				select {
				case msgChan <- msg:
				case <-ctx.Done():
					return
				}
			}
		}()

		for {
			select {
			case <-ctx.Done():
				if old := conn.Swap(nil); old != nil {
					old.Close()
				}
				return
			case err := <-errChan:
				c.logger.Error("websocket read failed", "error", err.Error())
				return
			case msg := <-msgChan:
				var message BinanceStreamMessage
				if err := json.Unmarshal(msg, &message); err != nil {
					c.logger.Error("An Error occured while parsing binance currency payload, dropping and moving on", "error", err.Error(), "bytes", msg)
					continue
				}
				code, ok := strings.CutSuffix(message.Data.Symbol, BASE)
				if !ok {
					c.logger.Error("Binance payload appears to be malformed, dropping and moving on", "payload", message)
					continue
				}
				price, err := strconv.ParseFloat(message.Data.Price, 64)
				if err != nil {
					c.logger.Error("An Error occured while parsing the price, dropping and moving on", "error", err.Error())
					continue
				}
				resp := CurrencyUpdateMessage{
					code:  code,
					price: price,
				}
				currencyUpdateChan <- resp
			}
		}
	}()
	return nil
}

func (c *CurrencyService) connect(ctx context.Context, dialer websocket.Dialer, url string) (*websocket.Conn, error) {
	var lastErr error
	for attempt := range 3 {
		conn, _, err := dialer.DialContext(ctx, url, nil)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * time.Second):
		}
	}
	return nil, lastErr
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

func InitCurrencyService(ctx context.Context, repo *repository.Repository, logger *slog.Logger) (*CurrencyServiceMessenger, error) {
	logger = logger.With("service", "currency_service")
	service := CurrencyService{
		repo:   repo,
		logger: logger,
		store:  make(map[string]models.Currency),
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
