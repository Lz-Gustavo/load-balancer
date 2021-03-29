package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"load-balancer/pb"
	"net"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/protobuf/proto"
)

type Client struct {
	Receive chan *pb.Request
	reader  *bufio.Reader
	writer  *bufio.Writer

	conn   net.Conn
	logger hclog.Logger
	cancel context.CancelFunc
}

func NewClient(ctx context.Context, con net.Conn) *Client {
	ctx, c := context.WithCancel(ctx)
	cl := &Client{
		Receive: make(chan *pb.Request),
		reader:  bufio.NewReader(con),
		writer:  bufio.NewWriter(con),
		logger: hclog.New(&hclog.LoggerOptions{
			Name:       "client",
			Level:      hclog.LevelFromString(logLevel),
			TimeFormat: time.Kitchen,
			Output:     os.Stderr,
		}),
		conn:   con,
		cancel: c,
	}
	go cl.Listen(ctx)
	return cl
}

func (cl *Client) Listen(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		default:
			raw, err := cl.reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					cl.logger.Warn("client disconnected")
					return
				}
				cl.logger.Error(fmt.Sprint("got undefined error while reading request, err: ", err.Error()))
			}

			data := bytes.TrimSuffix(raw, []byte("\n"))
			req := &pb.Request{}
			err = proto.Unmarshal(data, req)
			if err != nil {
				cl.logger.Warn(fmt.Sprint("failed parsing request, got err: ", err.Error()))
				break
			}
			cl.Receive <- req
		}
	}
}

func (cl *Client) Disconnect() {
	cl.cancel()
	cl.conn.Close()
}
