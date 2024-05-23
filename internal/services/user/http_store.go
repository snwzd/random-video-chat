package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"rvc/internal/models"
	"time"
)

type HttpStore interface {
	// User: User related operations

	addUserEntry(context.Context, *models.User) error
	removeUserEntry(context.Context, string) error
	cleanupUserEntry(context.Context, string) error
	addToUnpairedPool(context.Context, ...string) error
	removeExistingMatch(context.Context, string) error
	getMatchCandidate(context.Context, string) (string, error)
	enqueueMatchRequest(context.Context, string, string) error

	// Chat: Needed for chat operations

	outgoingMessage(context.Context, string, []byte) error
	incomingMessage(context.Context, string) *redis.PubSub
}

type HttpStorage struct {
	RedisClient *redis.Client
}

// User

func (s *HttpStorage) addUserEntry(ctx context.Context, user *models.User) error {
	return s.RedisClient.HSet(ctx, fmt.Sprintf("user_entry:%s", user.UserID),
		"username", user.Username, "ip_addr", user.IPAddr, "match_id", user.MatchID).Err()
}

func (s *HttpStorage) removeUserEntry(ctx context.Context, userID string) error {
	return s.RedisClient.Del(ctx, fmt.Sprintf("user_entry:%s", userID)).Err()
}

func (s *HttpStorage) cleanupUserEntry(ctx context.Context, userID string) error {
	matchID, err := s.RedisClient.HGet(context.Background(), fmt.Sprintf("user_entry:%s", userID), "match_id").Result()
	if err != nil {
		return err
	}

	if !s.RedisClient.SIsMember(context.Background(), "unpaired_pool", userID).Val() {
		// publish request on delete_match_session
		if err := s.RedisClient.Publish(context.Background(), "delete_match_session", matchID).Err(); err != nil {
			return err
		}
	}

	// delete match_entry
	if err := s.RedisClient.Del(context.Background(), fmt.Sprintf("match_entry:%s", matchID)).Err(); err != nil {
		return err
	}

	// remove from unpaired_pool
	if err := s.RedisClient.SRem(context.Background(), "unpaired_pool", userID).Err(); err != nil {
		return err
	}

	return nil
}

func (s *HttpStorage) addToUnpairedPool(ctx context.Context, users ...string) error {
	for _, user := range users {
		err := s.RedisClient.SAdd(ctx, "unpaired_pool", user).Err()
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *HttpStorage) removeExistingMatch(ctx context.Context, userID string) error {
	matchID, err := s.RedisClient.HGet(ctx, fmt.Sprintf("user_entry:%s", userID), "match_id").Result()
	if err != nil {
		return err
	}

	if matchID != "" {
		users, err := s.RedisClient.HGetAll(ctx, fmt.Sprintf("match_entry:%s", matchID)).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return err
		}

		if !errors.Is(err, redis.Nil) {
			for _, user := range users {
				// add user to unpaired_pool
				if err := s.RedisClient.SAdd(ctx, "unpaired_pool", user).Err(); err != nil {
					return err
				}
			}

			// delete match_entry
			if err := s.RedisClient.Del(ctx, fmt.Sprintf("match_entry:%s", matchID)).Err(); err != nil {
				return err
			}

			// publish request on delete_match_session
			if err := s.RedisClient.Publish(ctx, "delete_match_session", matchID).Err(); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *HttpStorage) getMatchCandidate(ctx context.Context, userID string) (string, error) {
	for attempt := 1; attempt <= 5; attempt++ {
		setSize, err := s.RedisClient.SCard(ctx, "unpaired_pool").Result()
		if err != nil {
			return "", err
		}

		if setSize < 1 { // set not large enough
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
			continue
		}

		candidate, err := s.RedisClient.SRandMemberN(ctx, "unpaired_pool", 1).Result()
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

func (s *HttpStorage) enqueueMatchRequest(ctx context.Context, userID1 string, userID2 string) error {
	matchJSON, err := json.Marshal(&models.MatchRequest{UserID1: userID1, UserID2: userID2})
	if err != nil {
		return err
	}

	return s.RedisClient.LPush(ctx, "match_request_queue", matchJSON).Err()
}

// Chat

func (s *HttpStorage) outgoingMessage(ctx context.Context, userID string, message []byte) error {
	return s.RedisClient.Publish(ctx, userID+":outgoing", message).Err()
}

func (s *HttpStorage) incomingMessage(ctx context.Context, userID string) *redis.PubSub {
	return s.RedisClient.Subscribe(ctx, userID+":incoming")
}
