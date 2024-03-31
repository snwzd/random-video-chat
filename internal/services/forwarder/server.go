package forwarder

import (
	"context"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"os"
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

	go func() {
		http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})
		http.Handle("/metrics", promhttp.Handler())

		err := http.ListenAndServe(":"+os.Getenv("FORWARDER_SERVICE_PORT"), nil)
		if err != nil {
			return
		}
	}()

	wg.Wait()
}
