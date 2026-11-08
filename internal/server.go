package internal

import (
	"log/slog"
	"os"

	"github.com/0mjs/zinc"
	"github.com/0mjs/zinc/middleware/logger"
	"github.com/0mjs/zinc/middleware/recover"
	"github.com/0mjs/zinc/middleware/requestid"
	"github.com/ajaka/triage/internal/common"
	"github.com/ajaka/triage/internal/middlewares"
)

func Listen() error {
	baseLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	app := zinc.New()

	app.Use(
		requestid.New(),
		middlewares.CreateAndAttachScopedLogger(baseLogger),
		logger.New(logger.Config{
			Log: func(c *zinc.Context, v logger.Values) error {
				logger := common.GetLogger(c)
				logger.Info("Request Ended",
					"method", v.Method,
					"route", v.RoutePath,
					"status", v.Status,
					"latency", v.Latency,
				)
				return nil
			},
		}),
		recover.New(),
	)
	app.Get("/", func(c *zinc.Context) error {
		logger := common.GetLogger(c)
		logger.Info("Yep!")
		return c.Status(zinc.StatusOK).JSON(zinc.Map{"success": true})
	})
	return app.Listen(":8080")
}
