package session

import (
	"context"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	port     string
	handlers ServerHandler
}

func NewServer(port string, handlers ServerHandler) *Server {
	return &Server{
		port:     port,
		handlers: handlers,
	}
}

func (svc *Server) Run(ctx context.Context) {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		svc.handlers.createSession(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		go svc.handlers.deleteSession(ctx)
	}()

	go func() {
		http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
			_, err := writer.Write([]byte("healthy"))
			if err != nil {
				return
			}
		})

		http.Handle("/metrics", promhttp.Handler())

		err := http.ListenAndServe(svc.port, nil)
		if err != nil {
			return
		}
	}()

	wg.Wait()
}
