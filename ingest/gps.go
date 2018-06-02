package ingest

import (
	"path"

	"github.com/bikeos/bosd/gps"
)

func NewGPSChan(baseDir string) (<-chan gps.NMEA, error) {
	names, err := timeSortedNames(baseDir)
	if err != nil {
		return nil, err
	}
	ch := make(chan gps.NMEA, 8)
	go func() {
		defer close(ch)
		for _, name := range names {
			p := path.Join(baseDir, name, "gps", "nmea.log")
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
