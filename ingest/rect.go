package ingest

type LatLon struct {
	Lat float64
	Lon float64
}

type Rect struct {
	TL LatLon
	BR LatLon
}

func (r *Rect) Contains(ll LatLon) bool {
	if ll.Lat < r.TL.Lat || ll.Lat > r.BR.Lat {
		return false
	}
	if ll.Lon > r.TL.Lon || ll.Lon < r.BR.Lon {
		return false
	}
	return true
}
