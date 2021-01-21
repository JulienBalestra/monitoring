package shelly

import (
	"testing"

	"github.com/magiconair/properties/assert"
)

func TestNewShelly(t *testing.T) {
	m := parseMac("807D3A021C15")
	assert.Equal(t, m, "80-7d-3a-02-1c-15")
}
