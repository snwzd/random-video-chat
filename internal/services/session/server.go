package session

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type PromMetrics struct {
	msgGauge prometheus.Gauge
}

func NewPromMetrics() *PromMetrics {
	msgGauge := promauto.NewGauge(prometheus.GaugeOpts{
		Name: "num_session_gauge",
		Help: "counter of number of sessions currently running in the instance",
	})

	return &PromMetrics{
		msgGauge: msgGauge,
	}
}

func (p *PromMetrics) Counter(goroutines map[string]context.CancelFunc) {
	for {
		p.msgGauge.Set(float64(len(goroutines)))
		time.Sleep(2 * time.Second)
	}
}

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
