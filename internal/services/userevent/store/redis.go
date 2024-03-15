package store

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"snwzt/rvc/internal/models"
	"time"
)

type Store interface {
	DequeueMatchRequest() (*models.MatchRequest, error)
	ValidateMatch(*models.MatchRequest) bool
	CreateMatch(*models.MatchRequest) (*models.Match, error)
	EnqueueForwarderRequest(*models.Match) error

	DequeueRemoveUserRequest() (string, error)
	Cleanup(string) error
	RemoveUserEntry(string) error
}

type Storage struct {
	Redis *redis.Client
}

func (s *Storage) DequeueMatchRequest() (*models.MatchRequest, error) {
	matchRequestJSON, err := s.Redis.BRPop(context.Background(), 60*time.Second, "match_request_queue").Result()
	if err != nil {
		return nil, err
	}

	var match models.MatchRequest

	if err := json.Unmarshal([]byte(matchRequestJSON[1]), &match); err != nil {
		return nil, err
	}

	return &match, nil
}

func (s *Storage) ValidateMatch(matchRequest *models.MatchRequest) bool {
	return s.Redis.SIsMember(context.Background(), "unpaired_pool", matchRequest.UserID1).Val() ||
		s.Redis.SIsMember(context.Background(), "unpaired_pool", matchRequest.UserID2).Val()
}

func (s *Storage) CreateMatch(matchRequest *models.MatchRequest) (*models.Match, error) {
	match := models.Match{
		ID:      fmt.Sprintf("match:%s", uuid.New().String()),
		UserID1: matchRequest.UserID1,
		UserID2: matchRequest.UserID2,
	}

	if err := s.Redis.SRem(context.Background(), "unpaired_pool", match.UserID1).Err(); err != nil {
		return nil, err
	}

	if err := s.Redis.SRem(context.Background(), "unpaired_pool", match.UserID2).Err(); err != nil {
		return nil, err
	}

	if err := s.Redis.HSet(context.Background(), fmt.Sprintf("match:%s", match.ID),
		"user1", match.UserID1, "user2", match.UserID2).Err(); err != nil {
		return nil, err
	}

	if err := s.Redis.HSet(context.Background(), fmt.Sprintf("user_entry:%s", match.UserID1),
		"match_id", match.ID).Err(); err != nil {
		return nil, err
	}

	if err := s.Redis.HSet(context.Background(), fmt.Sprintf("user_entry:%s", match.UserID2),
		"match_id", match.ID).Err(); err != nil {
		return nil, err
	}

	return &match, nil
}

func (s *Storage) EnqueueForwarderRequest(match *models.Match) error {
	matchJSON, err := json.Marshal(match)
	if err != nil {
		return err
	}

	return s.Redis.LPush(context.Background(), "create_forwarder_queue", matchJSON).Err()
}

func (s *Storage) DequeueRemoveUserRequest() (string, error) {
	user, err := s.Redis.BRPop(context.Background(), 60*time.Second, "remove_user_request_queue").Result()
	if err != nil {
		return "", err
	}

	return user[1], nil
}

func (s *Storage) Cleanup(userID string) error {
	matchID, err := s.Redis.HGet(context.Background(), fmt.Sprintf("user_entry:%s", userID), "match_id").Result()
	if err != nil {
		return err
	}

	if !s.Redis.SIsMember(context.Background(), "unpaired_pool", userID).Val() {
		// delete forwarder
		if err := s.Redis.Publish(context.Background(), "delete_match_channel", matchID).Err(); err != nil {
			return err
		}
	}

	// delete match entry
	if err := s.Redis.Del(context.Background(), fmt.Sprintf("match:%s", matchID)).Err(); err != nil {
		return err
	}

	// remove from unpaired pool
	if err := s.Redis.SRem(context.Background(), "unpaired_pool", userID).Err(); err != nil {
		return err
	}

	return nil
}

func (s *Storage) RemoveUserEntry(userID string) error {
	return s.Redis.Del(context.Background(), fmt.Sprintf("user_entry:%s", userID)).Err()
}
