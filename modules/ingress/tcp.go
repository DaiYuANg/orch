package ingress

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

type tcpListener struct {
	listenAddr string
	backend    string

	listener net.Listener
	stopCh   chan struct{}
}

func (i *Ingress) registerTCP(route Route) error {
	if route.ListenPort == 0 || route.Backend == "" {
		return errors.New("tcp route must specify ListenPort and Backend")
	}

	addr := fmt.Sprintf(":%d", route.ListenPort)

	// 如果已有监听，判断是否backend更新
	if l, ok := i.tcpListeners[route.ListenPort]; ok {
		if l.backend != route.Backend {
			l.backend = route.Backend
		}
		return nil
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen tcp %s: %w", addr, err)
	}

	tl := &tcpListener{
		listenAddr: addr,
		backend:    route.Backend,
		listener:   ln,
		stopCh:     make(chan struct{}),
	}
	i.tcpListeners[route.ListenPort] = tl

	go tl.serve()
	return nil
}

func (tl *tcpListener) serve() {
	for {
		conn, err := tl.listener.Accept()
		if err != nil {
			select {
			case <-tl.stopCh:
				return
			default:
				log.Printf("tcp accept error: %v", err)
				continue
			}
		}
		go tl.handleConn(conn)
	}
}

func (tl *tcpListener) handleConn(client net.Conn) {
	defer client.Close()

	backendConn, err := net.DialTimeout("tcp", tl.backend, 5*time.Second)
	if err != nil {
		log.Printf("tcp dial backend error: %v", err)
		return
	}
	defer backendConn.Close()

	// 双向拷贝
	doneCh := make(chan struct{})
	go func() {
		io.Copy(backendConn, client)
		backendConn.(*net.TCPConn).CloseWrite()
		doneCh <- struct{}{}
	}()
	go func() {
		io.Copy(client, backendConn)
		client.(*net.TCPConn).CloseWrite()
		doneCh <- struct{}{}
	}()
	<-doneCh
	<-doneCh
}

func (i *Ingress) unregisterTCP(route Route) error {
	tl, ok := i.tcpListeners[route.ListenPort]
	if !ok {
		return nil
	}

	close(tl.stopCh)
	tl.listener.Close()
	delete(i.tcpListeners, route.ListenPort)
	return nil
}
