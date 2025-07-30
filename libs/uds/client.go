package uds

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"sync"
)

// Client 简单 UDS 客户端封装
type Client struct {
	socketPath string
	conn       net.Conn
	mu         sync.Mutex
	scanner    *bufio.Scanner
}

// NewClient 创建客户端
func NewClient(socketPath string) *Client {
	return &Client{
		socketPath: socketPath,
	}
}

// Connect 连接服务器
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return errors.New("already connected")
	}
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return err
	}
	c.conn = conn
	c.scanner = bufio.NewScanner(conn)
	return nil
}

// Send 发送消息并等待响应
func (c *Client) Send(msg string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return "", errors.New("not connected")
	}
	_, err := fmt.Fprintf(c.conn, "%s\n", msg)
	if err != nil {
		return "", err
	}
	if c.scanner.Scan() {
		return c.scanner.Text(), nil
	}
	return "", c.scanner.Err()
}

// Close 关闭连接
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}
