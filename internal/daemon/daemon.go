package daemon

import (
	log "github.com/sirupsen/logrus"
	"sync"

	"github.com/bikeos/bosd/gps"
)

type Config struct {
	OutDirPath string
}

type daemon struct {
	cfg   Config
	wg    sync.WaitGroup
	errc  chan error
	donec chan struct{}
}

func Run(cfg Config) error {
	errc := make(chan error, 1)
	d := &daemon{
		cfg:   cfg,
		errc:  make(chan error),
		donec: make(chan struct{}),
	}
	go func() { errc <- d.run() }()
	return <-errc
}

func (d *daemon) run() (err error) {
	defer func() {
		close(d.donec)
		d.wg.Wait()
	}()
	s, serr := newStore(d.cfg.OutDirPath)
	if serr != nil {
		return serr
	}
	// TODO: GPS hotplug
	g, gerr := newGPS()
	if gerr != nil {
		return gerr
	}
	d.worker(func() error { return gpsLogger(g, s) })
	return <-d.errc
}

func (d *daemon) worker(f func() error) {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		select {
		case d.errc <- f():
		case <-d.donec:
		}
	}()
}

func newGPS() (*gps.GPS, error) {
	gs, err := gps.Enumerate()
	if err != nil {
		return nil, err
	}
	var gg *gps.GPS
	for _, g := range gs {
		if gg, err = gps.NewGPS(g); err == nil {
			log.Infof("reading from GPS %q", g)
			break
		}
	}
	return gg, err
}

func gpsLogger(g *gps.GPS, s *store) (err error) {
	defer func() {
		if cerr := g.Close(); err == nil {
			err = cerr
		}
	}()
	w, err := s.GPS()
	if err != nil {
		return err
	}
	for msg := range g.NMEA() {
		l := []byte(msg.Line())
		if _, err := w.Write(l); err != nil {
			return err
		}
	}
	return nil
}
