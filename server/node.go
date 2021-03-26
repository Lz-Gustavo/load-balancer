package main

import (
	"bufio"
	"context"
	"io"
	"load-balancer/pb"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
)

const (
	heartbeatTimeout = time.Second
	chanBuffSize     = 512
)

type NodeInstance struct {
	Load   int32
	mu     sync.Mutex
	t      *time.Timer
	cancel context.CancelFunc
}

// NewNodeInstance ...
func NewNodeInstance(ctx context.Context) *NodeInstance {
	ct, cn := context.WithCancel(ctx)
	nd := &NodeInstance{
		t:      time.NewTimer(heartbeatTimeout),
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

	jr := &pb.JoinRequest{Ip: listenAddr}
	raw, err := proto.Marshal(jr)
	if err != nil {
		return err
	}

	wr := bufio.NewWriter(con)
	_, err = wr.Write(raw)
	if err != nil {
		return err
	}
	return nil
}

func (nd *NodeInstance) Listen(ctx context.Context) {
	port := strings.Split(listenAddr, ":")[1]
	listener, err := net.Listen("tcp", port)
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

			go nd.ReceiveLoads(ctx, con)
			go nd.SendHeartbeat(ctx, con)
		}
	}
}

func (nd *NodeInstance) ReceiveLoads(ctx context.Context, con net.Conn) {
	rd := bufio.NewReader(con)
	for {
		select {
		case <-ctx.Done():
			return

		default:
			msg, err := rd.ReadBytes('\n')
			if err == nil && len(msg) > 1 {
				err = nd.parseLoadRequest(msg)
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
	wr := bufio.NewWriter(con)
	for {
		select {
		case <-ctx.Done():
			return

		case <-nd.t.C:
			nd.mu.Lock()
			raw, err := nd.generateHeartbeatMessage()
			if err != nil {
				log.Fatalln("failed to generate heartbeat message, err:", err.Error())
			}

			_, err = wr.Write(raw)
			if err != nil {
				log.Fatalln("failed to send heartbeat message, err:", err.Error())
			}
			nd.mu.Unlock()
		}
	}
}

func (nd *NodeInstance) Shutdown() {
	nd.cancel()
}

func (nd *NodeInstance) parseLoadRequest(req []byte) error {
	return nil
}

func (nd *NodeInstance) applyLoadRequest() {}

func (nd *NodeInstance) generateHeartbeatMessage() ([]byte, error) {
	return nil, nil
}
