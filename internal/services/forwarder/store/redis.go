package store

import (
	"context"
	"encoding/json"
	"fmt"
	"snwzt/rvc/internal/models"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store interface {
	DequeueForwarderRequest() (models.Match, error)
	ListenIncoming(string) *redis.PubSub
	ListenOutgoing(string) *redis.PubSub
	GetExchange(string, bool) (*models.Message, error)
	WriteMessage(string, interface{}) error

	SubscribeDeleteForwarder() *redis.PubSub
}

type Storage struct {
	Redis *redis.Client
}

func (s *Storage) DequeueForwarderRequest() (models.Match, error) {
	matchJSON, err := s.Redis.BRPop(context.Background(), 60*time.Second, "create_forwarder_queue").Result()
	if err != nil {
		return models.Match{}, err
	}

	var match models.Match

	if err := json.Unmarshal([]byte(matchJSON[1]), &match); err != nil {
		return models.Match{}, err
	}

	return match, nil
}

func (s *Storage) ListenIncoming(channel string) *redis.PubSub {
	return s.Redis.Subscribe(context.Background(), channel)
}

func (s *Storage) ListenOutgoing(channel string) *redis.PubSub {
	return s.Redis.Subscribe(context.Background(), channel)
}

func (s *Storage) GetExchange(user string, initiator bool) (*models.Message, error) {
	username, err := s.Redis.HGet(context.Background(), fmt.Sprintf("user_entry:%s", user), "username").Result()
	if err != nil {
		return nil, err
	}

	return &models.Message{
		Event: "exchange",
		Data: &models.Exchange{
			Username:  username,
			Initiator: initiator,
		},
	}, nil
}

func (s *Storage) WriteMessage(channel string, msg interface{}) error {
	return s.Redis.Publish(context.Background(), channel, msg).Err()
}

func (s *Storage) SubscribeDeleteForwarder() *redis.PubSub {
	return s.Redis.Subscribe(context.Background(), "delete_match_channel")
}
