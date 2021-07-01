package forward

import (
	"bytes"
	"compress/zlib"
	"context"
	"net/url"
	"sync"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/datadog"
	"go.uber.org/zap"
)

const (
	DatadogZapScheme  = "datadog"
	DatadogZapOutput  = DatadogZapScheme + "://zap"
	bufferSize        = 1000000
	bufferSyncTrigger = bufferSize + (bufferSize * 0.15)
)

type Forwarder struct {
	mu     *sync.Mutex
	buffer *bytes.Buffer

	c *datadog.Client

	lastSync time.Time
	ctx      context.Context
}

func (f *Forwarder) Write(p []byte) (n int, err error) {
	f.mu.Lock()
	since := time.Since(f.lastSync)
	nn, err := f.buffer.Write(p)
	bufferLen := f.buffer.Len()
	f.mu.Unlock()
	if err != nil {
		return nn, err
	}
	if since > time.Minute || bufferLen > bufferSyncTrigger {
		//	return nn, f.Sync()
	}
	return nn, nil
}

func (f *Forwarder) Sync() error {
	ctx, cancel := context.WithTimeout(f.ctx, time.Second*45)
	defer cancel()
	var zb bytes.Buffer
	w, err := zlib.NewWriterLevel(&zb, zlib.BestCompression)
	if err != nil {
		return err
	}

	f.mu.Lock()
	_, err = w.Write(f.buffer.Bytes())
	if err != nil {
		return err
	}
	f.mu.Unlock()

	err = w.Close()
	if err != nil {
		return err
	}
	err = f.c.SendLogs(ctx, &zb)
	if err != nil {
		return err
	}
	f.buffer.Reset()
	f.lastSync = time.Now()
	return nil
}

func (f *Forwarder) Close() error {
	return f.Sync()
}

func NewDatadogForwarder(ctx context.Context, c *datadog.Client) func(*url.URL) (zap.Sink, error) {
	return func(_ *url.URL) (zap.Sink, error) {
		return &Forwarder{
			mu:       &sync.Mutex{},
			buffer:   bytes.NewBuffer(make([]byte, 0, bufferSize)),
			c:        c,
			lastSync: time.Now(),
			ctx:      ctx,
		}, nil
	}
}
