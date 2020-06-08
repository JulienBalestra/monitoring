package mac

import (
	"strings"

	"github.com/JulienBalestra/monitoring/pkg/mac/generated"
)

func GetVendorWithPrefix(macPrefix string) (string, bool) {
	v, ok := generated.MacPrefixToVendor[macPrefix]
	return v, ok
}

func GetVendorWithMac(mac string) (string, bool) {
	return GetVendorWithPrefix(mac[:8])
}

func GetVendor(s string) (string, bool) {
	if len(s) < 8 {
		return "", false
	}
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, ":", "-")
	if len(s) == 8 {
		return GetVendorWithPrefix(s)
	}
	return GetVendorWithMac(s)
}
