package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/0mjs/zinc"
	"github.com/0mjs/zinc/middleware/logger"
	"github.com/0mjs/zinc/middleware/recover"
	"github.com/0mjs/zinc/middleware/requestid"
	"github.com/ajaka/triage/internal/common"
	"github.com/ajaka/triage/internal/configs"
	"github.com/ajaka/triage/internal/middlewares"
	"github.com/ajaka/triage/internal/repository"
	"github.com/ajaka/triage/internal/services/currency"
)

func Listen() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	baseLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	env, err := configs.LoadEnv(baseLogger)
	if err != nil {
		return err
	}

	pool, err := configs.ConnectToPostgres(ctx, baseLogger, env.DATABASE_URL)
	if err != nil {
		return err
	}

	repo := repository.InitRepository(pool)

	_, err = currency.InitCurrencyService(ctx, repo, env, baseLogger)
	if err != nil {
		return err
	}

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

	baseLogger.Info(fmt.Sprintf("Service listening on port %s", env.PORT))
	return app.Listen(env.PORT)
}
