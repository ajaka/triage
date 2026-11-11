package currency

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ajaka/triage/internal/configs"
	"github.com/ajaka/triage/internal/errs"
	"github.com/ajaka/triage/internal/models"
	"github.com/gorilla/websocket"
)

type mockRepository struct {
	getActiveCurrenciesFunc func(ctx context.Context) ([]models.Currency, error)
	updateCurrenciesFunc    func(ctx context.Context, currencies []models.Currency) error
}

func (m *mockRepository) GetActiveCurrencies(ctx context.Context) ([]models.Currency, error) {
	if m.getActiveCurrenciesFunc != nil {
		return m.getActiveCurrenciesFunc(ctx)
	}
	return nil, nil
}

func (m *mockRepository) UpdateCurrencies(ctx context.Context, currencies []models.Currency) error {
	if m.updateCurrenciesFunc != nil {
		return m.updateCurrenciesFunc(ctx, currencies)
	}
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testEnv(httpURL, wsURL string) *configs.Env {
	return &configs.Env{
		PORT:                                 "8080",
		BASE_URL_FOR_LISTING_AVAILABLE_SPOTS: httpURL,
		BASE_URL_FOR_LIVE_FEED:               wsURL,
		BINANCE_ALLOWED_TERM:                 "TRADING",
		BINANCE_QUERY_NAME_FOR_LISTING_SPOTS: "symbols",
		BINANCE_QUERY_NAME_FOR_LIVE_FEED:     "streams",
	}
}

// ----------------------------------------------------------------------------
// Encode tests
// ----------------------------------------------------------------------------

func TestEncode(t *testing.T) {
	service := &CurrencyService{logger: testLogger()}

	t.Run("successfully encodes symbols into query param", func(t *testing.T) {
		baseURL := "https://api.binance.com/api/v3/exchangeInfo"
		symbols := []string{"EURUSDT", "GBPUSDT"}

		encodedURL, err := service.encode(baseURL, "symbols", symbols)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedQuery := `symbols=%5B%22EURUSDT%22%2C%22GBPUSDT%22%5D`
		if !strings.Contains(encodedURL, expectedQuery) {
			t.Errorf("expected url to contain %q, got %q", expectedQuery, encodedURL)
		}
	})

	t.Run("fails on invalid base url", func(t *testing.T) {
		invalidURL := "http:// invalid url with spaces"
		_, err := service.encode(invalidURL, "symbols", []string{"EURUSDT"})
		if err == nil {
			t.Fatal("expected error on invalid url, got nil")
		}
	})
}

// ----------------------------------------------------------------------------
// CheckValidityAndBuildAllowedCurrencies tests
// ----------------------------------------------------------------------------

func TestCheckValidityAndBuildAllowedCurrencies(t *testing.T) {
	t.Run("successfully validates and builds allowed symbols", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := BinanceResponse{
				Symbols: []SimpleSymbol{
					{Symbol: "EURUSDT", Status: "TRADING"},
					{Symbol: "GBPUSDT", Status: "TRADING"},
					{Symbol: "JPYUSDT", Status: "BREAK"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer ts.Close()

		env := testEnv(ts.URL, "")
		service := &CurrencyService{
			env:    env,
			logger: testLogger(),
			store:  make(map[string]models.Currency),
		}

		currencies := []models.Currency{
			{Id: 1, Code: "EUR", Active: true, LastKnownRateToDollars: 1.08},
			{Id: 2, Code: "GBP", Active: true, LastKnownRateToDollars: 1.25},
			{Id: 3, Code: "XYZ", Active: true, LastKnownRateToDollars: 0.50}, // not in ALLOWED_PAIRS
		}

		allowed, err := service.checkValidityAndBuildAllowedCurrencies(context.Background(), &currencies)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(allowed) != 2 {
			t.Fatalf("expected 2 allowed symbols, got %d", len(allowed))
		}
		if _, ok := allowed["EURUSDT"]; !ok {
			t.Errorf("expected EURUSDT to be allowed")
		}
		if _, ok := allowed["GBPUSDT"]; !ok {
			t.Errorf("expected GBPUSDT to be allowed")
		}

		if _, ok := service.store["EUR"]; !ok {
			t.Errorf("expected EUR in service store")
		}
		if _, ok := service.store["GBP"]; !ok {
			t.Errorf("expected GBP in service store")
		}
		if _, ok := service.store["XYZ"]; ok {
			t.Errorf("XYZ should not be in service store")
		}
	})

	t.Run("excludes currency in ALLOWED_PAIRS if Binance returns non-TRADING status", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := BinanceResponse{
				Symbols: []SimpleSymbol{
					{Symbol: "EURUSDT", Status: "TRADING"},
					{Symbol: "GBPUSDT", Status: "HALT"}, // in ALLOWED_PAIRS, but non-TRADING
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer ts.Close()

		env := testEnv(ts.URL, "")
		service := &CurrencyService{
			env:    env,
			logger: testLogger(),
			store:  make(map[string]models.Currency),
		}

		currencies := []models.Currency{
			{Id: 1, Code: "EUR", Active: true, LastKnownRateToDollars: 1.08},
			{Id: 2, Code: "GBP", Active: true, LastKnownRateToDollars: 1.25},
		}

		allowed, err := service.checkValidityAndBuildAllowedCurrencies(context.Background(), &currencies)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, ok := allowed["EURUSDT"]; !ok {
			t.Errorf("expected EURUSDT to be allowed")
		}
		if _, ok := allowed["GBPUSDT"]; ok {
			t.Errorf("expected GBPUSDT to be absent from allowed map because status is HALT")
		}
	})

	t.Run("returns error when Binance responds with non-200 status", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()

		env := testEnv(ts.URL, "")
		service := &CurrencyService{
			env:    env,
			logger: testLogger(),
			store:  make(map[string]models.Currency),
		}

		currencies := []models.Currency{
			{Id: 1, Code: "EUR", Active: true, LastKnownRateToDollars: 1.08},
		}

		_, err := service.checkValidityAndBuildAllowedCurrencies(context.Background(), &currencies)
		if !errors.Is(err, errs.ERR_BINANCE_BAD_RESPONSE) {
			t.Fatalf("expected ERR_BINANCE_BAD_RESPONSE, got: %v", err)
		}
	})

	t.Run("returns error when Binance response body is invalid JSON", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("invalid json"))
		}))
		defer ts.Close()

		env := testEnv(ts.URL, "")
		service := &CurrencyService{
			env:    env,
			logger: testLogger(),
			store:  make(map[string]models.Currency),
		}

		currencies := []models.Currency{
			{Id: 1, Code: "EUR", Active: true, LastKnownRateToDollars: 1.08},
		}

		_, err := service.checkValidityAndBuildAllowedCurrencies(context.Background(), &currencies)
		if err == nil {
			t.Fatal("expected error decoding malformed JSON, got nil")
		}
	})

	t.Run("fails when context is already cancelled", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		env := testEnv(ts.URL, "")
		service := &CurrencyService{
			env:    env,
			logger: testLogger(),
			store:  make(map[string]models.Currency),
		}

		currencies := []models.Currency{
			{Id: 1, Code: "EUR", Active: true, LastKnownRateToDollars: 1.08},
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := service.checkValidityAndBuildAllowedCurrencies(ctx, &currencies)
		if err == nil {
			t.Fatal("expected context error, got nil")
		}
	})
}

