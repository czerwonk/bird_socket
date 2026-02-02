package birdsocket

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBirdSocketConnection simulate a scenario in
// which the BirdSocket read buffer contains the
// string sent in output by Bird whenever a new
// connection is initiated
func TestBirdSocketConnection(t *testing.T) {
	out := "0001 BIRD 1.6.4 ready.\n"
	completed := containsActionCompletedCode([]byte(out))

	assert.True(t, completed, "'connect' successfully completed")
}

// TestBirdShowProtocols simulate a scenario in which
// the full output of the 'show protocols' command is
// saved in the BirdSocket read buffer
func TestBirdShowProtocols(t *testing.T) {
	out := "2002-name     proto    table    state  since       info\n" +
		"1002-device1  Device   master   up     2018-12-21 12:35:11\n" +
		" direct1  Direct   master   up     2018-12-21 12:35:11\n" +
		" kernel1  Kernel   master   up     2018-12-21 12:35:11\n" +
		" SAR2     BGP      master   down   2018-12-21 12:35:11  Error: Invalid next hop\n" +
		" SAR1     BGP      master   down   2018-12-21 12:35:11  Error: Invalid next hop\n" +
		"0000\n"
	completed := containsActionCompletedCode([]byte(out))

	assert.True(t, completed, "'show protocols' successfully completed")
}

// TestIncompleteBirdShowProtocols simulate a scenario in which
// the 'action successfully completed' Bird response code
// is not present in the output of the 'show protocols' command
// saved in the BirdSocket read buffer
func TestIncompleteBirdShowProtocols(t *testing.T) {
	out := "1002-device1  Device   master   up     2018-12-21 12:35:11\n" +
		" direct1  Direct   master   up     2018-12-21 12:35:11\n" +
		" kernel1  Kernel   master   up     2018-12-21 12:35:11\n"
	completed := containsActionCompletedCode([]byte(out))

	assert.False(t, completed, "'show protocols' successfully completed")
}

// TestTruncatedBirdShowProtocols simulate a scenario in which
// the 'action successfully completed' Bird response code
// is present in the output of the 'show protocols' command
// saved in the BirdSocket read buffer,
// however the response of the Bird deamon is not yet completed
// (the final new line is not present in the buffer)
func TestTruncatedBirdShowProtocols(t *testing.T) {
	out := "1002-device1  Device   master   up     2018-12-21 12:35:11\n" +
		" direct1  Direct   master   up     2018-12-21 12:35:11\n" +
		" kernel1  Kernel   master   up     2018-12-21 12:35:11\n" +
		" SAR2     BGP      master   down   2018-12-21 12:35:11  Error: Invalid next hop\n" +
		" SAR1     BGP      master   down   2018-12-21 12:35:11  Error: Invalid next hop\n" +
		"0000"
	completed := containsActionCompletedCode([]byte(out))

	assert.True(t, completed, "'show protocols' successfully completed")
}

// TestBirdShowStatus simulate a scenario in which
// the full output of the 'show status' command is
// saved in the BirdSocket read buffer
func TestBirdShowStatus(t *testing.T) {
	out := "1000-BIRD 1.6.4\n" +
		"1011-Router ID is 192.168.1.9\n" +
		" Current server time is 2018-12-27 12:15:01\n" +
		" Last reboot on 2018-12-21 12:35:11\n" +
		" Last reconfiguration on 2018-12-21 12:35:11\n" +
		"0013 Daemon is up and running\n"
	completed := containsActionCompletedCode([]byte(out))

	assert.True(t, completed, "'show status' successfully completed")
}

// TestIncompleteBirdShowStatus simulate a scenario in which
// the 'action successfully completed' Bird response code
// is not present in the output of the 'show status' command
// saved in the BirdSocket read buffer
func TestIncompleteBirdShowStatus(t *testing.T) {
	out := "1011-Router ID is 192.168.1.9\n" +
		" Current server time is 2018-12-27 12:15:01\n" +
		" Last reboot on 2018-12-21 12:35:11\n"
	completed := containsActionCompletedCode([]byte(out))

	assert.False(t, completed, "'show status' successfully completed")
}

// TestTruncatedBirdShowStatus simulate a scenario in which
// the 'action successfully completed' Bird response code
// is present in the output of the 'show status' command
// saved in the BirdSocket read buffer,
// however the response of the Bird deamon is not yet completed
// (the final new line is not present in the buffer)
func TestTruncatedBirdShowStatus(t *testing.T) {
	out := "1011-Router ID is 192.168.1.9\n" +
		" Current server time is 2018-12-27 12:15:01\n" +
		" Last reboot on 2018-12-21 12:35:11\n" +
		" Last reconfiguration on 2018-12-21 12:35:11\n" +
		"0013 Daemon is up and running"
	completed := containsActionCompletedCode([]byte(out))

	assert.True(t, completed, "'show status' successfully completed")
}

func TestExitCodes(t *testing.T) {
	testCases := []struct {
		out string
	}{
		{
			out: "9000 Command too long",
		},
		{
			out: "8000 Reply too long",
		},
	}
	for _, tc := range testCases {
		completed := containsActionCompletedCode([]byte(tc.out))
		assert.True(t, completed, "exit codes completed")
	}
}
