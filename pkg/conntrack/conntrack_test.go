package conntrack

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetConntrackRecords(t *testing.T) {
	for name, tc := range map[string]struct {
		line   string
		record *Record
	}{
		"udp:replied": {
			line: "udp      17 15 src=127.0.0.1 dst=127.0.0.1 sport=51251 dport=53 packets=1 bytes=60 src=127.0.0.1 dst=127.0.0.1 sport=53 dport=51251 packets=1 bytes=74 mark=0 use=2",
			record: &Record{
				From: &Track{
					Quad: &Quad{
						Source:          "127.0.0.1",
						SourcePort:      51251,
						Destination:     "127.0.0.1",
						DestinationPort: 53,
					},
					Bytes:   60,
					Packets: 1,
				},
				To: &Track{
					Quad: &Quad{
						Source:          "127.0.0.1",
						SourcePort:      53,
						Destination:     "127.0.0.1",
						DestinationPort: 51251,
					},
					Bytes:   74,
					Packets: 1,
				},
				Protocol: "udp",
				State:    StateReplied,
			},
		},
		"udp:unreplied": {
			line: "udp      17 118 src=192.168.1.135 dst=255.255.255.255 sport=49154 dport=6666 packets=1657 bytes=336371 [UNREPLIED] src=255.255.255.255 dst=192.168.1.135 sport=6666 dport=49154 packets=0 bytes=0 mark=0 use=2",
			record: &Record{
				From: &Track{
					Quad: &Quad{
						Source:          "192.168.1.135",
						SourcePort:      49154,
						Destination:     "255.255.255.255",
						DestinationPort: 6666,
					},
					Bytes:   336371,
					Packets: 1657,
				},
				To: &Track{
					Quad: &Quad{
						Source:          "255.255.255.255",
						SourcePort:      6666,
						Destination:     "192.168.1.135",
						DestinationPort: 49154,
					},
					Bytes:   0,
					Packets: 0,
				},
				Protocol: "udp",
				State:    StateUnreplied,
			},
		},
		"tcp:replied": {
			line: "tcp      6 3554 ESTABLISHED src=192.168.1.147 dst=64.233.167.188 sport=43338 dport=5228 packets=119 bytes=7180 src=64.233.167.188 dst=78.194.244.189 sport=5228 dport=43338 packets=122 bytes=11689 [ASSURED] mark=0 use=2",
			record: &Record{
				From: &Track{
					Quad: &Quad{
						Source:          "192.168.1.147",
						SourcePort:      43338,
						Destination:     "64.233.167.188",
						DestinationPort: 5228,
					},
					Bytes:   7180,
					Packets: 119,
				},
				To: &Track{
					Quad: &Quad{
						Source:          "64.233.167.188",
						SourcePort:      5228,
						Destination:     "78.194.244.189",
						DestinationPort: 43338,
					},
					Bytes:   11689,
					Packets: 122,
				},
				Protocol: "tcp",
				State:    StateEstablished,
			},
		},
		"tcp:ESTABLISHED": {
			line: "tcp      6 3554 ESTABLISHED src=192.168.1.147 dst=64.233.167.188 sport=43338 dport=5228 packets=119 bytes=7180 src=64.233.167.188 dst=78.194.244.189 sport=5228 dport=43338 packets=122 bytes=11689 [ASSURED] mark=0 use=2",
			record: &Record{
				From: &Track{
					Quad: &Quad{
						Source:          "192.168.1.147",
						SourcePort:      43338,
						Destination:     "64.233.167.188",
						DestinationPort: 5228,
					},
					Bytes:   7180,
					Packets: 119,
				},
				To: &Track{
					Quad: &Quad{
						Source:          "64.233.167.188",
						SourcePort:      5228,
						Destination:     "78.194.244.189",
						DestinationPort: 43338,
					},
					Bytes:   11689,
					Packets: 122,
				},
				Protocol: "tcp",
				State:    StateEstablished,
			},
		},
		"icmp:replied": {
			line: "icmp     1 0 src=192.168.1.134 dst=8.8.8.8 type=8 code=0 id=3276 packets=2 bytes=168 src=8.8.8.8 dst=78.194.244.189 type=0 code=0 id=3276 packets=2 bytes=168 mark=0 use=2",
			record: &Record{
				From: &Track{
					Quad: &Quad{
						Source:          "192.168.1.134",
						SourcePort:      0,
						Destination:     "8.8.8.8",
						DestinationPort: 0,
					},
					Bytes:   168,
					Packets: 2,
				},
				To: &Track{
					Quad: &Quad{
						Source:          "8.8.8.8",
						SourcePort:      0,
						Destination:     "78.194.244.189",
						DestinationPort: 0,
					},
					Bytes:   168,
					Packets: 2,
				},
				Protocol: "icmp",
				State:    StateReplied,
			},
		},
		"icmp:unreplied": {
			line: "icmp     1 2 src=192.168.1.1 dst=192.168.1.123 type=8 code=0 id=35924 packets=1 bytes=48 [UNREPLIED] src=192.168.1.123 dst=192.168.1.1 type=0 code=0 id=35924 packets=0 bytes=0 mark=0 use=2",
			record: &Record{
				From: &Track{
					Quad: &Quad{
						Source:          "192.168.1.1",
						SourcePort:      0,
						Destination:     "192.168.1.123",
						DestinationPort: 0,
					},
					Bytes:   48,
					Packets: 1,
				},
				To: &Track{
					Quad: &Quad{
						Source:          "192.168.1.123",
						SourcePort:      0,
						Destination:     "192.168.1.1",
						DestinationPort: 0,
					},
					Bytes:   0,
					Packets: 0,
				},
				Protocol: "icmp",
				State:    StateUnreplied,
			},
		},
		"unknown:unreplied": {
			line: "unknown  2 232 src=192.168.3.1 dst=224.0.0.251 packets=7 bytes=224 [UNREPLIED] src=224.0.0.251 dst=192.168.3.1 packets=0 bytes=0 mark=0 use=2",
			record: &Record{
				From: &Track{
					Quad: &Quad{
						Source:          "192.168.3.1",
						SourcePort:      0,
						Destination:     "224.0.0.251",
						DestinationPort: 0,
					},
					Bytes:   224,
					Packets: 7,
				},
				To: &Track{
					Quad: &Quad{
						Source:          "224.0.0.251",
						SourcePort:      0,
						Destination:     "192.168.3.1",
						DestinationPort: 0,
					},
					Bytes:   0,
					Packets: 0,
				},
				Protocol: "unknown",
				State:    StateUnreplied,
			},
		},
		"unknown:replied": {
			line: "unknown  2 232 src=192.168.3.1 dst=224.0.0.251 packets=7 bytes=224 src=224.0.0.251 dst=192.168.3.1 packets=2 bytes=42 mark=0 use=2",
			record: &Record{
				From: &Track{
					Quad: &Quad{
						Source:          "192.168.3.1",
						SourcePort:      0,
						Destination:     "224.0.0.251",
						DestinationPort: 0,
					},
					Bytes:   224,
					Packets: 7,
				},
				To: &Track{
					Quad: &Quad{
						Source:          "224.0.0.251",
						SourcePort:      0,
						Destination:     "192.168.3.1",
						DestinationPort: 0,
					},
					Bytes:   42,
					Packets: 2,
				},
				Protocol: "unknown",
				State:    StateReplied,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			r, err := parseRecordFromLine([]byte(tc.line))
			require.NoError(t, err)
			assert.EqualValues(t, tc.record.From, r.From)
			assert.EqualValues(t, tc.record.To.Packets, r.To.Packets)
			assert.EqualValues(t, tc.record.To.Bytes, r.To.Bytes)
			assert.EqualValues(t, *tc.record.To.Quad, *r.To.Quad)
			assert.EqualValues(t, *tc.record.From.Quad, *r.From.Quad)
			assert.EqualValues(t, tc.record.State, r.State)
			assert.EqualValues(t, tc.record.Protocol, r.Protocol)
		})
	}
}

func TestGetConntrackRecords2(t *testing.T) {
	r, _, err := GetConntrackRecords("fixtures/conntrack.txt")
	require.NoError(t, err)
	require.Len(t, r, 153)
}
