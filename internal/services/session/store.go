package session

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"rvc/internal/models"
	"time"
)

type Store interface {
	// create session

	dequeueCreateSessionRequest(context.Context) (models.Match, error)
	listenIncoming(context.Context, string) *redis.PubSub
	listenOutgoing(context.Context, string) *redis.PubSub
	getExchange(context.Context, string, bool) (*models.Message, error)
	writeMessage(context.Context, string, interface{}) error

	// delete session

	listenDeleteSession(context.Context) *redis.PubSub
}

type Storage struct {
	RedisClient *redis.Client
}

func (s *Storage) dequeueCreateSessionRequest(ctx context.Context) (models.Match, error) {
	matchJSON, err := s.RedisClient.BRPop(ctx, 60*time.Second, "create_session_queue").Result()
	if err != nil {
		return models.Match{}, err
	}

	var match models.Match

	if err := json.Unmarshal([]byte(matchJSON[1]), &match); err != nil {
		return models.Match{}, err
	}

	return match, nil
}

func (s *Storage) listenIncoming(ctx context.Context, channel string) *redis.PubSub {
	return s.RedisClient.Subscribe(ctx, channel)
}

func (s *Storage) listenOutgoing(ctx context.Context, channel string) *redis.PubSub {
	return s.RedisClient.Subscribe(ctx, channel)
}

func (s *Storage) getExchange(ctx context.Context, user string, initiator bool) (*models.Message, error) {
	username, err := s.RedisClient.HGet(ctx, fmt.Sprintf("user_entry:%s", user), "username").Result()
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

func (s *Storage) writeMessage(ctx context.Context, channel string, msg interface{}) error {
	return s.RedisClient.Publish(ctx, channel, msg).Err()
}

func (s *Storage) listenDeleteSession(ctx context.Context) *redis.PubSub {
	return s.RedisClient.Subscribe(ctx, "delete_match_session")
}
