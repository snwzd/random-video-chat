package handlers

import (
	"context"
	"errors"
	"snwzt/rvc/internal/services/userevent/store"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type ServerHandler interface {
	Match(context.Context)
	Remove(context.Context)
}

type ServerHandle struct {
	Store  store.Store
	Logger *zerolog.Logger
}

func (h *ServerHandle) Match(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			matchRequest, err := h.Store.DequeueMatchRequest()
			if err != nil {
				if !errors.Is(err, redis.Nil) {
					h.Logger.Err(err).Msg("unable to dequeue from matchRequest queue")
				}
				continue
			}

			if !h.Store.ValidateMatch(matchRequest) {
				continue
			}

			match, err := h.Store.CreateMatch(matchRequest)
			if err != nil {
				h.Logger.Err(err).Msg("unable to create match model")
				continue
			}

			if err := h.Store.EnqueueForwarderRequest(match); err != nil {
				h.Logger.Err(err).Msg("unable to enqueue to match queue")
				continue
			}

			h.Logger.Info().Msg("matched " + match.UserID1 + " " + match.UserID2)
		}
	}
}

func (h *ServerHandle) Remove(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			userID, err := h.Store.DequeueRemoveUserRequest()
			if err != nil {
				if !errors.Is(err, redis.Nil) {
					h.Logger.Err(err).Msg("unable to dequeue from user remove request queue")
				}
				continue
			}

			h.Logger.Info().Msg("received request to remove user " + userID)

			if err := h.Store.Cleanup(userID); err != nil {
				h.Logger.Err(err).Msg("unable to cleanup user: " + userID)
				continue
			}

			if err := h.Store.RemoveUserEntry(userID); err != nil {
				h.Logger.Err(err).Msg("unable to remove user entry: " + userID)
				continue
			}

			h.Logger.Info().Msg("removed user " + userID)
		}
	}
}
