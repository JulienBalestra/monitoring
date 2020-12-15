package catalog

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestParseCollectorConfigFile(t *testing.T) {
	c, err := ParseConfigFile("./fixtures/collectors.yaml")
	require.NoError(t, err)

	assert.Len(t, c.Collectors, 2)
}

func TestGenerateCollectorConfigFile(t *testing.T) {
	err := GenerateCollectorConfigFile("./fixtures/gen-collectors.yaml")
	require.NoError(t, err)
}
