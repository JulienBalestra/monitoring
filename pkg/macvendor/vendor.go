package macvendor

import (
	"strings"

	"github.com/JulienBalestra/monitoring/pkg/macvendor/generated"
)

const UnknownVendor = "unknown"

func GetVendorWithPrefix(p string) (string, bool) {
	p = NormaliseMacAddress(p)
	v, ok := generated.MacPrefixToVendor[p]
	return v, ok
}

func GetVendorWithMac(s string) (string, bool) {
	if len(s) < 8 {
		return "", false
	}
	return GetVendorWithPrefix(s[:8])
}

func GetVendorWithMacOrUnknown(s string) string {
	m, ok := GetVendorWithPrefix(s[:8])
	if !ok {
		return UnknownVendor
	}
	return m
}

func GetVendor(s string) (string, bool) {
	if len(s) < 8 {
		return "", false
	}
	if len(s) == 8 {
		return GetVendorWithPrefix(s)
	}
	return GetVendorWithMac(s)
}

func NormaliseMacAddress(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, ":", "-")
	return s
}

func NormaliseMacAddressBytes(b []byte) string {
	return NormaliseMacAddress(string(b))
}
