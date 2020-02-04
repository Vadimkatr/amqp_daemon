package amqpctl

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"sync"
	"time"

	"github.com/streadway/amqp"

	"github.com/Vadimkatr/amqp_daemon/internal/app/logger"
)

type Config struct {
	DSN                  string        `mapstructure:"dsn"`
	ReconnectionInterval time.Duration `mapstructure:"reconnection_interval"`
	ReconnectionRetries  int           `mapstructure:"reconnection_retries"`

	// TLS
	CaPath     string `mapstructure:"ca_path"`
	CertPath   string `mapstructure:"cert_path"`
	KeyPath    string `mapstructure:"key_path"`
	SkipVerify bool   `mapstructure:"skip_verify"`
	ServerName string `mapstructure:"server_name"`
}

type ConnectionController struct {
	con               *amqp.Connection
	reconnectionTimer *time.Timer
	m                 sync.Mutex

	log logger.Log
	cfg Config

	dialConfigCustom bool
	dialConfig       amqp.Config

	connectionString string // connection string without creds
}

// nolint: dupl
func NewConnectionController(log logger.Log, cfg Config) (*ConnectionController, error) {

	if log == nil {
		return nil, nil
	}

	log = log.With("amqp_con_ctrl")

	if cfg.ReconnectionInterval == 0 {
		return nil, errors.New("amqp reconnection interval is not passed")
	}

	if cfg.DSN == "" {
		return nil, errors.New("amqp dsn is not passed")
	}

	u, err := url.Parse(cfg.DSN)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	return &ConnectionController{
		log:              log,
		cfg:              cfg,
		connectionString: fmt.Sprintf("%s:%s%s", u.Hostname(), u.Port(), u.EscapedPath()),
	}, nil
}

// NewConnectionControllerWithDialConfig creates controller with custom github.com/streadway/amqp config,
// transferring it by value, for example:
// 	dialConfig := amqp.Config{
//		Heartbeat: time.Minute,
//		Locale:    "en_US",
//	}
//  ctrl, err := NewConnectionControllerWithDialConfig(log, cfg, dialConfig)
func NewConnectionControllerWithDialConfig(log logger.Log, cfg Config, dialConfig amqp.Config) (*ConnectionController, error) {
	ctrl, err := NewConnectionController(log, cfg)
	if err != nil {
		return nil, err
	}

	ctrl.dialConfigCustom = true
	ctrl.dialConfig = dialConfig
	return ctrl, nil
}

func (s *ConnectionController) startReconnectionTimer() {
	s.reconnectionTimer = time.NewTimer(s.cfg.ReconnectionInterval)
}

func (s *ConnectionController) Close() {
	s.m.Lock()
	defer s.m.Unlock()

	if s.con != nil {
		s.con.Close()
	}
}

// OpenChannel opens amqp.Channel in concurrent-safe way; the result should be .Close()-ed after use.
// In case of connection/network error wait for timeout before continuation
// Use should be as follows:
//	for {
//		ch, err := ctrl.OpenChannel(ctx)
//		if err != nil {
//			if ctx.Err() != nil {
//				return
//			}
//
//			log.Errorf("Failed to open channel: %s", err)
//			continue
//		}
//		defer ch.Close()
//
//		// do things
//	}
func (s *ConnectionController) OpenChannel(ctx context.Context) (*amqp.Channel, error) {
	s.m.Lock()
	defer s.m.Unlock()

	if s.reconnectionTimer != nil {
		select {
		case <-s.reconnectionTimer.C:
			s.reconnectionTimer = nil
		case <-ctx.Done():
			if err := ctx.Err(); err != nil {
				s.log.Errorf("con_ctl context error: %s", err)
				return nil, err
			}
			return nil, nil
		}
	}

	if s.con == nil {
		con, err := AmqpDialWithTlS(s.cfg.DSN, s.cfg.CaPath, s.cfg.CertPath, s.cfg.KeyPath, s.cfg.ServerName,
			s.cfg.SkipVerify, s.dialConfigCustom, s.dialConfig)
		if err != nil {
			s.startReconnectionTimer()
			s.log.Errorf("unable to connect to rmq: %s", err)
			return nil, err
		}
		s.log.Infof("connected to amqp: %s", s.connectionString)

		amqpClose := make(chan *amqp.Error)
		con.NotifyClose(amqpClose)

		s.con = con

		go func() {
			closeError := <-amqpClose
			s.m.Lock()
			defer s.m.Unlock()

			// ConnectionController.Close() makes closeError = nil
			if closeError != nil {
				s.log.Warnf("AMQP connection closing [con_string=%s]: %v", s.connectionString, closeError)
			}
			s.con = nil

			s.startReconnectionTimer()
		}()
	}

	ch, err := s.con.Channel()
	if err != nil {
		s.log.Errorf("unable to open rmq channel: %s", err)
		return nil, err
	}
	return ch, nil
}

func AmqpDialWithTlS(dsn, ca, cert, key, serverName string, skipVerify, dialCustomConfig bool,
	customConfig amqp.Config) (*amqp.Connection, error) {

	if ca != "" && cert != "" && key != "" {

		cfg := new(tls.Config)

		if skipVerify {
			cfg.InsecureSkipVerify = true
		}

		if serverName != "" {
			cfg.ServerName = serverName
		}

		cfg.RootCAs = x509.NewCertPool()

		// nolint: gosec
		caBody, err := ioutil.ReadFile(ca)
		if err != nil {
			return nil, fmt.Errorf("can't read CA file: %s", err)
		}

		cfg.RootCAs.AppendCertsFromPEM(caBody)

		cert, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			return nil, fmt.Errorf("can't read either key or cert file: %s", err)
		}

		cfg.Certificates = append(cfg.Certificates, cert)

		if dialCustomConfig {
			dialConfig := customConfig
			dialConfig.TLSClientConfig = cfg
			return amqp.DialConfig(dsn, dialConfig)
		}

		return amqp.DialTLS(dsn, cfg)
	}

	if dialCustomConfig {
		return amqp.DialConfig(dsn, customConfig)
	}

	return amqp.Dial(dsn)
}
