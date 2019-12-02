package server

import (
	"bufio"
	"bytes"
	"context"
	"log"
	"net"
	"strconv"
	"time"
)

const maxCapOfMsgBuf int = 100

type client struct {
	c       chan []byte
	address string
}

func newClient(addr string) *client {
	return &client{
		c:       make(chan []byte, maxCapOfMsgBuf),
		address: addr,
	}
}

type multiEchoServer struct {
	clients  map[string]*client
	message  chan []byte
	listener net.Listener
	cancel   context.CancelFunc
}

// New creates and returns (but does not start) a new MultiEchoServer.
func New() MultiEchoServer {
	return &multiEchoServer{
		clients: make(map[string]*client),
		message: make(chan []byte, maxCapOfMsgBuf),
	}
}

func (mes *multiEchoServer) Start(port int) error {
	log.Printf("server start")
	//listen port
	l, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return err
	}
	mes.listener = l

	baseCtx, cancel := context.WithCancel(context.Background())
	mes.cancel = cancel

	//send messages to all connected clients
	go mes.broadcast(baseCtx)

	//serve connection
	return mes.serve(baseCtx, l)
}

func (mes *multiEchoServer) Close() {
	//close listener
	//close connections
	//any goroutines should be signaled to return
	mes.cancel()

	log.Printf("server exit")
}

func (mes *multiEchoServer) Count() int {
	//return the number of clients connected to the server
	return len(mes.clients)
}

func (mes *multiEchoServer) serve(ctx context.Context, l net.Listener) error {
	var tmpDelay time.Duration
	for {
		select {
		case <-ctx.Done():
			_ = l.Close()
			return nil
		default:
		}
		conn, err := l.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tmpDelay == 0 {
					tmpDelay = 5 * time.Millisecond
				} else {
					tmpDelay *= 2
				}
				if max := 1 * time.Second; tmpDelay > max {
					tmpDelay = max
				}
				log.Printf("http: Accept error: %v; retrying in %v", err, tmpDelay)
				time.Sleep(tmpDelay)
				continue
			}
			return err
		}
		tmpDelay = 0
		go mes.handleConn(ctx, conn)
	}
}

func (mes *multiEchoServer) broadcast(ctx context.Context) {
	for {
		select {
		case msg := <-mes.message:
			for _, client := range mes.clients {
				select {
				case client.c <- msg:
				default:
					//queue is full
					clearChan(client.c)
					client.c <- msg
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (mes *multiEchoServer) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	addr := conn.RemoteAddr().String()
	cli := newClient(addr)
	//add client
	mes.clients[addr] = cli
	//write to client
	go writeToClient(ctx, cli, conn)

	//broadcast who login
	mes.message <- makeMsg(*cli, "login")

	quit := make(chan struct{})
	hasData := make(chan bool)

	//read data from client
	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			buf := make([]byte, 2048)
			for {
				r := bufio.NewReader(conn)
				n, err := r.Read(buf)
				if err != nil || n == 0 {
					// client disconnect
					log.Printf("client %s read error: %v", addr, err)
					close(quit)
					return
				}
				if n == 3 && string(buf[:n]) == "who" {
					var buf bytes.Buffer
					buf.WriteString("user lists :\n")
					for k := range mes.clients {
						buf.WriteString("-" + k)
					}
					_, _ = conn.Write(buf.Bytes())
				} else {
					//broadcast message
					mes.message <- buf[:n]
				}
			}
		}
	}(ctx)

	//manage clients
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	for {
		timer.Reset(30 * time.Second)
		select {
		case <-hasData:
		case <-quit:
			delete(mes.clients, addr)
			mes.message <- makeMsg(*cli, "logout")
			conn.Close()
			return
		case <-timer.C:
			//timeout quit
			delete(mes.clients, addr)
			mes.message <- makeMsg(*cli, "timeout logout")
			conn.Close()
			return
		case <-ctx.Done():
			delete(mes.clients, addr)
			conn.Close()
			return
		}
	}
}

func makeMsg(cli client, msg string) []byte {
	var buf bytes.Buffer
	buf.WriteString("[")
	buf.WriteString(cli.address)
	buf.WriteString("]: ")
	buf.WriteString(msg)
	return buf.Bytes()
}

func writeToClient(ctx context.Context, cli *client, conn net.Conn) {
	for {
		select {
		case msg := <-cli.c:
			_, _ = conn.Write(append(msg, '\n'))
		case <-ctx.Done():
			return
		}
	}
}

func clearChan(ch chan []byte) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}
