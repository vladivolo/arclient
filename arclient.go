package arclient

import (
	"bytes"
	"fmt"
	"net"
	"time"
)

type Client struct {
	Conn          net.Conn
	rbuf          *bytes.Buffer
	maxRetries    int           // Default 10
	retryInterval time.Duration // Default 1sec
}

func Dial(network, addr string) (*Client, error) {
	conn, err := net.Dial(network, addr)
	if err != nil {
		return nil, err
	}

	return &Client{Conn: conn, rbuf: bytes.NewBuffer([]byte{}), maxRetries: 10, retryInterval: 1 * time.Second}, nil
}

func (c *Client) reconnect() error {
	conn, err := net.Dial(c.Conn.RemoteAddr().Network(), c.Conn.RemoteAddr().String())
	if err != nil {
		return err
	}

	c.Conn.Close()
	c.Conn = conn

	c.Conn.SetReadDeadline(time.Now().Add(c.retryInterval))

	return nil
}

func (c *Client) ReadString(delim byte) (string, error) {
	if s, err := c.rbuf.ReadString(delim); err == nil {
		return s, nil
	}

	disconnected := false
	t := c.retryInterval
	b := []byte{}

	for i := 0; i < c.maxRetries; i++ {
		if disconnected {
			time.Sleep(t)
			t *= 2

			if err := c.reconnect(); err != nil {
				continue
			}

			disconnected = false
		}

		if _, err := c.Conn.Read(b); err != nil {
			// Check for timeout & return
			if err, ok := err.(net.Error); ok && err.Timeout() {
				return "", fmt.Errorf("ErrMaxRetries")
			}

			disconnected = true
			continue
		}

		c.rbuf.Write(b)

		return c.ReadString(delim)
	}

	return "", fmt.Errorf("ErrMaxRetries")
}

func (c *Client) Write(message []byte) error {
	if _, err := c.Conn.Write(message); err == nil {
		return nil
	}

	disconnected := true
	t := c.retryInterval

	for i := 0; i < c.maxRetries; i++ {
		if disconnected {
			time.Sleep(t)
			t *= 2

			if err := c.reconnect(); err != nil {
				continue
			}
		}

		return c.Write(message)
	}

	return fmt.Errorf("ErrMaxRetries")
}
