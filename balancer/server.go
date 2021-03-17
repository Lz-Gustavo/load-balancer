package main

import (
	"context"
	"os"
	"sync"

	"github.com/hashicorp/go-hclog"
)

const (
	listenPort = ":9000"
	logLevel   = "DEBUG"
)

type ServerInfo struct {
	cpu        int
	hearthbeat <-chan []byte
	send       chan<- []byte
}

type LoadBalancer struct {
	incoming chan []byte
	logger   hclog.Logger

	nodes map[string]ServerInfo
	mu    sync.Mutex
}

// NewLoadBalancer instantiates the load balancer process
func NewLoadBalancer(ctx context.Context) *LoadBalancer {
	lb := &LoadBalancer{
		logger: hclog.New(&hclog.LoggerOptions{
			Name:   "load",
			Level:  hclog.LevelFromString(logLevel),
			Output: os.Stderr,
		}),
	}

	go lb.Listen(ctx)
	return lb
}

func (lb *LoadBalancer) Listen(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

			// TODO: instantiate the new server here
		}
	}
}
