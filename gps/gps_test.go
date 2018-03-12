package gps

import (
	"io"
	"strings"
	"testing"
)

type rc struct{ io.Reader }

func (r *rc) Close() error { return nil }

func TestRMC(t *testing.T) {
	s := `$GPRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230318,003.1,W*6A
$GPRMC,025503.00,A,3724.59431,N,12206.83813,W,0.003,,120318,,,D*62
`
	g := newGPS(&rc{strings.NewReader(s)})
	for i := 0; i < 2; i++ {
		msg, ok := <-g.NMEA()
		if !ok {
			t.Fatal("expected message, got closed channel")
		}
		if msg.Fix().IsZero() {
			t.Fatal("wrong time")
		}
		t.Log(msg.Fix())
	}
}
