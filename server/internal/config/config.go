package config

import (
	"errors"
	"os"
)

var (
	DB_URL     string
	JWT_SECRET string
)

func InitEnvValues() error {
	DB_URL = os.Getenv("DB_URL")
	JWT_SECRET = os.Getenv("JWT_SECRET")

	if DB_URL == "" {
		return errors.New("DB_URL is not set in .env file")
	}

	if JWT_SECRET == "" {
		return errors.New("JWT_SECRET is not set in .env file")
	}

	return nil
}
