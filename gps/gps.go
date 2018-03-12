package gps

import (
	"bufio"
	"context"
	"io"
	"os"
)

// GPS is an instance of an opened GPS stream.
type GPS struct {
	f      io.ReadCloser
	ch     chan NMEA
	err    error
	donec  chan struct{}
	ctx    context.Context
	cancel context.CancelFunc
}

// NewGPS opens a GPS stream from a given device path.
func NewGPS(p string) (*GPS, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	return newGPS(f), nil
}

func newGPS(f io.ReadCloser) *GPS {
	ctx, cancel := context.WithCancel(context.Background())
	ret := &GPS{
		f:      f,
		ch:     make(chan NMEA),
		donec:  make(chan struct{}),
		ctx:    ctx,
		cancel: cancel,
	}
	go ret.read()
	return ret
}

// NMEA streams GPS messages. Closes on error.
func (g *GPS) NMEA() <-chan NMEA { return g.ch }

// Close terminates the GPS stream and returns any error encountered.
func (g *GPS) Close() error {
	g.cancel()
	g.f.Close()
	<-g.donec
	return g.err
}

func (g *GPS) read() {
	defer func() {
		close(g.ch)
		close(g.donec)
	}()
	r := bufio.NewReader(g.f)
	ng := &nmeaGrammar{}
	ng.Init()
	for g.err == nil {
		line, pfx, err := r.ReadLine()
		if pfx && err == nil {
			err = io.ErrUnexpectedEOF
		}
		if err != nil {
			g.err = err
			break
		}
		ng.Buffer = string(line) + "\n"
		ng.Reset()
		if err = ng.Parse(); err != nil {
			continue
		}
		ng.Execute()
		select {
		case g.ch <- ng.nmea:
		case <-g.ctx.Done():
			g.err = g.ctx.Err()
		}
	}
}
