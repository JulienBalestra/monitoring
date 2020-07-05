package catalog

import (
	"testing"
	"time"

	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCollectorConfigFile(t *testing.T) {
	c, err := ParseConfigFile("./fixtures/collectors.yaml")
	require.NoError(t, err)

	assert.Equal(t, c.Collectors["shelly"].Interval, time.Second*5)
}

func TestGenerateCollectorConfigFile(t *testing.T) {
	err := GenerateCollectorConfigFile("./fixtures/gen-collectors.yaml")
	require.NoError(t, err)
}
