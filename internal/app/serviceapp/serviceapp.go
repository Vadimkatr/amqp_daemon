package serviceapp

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kardianos/service"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/streadway/amqp"

	"github.com/Vadimkatr/amqp_daemon/internal/app/amqpctl"
	"github.com/Vadimkatr/amqp_daemon/internal/app/logger"
)

type CounterApp struct {
	Logger service.Logger
}

var messageCounter = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "message_counter",
		Help: "Total number of send messages.",
	},
)

func (capp *CounterApp) Start(s service.Service) error {
	capp.Logger.Infof("start service app manager.\n")

	go capp.runTask()

	return nil
}

func (capp *CounterApp) runTask() {
	lg := logger.CustomLogger{}
	lg.Init()
	cfg := amqpctl.Config{
		DSN:                  os.Getenv("RABBITMQ_URL"), // by default is "amqp://guest:guest@localhost:5672/"
		ReconnectionInterval: time.Duration(time.Minute * 10),
		ReconnectionRetries:  1,
		CaPath:               "",
		CertPath:             "",
		KeyPath:              "",
		SkipVerify:           false,
		ServerName:           "",
	}
	conn, err := amqpctl.NewConnectionController(lg, cfg)
	if err != nil {
		capp.Logger.Errorf("error while creating amqpctl conn: %s", err)
		return
	}
	defer conn.Close()

	ch, err := conn.OpenChannel(context.Background())
	if err != nil {
		capp.Logger.Errorf("error while opening channel: %s", err)
		return
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		os.Getenv("RABBITMQ_USED_QUEUE"),
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		capp.Logger.Errorf("error while queue declare: %s", err)
		return
	}

	counter := 0
	for {
		time.Sleep(1 * time.Second)
		msg := fmt.Sprintf("%d", counter)
		err = ch.Publish(
			"",
			q.Name,
			false,
			false,
			amqp.Publishing{
				DeliveryMode: amqp.Persistent,
				ContentType:  "text/plain",
				Body:         []byte(msg),
			},
		)
		if err != nil {
			capp.Logger.Errorf("error while publishing msg: %s", err)
			return
		}

		messageCounter.Inc()
		capp.Logger.Infof("send msg to: `%s`", msg)
		counter++
	}
}

func (capp *CounterApp) Stop(s service.Service) error {
	capp.Logger.Infof("service is stopped.")
	return nil
}
