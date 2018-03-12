//go:generate peg -strict nmea.peg
package gps

import (
	"strconv"
	"time"
)

type NMEA interface {
	// Fix is the time of the message's Fix, if any.
	Fix() time.Time
	// Line returns the raw NMEA string.
	Line() string
}

type nmeaBase struct{}

func (n *nmeaBase) Fix() (ret time.Time) { return ret }
func (n *nmeaBase) Line() string         { panic("oops") }

type nmeaLine struct {
	line string
	NMEA
}

func (u *nmeaLine) Line() string { return u.line }

type RMC struct {
	fix    string
	status string
	lat    string
	lon    string
	knots  Knots
	track  string
	date   string
	magvar string
	chksum string

	nmeaBase
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
