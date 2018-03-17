package daemon

import (
	"io"
	"os"
	"path/filepath"
	"time"
)

type store struct {
	nowdir string
	gps    *os.File
}

func newStore(outdir string) (*store, error) {
	if _, err := os.Stat(outdir); err != nil {
		return nil, err
	}
	nd := filepath.Join(outdir, time.Now().UTC().Format(time.RFC3339))
	if err := os.MkdirAll(nd, 0755); err != nil {
		return nil, err
	}
	s := &store{
		nowdir: nd,
	}
	return s, nil
}

func (s *store) Close() (err error) {
	if s.gps != nil {
		err = s.gps.Close()
	}
	return err
}

// GPS is the stream for NMEA message output.
func (s *store) GPS() (io.Writer, error) {
	if s.gps != nil {
		return s.gps, nil
	}
	gpsd := filepath.Join(s.nowdir, "gps")
	if err := os.MkdirAll(gpsd, 0755); err != nil {
		return nil, err
	}
	gpslog := filepath.Join(gpsd, "nmea")
	f, err := os.OpenFile(gpslog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	s.gps = f
	return s.gps, nil
}
