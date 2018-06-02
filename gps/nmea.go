//go:generate peg -strict nmea.peg
package gps

import (
	"math"
	"strconv"
	"time"
)

type NMEA struct {
	line string
	NMEAi
}

func (n NMEA) Msg() NMEAi { return n.NMEAi }

// Line returns the raw NMEA string.
func (n NMEA) Line() string { return n.line }

type NMEAi interface {
	Fix() time.Time
	Longitude() float64
	Latitude() float64
}

type nmeaUnk struct{}

func (n *nmeaUnk) Fix() (ret time.Time) { return ret }
func (n *nmeaUnk) Longitude() float64   { return math.NaN() }
func (n *nmeaUnk) Latitude() float64    { return math.NaN() }

type RMC struct {
	fix    string
	status string
	lat    string
	ns     string
	lon    string
	we     string
	knots  Knots
	track  string
	date   string
	magvar string
	chksum string
}

type Knots float64

func (k *Knots) parse(text string) { *k = Knots(mustFloat64(text)) }

func mustFloat64(text string) float64 {
	ret, err := strconv.ParseFloat(text, 64)
	if err != nil {
		panic(err)
	}
	return ret
}

func mustInt(text string) int {
	ret, err := strconv.ParseInt(text, 10, 32)
	if err != nil {
		panic(err)
	}
	return int(ret)
}

func (r *RMC) Fix() time.Time {
	// DD/MM/YY HH:MM:SS
	return time.Date(
		2000+mustInt(r.date[4:6]),
		time.Month(mustInt(r.date[2:4])),
		mustInt(r.date[:2]),
		mustInt(r.fix[:2]),
		mustInt(r.fix[2:4]),
		mustInt(r.fix[4:6]),
		0,
		time.UTC)
}

func (r *RMC) Longitude() float64 {
	if len(r.lon) == 0 {
		return math.NaN()
	}
	// DDDMM.MMMMM,W = longitude
	londeg, _ := strconv.ParseInt(r.lon[0:3], 10, 32)
	lonmin, _ := strconv.ParseFloat(r.lon[3:], 64)
	lonv := float64(londeg) + (lonmin / 60.0)
	if r.we == "W" {
		lonv = -lonv
	}
	return lonv
}

func (r *RMC) Latitude() float64 {
	if len(r.lat) == 0 {
		return math.NaN()
	}
	// DDMM.MMMMM,N = latitude
	latdeg, _ := strconv.ParseInt(r.lat[0:2], 10, 32)
	latmin, _ := strconv.ParseFloat(r.lat[2:], 64)
	latv := float64(latdeg) + (latmin / 60.0)
	if r.ns == "S" {
		latv = -latv
	}
	return latv
}
