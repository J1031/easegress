package rest

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/hexdecteam/easegateway/pkg/cluster/gateway"
	"github.com/hexdecteam/easegateway/pkg/engine"
	"github.com/hexdecteam/easegateway/pkg/logger"
	"github.com/hexdecteam/easegateway/pkg/option"

	"github.com/go-openapi/spec"
)

var contact = spec.ContactInfo{
	Name:  "megaease",
	Email: "service@megaease.com",
	URL:   "http://megaease.com",
}
var version1 = "1.0.0"

type Rest struct {
	sync.Mutex
	gateway *engine.Gateway
	gc      *gateway.GatewayCluster
	server  *http.Server
	done    chan error
	stopped bool
}

func NewRest(gateway *engine.Gateway) (*Rest, error) {
	if gateway == nil {
		return nil, fmt.Errorf("gateway engine is nil")
	}

	return &Rest{
		gateway: gateway,
		gc:      gateway.Cluster(),
		done:    make(chan error, 1),
	}, nil
}

func (s *Rest) Start() (<-chan error, string, error) {
	s.Lock()
	defer s.Unlock()

	listenAddr := fmt.Sprintf("%s:9090", option.RestHost)

	adminServer, err := newAdminServer(s.gateway, s.gc)
	if err != nil {
		logger.Errorf("[create admin rest server failed: %v]", err)
		return nil, listenAddr, err
	}
	statisticsServer, err := newStatisticsServer(s.gateway)
	if err != nil {
		logger.Errorf("[create statistics rest server failed: %v]", err)
		return nil, listenAddr, err
	}
	healthCheckServer, err := newHealthCheckServer(s.gateway)
	if err != nil {
		logger.Errorf("[create healthcheck rest server failed: %v]", err)
		return nil, listenAddr, err
	}
	adminApi, err := adminServer.Api()
	if err != nil {
		logger.Errorf("[create admin api failed: %v]", err)
		return nil, listenAddr, err
	} else {
		logger.Debugf("[admin api created]")
	}
	statisticsApi, err := statisticsServer.Api()
	if err != nil {
		logger.Errorf("[create statistics api failed: %v]", err)
		return nil, listenAddr, err
	} else {
		logger.Debugf("[statistics api created]")
	}
	healthCheckApi, err := healthCheckServer.Api()
	if err != nil {
		logger.Errorf("[create healthcheck api failed: %v]", err)
		return nil, listenAddr, err
	} else {
		logger.Debugf("[healthcheck api created]")
	}

	http.Handle("/admin/", http.StripPrefix("/admin", adminApi.MakeHandler()))
	http.Handle("/statistics/", http.StripPrefix("/statistics", statisticsApi.MakeHandler()))
	http.Handle("/health/", http.StripPrefix("/health", healthCheckApi.MakeHandler()))

	clusterAdminServer, err := newClusterAdminServer(s.gateway, s.gc)
	if err != nil {
		logger.Errorf("[create cluster admin rest server failed: %v]", err)
		return nil, listenAddr, err
	}
	clusterStatisticsServer, err := newClusterStatisticsServer(s.gateway, s.gc)
	if err != nil {
		logger.Errorf("[create cluster statistics rest server failed: %v]", err)
		return nil, listenAddr, err
	}
	clusterAdminApi, err := clusterAdminServer.Api()
	if err != nil {
		logger.Errorf("[create cluster admin api failed: %v]", err)
		return nil, listenAddr, err
	} else {
		logger.Debugf("[cluster admin api created]")
	}
	clusterStatisticsApi, err := clusterStatisticsServer.Api()
	if err != nil {
		logger.Errorf("[create cluster statistics api failed: %v]", err)
		return nil, listenAddr, err
	} else {
		logger.Debugf("[cluster statistics api created]")
	}

	clusterHealthServer, err := newClusterHealthServer(s.gateway, s.gc)
	if err != nil {
		logger.Errorf("[create cluster health server failed: %v]", err)
		return nil, listenAddr, err
	}
	logger.Debugf("[cluster health api created]")

	http.Handle("/cluster/admin/", http.StripPrefix("/cluster/admin", clusterAdminApi.MakeHandler()))
	http.Handle("/cluster/statistics/",
		http.StripPrefix("/cluster/statistics", clusterStatisticsApi.MakeHandler()))
	// So swagger doc api path in clusterHealthServer is prefixed with `/cluster/health/`
	http.Handle("/cluster/health/", http.StripPrefix("/cluster/health", clusterHealthServer.GetHandler()))

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, listenAddr, err
	}

	s.server = &http.Server{}

	go func() {
		defer func() {
			// server exits after closing channel, ignore safely
			recover()
		}()

		err := s.server.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)})
		if err != nil && !s.stopped {
			s.done <- err
		}
		s.done <- nil
	}()

	return s.done, listenAddr, nil
}

func (s *Rest) Stop() {
	s.Lock()
	defer s.Unlock()

	s.stopped = true

	if s.server != nil {
		err := s.server.Shutdown(context.Background())
		if err != nil {
			logger.Errorf("[shut rest interface down failed: %s]", err)
		} else {
			logger.Debugf("[rest interface is shut down gracefully]")
		}
	} else {
		s.done <- nil
	}
}

func (s *Rest) Close() {
	s.Lock()
	defer s.Unlock()

	close(s.done)
}

////

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}