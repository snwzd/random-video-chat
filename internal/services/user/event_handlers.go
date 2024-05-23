package user

import (
	"context"
	"errors"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type EventServerHandler interface {
	Match(context.Context)
}

type EventServerHandle struct {
	Store  EventStore
	Logger *zerolog.Logger
}

func (h *EventServerHandle) Match(ctx context.Context) {
	localCtx := context.Background()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			matchRequest, err := h.Store.dequeueMatchRequest(localCtx)
			if err != nil {
				if !errors.Is(err, redis.Nil) {
					h.Logger.Err(err).Msg("unable to dequeue from matchRequest queue")
				}
				continue
			}

			if !h.Store.validateMatch(localCtx, matchRequest) {
				continue
			}

			match, err := h.Store.createMatchEntry(localCtx, matchRequest)
			if err != nil {
				h.Logger.Err(err).Msg("unable to create match model")
				continue
			}

			if err := h.Store.enqueueCreateSessionRequest(localCtx, match); err != nil {
				h.Logger.Err(err).Msg("unable to enqueue to match queue")
				continue
			}

			h.Logger.Info().Msg("matched " + match.UserID1 + " " + match.UserID2)
		}
	}
}
