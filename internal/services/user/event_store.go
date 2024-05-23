package user

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"rvc/internal/models"
	"strings"
	"time"
)

type EventStore interface {
	// Events: Event related operations

	dequeueMatchRequest(context.Context) (*models.MatchRequest, error)
	validateMatch(context.Context, *models.MatchRequest) bool
	createMatchEntry(context.Context, *models.MatchRequest) (*models.Match, error)
	enqueueCreateSessionRequest(context.Context, *models.Match) error
}

type EventStorage struct {
	RedisClient *redis.Client
}

// Event

func (s *EventStorage) dequeueMatchRequest(ctx context.Context) (*models.MatchRequest, error) {
	matchRequestJSON, err := s.RedisClient.BRPop(ctx, 60*time.Second, "match_request_queue").Result()
	if err != nil {
		return nil, err
	}

	var match models.MatchRequest

	if err := json.Unmarshal([]byte(matchRequestJSON[1]), &match); err != nil {
		return nil, err
	}

	return &match, nil
}

func (s *EventStorage) validateMatch(ctx context.Context, matchRequest *models.MatchRequest) bool {
	return s.RedisClient.SIsMember(ctx, "unpaired_pool", matchRequest.UserID1).Val() ||
		s.RedisClient.SIsMember(ctx, "unpaired_pool", matchRequest.UserID2).Val()
}

func (s *EventStorage) createMatchEntry(ctx context.Context, matchRequest *models.MatchRequest) (*models.Match, error) {
	match := models.Match{
		MatchID: matchRequest.UserID1 + "match" + matchRequest.UserID2 + "-" +
			strings.ReplaceAll(uuid.New().String(), "-", ""),
		UserID1: matchRequest.UserID1,
		UserID2: matchRequest.UserID2,
	}

	if err := s.RedisClient.SRem(ctx, "unpaired_pool", match.UserID1).Err(); err != nil {
		return nil, err
	}

	if err := s.RedisClient.SRem(ctx, "unpaired_pool", match.UserID2).Err(); err != nil {
		return nil, err
	}

	if err := s.RedisClient.HSet(ctx, fmt.Sprintf("match_entry:%s", match.MatchID),
		"user1", match.UserID1, "user2", match.UserID2).Err(); err != nil {
		return nil, err
	}

	if err := s.RedisClient.HSet(ctx, fmt.Sprintf("user_entry:%s", match.UserID1),
		"match_id", match.MatchID).Err(); err != nil {
		return nil, err
	}

	if err := s.RedisClient.HSet(ctx, fmt.Sprintf("user_entry:%s", match.UserID2),
		"match_id", match.MatchID).Err(); err != nil {
		return nil, err
	}

	return &match, nil
}

func (s *EventStorage) enqueueCreateSessionRequest(ctx context.Context, match *models.Match) error {
	matchJSON, err := json.Marshal(match)
	if err != nil {
		return err
	}

	return s.RedisClient.LPush(ctx, "create_session_queue", matchJSON).Err()
}
