package birdsocket

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBufferSize       = 4096
	defaultTimeout          = 30 * time.Second
	defaultMaxResponseBytes = 64 << 20
)

var (
	// ErrNotConnected is returned when a query is attempted before Connect.
	ErrNotConnected = errors.New("bird socket is not connected")
	// ErrResponseTooLarge is returned before a response can exceed the configured limit.
	ErrResponseTooLarge = errors.New("bird socket response exceeds configured limit")
)

// ReplyError represents a terminal BIRD 8xxx or 9xxx reply.
type ReplyError struct {
	Code    int
	Message string
}

func (e *ReplyError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("bird command failed with reply code %04d", e.Code)
	}

	return fmt.Sprintf("bird command failed with reply code %04d: %s", e.Code, e.Message)
}

// BirdSocket encapsulates communication with the BIRD routing daemon.
// A BirdSocket must not be used concurrently.
type BirdSocket struct {
	socketPath       string
	bufferSize       int
	timeout          time.Duration
	maxResponseBytes int
	conn             net.Conn
}

// Option applies an option to BirdSocket.
type Option func(*BirdSocket)

// WithBufferSize sets the socket read buffer size.
func WithBufferSize(bufferSize int) Option {
	return func(s *BirdSocket) {
		s.bufferSize = bufferSize
	}
}

// WithTimeout sets the maximum duration of a connect or query operation.
func WithTimeout(timeout time.Duration) Option {
	return func(s *BirdSocket) {
		s.timeout = timeout
	}
}

// WithMaxResponseBytes sets the maximum accepted size of one BIRD reply.
func WithMaxResponseBytes(maxResponseBytes int) Option {
	return func(s *BirdSocket) {
		s.maxResponseBytes = maxResponseBytes
	}
}

// NewSocket creates a new socket client.
func NewSocket(socketPath string, opts ...Option) *BirdSocket {
	socket := &BirdSocket{
		socketPath:       socketPath,
		bufferSize:       defaultBufferSize,
		timeout:          defaultTimeout,
		maxResponseBytes: defaultMaxResponseBytes,
	}

	for _, option := range opts {
		option(socket)
	}

	return socket
}

// Query sends an ad hoc query to BIRD and waits for the reply.
func Query(socketPath, query string) ([]byte, error) {
	return QueryContext(context.Background(), socketPath, query)
}

