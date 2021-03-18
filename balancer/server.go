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
	joinsPort    = ":9000"
	logLevel     = "DEBUG"
	chanBuffSize = 512
)

type handleFunc func(net.Conn)

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

	go lb.Listen(ctx, requestPort, lb.AddClient)
	go lb.Listen(ctx, joinsPort, lb.AddServer)
	return lb
}

func (lb *LoadBalancer) AddServer(con net.Conn) {
	s := NewServerSession(con)
	lb.mu.Lock()
	lb.nodes[con.RemoteAddr().String()] = s
	lb.mu.Unlock()
}

func (lb *LoadBalancer) AddClient(con net.Conn) {
	ctx := context.Background()
	c := NewClient(ctx, con)
	lb.clients[con.RemoteAddr().String()] = c

	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case data := <-c.Receive:
				lb.Incoming <- data
			}
		}
	}()
}

func (lb *LoadBalancer) DistributeLoad(ctx context.Context) {
	// TODO: implement a kind of "selective" fanout algorithm, distributing messages
	// from lb.Incoming based on load parameter and current load of the top N/2 nodes
	// with less load
}

func (lb *LoadBalancer) Listen(ctx context.Context, port string, handle handleFunc) {
	listener, err := net.Listen("tcp", port)
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
			handle(conn)
		}
	}
}
