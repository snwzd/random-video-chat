package common

import "github.com/redis/go-redis/v9"

func NewRedisStore(url string) (*redis.Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}

	return redis.NewClient(opts), err
}
