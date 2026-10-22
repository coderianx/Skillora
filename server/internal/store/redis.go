package store

import (
	"context"

	"github.com/redis/go-redis/v9"
)

var RedisClient = redis.NewClient(&redis.Options{
	Addr:     "localhost:6379",
	Password: "",
	DB:       0,
})

func ConnectRedis() error {
	if err := RedisClient.Ping(context.Background()).Err(); err != nil {
		return err
	}

	return nil
}
