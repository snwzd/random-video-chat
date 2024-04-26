package handlers

import (
	"context"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

var Upgrade = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ServerHandler interface {
	CheckHealth(echo.Context) error
	Connection(echo.Context) error
}

type ServerHandle struct {
	Redis  *redis.Client
	Logger *zerolog.Logger
	Ctx    context.Context
}

func (h *ServerHandle) CheckHealth(c echo.Context) error {
	return c.String(http.StatusOK, "healthy")
}

func (h *ServerHandle) Connection(c echo.Context) error {
	userID := c.Param("id")
	if userID == "" {
		return echo.NewHTTPError(http.StatusBadRequest)
	}

	ws, err := Upgrade.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		h.Logger.Err(err).Msg("unable to upgrade to websocket")
		return echo.NewHTTPError(http.StatusBadRequest, "unable to upgrade to websocket")
	}

	defer func(ws *websocket.Conn) {
		err := ws.Close()
		if err != nil {
			h.Logger.Err(err).Msg("unable to close websocket properly: " + userID)
			return
		}
	}(ws)

	h.Logger.Info().Msg("established websocket conn " + userID)

	outMsgChannel := userID + ":outgoing"
	incMsgChannel := userID + ":incoming"

	var wg sync.WaitGroup

	localCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// userSource -> userTarget (outgoing for source)
	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			select {
			case <-h.Ctx.Done():
				return
			default:
				msgType, message, err := ws.ReadMessage()

				if err != nil && msgType != -1 {
					h.Logger.Err(err).Msg("unable to read from websocket")
				}

				if msgType == -1 {
					cancel()

					if err := h.Redis.LPush(context.Background(), "remove_user_request_queue", userID).Err(); err != nil {
						h.Logger.Err(err).Msg("unable to enqueue to remove user request queue")
					}

					h.Logger.Info().Msg("closed websocket conn " + userID)

					return
				}

				if err := h.Redis.Publish(context.Background(), outMsgChannel, message).Err(); err != nil {
					h.Logger.Err(err).Msg("unable to publish to " + outMsgChannel)
				}
			}
		}
	}()

	// userTarget -> userSource (incoming for source)
	wg.Add(1)
	go func() {
		defer wg.Done()

		pubSub := h.Redis.Subscribe(context.Background(), incMsgChannel)

		defer func(pubSub *redis.PubSub) {
			err := pubSub.Close()
			if err != nil {
				h.Logger.Err(err).Msg("unable to close redis channel properly: " + incMsgChannel)
				return
			}
		}(pubSub)

		for {
			select {
			case <-h.Ctx.Done():
				return
			case <-localCtx.Done():
				return
			default:
				msg, err := pubSub.ReceiveMessage(context.Background())
				if err != nil {
					h.Logger.Err(err).Msg("unable to get message from " + incMsgChannel)
				}

				if err := ws.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
					h.Logger.Err(err).Msg("unable to write to websocket")
				}
			}
		}
	}()

	wg.Wait()

	return nil
}
