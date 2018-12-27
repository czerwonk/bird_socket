package birdsocket

import (
	"testing"

	"github.com/czerwonk/testutils/assert"
)

func TestBirdSocketConnection(t *testing.T) {
	out := "0001 BIRD 1.6.4 ready.\n"
	completed := isBirdCommandCompleted([]byte(out))

	assert.True("Connect completed", completed, t)
}

func TestBirdShowProtocols(t *testing.T) {
	out := "show protocols\n" +
		"2002-name     proto    table    state  since       info\n" +
		"1002-device1  Device   master   up     2018-12-21 12:35:11\n" +
		" direct1  Direct   master   up     2018-12-21 12:35:11\n" +
		" kernel1  Kernel   master   up     2018-12-21 12:35:11\n" +
		" SAR2     BGP      master   down   2018-12-21 12:35:11  Error: Invalid next hop\n" +
		" SAR1     BGP      master   down   2018-12-21 12:35:11  Error: Invalid next hop\n" +
		"0000\n"
	completed := isBirdCommandCompleted([]byte(out))

	assert.True("'show protocols' completed", completed, t)
}

func TestIncompleteBirdShowProtocols(t *testing.T) {
	out := "show protocols\n" +
		"2002-name     proto    table    state  since       info\n" +
		"1002-device1  Device   master   up     2018-12-21 12:35:11\n" +
		" direct1  Direct   master   up     2018-12-21 12:35:11\n" +
		" kernel1  Kernel   master   up     2018-12-21 12:35:11\n"
	completed := isBirdCommandCompleted([]byte(out))

	assert.False("'show protocols' completed", completed, t)
}

func TestBirdShowStatus(t *testing.T) {
	out := "show status\n" +
		"1000-BIRD 1.6.4\n" +
		"1011-Router ID is 192.168.1.9\n" +
		" Current server time is 2018-12-27 12:15:01\n" +
		" Last reboot on 2018-12-21 12:35:11\n" +
		" Last reconfiguration on 2018-12-21 12:35:11\n" +
		"0013 Daemon is up and running\n"
	completed := isBirdCommandCompleted([]byte(out))

	assert.True("'show protocols' completed", completed, t)
}

func TestIncompleteBirdShowStatus(t *testing.T) {
	out := "show status\n" +
		"1000-BIRD 1.6.4\n" +
		"1011-Router ID is 192.168.1.9\n" +
		" Current server time is 2018-12-27 12:15:01\n" +
		" Last reboot on 2018-12-21 12:35:11\n"
	completed := isBirdCommandCompleted([]byte(out))

	assert.False("'show protocols' completed", completed, t)
}
