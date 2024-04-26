package handlers

import (
	"net/http"
	"os"
	"snwzt/rvc/internal/models"
	"snwzt/rvc/internal/services/user/store"

	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

type ServerHandler interface {
	CheckHealth(echo.Context) error
	Home(echo.Context) error
	RegisterUser(echo.Context) error
	MatchUser(echo.Context) error
}

type ServerHandle struct {
	Store        store.Store
	SessionStore *sessions.CookieStore
	Logger       *zerolog.Logger
}

func (h *ServerHandle) CheckHealth(c echo.Context) error {
	return c.String(http.StatusOK, "healthy")
}

func (h *ServerHandle) Home(c echo.Context) error {
	return c.Render(http.StatusOK, "register.html", nil)
}

func (h *ServerHandle) RegisterUser(c echo.Context) error {
	username := c.FormValue("username")

	if username == "" {
		return echo.NewHTTPError(http.StatusBadRequest)
	}

	userID := username + ":" + uuid.New().String()

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

	if err := h.Store.AddUser(&models.UserEntry{
		ID:       userID,
		Username: username,
		IPAddr:   c.RealIP(),
		MatchID:  "",
	}); err != nil {
		h.Logger.Err(err).Msg("unable to add user entry")
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	if err := h.Store.UnpairUsers(userID); err != nil {
		h.Logger.Err(err).Msg("unable to add user to unpaired pool")
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	h.Logger.Info().Msg("registered new user: " + userID)

	WsAddr := "ws://" + c.Request().Host + "/connection/" + userID

	if os.Getenv("PRODUCTION_FLAG") == "1" {
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

func (h *ServerHandle) MatchUser(c echo.Context) error {
	session, err := h.SessionStore.Get(c.Request(), "random-video-chat-session")
	if err != nil {
		h.Logger.Err(err).Msg("unable to find session")
		return echo.NewHTTPError(http.StatusInternalServerError, "unable to find session")
	}

	userID := session.Values["userID"].(string)

	if err := h.Store.RemoveExistingMatch(userID); err != nil {
		h.Logger.Err(err).Msg("unable to remove old match of the user")
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	candidateID, err := h.Store.GetMatchCandidate(userID)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	if err := h.Store.EnqueueMatchRequest(userID, candidateID); err != nil {
		h.Logger.Err(err).Msg("unable to enqueue to match request")
		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}