// ----------------------------------------------------------------------------
// HandleWebsocketConn and message processing tests
// ----------------------------------------------------------------------------

func TestHandleWebsocketConn(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	t.Run("successfully receives and processes trade stream messages", func(t *testing.T) {
		serverDone := make(chan struct{})
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Errorf("upgrade error: %v", err)
				return
			}
			defer conn.Close()

			trade := BinanceStreamMessage{
				Stream: "eurusdt@trade",
				Data: BinanceTrade{
					Symbol: "EURUSDT",
					Price:  "1.0950",
				},
			}
			msgBytes, _ := json.Marshal(trade)
			_ = conn.WriteMessage(websocket.TextMessage, msgBytes)

			<-serverDone
		}))
		defer func() {
			close(serverDone)
			ts.Close()
		}()

		wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/stream"
		env := testEnv("", wsURL)

		service := &CurrencyService{
			env:    env,
			logger: testLogger(),
			store:  make(map[string]models.Currency),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		updateChan := make(chan CurrencyUpdateMessage, 5)
		err := service.HandleWebsocketConn(ctx, []string{"eurusdt"}, updateChan)
		if err != nil {
			t.Fatalf("HandleWebsocketConn failed: %v", err)
		}

		select {
		case msg := <-updateChan:
			if msg.code != "EUR" {
				t.Errorf("expected code EUR, got %s", msg.code)
			}
			if msg.price != 1.0950 {
				t.Errorf("expected price 1.0950, got %f", msg.price)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for trade update message")
		}
	})

	t.Run("filters out malformed json and invalid prices", func(t *testing.T) {
		serverDone := make(chan struct{})
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()

			// 1. Malformed JSON
			_ = conn.WriteMessage(websocket.TextMessage, []byte("{invalid-json"))

			// 2. Missing USDT base suffix
			tradeBadBase := BinanceStreamMessage{
				Stream: "bad@trade",
				Data:   BinanceTrade{Symbol: "EURBTC", Price: "1.0"},
			}
			b1, _ := json.Marshal(tradeBadBase)
			_ = conn.WriteMessage(websocket.TextMessage, b1)

			// 3. Non-numeric or invalid price
			tradeBadPrice := BinanceStreamMessage{
				Stream: "eurusdt@trade",
				Data:   BinanceTrade{Symbol: "EURUSDT", Price: "-5.0"},
			}
			b2, _ := json.Marshal(tradeBadPrice)
			_ = conn.WriteMessage(websocket.TextMessage, b2)

			// 3b. Zero price "0.0"
			tradeZeroPrice := BinanceStreamMessage{
				Stream: "eurusdt@trade",
				Data:   BinanceTrade{Symbol: "EURUSDT", Price: "0.0"},
			}
			bZero, _ := json.Marshal(tradeZeroPrice)
			_ = conn.WriteMessage(websocket.TextMessage, bZero)

			// 4. Valid trade
			tradeValid := BinanceStreamMessage{
				Stream: "gbpusdt@trade",
				Data:   BinanceTrade{Symbol: "GBPUSDT", Price: "1.2850"},
			}
			b3, _ := json.Marshal(tradeValid)
			_ = conn.WriteMessage(websocket.TextMessage, b3)

			<-serverDone
		}))
		defer func() {
			close(serverDone)
			ts.Close()
		}()

		wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/stream"
		env := testEnv("", wsURL)

		service := &CurrencyService{
			env:    env,
			logger: testLogger(),
			store:  make(map[string]models.Currency),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		updateChan := make(chan CurrencyUpdateMessage, 5)
		err := service.HandleWebsocketConn(ctx, []string{"eurusdt", "gbpusdt"}, updateChan)
		if err != nil {
			t.Fatalf("HandleWebsocketConn failed: %v", err)
		}

		select {
		case msg := <-updateChan:
			// The only valid message is the GBP trade
			if msg.code != "GBP" || msg.price != 1.2850 {
				t.Fatalf("expected first valid message to be GBP:1.2850, got %+v", msg)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for valid trade update")
		}
	})

	t.Run("drops price of 0.0", func(t *testing.T) {
		serverDone := make(chan struct{})
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()

			// Send zero price first
			tradeZero := BinanceStreamMessage{
				Stream: "eurusdt@trade",
				Data:   BinanceTrade{Symbol: "EURUSDT", Price: "0.0"},
			}
			b0, _ := json.Marshal(tradeZero)
			_ = conn.WriteMessage(websocket.TextMessage, b0)

			// Then send valid price
			tradeValid := BinanceStreamMessage{
				Stream: "eurusdt@trade",
				Data:   BinanceTrade{Symbol: "EURUSDT", Price: "1.0850"},
			}
			b1, _ := json.Marshal(tradeValid)
			_ = conn.WriteMessage(websocket.TextMessage, b1)

			<-serverDone
		}))
		defer func() {
			close(serverDone)
			ts.Close()
		}()

		wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/stream"
		env := testEnv("", wsURL)

		service := &CurrencyService{
			env:    env,
			logger: testLogger(),
			store:  make(map[string]models.Currency),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		updateChan := make(chan CurrencyUpdateMessage, 5)
		err := service.HandleWebsocketConn(ctx, []string{"eurusdt"}, updateChan)
		if err != nil {
			t.Fatalf("HandleWebsocketConn failed: %v", err)
		}

		select {
		case msg := <-updateChan:
			if msg.price != 1.0850 {
				t.Fatalf("expected price 1.0850 (0.0 should be dropped), got %f", msg.price)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for trade update")
		}
	})

	t.Run("reconnects on unexpected disconnect and continues receiving", func(t *testing.T) {
		var connCount atomic.Int32
		serverDone := make(chan struct{})

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := connCount.Add(1)
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}

			if count == 1 {
				// Send a trade then close the connection immediately to trigger reconnect
				trade := BinanceStreamMessage{
					Stream: "eurusdt@trade",
					Data:   BinanceTrade{Symbol: "EURUSDT", Price: "1.0800"},
				}
				b, _ := json.Marshal(trade)
				_ = conn.WriteMessage(websocket.TextMessage, b)
				time.Sleep(50 * time.Millisecond)
				_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
				conn.Close()
				return
			}

			// Connection 2: send another trade
			trade := BinanceStreamMessage{
				Stream: "eurusdt@trade",
				Data:   BinanceTrade{Symbol: "EURUSDT", Price: "1.0850"},
			}
			b, _ := json.Marshal(trade)
			_ = conn.WriteMessage(websocket.TextMessage, b)

			<-serverDone
			conn.Close()
		}))
		defer func() {
			close(serverDone)
			ts.Close()
		}()

		wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/stream"
		env := testEnv("", wsURL)

		service := &CurrencyService{
			env:    env,
			logger: testLogger(),
			store:  make(map[string]models.Currency),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		updateChan := make(chan CurrencyUpdateMessage, 10)
		err := service.HandleWebsocketConn(ctx, []string{"eurusdt"}, updateChan)
		if err != nil {
			t.Fatalf("HandleWebsocketConn failed: %v", err)
		}

		// Message 1 from first connection
		select {
		case msg := <-updateChan:
			if msg.price != 1.0800 {
				t.Errorf("expected 1.0800, got %f", msg.price)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for first message")
		}

		// Message 2 from reconnected connection
		select {
		case msg := <-updateChan:
			if msg.price != 1.0850 {
				t.Errorf("expected 1.0850 after reconnect, got %f", msg.price)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for message after reconnect")
		}
	})

	t.Run("fails when initial connection cannot be established", func(t *testing.T) {
		env := testEnv("", "ws://127.0.0.1:1/invalid")
		service := &CurrencyService{
			env:    env,
			logger: testLogger(),
			store:  make(map[string]models.Currency),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		updateChan := make(chan CurrencyUpdateMessage, 1)
		err := service.HandleWebsocketConn(ctx, []string{"eurusdt"}, updateChan)
		if err == nil {
			t.Fatal("expected connection error, got nil")
		}
	})
}

// ----------------------------------------------------------------------------
// Actor and CurrencyService.Init tests
// ----------------------------------------------------------------------------

func TestCurrencyServiceInitAndActorLoop(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	t.Run("calculates correct rates and updates store from worker stream", func(t *testing.T) {
		// Mock HTTP exchangeInfo endpoint
		httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := BinanceResponse{
				Symbols: []SimpleSymbol{
					{Symbol: "EURUSDT", Status: "TRADING"},
					{Symbol: "GBPUSDT", Status: "TRADING"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer httpSrv.Close()

		// Mock WebSocket endpoint
		wsConnChan := make(chan *websocket.Conn, 1)
		wsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			wsConnChan <- conn
		}))
		defer wsSrv.Close()

		wsURL := "ws" + strings.TrimPrefix(wsSrv.URL, "http") + "/stream"
		env := testEnv(httpSrv.URL, wsURL)

		var updatedCurrencies []models.Currency
		var updateMu sync.Mutex
		mockRepo := &mockRepository{
			getActiveCurrenciesFunc: func(ctx context.Context) ([]models.Currency, error) {
				return []models.Currency{
					{Id: 1, Code: "EUR", Active: true, LastKnownRateToDollars: 1.10},
					{Id: 2, Code: "GBP", Active: true, LastKnownRateToDollars: 1.30},
				}, nil
			},
			updateCurrenciesFunc: func(ctx context.Context, currencies []models.Currency) error {
				updateMu.Lock()
				defer updateMu.Unlock()
				updatedCurrencies = currencies
				return nil
			},
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		reqChan := make(chan CurrencyRequest, 10)
		service := &CurrencyService{
			repo:   mockRepo,
			logger: testLogger(),
			store:  make(map[string]models.Currency),
			env:    env,
		}

		err := service.Init(ctx, reqChan)
		if err != nil {
			t.Fatalf("Init failed: %v", err)
		}

		messenger := &CurrencyServiceMessenger{messageChan: reqChan}

		// 1. Initial rate calculation: EUR to GBP
		resp, err := messenger.MakeCurrencyRequest(ctx, "EUR", "GBP")
		if err != nil {
			t.Fatalf("MakeCurrencyRequest failed: %v", err)
		}
		if resp.Err != nil {
			t.Fatalf("unexpected resp error: %v", resp.Err)
		}
		expectedRate := 1.10 / 1.30
		if math.Abs(resp.Rate-expectedRate) > 1e-9 {
			t.Errorf("expected rate %f, got %f", expectedRate, resp.Rate)
		}
		if resp.DollarRate != 1.10 {
			t.Errorf("expected DollarRate 1.10, got %f", resp.DollarRate)
		}

		// 2. Same currency: EUR to EUR
		respEUR, err := messenger.MakeCurrencyRequest(ctx, "EUR", "EUR")
		if err != nil || respEUR.Err != nil {
			t.Fatalf("failed EUR to EUR request: err=%v, resp.Err=%v", err, respEUR.Err)
		}
		if respEUR.Rate != 1.0 {
			t.Errorf("expected rate 1.0 for same currency, got %f", respEUR.Rate)
		}

		// 3. Unsupported currency
		respUnsupported, err := messenger.MakeCurrencyRequest(ctx, "UNKNOWN", "EUR")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !errors.Is(respUnsupported.Err, errs.ERR_UNSUPPORTED_CURRENCY) {
			t.Fatalf("expected ERR_UNSUPPORTED_CURRENCY, got %v", respUnsupported.Err)
		}

		respUnsupportedTo, err := messenger.MakeCurrencyRequest(ctx, "EUR", "UNKNOWN")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !errors.Is(respUnsupportedTo.Err, errs.ERR_UNSUPPORTED_CURRENCY) {
			t.Fatalf("expected ERR_UNSUPPORTED_CURRENCY, got %v", respUnsupportedTo.Err)
		}

		// 4. Send live update via WebSocket and verify updated rate
		var wsConn *websocket.Conn
		select {
		case wsConn = <-wsConnChan:
			defer wsConn.Close()
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for ws client to connect")
		}

		tradeUpdate := BinanceStreamMessage{
			Stream: "eurusdt@trade",
			Data:   BinanceTrade{Symbol: "EURUSDT", Price: "1.2000"},
		}
		updateBytes, _ := json.Marshal(tradeUpdate)
		_ = wsConn.WriteMessage(websocket.TextMessage, updateBytes)

		// Wait briefly for actor goroutine to process the update
		time.Sleep(100 * time.Millisecond)

		respUpdated, err := messenger.MakeCurrencyRequest(ctx, "EUR", "GBP")
		if err != nil {
			t.Fatalf("MakeCurrencyRequest failed after update: %v", err)
		}
		newExpectedRate := 1.20 / 1.30
		if math.Abs(respUpdated.Rate-newExpectedRate) > 1e-9 {
			t.Errorf("expected rate %f after live update, got %f", newExpectedRate, respUpdated.Rate)
		}
		if respUpdated.DollarRate != 1.20 {
			t.Errorf("expected DollarRate 1.20, got %f", respUpdated.DollarRate)
		}

		// 5. Test Shutdown on context cancellation
		cancel()
		time.Sleep(100 * time.Millisecond)

		updateMu.Lock()
		count := len(updatedCurrencies)
		updateMu.Unlock()

		if count != 2 {
			t.Errorf("expected 2 currencies flushed to db on shutdown, got %d", count)
		}
	})

	t.Run("prunes non-trading currency from store and does not subscribe to websocket", func(t *testing.T) {
		httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := BinanceResponse{
				Symbols: []SimpleSymbol{
					{Symbol: "EURUSDT", Status: "TRADING"},
					{Symbol: "GBPUSDT", Status: "BREAK"}, // non-trading status
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer httpSrv.Close()

		var subscribedStreams string
		var wsMu sync.Mutex
		wsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wsMu.Lock()
			subscribedStreams = r.URL.Query().Get("streams")
			wsMu.Unlock()

			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
		}))
		defer wsSrv.Close()

		wsURL := "ws" + strings.TrimPrefix(wsSrv.URL, "http") + "/stream"
		env := testEnv(httpSrv.URL, wsURL)

		mockRepo := &mockRepository{
			getActiveCurrenciesFunc: func(ctx context.Context) ([]models.Currency, error) {
				return []models.Currency{
					{Id: 1, Code: "EUR", Active: true, LastKnownRateToDollars: 1.10},
					{Id: 2, Code: "GBP", Active: true, LastKnownRateToDollars: 1.30},
				}, nil
			},
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		reqChan := make(chan CurrencyRequest, 10)
		service := &CurrencyService{
			repo:   mockRepo,
			logger: testLogger(),
			store:  make(map[string]models.Currency),
			env:    env,
		}

		err := service.Init(ctx, reqChan)
		if err != nil {
			t.Fatalf("Init failed: %v", err)
		}

		// 1. Assert GBP is pruned from store
		if _, ok := service.store["GBP"]; ok {
			t.Errorf("expected GBP to be pruned from store, but it was found")
		}
		if _, ok := service.store["EUR"]; !ok {
			t.Errorf("expected EUR to remain in store")
		}

		// 2. Assert websocket stream query contains eurusdt and does NOT contain gbpusdt
		wsMu.Lock()
		streams := subscribedStreams
		wsMu.Unlock()

		if strings.Contains(streams, "gbpusdt") {
			t.Errorf("expected websocket streams NOT to contain gbpusdt, got %q", streams)
		}
		if !strings.Contains(streams, "eurusdt@trade") {
			t.Errorf("expected websocket streams to contain eurusdt@trade, got %q", streams)
		}

		// 3. Assert requesting GBP returns ERR_UNSUPPORTED_CURRENCY
		messenger := &CurrencyServiceMessenger{messageChan: reqChan}
		resp, err := messenger.MakeCurrencyRequest(ctx, "GBP", "EUR")
		if err != nil {
			t.Fatalf("unexpected request error: %v", err)
		}
		if !errors.Is(resp.Err, errs.ERR_UNSUPPORTED_CURRENCY) {
			t.Errorf("expected ERR_UNSUPPORTED_CURRENCY for pruned GBP, got %v", resp.Err)
		}
	})

	t.Run("Init fails when repo.GetActiveCurrencies returns error", func(t *testing.T) {
		repoErr := errors.New("db query failed")
		mockRepo := &mockRepository{
			getActiveCurrenciesFunc: func(ctx context.Context) ([]models.Currency, error) {
				return nil, repoErr
			},
		}

		service := &CurrencyService{
			repo:   mockRepo,
			logger: testLogger(),
			store:  make(map[string]models.Currency),
			env:    testEnv("", ""),
		}

		err := service.Init(context.Background(), make(chan CurrencyRequest))
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected %v, got %v", repoErr, err)
		}
	})

	t.Run("Init fails with ERR_EMPTY_CURRENCY_LIST when no currencies match allowed pairs", func(t *testing.T) {
		httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := BinanceResponse{Symbols: []SimpleSymbol{}}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer httpSrv.Close()

		mockRepo := &mockRepository{
			getActiveCurrenciesFunc: func(ctx context.Context) ([]models.Currency, error) {
				// No currency in ALLOWED_PAIRS ("EURUSDT", "GBPUSDT")
				return []models.Currency{
					{Id: 1, Code: "JPY", Active: true, LastKnownRateToDollars: 0.007},
				}, nil
			},
		}

		service := &CurrencyService{
			repo:   mockRepo,
			logger: testLogger(),
			store:  make(map[string]models.Currency),
			env:    testEnv(httpSrv.URL, ""),
		}

		err := service.Init(context.Background(), make(chan CurrencyRequest))
		if !errors.Is(err, errs.ERR_EMPTY_CURRENCY_LIST) {
			t.Fatalf("expected ERR_EMPTY_CURRENCY_LIST, got %v", err)
		}
	})
}

// ----------------------------------------------------------------------------
// Messenger tests
// ----------------------------------------------------------------------------

func TestMakeCurrencyRequest(t *testing.T) {
	t.Run("returns ctx.Err when context is cancelled beforehand", func(t *testing.T) {
		ch := make(chan CurrencyRequest)
		messenger := &CurrencyServiceMessenger{messageChan: ch}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := messenger.MakeCurrencyRequest(ctx, "EUR", "GBP")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got: %v", err)
		}
	})

	t.Run("returns timeout when request channel is full/unresponsive", func(t *testing.T) {
		// Unbuffered channel with nobody reading
		ch := make(chan CurrencyRequest)
		messenger := &CurrencyServiceMessenger{messageChan: ch}

		// Test using a short timeout context to prevent long test run
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := messenger.MakeCurrencyRequest(ctx, "EUR", "GBP")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context.DeadlineExceeded, got: %v", err)
		}
	})

	t.Run("returns response when actor handles request successfully", func(t *testing.T) {
		ch := make(chan CurrencyRequest, 1)
		messenger := &CurrencyServiceMessenger{messageChan: ch}

		go func() {
			req := <-ch
			req.respChan <- CurrencyResponse{
				Rate:       1.25,
				DollarRate: 1.08,
				Err:        nil,
			}
		}()

		resp, err := messenger.MakeCurrencyRequest(context.Background(), "EUR", "GBP")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Rate != 1.25 || resp.DollarRate != 1.08 {
			t.Errorf("unexpected response content: %+v", resp)
		}
	})
}

