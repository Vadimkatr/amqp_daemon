package main

import (
	"flag"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/kardianos/service"
	"github.com/Vadimkatr/amqp_daemon/internal/app/serviceapp"
)

func main() {
	svcFlag := flag.String("serviceapp", "", "Control the system serviceapp.")
	flag.Parse()

	svcConfig := &service.Config{
		Name:        "rabbitmq_counter_queue",
		DisplayName: "Go Service Example for RabbitMQ",
		Description: "This is an example Go service serviceapp that sends 1s to rabbitmq queue.",
	}

	capp := &serviceapp.CounterApp{}
	s, err := service.New(capp, svcConfig)
	if err != nil {
		log.Fatalf("fatal error while creating serviceapp: %s", err)
	}

	serviceLogger, err := s.Logger(nil)
	if err != nil {
		log.Fatalf("fatal error while creating serviceapp logger: %s", err)
	}
	capp.Logger = serviceLogger

	if len(*svcFlag) != 0 {
		err := service.Control(s, *svcFlag)
		if err != nil {
			log.Printf("Valid actions: %q\n", service.ControlAction)
			log.Fatalf("fatal error while control servic action: %s", err)
		}
		log.Printf("Lets start!\n")
		return
	}

	go runMonitoring()

	err = s.Run()
	if err != nil {
		log.Fatalf("fatal error while running serviceapp: %s", err)
	}
}

func runMonitoring() {
	srv := &http.Server{Addr: ":2112"}
	http.Handle("/metrics", promhttp.Handler())

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Listen prometheus server error: %s\n", err)
		}
	}()
	log.Printf("Prometheus server started")

	<-done
	log.Printf("Prometheus server stoped")
}
