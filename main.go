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
	service := serviceapp.ServiceApp{
		Logger: lg,
		Err:    serviceErr,
	}
	service.Start(ctx)

	select {
	case <-ctx.Done():
		{

		}
	case err := <- service.Err:
		{
			lg.Infof("have service error: %s; stopping...", err)
		}
	}

	// graceful shutdown
	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()

	// stop daemon task
	service.Stop(ctxShutDown)

	// stop prometheus server
	if err := srv.Shutdown(ctxShutDown); err != nil {
		lg.Errorf("Prometheus server shutdown failed: %s", err)
		return
	}
	lg.Info("Prometheus server stopped")
}
