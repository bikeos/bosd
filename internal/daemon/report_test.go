package daemon

import (
	"testing"
)

func TestHaversine(t *testing.T) {
	Rmeter := 6371e3
	var tts = []struct {
		last gpsStatus
		cur  gpsStatus

		want float64
	}{
		{
			gpsStatus{lat: 1.0},
			gpsStatus{lat: 2.0},
			// 111321,
			111194,
		},
		{
			gpsStatus{lon: 1.0},
			gpsStatus{lon: 2.0},
			111194,
		},
		{
			gpsStatus{},
			gpsStatus{},
			0,
		},
	}
	for i, tt := range tts {
		v := Rmeter * haversine(tt.last, tt.cur)
		if int64(v) != int64(tt.want) {
			t.Errorf("#%d: wanted %g, got %g", i, tt.want, v)
		}
	}

}
