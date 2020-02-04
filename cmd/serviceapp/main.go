package main

import (
	"flag"
	"log"

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

	err = s.Run()
	if err != nil {
		log.Fatalf("fatal error while running serviceapp: %s", err)
	}
}
