package eserviceregistry

import (
	"context"
	"sync"
	"time"

	"github.com/ghodss/yaml"
	"github.com/megaease/easegateway/pkg/logger"
	"github.com/megaease/easegateway/pkg/object/serviceregistry"
	"github.com/megaease/easegateway/pkg/supervisor"

	"go.etcd.io/etcd/clientv3"
)

const (
	// Category is the category of EtcdServiceRegistry.
	Category = supervisor.CategoryBusinessController

	// Kind is the kind of EtcdServiceRegistry.
	Kind = "EtcdServiceRegistry"
)

func init() {
	supervisor.Register(&EtcdServiceRegistry{})
}

type (
	// EtcdServiceRegistry is Object EtcdServiceRegistry.
	EtcdServiceRegistry struct {
		super     *supervisor.Supervisor
		superSpec *supervisor.Spec
		spec      *Spec

		clientMutex sync.RWMutex
		client      *clientv3.Client

		statusMutex sync.Mutex
		serversNum  map[string]int

		done chan struct{}
	}

	// Spec describes the EtcdServiceRegistry.
	Spec struct {
		Endpoints    []string `yaml:"endpoints" jsonschema:"required,uniqueItems=true"`
		Prefix       string   `yaml:"prefix" jsonschema:"required,pattern=^/"`
		CacheTimeout string   `yaml:"cacheTimeout" jsonschema:"required,format=duration"`
	}

	// Status is the status of EtcdServiceRegistry.
	Status struct {
		Health     string         `yaml:"health"`
		ServersNum map[string]int `yaml:"serversNum"`
	}
)

// Category returns the category of EtcdServiceRegistry.
func (e *EtcdServiceRegistry) Category() supervisor.ObjectCategory {
	return Category
}

// Kind returns the kind of EtcdServiceRegistry.
func (e *EtcdServiceRegistry) Kind() string {
	return Kind
}

// DefaultSpec returns the default spec of EtcdServiceRegistry.
func (e *EtcdServiceRegistry) DefaultSpec() interface{} {
	return &Spec{
		Prefix:       "/services/",
		CacheTimeout: "60s",
	}
}

// Init initilizes EtcdServiceRegistry.
func (e *EtcdServiceRegistry) Init(superSpec *supervisor.Spec, super *supervisor.Supervisor) {
	e.superSpec, e.spec, e.super = superSpec, superSpec.ObjectSpec().(*Spec), super
	e.reload()
}

// Inherit inherits previous generation of EtcdServiceRegistry.
func (e *EtcdServiceRegistry) Inherit(superSpec *supervisor.Spec,
	previousGeneration supervisor.Object, super *supervisor.Supervisor) {

	previousGeneration.Close()
	e.Init(superSpec, super)
}

func (e *EtcdServiceRegistry) reload() {
	e.serversNum = map[string]int{}
	e.done = make(chan struct{})

	_, err := e.getClient()
	if err != nil {
		logger.Errorf("%s get etcd client failed: %v", e.superSpec.Name(), err)
	}

	go e.run()
}

func (e *EtcdServiceRegistry) getClient() (*clientv3.Client, error) {
	e.clientMutex.RLock()
	if e.client != nil {
		client := e.client
		e.clientMutex.RUnlock()
		return client, nil
	}
	e.clientMutex.RUnlock()

	return e.buildClient()
}

func (e *EtcdServiceRegistry) buildClient() (*clientv3.Client, error) {
	e.clientMutex.Lock()
	defer e.clientMutex.Unlock()

	// DCL
	if e.client != nil {
		return e.client, nil
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:            e.spec.Endpoints,
		AutoSyncInterval:     1 * time.Minute,
		DialTimeout:          10 * time.Second,
		DialKeepAliveTime:    1 * time.Minute,
		DialKeepAliveTimeout: 1 * time.Minute,
		LogConfig:            logger.EtcdClientLoggerConfig(e.super.Options(), "object_"+e.superSpec.Name()),
	})

	if err != nil {
		return nil, err
	}

	e.client = client

	return client, nil
}

func (e *EtcdServiceRegistry) closeClient() {
	e.clientMutex.Lock()
	defer e.clientMutex.Unlock()

	if e.client == nil {
		return
	}
	err := e.client.Close()
	if err != nil {
		logger.Errorf("%s close etcd client failed: %v", e.superSpec.Name(), err)
	}
	e.client = nil
}

func (e *EtcdServiceRegistry) run() {
	cacheTimeout, err := time.ParseDuration(e.spec.CacheTimeout)
	if err != nil {
		logger.Errorf("BUG: parse duration %s failed: %v",
			e.spec.CacheTimeout, err)
		return
	}

	e.update()

	for {
		select {
		case <-e.done:
			return
		case <-time.After(cacheTimeout):
			e.update()
		}
	}
}

func (e *EtcdServiceRegistry) update() {
	client, err := e.getClient()
	if err != nil {
		logger.Errorf("%s get etcd client failed: %v",
			e.superSpec.Name(), err)
		return
	}
	resp, err := client.Get(context.Background(), e.spec.Prefix, clientv3.WithPrefix())
	if err != nil {
		logger.Errorf("%s pull services failed: %v",
			e.superSpec.Name(), err)
		return
	}

	servers := []*serviceregistry.Server{}
	serversNum := map[string]int{}
	for _, kv := range resp.Kvs {
		server := &serviceregistry.Server{}
		err := yaml.Unmarshal(kv.Value, server)
		if err != nil {
			logger.Errorf("%s: unmarshal %s to yaml failed: %v",
				kv.Key, kv.Value, err)
			continue
		}
		if err := server.Validate(); err != nil {
			logger.Errorf("%s is invalid: %v", kv.Value, err)
			continue
		}

		servers = append(servers, server)
		serversNum[server.ServiceName]++
	}

	serviceregistry.Global.ReplaceServers(e.superSpec.Name(), servers)

	e.statusMutex.Lock()
	e.serversNum = serversNum
	e.statusMutex.Unlock()
}

// Status returns status of EtcdServiceRegister.
func (e *EtcdServiceRegistry) Status() *supervisor.Status {
	s := &Status{}

	_, err := e.getClient()
	if err != nil {
		s.Health = err.Error()
	} else {
		s.Health = "ready"
	}

	e.statusMutex.Lock()
	serversNum := e.serversNum
	e.statusMutex.Unlock()

	s.ServersNum = serversNum

	return &supervisor.Status{
		ObjectStatus: s,
	}
}

// Close closes EtcdServiceRegistry.
func (e *EtcdServiceRegistry) Close() {
	e.closeClient()
	close(e.done)

	serviceregistry.Global.CloseRegistry(e.superSpec.Name())
}
