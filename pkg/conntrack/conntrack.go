package conntrack

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/JulienBalestra/dry/pkg/fnv"
)

const (
	ProtocolTCP      = "tcp"
	StateEstablished = "ESTABLISHED"

	ProtocolUDP = "udp"

	ProtocolICMP = "icmp"

	ProtocolUnknown = "unknown"

	StateUnreplied = "UNREPLIED"
	StateReplied   = "REPLIED"
)

var (
	unRepliedBytes = []byte("[UNREPLIED]")
)

type Quad struct {
	Source     string
	SourcePort int

	Destination     string
	DestinationPort int
}

func (q *Quad) Hash() uint64 {
	h := fnv.NewHash()

	h = fnv.AddString(h, q.Source)
	h = fnv.Add(h, uint64(q.SourcePort))

	h = fnv.AddString(h, q.Destination)
	h = fnv.Add(h, uint64(q.DestinationPort))
	return h
}

type Track struct {
	Quad *Quad

	Bytes   float64
	Packets float64
}

func (t *Track) Hash() uint64 {
	return t.Quad.Hash()
}

type Record struct {
	From *Track
	To   *Track

	Deadline time.Time
	Protocol string
	State    string
}

func (r *Record) Hash() uint64 {
	h := fnv.NewHash()
	h = fnv.AddString(h, r.Protocol)
	h = fnv.AddString(h, r.State)
	h = fnv.Add(h, r.From.Hash())
	h = fnv.Add(h, r.To.Hash())
	return h
}

func getQuadruplet(srcIPIndex int, line [][]byte) (*Quad, error) {
	var err error
	q := &Quad{
		Source:          string(line[srcIPIndex][4:]),
		SourcePort:      0,
		Destination:     string(line[srcIPIndex+1][4:]),
		DestinationPort: 0,
	}
	q.SourcePort, err = strconv.Atoi(string(line[srcIPIndex+2][6:]))
	if err != nil {
		return nil, err
	}
	q.DestinationPort, err = strconv.Atoi(string(line[srcIPIndex+3][6:]))
	if err != nil {
		return nil, err
	}
	return q, nil
}

func getTraffic(packets, bytes []byte) (float64, float64, error) {
	packetsCount, err := strconv.ParseFloat(string(packets[8:]), 10)
	if err != nil {
		return 0, 0, err
	}
	bytesCount, err := strconv.ParseFloat(string(bytes[6:]), 10)
	if err != nil {
		return 0, 0, err
	}
	return packetsCount, bytesCount, nil
}

func parseRecordFromLine(line []byte) (*Record, error) {
	fields := bytes.Fields(line)
	ttl, err := strconv.Atoi(string(fields[1]))
	if err != nil {
		return nil, err
	}
	r := &Record{
		Deadline: time.Now().Add(time.Duration(ttl) * time.Second),
		From:     &Track{},
		To:       &Track{},
	}
	r.Protocol, fields = string(fields[0]), fields[3:]
	switch r.Protocol {
	case ProtocolTCP:
		r.State = string(fields[0])
		r.From.Quad, err = getQuadruplet(1, fields)
		if err != nil {
			return nil, err
		}
		r.From.Packets, r.From.Bytes, err = getTraffic(fields[5], fields[6])
		if err != nil {
			return nil, err
		}
		if bytes.Equal(fields[7], unRepliedBytes) {
			r.To.Quad, err = getQuadruplet(8, fields)
			if err != nil {
				return nil, err
			}
			return r, nil
		}
		r.To.Quad, err = getQuadruplet(7, fields)
		if err != nil {
			return nil, err
		}
		r.To.Packets, r.To.Bytes, err = getTraffic(fields[11], fields[12])
		if err != nil {
			return nil, err
		}
	case ProtocolUDP:
		r.From.Quad, err = getQuadruplet(0, fields)
		if err != nil {
			return nil, err
		}
		r.From.Packets, r.From.Bytes, err = getTraffic(fields[4], fields[5])
		if err != nil {
			return nil, err
		}
		if bytes.Equal(fields[6], unRepliedBytes) {
			r.State = StateUnreplied
			r.To.Quad, err = getQuadruplet(7, fields)
			if err != nil {
				return nil, err
			}
			return r, nil
		}
		r.State = StateReplied
		r.To.Quad, err = getQuadruplet(6, fields)
		if err != nil {
			return nil, err
		}
		r.To.Packets, r.To.Bytes, err = getTraffic(fields[10], fields[11])
		if err != nil {
			return nil, err
		}
	case ProtocolICMP:
		r.From.Quad = &Quad{
			Source:      string(fields[0][4:]),
			Destination: string(fields[1][4:]),
		}
		r.From.Packets, r.From.Bytes, err = getTraffic(fields[5], fields[6])
		if err != nil {
			return nil, err
		}
		if bytes.Equal(fields[7], unRepliedBytes) {
			r.State = StateUnreplied
			r.To.Quad = &Quad{
				Source:      string(fields[8][4:]),
				Destination: string(fields[9][4:]),
			}
			return r, nil
		}
		r.State = StateReplied
		r.To.Quad = &Quad{
			Source:      string(fields[7][4:]),
			Destination: string(fields[8][4:]),
		}
		r.To.Packets, r.To.Bytes, err = getTraffic(fields[12], fields[13])
		if err != nil {
			return nil, err
		}
	case ProtocolUnknown:
		r.From.Quad = &Quad{
			Source:      string(fields[0][4:]),
			Destination: string(fields[1][4:]),
		}
		r.From.Packets, r.From.Bytes, err = getTraffic(fields[2], fields[3])
		if err != nil {
			return nil, err
		}
		if bytes.Equal(fields[4], unRepliedBytes) {
			r.State = StateUnreplied
			r.To.Quad = &Quad{
				Source:      string(fields[5][4:]),
				Destination: string(fields[6][4:]),
			}
			return r, nil
		}
		r.State = StateReplied
		r.To.Quad = &Quad{
			Source:      string(fields[4][4:]),
			Destination: string(fields[5][4:]),
		}
		r.To.Packets, r.To.Bytes, err = getTraffic(fields[6], fields[7])
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid protocol: %q", r.Protocol)
	}
	return r, nil
}

func GetConntrackRecords(conntrackFile string) (map[uint64]*Record, time.Time, error) {
	closestDeadline := time.Now().Add(time.Hour * 48)
	records := make(map[uint64]*Record)

	file, err := os.Open(conntrackFile)
	if err != nil {
		return records, closestDeadline, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	for {
		// TODO improve this reader
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return records, closestDeadline, err
		}
		record, err := parseRecordFromLine(line)
		if err != nil {
			zap.L().Error("failed to parse conntrack record", zap.ByteString("line", line), zap.Error(err))
			continue
		}
		records[record.Hash()] = record
		if record.Deadline.Before(closestDeadline) {
			closestDeadline = record.Deadline
		}
	}
	return records, closestDeadline, nil
}