// QueryContext sends an ad hoc query to BIRD and bounds the whole connect-query operation.
func QueryContext(ctx context.Context, socketPath, query string, opts ...Option) ([]byte, error) {
	socket := NewSocket(socketPath, opts...)
	ctx, cancel, err := socket.operationContext(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()

	if _, err = socket.connect(ctx); err != nil {
		return nil, err
	}
	defer socket.Close()

	return socket.query(ctx, query)
}

// Connect connects to the BIRD Unix socket and reads its greeting.
func (s *BirdSocket) Connect() ([]byte, error) {
	return s.ConnectContext(context.Background())
}

// ConnectContext connects to the BIRD Unix socket and reads its greeting.
func (s *BirdSocket) ConnectContext(ctx context.Context) ([]byte, error) {
	ctx, cancel, err := s.operationContext(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()

	return s.connect(ctx)
}

func (s *BirdSocket) connect(ctx context.Context) ([]byte, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}

	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", s.socketPath)
	if err != nil {
		return nil, normalizeContextError(ctx, err)
	}

	if s.conn != nil {
		_ = s.conn.Close()
	}
	s.conn = conn

	greeting, err := s.readReply(ctx, conn)
	if err != nil {
		_ = conn.Close()
		s.conn = nil
		return greeting, err
	}

	return greeting, nil
}

// Close closes the connection to the socket.
func (s *BirdSocket) Close() {
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
}

// Query sends a query to BIRD and waits for the reply.
func (s *BirdSocket) Query(query string) ([]byte, error) {
	return s.QueryContext(context.Background(), query)
}

// QueryContext sends a query to an already connected BIRD socket.
func (s *BirdSocket) QueryContext(ctx context.Context, query string) ([]byte, error) {
	ctx, cancel, err := s.operationContext(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()

	return s.query(ctx, query)
}

func (s *BirdSocket) query(ctx context.Context, query string) ([]byte, error) {
	if s.conn == nil {
		return nil, ErrNotConnected
	}

	stop, err := watchContext(ctx, s.conn)
	if err != nil {
		return nil, err
	}
	_, writeErr := io.WriteString(s.conn, strings.TrimRight(query, "\n")+"\n")
	stop()
	if writeErr != nil {
		return nil, normalizeContextError(ctx, writeErr)
	}

	return s.readReply(ctx, s.conn)
}

func (s *BirdSocket) readReply(ctx context.Context, conn net.Conn) ([]byte, error) {
	stop, err := watchContext(ctx, conn)
	if err != nil {
		return nil, err
	}
	defer stop()

	response := make([]byte, 0, min(s.bufferSize, s.maxResponseBytes))
	buffer := make([]byte, s.bufferSize)
	scanOffset := 0

	for {
		n, readErr := conn.Read(buffer)
		if n > 0 {
			if n > s.maxResponseBytes-len(response) {
				return response, fmt.Errorf("%w: limit=%d", ErrResponseTooLarge, s.maxResponseBytes)
			}

			response = append(response, buffer[:n]...)
			for {
				relativeEnd := bytes.IndexByte(response[scanOffset:], '\n')
				if relativeEnd < 0 {
					break
				}

				lineEnd := scanOffset + relativeEnd
				line := bytes.TrimSuffix(response[scanOffset:lineEnd], []byte{'\r'})
				scanOffset = lineEnd + 1

				code, message, terminal := terminalReply(line)
				if !terminal {
					continue
				}

				if code >= 8000 {
					return response, &ReplyError{Code: code, Message: message}
				}

				return response, nil
			}
		}

		if readErr != nil {
			return response, normalizeContextError(ctx, readErr)
		}
		if n == 0 {
			return response, io.ErrNoProgress
		}
	}
}

func terminalReply(line []byte) (int, string, bool) {
	if len(line) < 4 || (line[0] != '0' && line[0] != '8' && line[0] != '9') {
		return 0, "", false
	}
	for i := 0; i < 4; i++ {
		if line[i] < '0' || line[i] > '9' {
			return 0, "", false
		}
	}
	if len(line) > 4 && line[4] != ' ' {
		return 0, "", false
	}

	code, _ := strconv.Atoi(string(line[:4]))
	message := ""
	if len(line) > 5 {
		message = string(line[5:])
	}

	return code, message, true
}

func containsActionCompletedCode(data []byte) bool {
	for len(data) > 0 {
		lineEnd := bytes.IndexByte(data, '\n')
		if lineEnd < 0 {
			return false
		}

		line := bytes.TrimSuffix(data[:lineEnd], []byte{'\r'})
		if _, _, terminal := terminalReply(line); terminal {
			return true
		}
		data = data[lineEnd+1:]
	}

	return false
}

func (s *BirdSocket) operationContext(parent context.Context) (context.Context, context.CancelFunc, error) {
	if parent == nil {
		return nil, nil, errors.New("bird socket context must not be nil")
	}
	if err := s.validate(); err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(parent, s.timeout)
	return ctx, cancel, nil
}

func (s *BirdSocket) validate() error {
	switch {
	case s.socketPath == "":
		return errors.New("bird socket path must not be empty")
	case s.bufferSize <= 0:
		return errors.New("bird socket buffer size must be positive")
	case s.timeout <= 0:
		return errors.New("bird socket timeout must be positive")
	case s.maxResponseBytes <= 0:
		return errors.New("bird socket response limit must be positive")
	default:
		return nil
	}
}

func watchContext(ctx context.Context, conn net.Conn) (func(), error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return nil, errors.New("bird socket operation context must have a deadline")
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return nil, err
	}

	stopAfterFunc := context.AfterFunc(ctx, func() {
		_ = conn.SetDeadline(time.Now())
	})

	return func() {
		stopAfterFunc()
		_ = conn.SetDeadline(time.Time{})
	}, nil
}

func normalizeContextError(ctx context.Context, err error) error {
	if contextErr := ctx.Err(); contextErr != nil {
		return contextErr
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		if deadline, hasDeadline := ctx.Deadline(); hasDeadline && !time.Now().Before(deadline) {
			return context.DeadlineExceeded
		}
	}

	return err
}
