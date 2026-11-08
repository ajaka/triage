package common

import (
	"log/slog"

	"github.com/0mjs/zinc"
)

func GetLogger(c *zinc.Context) *slog.Logger {
	if l, ok := c.Get("logger"); ok {
		if logger, ok := l.(*slog.Logger); ok && logger != nil {
			return logger
		}
	}
	return slog.Default()
}
