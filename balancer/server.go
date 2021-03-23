package main

import (
	"bufio"
	"context"
	"log"
	"math/rand"
	"net"
	"os"
	"sort"
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

	for {
		select {
		case <-ctx.Done():
			return

		case req := <-lb.Incoming:
			nodes := lb.retrieveLeastBusyNodes()
			i := lb.raffleRoulette(nodes)
			nodes[i].Send <- req
		}
	}
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

// retrieveLeastBusyNodes fetches the top N/2 nodes with less load
func (lb *LoadBalancer) retrieveLeastBusyNodes() []*ServerSession {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	n := len(lb.nodes)
	nodes := make([]*ServerSession, n)
	for _, v := range lb.nodes {
		nodes = append(nodes, v)
	}

	sort.Sort(SortByLoad(nodes))
	nodes = nodes[:n/2]
	return nodes
}

// raffleRoulette raffles a single node from the list, based on its current load.
// For instance, consider an example with the [10, 10, 20, 40] input:
//  1. At first, creates a list of its complements, the available load:
//     [90, 90, 80, 60] --- total: 320
//
//  2. Calculate the percentage of each available load:
//    [28.125, 28.125, 25, 18.75]
//
//  3. Sum each percentage, creating roulette intervals:
//    [28.125, 56.25, 81.25, 100]
//
//  4. Draws a rand num from within [1, 100] range, and check on which roulette
//     interval it falls off
func (lb *LoadBalancer) raffleRoulette(nodes []*ServerSession) int {
	odds := make([]int, len(nodes), len(nodes))
	sum := 0
	for i, n := range nodes {
		c := 100 - n.Load
		odds[i] = c
		sum += c
	}

	interval := 0
	for i, odd := range odds {
		interval += odd * 100 / sum
		odds[i] = interval
	}

	num := rand.Intn(100) + 1
	for i, interval := range odds {
		if num <= interval {
			return i
		}
	}
	return 0
}

type SortByLoad []*ServerSession

func (a SortByLoad) Len() int           { return len(a) }
func (a SortByLoad) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a SortByLoad) Less(i, j int) bool { return a[i].Load < a[j].Load }
