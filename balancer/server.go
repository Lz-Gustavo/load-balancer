package main

import (
	"context"
	"log"
	"net"
	"os"
	"sync"

	"github.com/hashicorp/go-hclog"
)

const (
	listenPort = ":9000"
	logLevel   = "DEBUG"
)

type LoadBalancer struct {
	incoming chan []byte
	logger   hclog.Logger

	nodes map[string]ServerSession
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
	go lb.ListenForJoins(ctx)
	return lb
}

func (lb *LoadBalancer) AddServer(con net.Conn) {}

func (lb *LoadBalancer) Listen(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

			// TODO: Listen for client requests, and equaly distribute
		}
	}
}

func (lb *LoadBalancer) ListenForJoins(ctx context.Context) {
	listener, err := net.Listen("tcp", listenPort)
	if err != nil {
		log.Fatalf("failed to start listening: %s", err.Error())
	}

	for {
		select {
		case <-ctx.Done():
			return

		default:
			conn, err := listener.Accept()
			if err != nil {
				log.Fatalf("accept failed: %s", err.Error())
			}
			lb.AddServer(conn)
		}
	}
}
