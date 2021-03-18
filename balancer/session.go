package main

import (
	"bufio"
	"context"
	"io"
	"log"
	"net"
)

type ServerSession struct {
	Load       int
	Send       chan []byte
	Hearthbeat chan []byte

	reader *bufio.Reader
	writer *bufio.Writer
	conn   net.Conn
	cancel context.CancelFunc
}

func NewServerSession(con net.Conn) *ServerSession {
	ctx, c := context.WithCancel(context.Background())
	svr := &ServerSession{
		Load:       100,
		Send:       make(chan []byte),
		Hearthbeat: make(chan []byte),
		reader:     bufio.NewReader(con),
		writer:     bufio.NewWriter(con),
		conn:       con,
		cancel:     c,
	}
	svr.Listen(ctx)
	return svr
}

func (sv *ServerSession) Listen(ctx context.Context) {
	go sv.Read(ctx)
	go sv.Write(ctx)
}

func (sv *ServerSession) Read(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		default:
			line, err := sv.reader.ReadBytes('\n')
			if err == nil && len(line) > 1 {
				sv.Hearthbeat <- line

			} else if err == io.EOF {
				return
			}
		}
	}
}

func (sv *ServerSession) Write(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case data := <-sv.Send:
			_, err := sv.writer.Write(data)
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
