package daemon

import (
	"sync"

	log "github.com/sirupsen/logrus"
)

type Config struct {
	OutDirPath string
}

type daemon struct {
	cfg   Config
	wg    sync.WaitGroup
	errc  chan error
	donec chan struct{}

	s *store

	gs   gpsStatus
	gsMu sync.RWMutex
}

func Run(cfg Config) error {
	d := &daemon{
		cfg:   cfg,
		errc:  make(chan error),
		donec: make(chan struct{}),
	}
	return d.run()
}

func (d *daemon) run() (err error) {
	defer func() {
		close(d.donec)
		d.wg.Wait()
	}()
	if d.s, err = newStore(d.cfg.OutDirPath); err != nil {
		return err
	}
	if err = d.startGPS(); err != nil {
		return err
	}
	if err = d.startWifi(); err != nil {
		return err
	}
	if err = d.startAudio(); err != nil {
		log.Errorf("audio: %v", err)
	} else {
		if err = d.startReports(); err != nil {
			return err
		}
	}
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
