package grpcclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// Config defines settings
type Config struct {
	DialTimeout   time.Duration
	CallTimeout   time.Duration
	MaxRetries    int
	BackoffFactor float64
	PoolSize      int
	StreamRetry   int           // تعداد تلاش reconnect برای streaming
	StreamBackoff time.Duration // backoff برای reconnect stream
}

// WrapperClient handles both short RPC and streaming
type WrapperClient struct {
	pool       []*grpc.ClientConn
	poolCfg    Config
	streamConn *grpc.ClientConn
	streamMu   sync.Mutex
	poolMu     sync.Mutex
	next       int
	serverURL  string
	opts       []grpc.DialOption
}

// NewWrapperClient creates a new WrapperClient with pool and streaming connection
func NewWrapperClient(serverURL string, cfg Config, opts ...grpc.DialOption) (*WrapperClient, error) {
	if len(opts) == 0 {
		opts = []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	}

	client := &WrapperClient{
		serverURL: serverURL,
		opts:      opts,
		poolCfg:   cfg,
		pool:      make([]*grpc.ClientConn, cfg.PoolSize),
	}

	// Build pool for short RPC
	for i := 0; i < cfg.PoolSize; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
		conn, err := grpc.DialContext(ctx, serverURL, opts...)
		cancel() // cancel immediately after DialContext
		if err != nil {
			return nil, fmt.Errorf("failed to dial short RPC connection: %w", err)
		}
		client.pool[i] = conn
	}

	// Build initial streaming connection
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	streamConn, err := grpc.DialContext(ctx, serverURL, opts...)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("failed to dial streaming connection: %w", err)
	}
	client.streamConn = streamConn

	return client, nil
}

// ShortCall executes a short RPC call with pool, retry, and backoff
func (c *WrapperClient) ShortCall(ctx context.Context, rpcFunc func(ctx context.Context, conn *grpc.ClientConn) (interface{}, error)) (interface{}, error) {
	var lastErr error
	timeout := c.poolCfg.CallTimeout
	backoff := 100 * time.Millisecond

	for attempt := 0; attempt <= c.poolCfg.MaxRetries; attempt++ {
		conn := c.getPoolConn()
		ctxCall, cancel := context.WithTimeout(ctx, timeout)
		resp, err := rpcFunc(ctxCall, conn)
		cancel()

		if err == nil {
			return resp, nil
		}
		lastErr = err

		st, ok := status.FromError(err)
		if !ok || !isRetryable(st.Code()) {
			break
		}

		time.Sleep(backoff)
		backoff = time.Duration(float64(backoff) * c.poolCfg.BackoffFactor)
	}

	return nil, fmt.Errorf("ShortCall failed after retries: %w", lastErr)
}

// isRetryable returns true if the error code is retryable
func isRetryable(code codes.Code) bool {
	switch code {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
		return true
	default:
		return false
	}
}

// getPoolConn returns next connection from the pool (thread-safe)
func (c *WrapperClient) getPoolConn() *grpc.ClientConn {
	c.poolMu.Lock()
	conn := c.pool[c.next]
	c.next = (c.next + 1) % len(c.pool)
	c.poolMu.Unlock()
	return conn
}

// StreamCall executes a long-lived streaming call with auto-reconnect
func (c *WrapperClient) StreamCall(rpcFunc func(ctx context.Context, conn *grpc.ClientConn) error) error {
	var err error
	for attempt := 0; attempt <= c.poolCfg.StreamRetry; attempt++ {
		c.streamMu.Lock()
		conn := c.streamConn
		c.streamMu.Unlock()

		ctx := context.Background()
		err = rpcFunc(ctx, conn)
		if err == nil {
			return nil
		}

		// Reconnect logic
		time.Sleep(c.poolCfg.StreamBackoff)
		newConn, dialErr := grpc.DialContext(ctx, c.serverURL, c.opts...)
		if dialErr != nil {
			err = fmt.Errorf("stream reconnect failed: %w", dialErr)
			continue
		}

		c.streamMu.Lock()
		c.streamConn.Close()
		c.streamConn = newConn
		c.streamMu.Unlock()
	}
	return fmt.Errorf("StreamCall failed after reconnects: %w", err)
}

// Close closes all connections (pool + stream)
func (c *WrapperClient) Close() {
	for _, conn := range c.pool {
		if conn != nil {
			conn.Close()
		}
	}
	c.streamMu.Lock()
	if c.streamConn != nil {
		c.streamConn.Close()
	}
	c.streamMu.Unlock()
}
