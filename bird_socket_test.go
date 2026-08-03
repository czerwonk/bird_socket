package birdsocket

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainsActionCompletedCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data string
		want bool
	}{
		{name: "greeting", data: "0001 BIRD 3.0 ready.\n", want: true},
		{name: "success without message", data: "2002-table\n0000\n", want: true},
		{name: "success with message", data: "0013 Daemon is up and running\n", want: true},
		{name: "runtime error", data: "8000 Reply too long\n", want: true},
		{name: "syntax error", data: "9000 Command too long\n", want: true},
		{name: "continuation", data: "0000-still continuing\n", want: false},
		{name: "partial terminal line", data: "2002-table\n0000", want: false},
		{name: "digits inside name", data: "peer0900 BGP master up\n", want: false},
		{name: "no terminal code", data: "1002-peer Device master up\n", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, containsActionCompletedCode([]byte(tt.data)))
		})
	}
}

func TestTerminalReply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		line     string
		code     int
		message  string
		terminal bool
	}{
		{line: "0000", code: 0, terminal: true},
		{line: "0001 ready", code: 1, message: "ready", terminal: true},
		{line: "0013 Daemon is up", code: 13, message: "Daemon is up", terminal: true},
		{line: "8000 runtime", code: 8000, message: "runtime", terminal: true},
		{line: "9000 syntax", code: 9000, message: "syntax", terminal: true},
		{line: "0000-continuation", terminal: false},
		{line: "peer0900", terminal: false},
		{line: "000", terminal: false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			t.Parallel()
			code, message, terminal := terminalReply([]byte(tt.line))
			assert.Equal(t, tt.code, code)
			assert.Equal(t, tt.message, message)
			assert.Equal(t, tt.terminal, terminal)
		})
	}
}

func TestQueryContextHandlesEveryTerminalSplit(t *testing.T) {
	response := "2002-name proto table state since info\npeer0900 BGP master up now Established\n0000\n"

	for split := 1; split < len(response); split++ {
		t.Run("split_"+strconv.Itoa(split), func(t *testing.T) {
			chunks := []string{response[:split], response[split:]}
			path, wait := startUnixServer(t, chunks, false)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			got, err := QueryContext(ctx, path, "show protocols all", WithBufferSize(7))
			require.NoError(t, err)
			assert.Equal(t, response, string(got))
			wait()
		})
	}
}

func TestQueryContextReplyError(t *testing.T) {
	path, wait := startUnixServer(t, []string{"9000 Bad command\n"}, false)

	got, err := QueryContext(context.Background(), path, "bad", WithTimeout(time.Second))
	var replyErr *ReplyError
	require.ErrorAs(t, err, &replyErr)
	assert.Equal(t, 9000, replyErr.Code)
	assert.Equal(t, "Bad command", replyErr.Message)
	assert.Equal(t, "9000 Bad command\n", string(got))
	wait()
}

func TestQueryContextResponseLimit(t *testing.T) {
	path, wait := startUnixServer(t, []string{strings.Repeat("x", 128)}, false)

	got, err := QueryContext(
		context.Background(),
		path,
		"show protocols all",
		WithTimeout(time.Second),
		WithBufferSize(32),
		WithMaxResponseBytes(64),
	)
	require.ErrorIs(t, err, ErrResponseTooLarge)
	assert.LessOrEqual(t, len(got), 64)
	wait()
}

func TestQueryContextDeadline(t *testing.T) {
	path, wait := startUnixServer(t, []string{"2002-incomplete\n"}, true)

	started := time.Now()
	_, err := QueryContext(
		context.Background(),
		path,
		"show protocols all",
		WithTimeout(50*time.Millisecond),
	)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(started), time.Second)
	wait()
}

func TestQueryContextCancellation(t *testing.T) {
	path, wait := startUnixServer(t, []string{"2002-incomplete\n"}, true)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(25*time.Millisecond, cancel)

	_, err := QueryContext(ctx, path, "show protocols all", WithTimeout(time.Second))
	require.ErrorIs(t, err, context.Canceled)
	wait()
}

func TestQueryRequiresConnection(t *testing.T) {
	socket := NewSocket("/does/not/matter")
	_, err := socket.Query("show status")
	require.ErrorIs(t, err, ErrNotConnected)
}

func TestInvalidOptions(t *testing.T) {
	tests := []struct {
		name   string
		option Option
	}{
		{name: "buffer", option: WithBufferSize(0)},
		{name: "timeout", option: WithTimeout(0)},
		{name: "response limit", option: WithMaxResponseBytes(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			socket := NewSocket("/tmp/bird.ctl", tt.option)
			_, err := socket.Connect()
			require.Error(t, err)
		})
	}
}

func startUnixServer(t *testing.T, responseChunks []string, holdOpen bool) (string, func()) {
	t.Helper()

	path := filepath.Join(os.TempDir(), fmt.Sprintf("bird_socket_%d_%d.sock", os.Getpid(), time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.Remove(path) })
	listener, err := net.Listen("unix", path)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			done <- acceptErr
			return
		}
		defer conn.Close()

		if _, writeErr := conn.Write([]byte("0001 BIRD ready.\n")); writeErr != nil {
			done <- writeErr
			return
		}
		if _, readErr := bufio.NewReader(conn).ReadString('\n'); readErr != nil {
			done <- readErr
			return
		}

		for _, chunk := range responseChunks {
			if _, writeErr := conn.Write([]byte(chunk)); writeErr != nil {
				if holdOpen && errors.Is(writeErr, net.ErrClosed) {
					return
				}
				done <- writeErr
				return
			}
		}

		if holdOpen {
			buffer := make([]byte, 1)
			_, _ = conn.Read(buffer)
		}
	}()

	return path, func() {
		t.Helper()
		require.NoError(t, listener.Close())
		select {
		case serverErr := <-done:
			require.NoError(t, serverErr)
		case <-time.After(time.Second):
			t.Fatal("test Unix server did not stop")
		}
	}
}
