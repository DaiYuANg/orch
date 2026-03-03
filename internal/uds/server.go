//go:build linux
// +build linux

package uds

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"

	"github.com/panjf2000/ants/v2"
)

// HandlerFunc 定义服务器端处理函数，接收客户端消息，返回响应
type HandlerFunc func(msg string) (response string, err error)

// Server UDSServer 简单 UDS 服务器封装
type Server struct {
	socketPath string
	listener   net.Listener
	handler    HandlerFunc
	wg         sync.WaitGroup
	closed     bool
	mu         sync.Mutex
	pool       *ants.Pool
	logger     *slog.Logger
}

// NewServer 创建服务器
func NewServer(socketPath string, handler HandlerFunc, pool *ants.Pool, logger *slog.Logger) *Server {
	return &Server{
		socketPath: socketPath,
		handler:    handler,
		pool:       pool,
		logger:     logger,
	}
}

// Start 开始监听
func (s *Server) Start() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("server closed")
	}
	s.mu.Unlock()

	if err := os.RemoveAll(s.socketPath); err != nil {
		return fmt.Errorf("remove socket file: %w", err)
	}
	l, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listen unix socket: %w", err)
	}
	s.listener = l

	s.wg.Add(1)
	return s.pool.Submit(s.acceptLoop)
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			if s.closed {
				s.mu.Unlock()
				return
			}
			s.mu.Unlock()
			fmt.Println("accept error:", err)
			continue
		}

		s.wg.Add(1)
		err = s.pool.Submit(func() {
			s.handleConn(conn)
		})
		if err != nil {
			s.logger.Error("submit conn", "error", err)
		}
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		req := scanner.Text()
		resp, err := s.handler(req)
		if err != nil {
			resp = fmt.Sprintf("error: %v", err)
		}
		_, err = fmt.Fprintf(conn, "%s\n", resp)
		if err != nil {
			return
		}
	}
}

// Stop 停止服务器
func (s *Server) Stop() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	return os.Remove(s.socketPath)
}
