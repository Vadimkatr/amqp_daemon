package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/Vadimkatr/amqp_daemon/internal/app/logger"
	"github.com/Vadimkatr/amqp_daemon/internal/app/serviceapp"
)

const (
	ReconnectionIntervalDefault = 10
	ReconnectionRetriesDefault  = 1
)

func main() {
	lg := logger.CustomLogger{}
	lg.Init()

	done := make(chan os.Signal)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		oscall := <-done
		lg.Infof("system call: %v", oscall)
		cancel()
	}()

	// start prometheus server
	srv := &http.Server{Addr: ":2112"}

	http.Handle("/metrics", promhttp.Handler())

	go func() {
		lg.Info("Prometheus server started")

		if err := srv.ListenAndServe(); err != nil {
			lg.Errorf("Listen prometheus server: %v", err)
		}
	}()

	// start daemon task
	serviceErr := make(chan error)
	serviceCfg := serviceapp.ServiceConfig{
		DNS:                  os.Getenv("RABBITMQ_URL"), // by default is "amqp://guest:guest@localhost:5672/"
		ReconnectionInterval: ReconnectionIntervalDefault,
		ReconnectionRetries:  ReconnectionRetriesDefault,
		UsedQueueName:        os.Getenv("RABBITMQ_USED_QUEUE"),
	}
	service := serviceapp.ServiceApp{
		Cfg:    serviceCfg,
		Logger: lg,
		Err:    serviceErr,
	}
	service.Start(ctx)

	select {
	case err := <-service.Err:
		lg.Infof("have service error: %s; stopping...", err)
	case <-ctx.Done():
	}

	// graceful shutdown
	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// stop daemon task
	service.Stop()

	// stop prometheus server
	if err := srv.Shutdown(ctxShutDown); err != nil {
		lg.Errorf("Prometheus server shutdown failed: %s", err)
		return
	}

	lg.Info("Prometheus server stopped")
}
