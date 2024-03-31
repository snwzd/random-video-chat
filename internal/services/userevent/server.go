package userevent

import (
	"context"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"os"
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

	go func() {
		http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})
		http.Handle("/metrics", promhttp.Handler())

		err := http.ListenAndServe(":"+os.Getenv("USEREVENT_SERVICE_PORT"), nil)
		if err != nil {
			return
		}
	}()

	wg.Wait()
}
