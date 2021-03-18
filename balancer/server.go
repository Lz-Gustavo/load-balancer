package main

import (
	"bufio"
	"context"
	"log"
	"net"
	"os"
	"sync"

	"github.com/hashicorp/go-hclog"
)

const (
	requestPort  = ":8000"
	listenPort   = ":9000"
	logLevel     = "DEBUG"
	chanBuffSize = 512
)

type LoadBalancer struct {
	Incoming chan []byte
	reader   *bufio.Reader
	clients  map[string]*Client

	nodes  map[string]*ServerSession
	mu     sync.Mutex
	logger hclog.Logger
}

// NewLoadBalancer instantiates the load balancer process
func NewLoadBalancer(ctx context.Context) *LoadBalancer {
	lb := &LoadBalancer{
		Incoming: make(chan []byte, chanBuffSize),
		clients:  make(map[string]*Client),
		nodes:    make(map[string]*ServerSession),
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

func (lb *LoadBalancer) AddServer(con net.Conn) {
	s := NewServerSession(con)
	lb.mu.Lock()
	lb.nodes[con.RemoteAddr().String()] = s
	lb.mu.Unlock()
}

func (lb *LoadBalancer) AddClient(con net.Conn) {
	c := NewClient(con)
	lb.clients[con.RemoteAddr().String()] = c
}

func (lb *LoadBalancer) DistributeLoad(ctx context.Context) {}

func (lb *LoadBalancer) Listen(ctx context.Context) {
	listener, err := net.Listen("tcp", requestPort)
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
			lb.AddClient(conn)
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
