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
	assert.Equal(t, c.Collectors["temperature"].Interval, time.Second*120)
	assert.Equal(t, c.Collectors["temperature"].Options["temperature-divide"], "10")
	assert.Equal(t, c.Collectors["temperature"].Options["temperature-file"], "/proc/dmu/temperature")
}

func TestGenerateCollectorConfigFile(t *testing.T) {
	err := GenerateCollectorConfigFile("./fixtures/gen-collectors.yaml")
	require.NoError(t, err)
}
