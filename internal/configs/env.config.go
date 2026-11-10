package configs

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
)

type Env struct {
	PORT                               string `validate:"required"`
	ORGANIZATION_SECRET_ENCRYPTION_KEY string `validate:"required"`
	EMAIL_JWT_SECRET_KEY               string `validate:"required"`
	DATABASE_URL                       string `validate:"required"`
}

func LoadEnv(logger *slog.Logger) (*Env, error) {
	err := godotenv.Load()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			logger.Warn("Could not find enviroment vars, proceeding with system vars")
		} else {
			return nil, err
		}
	}

	env := Env{
		PORT:                               os.Getenv("PORT"),
		ORGANIZATION_SECRET_ENCRYPTION_KEY: os.Getenv("ORGANIZATION_SECRET_ENCRYPTION_KEY"),
		EMAIL_JWT_SECRET_KEY:               os.Getenv("EMAIL_JWT_SECRET_KEY"),
		DATABASE_URL:                       os.Getenv("DATABASE_URL"),
	}

	v := validator.New()
	if err := v.Struct(&env); err != nil {
		return nil, err
	}

	logger.Info("Successfully loaded and validated env")
	return &env, nil
}
