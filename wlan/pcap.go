package wlan

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Packet struct {
	t       time.Time
	srcAddr string
	dstAddr string
}

func (p Packet) Time() time.Time { return p.t }
func (p Packet) Src() string     { return p.srcAddr }
func (p Packet) Dst() string     { return p.dstAddr }

func NewPCapFileChan(pcapFile string) (<-chan Packet, error) {
	if _, err := os.Stat(pcapFile); os.IsNotExist(err) {
		return nil, err
	}
	var cmd *exec.Cmd
	if strings.HasSuffix(pcapFile, ".gz") {
		str := "zcat " + pcapFile + " | /usr/sbin/tcpdump -tt -e -n -r -"
		cmd = exec.Command("/bin/bash", "-c", str)
	} else {
		cmd = exec.Command("/usr/sbin/tcpdump", "-tt", "-e", "-n", "-r", pcapFile)
	}
	if cmd == nil {
		return nil, fmt.Errorf("no command for %q", pcapFile)
	}
	rc, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	pktc := make(chan Packet, 32)
	go func() {
		defer close(pktc)
		r := bufio.NewReader(rc)
		for {
			l, lerr := r.ReadString('\n')
			if lerr != nil {
				break
			}
			pkt := line2pkt(l)
			if pkt == nil {
				continue
			}
			pktc <- *pkt
		}
		cmd.Wait()
		rc.Close()
	}()
	return pktc, nil
}

func line2field(s string, fields []string) string {
	for _, f := range fields {
		if strings.Contains(s, f) {
			return strings.SplitN(strings.SplitN(s, f, 2)[1], " ", 2)[0]
		}
	}
	return ""
}

func line2pkt(s string) *Packet {
	da := line2field(s, []string{"DA:", "TA:"})
	sa := line2field(s, []string{"SA:", "RA:"})
	if sa == "" {
		return nil
	}
	t := strings.SplitN(s, " ", 2)[0]
	f, _ := strconv.ParseFloat(t, 64)
	return &Packet{
		t:       time.Unix(int64(f), int64((f-float64(int64(f)))*1e9)),
		srcAddr: sa,
		dstAddr: da,
	}
}
