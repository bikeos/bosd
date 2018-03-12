package gps

import (
	"io"
	"strings"
	"testing"
)

type rc struct{ io.Reader }

func (r *rc) Close() error { return nil }

func TestRMCFix(t *testing.T) {
	tts := []string{
		"$GPRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230318,003.1,W*6A\n",
		"$GPRMC,025503.00,A,1111.22222,N,01111.33333,W,0.003,,120318,,,D*62\n",
		"$GPRMC,043019.00,V,,,,,,,120318,,,N*7B\n",
		"$GPRMC,044735.00,A,2222.11111,N,02222.44444,W,1.339,,120318,,,A*6E\n",
	}
	for i, tt := range tts {
		g := newGPS(&rc{strings.NewReader(tt)})
		msg, ok := <-g.NMEA()
		if !ok {
			t.Fatalf("#%d: expected message, got closed channel", i)
		}
		if msg.Fix().IsZero() {
			t.Fatalf("#%d: wrong time", i)
		}
		t.Log(msg.Fix())
	}
}
