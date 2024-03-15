package forwarder

import (
	"context"
	"snwzt/rvc/internal/services/forwarder/handlers"
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

		svc.handlers.CreateForwarder(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		go svc.handlers.DeleteForwarder(ctx)
	}()

	wg.Wait()
}
