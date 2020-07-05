package mac

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetVendor(t *testing.T) {
	for _, tc := range []struct {
		s  string
		ok bool
		e  string
	}{
		{
			"b8-27-eb-8a-ee-5c",
			true,
			"raspberry-pi-foundation",
		},
		{
			"b8-27-eb",
			true,
			"raspberry-pi-foundation",
		},
		{
			"B8-27-eb",
			true,
			"raspberry-pi-foundation",
		},
		{
			"B8-27-EB",
			true,
			"raspberry-pi-foundation",
		},
		{
			"no-mac-EB",
			false,
			"",
		},
		{
			"",
			false,
			"",
		},
	} {
		t.Run("", func(t *testing.T) {
			s, ok := GetVendor(tc.s)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.e, s)
		})
	}
}