// ----------------------------------------------------------------------------
// Shutdown tests
// ----------------------------------------------------------------------------

func TestShutdown(t *testing.T) {
	t.Run("flushes currencies to repository", func(t *testing.T) {
		var savedCurrencies []models.Currency
		mockRepo := &mockRepository{
			updateCurrenciesFunc: func(ctx context.Context, currencies []models.Currency) error {
				savedCurrencies = currencies
				return nil
			},
		}

		service := &CurrencyService{
			repo:   mockRepo,
			logger: testLogger(),
			store: map[string]models.Currency{
				"EUR": {Id: 1, Code: "EUR", LastKnownRateToDollars: 1.09},
				"GBP": {Id: 2, Code: "GBP", LastKnownRateToDollars: 1.28},
			},
		}

		err := service.Shutdown(context.Background())
		if err != nil {
			t.Fatalf("Shutdown failed: %v", err)
		}

		if len(savedCurrencies) != 2 {
			t.Fatalf("expected 2 currencies updated, got %d", len(savedCurrencies))
		}
	})

	t.Run("handles nil repo gracefully", func(t *testing.T) {
		service := &CurrencyService{
			repo:   nil,
			logger: testLogger(),
			store:  make(map[string]models.Currency),
		}

		err := service.Shutdown(context.Background())
		if err != nil {
			t.Fatalf("expected nil error on nil repo, got: %v", err)
		}
	})

	t.Run("propagates repository update error", func(t *testing.T) {
		expectedErr := errors.New("database update error")
		mockRepo := &mockRepository{
			updateCurrenciesFunc: func(ctx context.Context, currencies []models.Currency) error {
				return expectedErr
			},
		}

		service := &CurrencyService{
			repo:   mockRepo,
			logger: testLogger(),
			store: map[string]models.Currency{
				"EUR": {Id: 1, Code: "EUR", LastKnownRateToDollars: 1.09},
			},
		}

		err := service.Shutdown(context.Background())
		if !errors.Is(err, expectedErr) {
			t.Fatalf("expected %v, got %v", expectedErr, err)
		}
	})
}

