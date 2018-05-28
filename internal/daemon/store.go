package daemon

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type store struct {
	basedir string
	nowdir  string
	gps     *os.File
}

func newStore(basedir string) (*store, error) {
	if _, err := os.Stat(basedir); err != nil {
		return nil, err
	}
	nd := filepath.Join(basedir, "log", time.Now().UTC().Format(time.RFC3339))
	if err := os.MkdirAll(nd, 0755); err != nil {
		return nil, err
	}
	s := &store{
		basedir: basedir,
		nowdir:  nd,
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
	gpslog := filepath.Join(gpsd, "nmea.log")
	f, err := os.OpenFile(gpslog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	s.gps = f
	return s.gps, nil
}

// WIFI is the directory for a wifi device.
func (s *store) Wifi(name string) (string, error) {
	wifid := filepath.Join(s.nowdir, "wifi", name)
	if err := os.MkdirAll(wifid, 0755); err != nil {
		return "", err
	}
	return wifid, nil
}

func (s *store) WifiCurrentPCap(name string) (string, error) {
	f, err := os.Open(filepath.Join(s.nowdir, "wifi", name))
	if err != nil {
		return "", err
	}
	defer f.Close()
	pcaps, err := f.Readdirnames(0)
	if err != nil {
		return "", err
	}
	for _, pcap := range pcaps {
		if !strings.HasSuffix(pcap, ".gz") {
			return filepath.Join(s.nowdir, "wifi", name, pcap), nil
		}
	}
	return "", io.EOF
}

func syspath(p string) string            { return filepath.Join("/usr/share/bikeos", p) }
func (s *store) cfgpath(p string) string { return filepath.Join(s.basedir, "config", "boot.mp3") }

func (s *store) ConfigPath(p string) string {
	cpath := s.cfgpath(p)
	if _, err := os.Stat(cpath); err == nil {
		return cpath
	}
	return syspath(p)
}
