package birdsocket

import (
	"net"
	"regexp"
	"strings"
)

var birdReturnCodeRegex *regexp.Regexp

func init() {
	// Requests are commands encoded as a single line of text,
	// replies are sequences of lines starting with a four-digit code
	// followed by either a space (if it's the last line of the reply)
	// or a minus sign (when the reply is going to continue with the next line),
	// the rest of the line contains a textual message semantics of which depends
	// on the numeric code.

	// https://gitlab.nic.cz/labs/bird/-/blob/master/doc/reply_codes
	birdReturnCodeRegex = regexp.MustCompile(`(?m)^([089]\d{3})`)
}

// BirdSocket encapsulates communication with Bird routing daemon
type BirdSocket struct {
	socketPath string
	bufferSize int
	conn       net.Conn
}

// BirdSocketOption applies options to BirdSocket
type Option func(*BirdSocket)

// WithBufferSize sets the buffer size for BirdSocket
func WithBufferSize(bufferSize int) Option {
	return func(s *BirdSocket) {
		s.bufferSize = bufferSize
	}
}

// NewSocket creates a new socket
func NewSocket(socketPath string, opts ...Option) *BirdSocket {
	socket := &BirdSocket{socketPath: socketPath, bufferSize: 4096}

	for _, o := range opts {
		o(socket)
	}

	return socket
}

// Query sends an ad hoc query to Bird and waits for the reply
func Query(socketPath, qry string) ([]byte, error) {
	s := NewSocket(socketPath)
	_, err := s.Connect()
	if err != nil {
		return nil, err
	}
	defer s.Close()

	return s.Query(qry)
}

// Connect connects to the Bird unix socket
func (s *BirdSocket) Connect() ([]byte, error) {
	var err error
	s.conn, err = net.Dial("unix", s.socketPath)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, s.bufferSize)
	n, err := s.conn.Read(buf[:])
	if err != nil {
		return nil, err
	}

	return buf[:n], err
}

// Close closes the connection to the socket
func (s *BirdSocket) Close() {
	if s.conn != nil {
		s.conn.Close()
	}
}

// Query sends an query to Bird and waits for the reply
func (s *BirdSocket) Query(qry string) ([]byte, error) {
	_, err := s.conn.Write([]byte(strings.Trim(qry, "\n") + "\n"))
	if err != nil {
		return nil, err
	}

	output, err := s.readFromSocket(s.conn)
	if err != nil {
		return nil, err
	}

	return output, nil
}

func (s *BirdSocket) readFromSocket(conn net.Conn) ([]byte, error) {
	b := make([]byte, 0)
	buf := make([]byte, s.bufferSize)

	done := false
	for !done {
		n, err := conn.Read(buf[:])
		if err != nil {
			return nil, err
		}

		b = append(b, buf[:n]...)
		done = containsActionCompletedCode(buf[:n])
	}

	return b, nil
}

func containsActionCompletedCode(b []byte) bool {
	return birdReturnCodeRegex.Match(b)
}
