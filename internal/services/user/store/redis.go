package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"snwzt/rvc/internal/models"
	"time"
)

type Store interface {
	AddUser(*models.UserEntry) error
	UnpairUsers(...string) error
	RemoveExistingMatch(string) error
	GetMatchCandidate(string) (string, error)
	EnqueueMatchRequest(string, string) error
}

type Storage struct {
	Redis *redis.Client
}

func (s *Storage) AddUser(user *models.UserEntry) error {
	return s.Redis.HSet(context.Background(), fmt.Sprintf("user_entry:%s", user.ID),
		"username", user.Username, "ip_addr", user.IPAddr, "match_id", user.MatchID).Err()
}

func (s *Storage) UnpairUsers(users ...string) error {
	for _, user := range users {
		err := s.Redis.SAdd(context.Background(), "unpaired_pool", user).Err()
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Storage) RemoveExistingMatch(userID string) error {
	matchID, err := s.Redis.HGet(context.Background(), fmt.Sprintf("user_entry:%s", userID), "match_id").Result()
	if err != nil {
		return err
	}

	if matchID != "" {
		users, err := s.Redis.HGetAll(context.Background(), fmt.Sprintf("match:%s", matchID)).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return err
		}

		if !errors.Is(err, redis.Nil) {
			for _, user := range users {
				// add to unpaired pool
				if err := s.Redis.SAdd(context.Background(), "unpaired_pool", user).Err(); err != nil {
					return err
				}
			}

			// delete match entry
			if err := s.Redis.Del(context.Background(), fmt.Sprintf("match:%s", matchID)).Err(); err != nil {
				return err
			}

			// delete match session
			if err := s.Redis.Publish(context.Background(), "delete_match_channel", matchID).Err(); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Storage) GetMatchCandidate(userID string) (string, error) {
	for attempt := 1; attempt <= 5; attempt++ {
		candidate, err := s.Redis.SRandMemberN(context.Background(), "unpaired_pool", 1).Result()
		if err != nil {
			return "", err
		}

		if userID == candidate[0] {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
			continue
		}

		return candidate[0], nil
	}

	return "", nil
}

func (s *Storage) EnqueueMatchRequest(userID1 string, userID2 string) error {
	matchJSON, err := json.Marshal(&models.MatchRequest{UserID1: userID1, UserID2: userID2})
	if err != nil {
		return err
	}

	return s.Redis.LPush(context.Background(), "match_request_queue", matchJSON).Err()
}