// ----------------------------------------------------------------------------
// InitCurrencyService tests
// ----------------------------------------------------------------------------

func TestInitCurrencyService(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	t.Run("successful initialization returns messenger", func(t *testing.T) {
		httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := BinanceResponse{
				Symbols: []SimpleSymbol{
					{Symbol: "EURUSDT", Status: "TRADING"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer httpSrv.Close()

		wsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
		}))
		defer wsSrv.Close()

		wsURL := "ws" + strings.TrimPrefix(wsSrv.URL, "http") + "/stream"
		env := testEnv(httpSrv.URL, wsURL)

		mockRepo := &mockRepository{
			getActiveCurrenciesFunc: func(ctx context.Context) ([]models.Currency, error) {
				return []models.Currency{
					{Id: 1, Code: "EUR", Active: true, LastKnownRateToDollars: 1.08},
				}, nil
			},
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		messenger, err := InitCurrencyService(ctx, mockRepo, env, testLogger())
		if err != nil {
			t.Fatalf("InitCurrencyService failed: %v", err)
		}
		if messenger == nil {
			t.Fatal("expected non-nil messenger")
		}
	})

	t.Run("returns error when repo fails", func(t *testing.T) {
		repoErr := errors.New("db failure")
		mockRepo := &mockRepository{
			getActiveCurrenciesFunc: func(ctx context.Context) ([]models.Currency, error) {
				return nil, repoErr
			},
		}

		_, err := InitCurrencyService(context.Background(), mockRepo, testEnv("", ""), testLogger())
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected %v, got %v", repoErr, err)
		}
	})
}

