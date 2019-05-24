package daemon

import (
	"context"
	"fmt"
	"io"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/bikeos/bosd/gps"
)

type gpsStatus struct {
	when time.Time
	lat  float64
	lon  float64
}

func (d *daemon) startGPS() error {
	// TODO: GPS hotplug
	gs, err := gps.Enumerate()
	if err != nil {
		return err
	}
	var gg *gps.GPS
	for _, g := range gs {
		if gg, err = gps.NewGPS(g); err == nil {
			log.Infof("reading from GPS %q", g)
			break
		}
	}
	if err != nil {
		return err
	}
	if gg == nil {
		return fmt.Errorf("gps: no gps found")
	}
	d.worker(func(ctx context.Context) error {
		return d.gpsLogger(gg, d.s)
	})
	return nil
}

func (d *daemon) gpsLogger(g *gps.GPS, s *store) (err error) {
	defer func() {
		if cerr := g.Close(); err == nil {
			err = cerr
		}
	}()
	w, err := s.GPS()
	if err != nil {
		return err
	}
	for {
		select {
		case msg, ok := <-g.NMEA():
			if !ok {
				return io.EOF
			}
			if !msg.Fix().IsZero() {
				lat, lon := msg.Latitude(), msg.Longitude()
				if lat != lat {
					continue
				}
				newStatus := gpsStatus{time.Now(), lat, lon}
				d.gsMu.Lock()
				d.gs = newStatus
				d.gsMu.Unlock()
			}
			l := []byte(msg.Line())
			if _, err := w.Write(l); err != nil {
				return err
			}
		case <-d.ctx.Done():
			return nil
		}
	}
	return nil
}
