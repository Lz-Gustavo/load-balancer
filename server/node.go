package main

import (
	"context"
	"fmt"
	"io"
	"load-balancer/pb"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/protobuf/proto"
)

const (
	heartbeatTimeout = 3 * time.Second
	logLevel         = "DEBUG"
	chanBuffSize     = 512
	ioBuffSize       = 256
)

type NodeInstance struct {
	Load   int32
	mu     sync.Mutex
	t      *time.Ticker
	logger hclog.Logger
	cancel context.CancelFunc
}

// NewNodeInstance ...
func NewNodeInstance(ctx context.Context) *NodeInstance {
	ct, cn := context.WithCancel(ctx)
	nd := &NodeInstance{
		t: time.NewTicker(heartbeatTimeout),
		logger: hclog.New(&hclog.LoggerOptions{
			Name:       "node",
			Level:      hclog.LevelFromString(logLevel),
			TimeFormat: time.Kitchen,
			Output:     os.Stderr,
		}),
		cancel: cn,
	}

	go nd.Listen(ct)
	return nd
}

func (nd *NodeInstance) Connect() error {
	con, err := net.Dial("tcp", joinAddr)
	if err != nil {
		return err
	}
	defer con.Close()

	jr := &pb.JoinRequest{Ip: listenAddr}
	raw, err := proto.Marshal(jr)
	if err != nil {
		return err
	}

	_, err = con.Write(raw)
	if err != nil {
		return err
	}

	nd.logger.Info(fmt.Sprint("sent connect to", joinAddr, "load balancer"))
	return nil
}

func (nd *NodeInstance) Listen(ctx context.Context) {
	port := strings.Split(listenAddr, ":")[1]
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalln("failed to start listening:", err.Error())
	}

	for {
		select {
		case <-ctx.Done():
			return

		default:
			con, err := listener.Accept()
			if err != nil {
				log.Fatalln("accept failed with err:", err.Error())
			}

			nd.logger.Info("load balancer connected!")
			go nd.ReceiveLoads(ctx, con)
			go nd.SendHeartbeat(ctx, con)
		}
	}
}

func (nd *NodeInstance) ReceiveLoads(ctx context.Context, con net.Conn) {
	for {
		select {
		case <-ctx.Done():
			return

		default:
			buff := make([]byte, ioBuffSize)
			ln, err := con.Read(buff)
			if err == nil && ln > 1 {
				err = nd.parseAndApplyLoadRequest(ctx, buff[:ln])
				if err != nil {
					log.Fatalln("failed to interpret load request, got err:", err.Error())
				}

			} else if err == io.EOF {
				return
			}
		}
	}
}

func (nd *NodeInstance) SendHeartbeat(ctx context.Context, con net.Conn) {
	for {
		select {
		case <-ctx.Done():
			return

		case <-nd.t.C:
			raw, err := nd.generateHeartbeatMessage()
			if err != nil {
				log.Fatalln("failed to generate heartbeat message, err:", err.Error())
			}
			msg := append(raw, []byte("\n")...)

			_, err = con.Write(msg)
			if err != nil {
				log.Fatalln("failed to send heartbeat message, err:", err.Error())
			}
		}
	}
}

func (nd *NodeInstance) Shutdown() {
	nd.cancel()
}

func (nd *NodeInstance) parseAndApplyLoadRequest(ctx context.Context, req []byte) error {
	r := &pb.Request{}
	err := proto.Unmarshal(req, r)
	if err != nil {
		return err
	}

	nd.mu.Lock()
	if nd.Load-r.Load >= 0 {
		nd.Load -= r.Load
	}
	nd.mu.Unlock()

	go nd.releaseResourceAfterExecTime(ctx, r)
	return nil
}

func (nd *NodeInstance) generateHeartbeatMessage() ([]byte, error) {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	hb := &pb.Heartbeat{CurrentLoad: nd.Load}
	return proto.Marshal(hb)
}

// releaseResourceAfterExecTime re-increments the node's current load after a random
// period of time, following the informed loadReq.MaxExecTime.
func (nd *NodeInstance) releaseResourceAfterExecTime(ctx context.Context, loadReq *pb.Request) {
	sec := rand.Int31n(loadReq.MaxExecTime)
	timer := time.NewTimer(time.Duration(sec) * time.Second)

	for {
		select {
		case <-ctx.Done():
			return

		case <-timer.C:
			nd.mu.Lock()
			nd.Load += loadReq.Load
			nd.mu.Unlock()
		}
	}
}
