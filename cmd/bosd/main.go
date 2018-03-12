package main

import (
	"fmt"
	"syscall"

	"github.com/bikeos/bosd/gps"

	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "bosd",
		Short: "The multi-purpose bikeOS binary and daemon.",
	}
	flagDevGPS  string
	flagSetTime bool
)

func init() {
	cobra.EnablePrefixMatching = true
	gpsCmd := &cobra.Command{
		Use:   "gps <subcommand> <value>",
		Short: "creates a key with a given value",
	}
	gpsCmd.PersistentFlags().StringVar(&flagDevGPS, "device", "/dev/ttyACM0", "device to query")
	gpsTimeCmd := &cobra.Command{
		Use:   "date",
		Short: "gets date and time from GPS",
		Run:   gpsTimeCommand,
	}
	gpsTimeCmd.Flags().BoolVar(&flagSetTime, "set", false, "set system time")
	gpsCmd.AddCommand(gpsTimeCmd)
	rootCmd.AddCommand(gpsCmd)
}

func gpsTimeCommand(cmd *cobra.Command, args []string) {
	g, err := gps.NewGPS(flagDevGPS)
	fatalIf(err)
	defer g.Close()
	for msg := range g.NMEA() {
		if t := msg.Fix(); !t.IsZero() {
			fmt.Println(t)
			if flagSetTime {
				ns := t.UnixNano()
				s := ns / 1e9
				us := (ns - s*1e9) * 1e3
				tv := syscall.Timeval{s, us}
				fatalIf(syscall.Settimeofday(&tv))
			}
			return
		}
	}
	panic("gps closed: " + g.Close().Error())
}

func main() {
	rootCmd.SetHelpTemplate(`{{.UsageString}}`)
	fatalIf(rootCmd.Execute())
}

func fatalIf(err error) {
	if err != nil {
		panic(err)
	}
}