// ----------------------------------------------------------------------------
// Concurrent Race Safety Test
// ----------------------------------------------------------------------------

func TestActorLoopConcurrentRaceSafety(t *testing.T) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := BinanceResponse{
			Symbols: []SimpleSymbol{
				{Symbol: "EURUSDT", Status: "TRADING"},
				{Symbol: "GBPUSDT", Status: "TRADING"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer httpSrv.Close()

	wsConnChan := make(chan *websocket.Conn, 1)
	wsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		wsConnChan <- conn
	}))
	defer wsSrv.Close()

	wsURL := "ws" + strings.TrimPrefix(wsSrv.URL, "http") + "/stream"
	env := testEnv(httpSrv.URL, wsURL)

	mockRepo := &mockRepository{
		getActiveCurrenciesFunc: func(ctx context.Context) ([]models.Currency, error) {
			return []models.Currency{
				{Id: 1, Code: "EUR", Active: true, LastKnownRateToDollars: 1.10},
				{Id: 2, Code: "GBP", Active: true, LastKnownRateToDollars: 1.30},
			}, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reqChan := make(chan CurrencyRequest, 100)
	service := &CurrencyService{
		repo:   mockRepo,
		logger: testLogger(),
		store:  make(map[string]models.Currency),
		env:    env,
	}

	err := service.Init(ctx, reqChan)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	messenger := &CurrencyServiceMessenger{messageChan: reqChan}

	var wsConn *websocket.Conn
	select {
	case wsConn = <-wsConnChan:
		defer wsConn.Close()
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ws client")
	}

	var wg sync.WaitGroup
	var wsWriteMu sync.Mutex
	numWriters := 3
	numReaders := 10
	iterations := 50

	// Concurrent WebSocket writers pushing trade updates
	for w := range numWriters {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := range iterations {
				price := fmt.Sprintf("%.4f", 1.05+float64(i)*0.001)
				trade := BinanceStreamMessage{
					Stream: "eurusdt@trade",
					Data:   BinanceTrade{Symbol: "EURUSDT", Price: price},
				}
				b, _ := json.Marshal(trade)
				wsWriteMu.Lock()
				_ = wsConn.WriteMessage(websocket.TextMessage, b)
				wsWriteMu.Unlock()
				time.Sleep(2 * time.Millisecond)
			}
		}(w)
	}

	// Concurrent Messenger readers making rate requests
	for r := range numReaders {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			for range iterations {
				resp, err := messenger.MakeCurrencyRequest(ctx, "EUR", "GBP")
				if err != nil {
					t.Errorf("concurrent MakeCurrencyRequest error: %v", err)
					return
				}
				if resp.Err != nil {
					t.Errorf("concurrent resp.Err: %v", resp.Err)
					return
				}
				if resp.Rate <= 0 || resp.DollarRate <= 0 {
					t.Errorf("invalid rate returned: %+v", resp)
					return
				}
				time.Sleep(2 * time.Millisecond)
			}
		}(r)
	}

	wg.Wait()
}
