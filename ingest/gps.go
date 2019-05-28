package ingest

import (
	"path"

	"github.com/bikeos/bosd/gps"
)

func NewGPSChan(dirs []string) (<-chan gps.NMEA, error) {
	ch := make(chan gps.NMEA, 8)
	go func() {
		defer close(ch)
		for _, name := range dirs {
			p := path.Join(name, "gps", "nmea.log")
			g, err := gps.NewGPS(p)
			if err != nil {
				continue
			}
			for v := range g.NMEA() {
				ch <- v
			}
			g.Close()
		}
	}()
	return ch, nil
}
