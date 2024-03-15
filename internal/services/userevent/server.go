package userevent

import (
	"context"
	"snwzt/rvc/internal/services/userevent/handlers"
	"sync"
)

type Server struct {
	handlers handlers.ServerHandler
}

func NewServer(handlers handlers.ServerHandler) *Server {
	return &Server{
		handlers: handlers,
	}
}

func (svc *Server) Run(ctx context.Context) {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		svc.handlers.Match(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		svc.handlers.Remove(ctx)
	}()

	wg.Wait()
}
