package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"snwzt/rvc/internal/models"
	"snwzt/rvc/internal/services/forwarder/store"
	"sync"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type ServerHandler interface {
	CreateForwarder(context.Context)
	DeleteForwarder(context.Context)
}

type ServerHandle struct {
	Store           store.Store
	Logger          *zerolog.Logger
	CancelForwarder chan string
}

func (h *ServerHandle) CreateForwarder(ctx context.Context) {
	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			wg.Wait() // draining mode

			return
		default:
			match, err := h.Store.DequeueForwarderRequest()
			if err != nil {
				if !errors.Is(err, redis.Nil) {
					h.Logger.Err(err).Msg("unable to dequeue from create forwarder queue")
				}
				continue
			}

			wg.Add(1)
			go forwarder(h.CancelForwarder, match, h.Store, h.Logger, &wg)
		}
	}
}

func (h *ServerHandle) DeleteForwarder(ctx context.Context) {
	listener := h.Store.SubscribeDeleteForwarder()
	defer func(listener *redis.PubSub) {
		err := listener.Close()
		if err != nil {
			h.Logger.Err(err).Msg("failed to properly remove subscription for delete forwarder")
			return
		}
	}(listener)

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-listener.Channel():
			h.Logger.Info().Msg("received cancellation request for " + msg.Payload)
			h.CancelForwarder <- msg.Payload
		}
	}
}

func forwarder(cancel chan string, match models.Match, store store.Store, logger *zerolog.Logger, wg *sync.WaitGroup) {
	defer wg.Done()

	User1Inc := match.UserID1 + ":incoming"
	User1Out := match.UserID1 + ":outgoing"
	User2Inc := match.UserID2 + ":incoming"
	User2Out := match.UserID2 + ":outgoing"

	user1Out := store.ListenOutgoing(User1Out)
	user2Out := store.ListenOutgoing(User2Out)

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

	msg1, err := store.GetExchange(match.UserID1, true)
	if err != nil {
		logger.Err(err).Msg("unable to create exchange for user: " + match.UserID1)
		return
	}

	msg2, err := store.GetExchange(match.UserID2, false)
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

	if err := store.WriteMessage(User1Inc, msgJSON2); err != nil {
		logger.Err(err).Msg("unable to write to user1inc")
	}

	if err := store.WriteMessage(User2Inc, msgJSON1); err != nil {
		logger.Err(err).Msg("unable to write to user2inc")
	}

	logger.Info().Msg(fmt.Sprintf("created %s for %s %s", match.ID,
		match.UserID1, match.UserID2))

	// user1out -> user2inc
	// user2out -> user1inc
	for {
		select {
		case <-cancel:
			logger.Info().Msg("forwarder " + match.ID + " has been removed")
			return

		case msg, ok := <-user1Out.Channel():
			if !ok {
				logger.Info().Msg("chat channel " + User1Out + " closed unexpectedly")
				continue
			}

			if err := store.WriteMessage(User2Inc, msg.Payload); err != nil {
				logger.Err(err).Msg("unable to publish to user2inc")
			}

		case msg, ok := <-user2Out.Channel():
			if !ok {
				logger.Info().Msg("chat channel " + User2Out + " closed unexpectedly")
				return
			}

			if err := store.WriteMessage(User1Inc, msg.Payload); err != nil {
				logger.Err(err).Msg("unable to publish to user1inc")
			}
		}
	}
}
