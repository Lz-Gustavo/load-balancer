package main

import (
	"bufio"
	"context"
	"io"
	"load-balancer/pb"
	"log"
	"net"

	"google.golang.org/protobuf/proto"
)

type Client struct {
	Send    chan []byte
	Receive chan *pb.Request

	reader *bufio.Reader
	writer *bufio.Writer
	conn   net.Conn
	cancel context.CancelFunc
}

func NewClient(ctx context.Context, con net.Conn) *Client {
	ctx, c := context.WithCancel(ctx)
	cl := &Client{
		Send:    make(chan []byte),
		Receive: make(chan *pb.Request),
		reader:  bufio.NewReader(con),
		writer:  bufio.NewWriter(con),
		conn:    con,
		cancel:  c,
	}
	cl.Listen(ctx)
	return cl
}

func (cl *Client) Listen(ctx context.Context) {
	go cl.Read(ctx)
	go cl.Write(ctx)
}

func (cl *Client) Read(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		default:
			buff := make([]byte, ioBuffSize)
			ln, err := cl.reader.Read(buff)
			if err == nil && ln > 1 {
				req := &pb.Request{}
				err = proto.Unmarshal(buff[:ln], req)
				if err != nil {
					log.Println("failed parsing request, got err:", err.Error())
					return
				}
				cl.Receive <- req

			} else if err == io.EOF {
				log.Println("client disconnected")
				return
			}
		}
	}
}

func (cl *Client) Write(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case data := <-cl.Send:
			_, err := cl.writer.Write(data)
			if err != nil {
				log.Fatalln("failed to send data, err:", err.Error())
			}
		}
	}
}

func (cl *Client) Disconnect() {
	cl.cancel()
	cl.conn.Close()
}
