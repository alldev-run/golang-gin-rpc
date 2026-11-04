package discovery

import (
	"testing"

	"github.com/alldev-run/golang-gin-rpc/plugins/actor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStart_SpawnsRegistryActor(t *testing.T) {
	sys := actor.New()
	defer sys.Shutdown()

	pid, err := Start(sys, newMockDiscovery(), "registry-under-test")
	require.NoError(t, err)
	require.NotNil(t, pid)

	// The spawned registry actor must be running and addressable in the system.
	assert.True(t, sys.Running())
}

func TestStart_DuplicateNameFails(t *testing.T) {
	sys := actor.New()
	defer sys.Shutdown()

	_, err := Start(sys, newMockDiscovery(), "registry-dup")
	require.NoError(t, err)

	_, err = Start(sys, newMockDiscovery(), "registry-dup")
	assert.Error(t, err, "spawning a second actor with the same name should fail")
}

func TestError_Format(t *testing.T) {
	err := Error{Op: "register", Err: "connection refused"}
	assert.Equal(t, "discovery register failed: connection refused", err.Error())
}
