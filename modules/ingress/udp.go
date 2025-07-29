package ingress

import (
	"errors"
	"fmt"
	"log"
	"net"
	"time"
)

type udpListener struct {
	listenAddr string
	backend    *net.UDPAddr

	conn   *net.UDPConn
	stopCh chan struct{}
	// 简单的客户端地址映射可自行扩展
}

func (i *Ingress) registerUDP(route Route) error {
	if route.ListenPort == 0 || route.Backend == "" {
		return errors.New("udp route must specify ListenPort and Backend")
	}

	addr := fmt.Sprintf(":%d", route.ListenPort)

	if ul, ok := i.udpListeners[route.ListenPort]; ok {
		// 简单只更新 backend
		backendAddr, err := net.ResolveUDPAddr("udp", route.Backend)
		if err != nil {
			return err
		}
		ul.backend = backendAddr
		return nil
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	backendAddr, err := net.ResolveUDPAddr("udp", route.Backend)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}

	ul := &udpListener{
		listenAddr: addr,
		backend:    backendAddr,
		conn:       conn,
		stopCh:     make(chan struct{}),
	}

	i.udpListeners[route.ListenPort] = ul
	go ul.serve()
	return nil
}

func (ul *udpListener) serve() {
	buf := make([]byte, 64*1024)
	for {
		n, clientAddr, err := ul.conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-ul.stopCh:
				return
			default:
				log.Printf("udp read error: %v", err)
				continue
			}
		}

		go ul.handlePacket(clientAddr, buf[:n])
	}
}

func (ul *udpListener) handlePacket(clientAddr *net.UDPAddr, data []byte) {
	backendConn, err := net.DialUDP("udp", nil, ul.backend)
	if err != nil {
		log.Printf("udp dial backend error: %v", err)
		return
	}
	defer backendConn.Close()

	_, err = backendConn.Write(data)
	if err != nil {
		log.Printf("udp write to backend error: %v", err)
		return
	}

	resp := make([]byte, 64*1024)
	ul.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, _, err := backendConn.ReadFrom(resp)
	if err != nil {
		log.Printf("udp read from backend error: %v", err)
		return
	}

	_, err = ul.conn.WriteToUDP(resp[:n], clientAddr)
	if err != nil {
		log.Printf("udp write to client error: %v", err)
		return
	}
}

func (i *Ingress) unregisterUDP(route Route) error {
	ul, ok := i.udpListeners[route.ListenPort]
	if !ok {
		return nil
	}
	close(ul.stopCh)
	ul.conn.Close()
	delete(i.udpListeners, route.ListenPort)
	return nil
}
