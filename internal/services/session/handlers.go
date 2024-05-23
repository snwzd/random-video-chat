package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"rvc/internal/models"
	"sync"
)

type ServerHandler interface {
	createSession(context.Context)
	deleteSession(context.Context)
}

type ServerHandle struct {
	Store  Store
	Logger *zerolog.Logger

	Goroutines map[string]context.CancelFunc
	mu         sync.RWMutex
}

func (h *ServerHandle) createSession(ctx context.Context) {
	localCtx := context.Background()

	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			wg.Wait() // draining mode

			return
		default:
			match, err := h.Store.dequeueCreateSessionRequest(localCtx)
			if err != nil {
				if !errors.Is(err, redis.Nil) {
					h.Logger.Err(err).Msg("unable to dequeue from create session queue")
				}
				continue
			}

			wg.Add(1)

			ctx, cancel := context.WithCancel(context.Background())

			h.mu.Lock()
			h.Goroutines[match.MatchID] = cancel
			h.mu.Unlock()

			go session(ctx, match, h.Store, h.Logger, &wg)
		}
	}
}

func (h *ServerHandle) deleteSession(ctx context.Context) {
	localCtx := context.Background()

	listener := h.Store.listenDeleteSession(localCtx)
	defer func() {
		err := listener.Close()
		if err != nil {
			h.Logger.Err(err).Msg("failed to properly remove subscription for delete session")
			return
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-listener.Channel():
			h.Logger.Info().Msg("received cancellation request for " + msg.Payload)

			h.mu.Lock()
			if cancelFunc, ok := h.Goroutines[msg.Payload]; ok {
				cancelFunc()
				delete(h.Goroutines, msg.Payload)
			}
			h.mu.Unlock()
		}
	}
}

func session(ctx context.Context, match models.Match, store Store, logger *zerolog.Logger, wg *sync.WaitGroup) {
	defer wg.Done()

	localCtx := context.Background()

	User1Inc := match.UserID1 + ":incoming"
	User1Out := match.UserID1 + ":outgoing"
	User2Inc := match.UserID2 + ":incoming"
	User2Out := match.UserID2 + ":outgoing"

	user1Out := store.listenOutgoing(localCtx, User1Out)
	user2Out := store.listenOutgoing(localCtx, User2Out)

	defer func() {
		if err := user1Out.Close(); err != nil {
			logger.Err(err).Msg("failed to properly remove subscription " + User1Out)
			return
		}
		if err := user2Out.Close(); err != nil {
			logger.Err(err).Msg("failed to properly remove subscription " + User2Out)
			return
		}
	}()

	msg1, err := store.getExchange(localCtx, match.UserID1, true)
	if err != nil {
		logger.Err(err).Msg("unable to create exchange for user: " + match.UserID1)
		return
	}

	msg2, err := store.getExchange(localCtx, match.UserID2, false)
	if err != nil {
		logger.Err(err).Msg("unable to create exchange for user: " + match.UserID2)
		return
	}

	msgJSON1, err := json.Marshal(msg1)
	if err != nil {
		logger.Err(err).Msg("unable to marshal message")
		return
	}

	msgJSON2, err := json.Marshal(msg2)
	if err != nil {
		logger.Err(err).Msg("unable to marshal message")
		return
	}

	if err := store.writeMessage(localCtx, User1Inc, msgJSON2); err != nil {
		logger.Err(err).Msg("unable to write to user1inc")
	}

	if err := store.writeMessage(localCtx, User2Inc, msgJSON1); err != nil {
		logger.Err(err).Msg("unable to write to user2inc")
	}

	logger.Info().Msg(fmt.Sprintf("created session %s for %s %s", match.MatchID,
		match.UserID1, match.UserID2))

	// user 1 out -> user 2 inc
	// user 2 out -> user 1 inc
	for {
		select {
		case <-ctx.Done():
			logger.Info().Msg("removed session " + match.MatchID)
			return

		case msg, ok := <-user1Out.Channel():
			if !ok {
				logger.Info().Msg("chat channel " + User1Out + " closed unexpectedly")
				continue
			}

			if err := store.writeMessage(localCtx, User2Inc, msg.Payload); err != nil {
				logger.Err(err).Msg("unable to publish to user2inc")
			}

		case msg, ok := <-user2Out.Channel():
			if !ok {
				logger.Info().Msg("chat channel " + User2Out + " closed unexpectedly")
				return
			}

			if err := store.writeMessage(localCtx, User1Inc, msg.Payload); err != nil {
				logger.Err(err).Msg("unable to publish to user1inc")
			}
		}
	}
}
