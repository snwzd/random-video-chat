package user

import (
	"context"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"net/http"
	"os"
	"rvc/internal/models"
	"strings"
	"sync"
)

var Upgrade = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type HttpServerHandler interface {
	checkHealth(echo.Context) error

	// Home renders page
	home(echo.Context) error

	// Actions

	registerUser(echo.Context) error
	connection(echo.Context) error
	matchUser(echo.Context) error
}

type HttpServerHandle struct {
	SessionStore *sessions.CookieStore
	Logger       *zerolog.Logger
	Ctx          context.Context

	Store HttpStore
}

func (h *HttpServerHandle) checkHealth(c echo.Context) error {
	return c.String(http.StatusOK, "healthy")
}

func (h *HttpServerHandle) home(c echo.Context) error {
	return c.Render(http.StatusOK, "register.html", nil)
}

func (h *HttpServerHandle) registerUser(c echo.Context) error {
	username := c.FormValue("username")

	if username == "" {
		return echo.NewHTTPError(http.StatusBadRequest)
	}

	userID := username + "-" + strings.ReplaceAll(uuid.New().String(), "-", "")

	session, err := h.SessionStore.New(c.Request(), "random-video-chat-session")
	if err != nil {
		h.Logger.Err(err).Msg("unable to create session")
		return echo.NewHTTPError(http.StatusInternalServerError, "unable to create session")
	}

	session.Values["userID"] = userID

	if err := session.Save(c.Request(), c.Response()); err != nil {
		h.Logger.Err(err).Msg("unable to save session")
		return echo.NewHTTPError(http.StatusInternalServerError, "unable to save session")
	}

	//deadline := time.Now().Add(5 * time.Second)
	//ctx, cancel := context.WithDeadline(context.Background(), deadline)
	//defer cancel()

	ctx := context.Background()

	if err := h.Store.addUserEntry(ctx, &models.User{
		UserID:   userID,
		Username: username,
		IPAddr:   c.RealIP(),
		MatchID:  "",
	}); err != nil {
		h.Logger.Err(err).Msg("unable to add user entry")
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	if err := h.Store.addToUnpairedPool(ctx, userID); err != nil {
		h.Logger.Err(err).Msg("unable to add user to unpaired pool")
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	h.Logger.Info().Msg("registered new user: " + userID)

	WsAddr := "ws://" + c.Request().Host + "/connection/" + userID

	if os.Getenv("SECURE_FLAG") == "1" {
		WsAddr = "wss://" + c.Request().Host + "/connection/" + userID
	}

	TurnUrl := os.Getenv("TURN_URL")
	TurnUser := os.Getenv("TURN_USERNAME")
	TurnCred := os.Getenv("TURN_CRED")

	return c.Render(http.StatusOK, "chat", map[string]string{
		"WsAddr":   WsAddr,
		"TurnUrl":  TurnUrl,
		"TurnUser": TurnUser,
		"TurnCred": TurnCred,
	})
}

func (h *HttpServerHandle) connection(c echo.Context) error {
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

	var wg sync.WaitGroup

	ctx := context.Background()

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

					// cleanup user
					if err := h.Store.cleanupUserEntry(ctx, userID); err != nil {
						h.Logger.Err(err).Msg("unable to cleanup user: " + userID)
					}

					// remove user
					if err := h.Store.removeUserEntry(ctx, userID); err != nil {
						h.Logger.Err(err).Msg("unable to remove user: " + userID)
					}

					h.Logger.Info().Msg("closed websocket conn " + userID)

					return
				}

				if err := h.Store.outgoingMessage(ctx, userID, message); err != nil {
					h.Logger.Err(err).Msg("unable to publish to " + userID + ":outgoing")
				}
			}
		}
	}()

	// userTarget -> userSource (incoming for source)
	wg.Add(1)
	go func() {
		defer wg.Done()

		listenInc := h.Store.incomingMessage(ctx, userID)

		defer func(pubSub *redis.PubSub) {
			err := pubSub.Close()
			if err != nil {
				h.Logger.Err(err).Msg("unable to close redis channel properly: " + userID + ":incoming")
				return
			}
		}(listenInc)

		for {
			select {
			case <-h.Ctx.Done():
				return
			case <-localCtx.Done():
				return
			default:
				msg, err := listenInc.ReceiveMessage(context.Background())
				if err != nil {
					h.Logger.Err(err).Msg("unable to get message from " + userID + ":incoming")
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

func (h *HttpServerHandle) matchUser(c echo.Context) error {
	session, err := h.SessionStore.Get(c.Request(), "random-video-chat-session")
	if err != nil {
		h.Logger.Err(err).Msg("unable to find session")
		return echo.NewHTTPError(http.StatusInternalServerError, "unable to find session")
	}

	userID := session.Values["userID"].(string)

	//deadline := time.Now().Add(5 * time.Second)
	//ctx, cancel := context.WithDeadline(context.Background(), deadline)
	//defer cancel()

	ctx := context.Background()

	if err := h.Store.removeExistingMatch(ctx, userID); err != nil {
		h.Logger.Err(err).Msg("unable to remove old match of the user")
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	candidateID, err := h.Store.getMatchCandidate(ctx, userID)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	if err := h.Store.enqueueMatchRequest(ctx, userID, candidateID); err != nil {
		h.Logger.Err(err).Msg("unable to enqueue to match request")
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}
