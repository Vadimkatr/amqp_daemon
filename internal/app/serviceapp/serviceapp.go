package serviceapp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/streadway/amqp"

	"github.com/Vadimkatr/amqp_daemon/internal/app/amqpctl"
	"github.com/Vadimkatr/amqp_daemon/internal/app/logger"
)

var messageCounter = promauto.NewCounter(
	prometheus.CounterOpts{
		Name: "message_counter",
		Help: "Total number of send messages.",
	},
)

type ServiceConfig struct {
	DNS                  string
	ReconnectionInterval time.Duration
	ReconnectionRetries  int

	UsedQueueName string
}

type ServiceApp struct {
	Cfg    ServiceConfig
	Logger logger.Log
	Err    chan error
	wg     sync.WaitGroup
}

func (s *ServiceApp) Start(ctx context.Context) {
	s.Logger.Info("service is started")
	s.wg = sync.WaitGroup{}
	s.wg.Add(1)

	go s.run(ctx)
}

func (s *ServiceApp) run(ctx context.Context) {
	defer s.wg.Done()

	cfg := amqpctl.Config{
		DSN:                  s.Cfg.DNS,
		ReconnectionInterval: time.Second * s.Cfg.ReconnectionInterval,
		ReconnectionRetries:  s.Cfg.ReconnectionRetries,
		CaPath:               "",
		CertPath:             "",
		KeyPath:              "",
		SkipVerify:           false,
		ServerName:           "",
	}
	conn, err := amqpctl.NewConnectionController(s.Logger, cfg)

	if err != nil {
		s.Err <- fmt.Errorf("error while creating amqpctl conn: %s", err)
		return
	}
	defer conn.Close()

	for {
		ch, err := conn.OpenChannel(ctx)

		if err != nil {
			if ctx.Err() != nil {
				s.Err <- fmt.Errorf("error while opening channel: %s", err)
				return
			}

			s.Logger.Errorf("failed to open channel: %s. Try again...", err)

			continue
		}

		// do things (sending msg "1" to queue using channel ch)
		err = s.doWork(ctx, ch)
		if err != nil {
			s.Logger.Errorf("failed while publishing msg: %s", err)
			continue
		}

		if ctx.Err() != nil {
			return
		}
	}
}

func (s *ServiceApp) doWork(ctx context.Context, ch *amqp.Channel) error {
	defer ch.Close()

	q, err := ch.QueueDeclare(
		s.Cfg.UsedQueueName,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("error while queue declare: %s", err)
	}

	counter := 0

	for {
		time.Sleep(1 * time.Second)

		select {
		case <-ctx.Done():
			s.Logger.Info("service start graceful shutdown")
			return nil
		default:
		}

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
			return fmt.Errorf("error while publishing msg: %s", err)
		}

		messageCounter.Inc()
		s.Logger.Infof("send msg to: `%s`", msg)
		counter++
	}
}

func (s *ServiceApp) Stop() {
	s.wg.Wait()
	s.Logger.Info("service is stopped.")
}
