package wl

import (
	"sort"
	"testing"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/stretchr/testify/assert"
)

func TestGetSSID(t *testing.T) {
	c := newWL(&collector.Config{})
	for name, tc := range map[string]struct {
		input string
		ssid  string
	}{
		"AP": {
			input: `SSID: "AP"
Mode: Managed (4097)	RSSI: 0 dBm	SNR: 0 dB	noise: -84 dBm	Channel: 1
BSSID: 3C:37:86:72:15:B6	Capability: ESS RRM 
Supported Rates: [ 1(b) 2(b) 5.5(b) 6 9 11(b) 12 18 24 36 48 54 ]
HT Capable:
	Chanspec: 2.4GHz channel 1 20MHz (0x1001)
	Primary channel: 1
	HT Capabilities: 
	Supported MCS : [ 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 ]

`,
			ssid: "AP",
		},
		"AP5": {
			input: `SSID: "AP5"
Mode: Managed (4097)	RSSI: 0 dBm	SNR: 0 dB	noise: -92 dBm	Channel: 52l
BSSID: 3C:37:86:72:15:C3	Capability: ESS RRM 
Supported Rates: [ 6(b) 9 12(b) 18 24(b) 36 48 54 ]
VHT Capable:
	Chanspec: 5GHz channel 54 40MHz (0xd836)
	Primary channel: 52
	HT Capabilities: 
	Supported MCS : [ 0 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 32 ]
	VHT Capabilities: 
	Supported VHT (tx) Rates:
		NSS: 1 MCS: 0-9
		NSS: 2 MCS: 0-9
		NSS: 3 MCS: 0-9
	Supported VHT (rx) Rates:
		NSS: 1 MCS: 0-9
		NSS: 2 MCS: 0-9
		NSS: 3 MCS: 0-9

`,
			ssid: "AP5",
		},
	} {
		t.Run(name, func(t *testing.T) {
			ssid, err := c.getSSID([]byte(tc.input))
			assert.NoError(t, err, tc.input)
			assert.Equal(t, tc.ssid, ssid)
		})
	}
}

func TestGetMacs(t *testing.T) {
	c := newWL(&collector.Config{})
	for name, tc := range map[string]struct {
		input string
		macs  []string
	}{
		"2": {
			input: `
assoclist 00:00:00:00:00:01
assoclist 00:00:00:00:00:02
`,
			macs: []string{
				"00:00:00:00:00:01",
				"00:00:00:00:00:02",
			},
		},
		"1": {
			input: `
assoclist 00:00:00:00:00:01
`,
			macs: []string{
				"00:00:00:00:00:01",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			macs := c.getMacs([]byte(tc.input))
			sort.Strings(macs)
			assert.Equal(t, tc.macs, macs)
		})
	}

}
