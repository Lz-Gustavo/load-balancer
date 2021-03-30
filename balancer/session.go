package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"load-balancer/pb"
	"log"
	"net"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/protobuf/proto"
)

type ServerSession struct {
	Load      int32
	Send      chan *pb.Request
	Heartbeat chan *pb.Heartbeat

	reader *bufio.Reader
	writer *bufio.Writer
	conn   net.Conn
	logger hclog.Logger
	cancel context.CancelFunc
}

func NewServerSession(ctx context.Context, cn context.CancelFunc, con net.Conn) *ServerSession {
	svr := &ServerSession{
		Load:      100,
		Send:      make(chan *pb.Request, chanBuffSize),
		Heartbeat: make(chan *pb.Heartbeat, chanBuffSize),
		reader:    bufio.NewReader(con),
		writer:    bufio.NewWriter(con),
		logger: hclog.New(&hclog.LoggerOptions{
			Name:       "session",
			Level:      hclog.LevelFromString(logLevel),
			TimeFormat: time.Kitchen,
			Color:      hclog.AutoColor,
			Output:     os.Stderr,
		}),
		conn:   con,
		cancel: cn,
	}
	svr.Run(ctx)
	return svr
}

func (sv *ServerSession) Run(ctx context.Context) {
	go sv.ReadHeartbeats(ctx)
	go sv.WriteRequests(ctx)
}

func (sv *ServerSession) ReadHeartbeats(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		default:
			raw, err := sv.reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					sv.logger.Warn("server disconnected")
					sv.Disconnect()
					return
				}
				sv.logger.Error(fmt.Sprint("got undefined error while reading heartbeat, err: ", err.Error()))
			}

			data := bytes.TrimSuffix(raw, []byte("\n"))
			hb := &pb.Heartbeat{}
			err = proto.Unmarshal(data, hb)
			if err != nil {
				sv.logger.Error(fmt.Sprint("failed parsing heartbeat, got err: ", err.Error()))
				return
			}
			sv.Heartbeat <- hb
		}
	}
}

func (sv *ServerSession) WriteRequests(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case req := <-sv.Send:
			raw, err := proto.Marshal(req)
			if err != nil {
				log.Fatalln("failed marshaling proto request, err:", err.Error())
			}
			msg := append(raw, []byte("\n")...)

			_, err = sv.conn.Write(msg)
			if err != nil {
				log.Fatalln("failed to send data, err:", err.Error())
			}
		}
	}
}

func (sv *ServerSession) Disconnect() {
	sv.cancel()
	sv.conn.Close()
}
