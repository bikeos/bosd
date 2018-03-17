package gps

import (
	"io/ioutil"
	"strings"
)

// Enumerate collects all gps device paths.
// TODO: udev enumeration.
func Enumerate() (ret []string, err error) {
	fs, err := ioutil.ReadDir("/dev/")
	if err != nil {
		return nil, err
	}
	for _, f := range fs {
		if strings.HasPrefix(f.Name(), "ttyACM") {
			ret = append(ret, "/dev/"+f.Name())
		}
	}
	return ret, nil
}
