package daemon

import (
	log "github.com/sirupsen/logrus"

	"github.com/bikeos/bosd/gps"
)

func (d *daemon) runGPS() error {
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
	d.worker(func() error { return gpsLogger(gg, d.s) })
	return nil
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
