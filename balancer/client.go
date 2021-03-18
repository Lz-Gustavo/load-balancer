package main

import (
	"bufio"
	"context"
	"io"
	"log"
	"net"
)

type Client struct {
	Send    chan []byte
	Receive chan []byte

	reader *bufio.Reader
	writer *bufio.Writer
	conn   net.Conn
	cancel context.CancelFunc
}

func NewClient(ctx context.Context, con net.Conn) *Client {
	ctx, c := context.WithCancel(ctx)
	cl := &Client{
		Send:    make(chan []byte),
		Receive: make(chan []byte),
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
			line, err := cl.reader.ReadBytes('\n')
			if err == nil && len(line) > 1 {
				cl.Receive <- line

			} else if err == io.EOF {
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
