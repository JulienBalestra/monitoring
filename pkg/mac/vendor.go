package mac

import (
	"strings"

	"github.com/JulienBalestra/monitoring/pkg/mac/generated"
)

const UnknownVendor = "unknown"

func GetVendorWithPrefix(macPrefix string) (string, bool) {
	v, ok := generated.MacPrefixToVendor[macPrefix]
	return v, ok
}

func GetVendorWithMac(mac string) (string, bool) {
	if len(mac) < 8 {
		return "", false
	}
	return GetVendorWithPrefix(mac[:8])
}

func GetVendorWithMacOrUnknown(mac string) string {
	m, ok := GetVendorWithPrefix(mac[:8])
	if !ok {
		return UnknownVendor
	}
	return m
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
