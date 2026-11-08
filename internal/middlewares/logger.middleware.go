package middlewares

import (
	"log/slog"

	"github.com/0mjs/zinc"
	"github.com/0mjs/zinc/middleware/requestid"
)

func CreateAndAttachScopedLogger(base *slog.Logger) zinc.HandlerFunc {
	return func(c *zinc.Context) error {
		reqId := requestid.Get(c)
		logger := base.With("reqId", reqId)
		logger.Info("Request started")
		c.Set("logger", logger)
		return c.Next()
	}
}
