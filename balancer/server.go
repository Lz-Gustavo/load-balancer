package main

import (
	"context"
	"fmt"
	"load-balancer/pb"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/protobuf/proto"
)

const (
	requestPort  = ":8000"
	joinsPort    = ":9000"
	logLevel     = "DEBUG"
	chanBuffSize = 512
	ioBuffSize   = 256
)

type handleFunc func(net.Conn) error

type LoadBalancer struct {
	Incoming chan *pb.Request
	clients  map[string]*Client
	nodes    map[string]*ServerSession

	mu     sync.Mutex
	logger hclog.Logger
	cancel context.CancelFunc
}

// NewLoadBalancer instantiates the load balancer process
func NewLoadBalancer(ctx context.Context) *LoadBalancer {
	ct, cn := context.WithCancel(ctx)
	lb := &LoadBalancer{
		Incoming: make(chan *pb.Request, chanBuffSize),
		clients:  make(map[string]*Client),
		nodes:    make(map[string]*ServerSession),
		logger: hclog.New(&hclog.LoggerOptions{
			Name:       "load",
			Level:      hclog.LevelFromString(logLevel),
			TimeFormat: time.Kitchen,
			Color:      hclog.AutoColor,
			Output:     os.Stderr,
		}),
		cancel: cn,
	}

	go lb.Listen(ct, requestPort, lb.AddClient)
	go lb.Listen(ct, joinsPort, lb.AddServer)
	go lb.DistributeLoad(ct)
	return lb
}

func (lb *LoadBalancer) AddServer(con net.Conn) error {
	lb.logger.Info("got server join request")
	buff := make([]byte, ioBuffSize)

	ln, err := con.Read(buff)
	if err != nil {
		return err
	}
	raw := buff[:ln]

	jr := &pb.JoinRequest{}
	err = proto.Unmarshal(raw, jr)
	if err != nil {
		return err
	}

	svr, err := net.Dial("tcp", jr.Ip)
	if err != nil {
		return err
	}
	ctx, cn := context.WithCancel(context.Background())

	s := NewServerSession(ctx, cn, svr)
	lb.mu.Lock()
	lb.nodes[jr.Ip] = s
	lb.mu.Unlock()

	go lb.updateServerLoad(ctx, cn, s, jr.Ip)
	lb.logger.Info(fmt.Sprint("server '", jr.Ip, "' added"))
	return nil
}

func (lb *LoadBalancer) AddClient(con net.Conn) error {
	lb.logger.Info("got client join request")
	ctx := context.Background()
	c := NewClient(ctx, con)
	addr := con.RemoteAddr().String()
	lb.clients[addr] = c

	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case req := <-c.Receive:
				lb.logger.Debug(fmt.Sprint("received req from client '", addr, "'"))
				lb.Incoming <- req
			}
		}
	}()

	lb.logger.Info(fmt.Sprint("client '", addr, "' just connected"))
	return nil
}

// DistributeLoad implements a kind of "selective" fanout algorithm, distributing
// messages from lb.Incoming based on load parameter and current load of the top
// N/2 nodes with less load.
func (lb *LoadBalancer) DistributeLoad(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case req := <-lb.Incoming:
			lb.mu.Lock()
			if len(lb.nodes) == 0 {
				lb.logger.Warn("dont have any subscripted nodes to distribute request, ignoring...")
				lb.mu.Unlock()
				break
			}

			nodes := lb.retrieveLeastBusyNodes()
			i := lb.raffleRoulette(nodes, req.Load)
			if i < 0 {
				lb.logger.Warn("dont have any available nodes to distribute request, ignoring...")
				lb.mu.Unlock()
				break
			}
			nodes[i].Send <- req
			lb.mu.Unlock()
		}
	}
}

func (lb *LoadBalancer) Listen(ctx context.Context, port string, handle handleFunc) {
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalln("failed to start listening:", err.Error())
	}
	lb.logger.Info(fmt.Sprint("listening for requests on ", port))

	for {
		select {
		case <-ctx.Done():
			return

		default:
			con, err := listener.Accept()
			if err != nil {
				log.Fatalln("accept failed with err:", err.Error())
			}

			err = handle(con)
			if err != nil {
				log.Fatalln("failed handling connection, got err:", err.Error())
			}
		}
	}
}

func (lb *LoadBalancer) Shutdown() {
	for _, c := range lb.clients {
		c.Disconnect()
	}
	for _, s := range lb.nodes {
		s.Disconnect()
	}
	lb.cancel()
}

// retrieveLeastBusyNodes fetches the top N/2 nodes with less load
func (lb *LoadBalancer) retrieveLeastBusyNodes() []*ServerSession {
	nodes := make([]*ServerSession, 0)
	for _, v := range lb.nodes {
		if v != nil {
			nodes = append(nodes, v)
		}
	}

	sort.Sort(SortByLoad(nodes))
	half := int(math.Ceil(float64(len(nodes)) / 2))
	return nodes[:half]
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
//
// If during step 1 the n-th node doesn't have the necessary available load to handle
// the current request, the n node and all nodes of index m, such as m > n, are
// discarded. If no node is capable to handle, index -1 is returned.
func (lb *LoadBalancer) raffleRoulette(nodes []*ServerSession, load int32) int {
	odds := make([]int32, len(nodes), len(nodes))
	sum := int32(0)
	for i, n := range nodes {
		c := 100 - n.Load
		odds[i] = c
		sum += c
	}

	interval := int32(0)
	for i, odd := range odds {
		if odd < load {
			odds = odds[:i]
			break
		}

		interval += odd * 100 / sum
		odds[i] = interval
	}

	num := int32(rand.Intn(100) + 1)
	for i, interval := range odds {
		if num <= interval {
			return i
		}
	}
	return -1
}

func (lb *LoadBalancer) updateServerLoad(ctx context.Context, cancel context.CancelFunc, sv *ServerSession, ip string) {
	for {
		select {
		case <-ctx.Done():
			lb.mu.Lock()
			delete(lb.nodes, ip)
			lb.mu.Unlock()
			lb.logger.Info(fmt.Sprint("server '", ip, "' removed"))
			return

		case hb := <-sv.Heartbeat:
			lb.mu.Lock()
			sv.Load = hb.CurrentLoad
			lb.mu.Unlock()
			lb.logger.Debug(fmt.Sprint(ip, " current load: ", hb.CurrentLoad))
		}
	}
}

type SortByLoad []*ServerSession

func (a SortByLoad) Len() int           { return len(a) }
func (a SortByLoad) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a SortByLoad) Less(i, j int) bool { return a[i].Load < a[j].Load }
