package currency

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ajaka/triage/internal/errs"
	"github.com/ajaka/triage/internal/models"
	"github.com/gorilla/websocket"
)

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
	newUrl, err := c.encode(c.env.BASE_URL_FOR_LISTING_AVAILABLE_SPOTS, c.env.BINANCE_QUERY_NAME_FOR_LISTING_SPOTS, symbols)
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
	if resp.StatusCode != http.StatusOK {
		return nil, errs.ERR_BINANCE_BAD_RESPONSE
	}
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
		if s.Status == c.env.BINANCE_ALLOWED_TERM {
			allowed[s.Symbol] = struct{}{}
		}
	}
	return allowed, nil
}

func (c *CurrencyService) HandleWebsocketConn(ctx context.Context, allowedSymbols []string, currencyUpdateChan chan CurrencyUpdateMessage) error {
	newUrl, err := url.Parse(c.env.BASE_URL_FOR_LIVE_FEED)
	if err != nil {
		return err
	}
	queries := newUrl.Query()
	streams := ""
	for _, s := range allowedSymbols {
		streams += s + "@trade" + "/"
	}
	streams, _ = strings.CutSuffix(streams, "/")

	queries.Set(c.env.BINANCE_QUERY_NAME_FOR_LIVE_FEED, streams)
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
							errChan <- err
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
				if err != nil || math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 {
					c.logger.Error("Invalid price from binance, dropping and moving on", "price", message.Data.Price, "error", err)
					continue
				}
				resp := CurrencyUpdateMessage{
					code:  code,
					price: price,
				}
				select {
				case currencyUpdateChan <- resp:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return nil
}
