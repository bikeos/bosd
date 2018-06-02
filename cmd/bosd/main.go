package main

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/bikeos/bosd/gps"
	"github.com/bikeos/bosd/internal/bench"
	"github.com/bikeos/bosd/internal/daemon"
	"github.com/bikeos/bosd/internal/ingest"
)

var (
	rootCmd = &cobra.Command{
		Use:   "bosd",
		Short: "The multi-purpose bikeOS binary and daemon.",
	}
	flagDevGPS     string
	flagSetTime    bool
	flagOutDirPath string
	flagLogDirPath string
	flagBenchDur   time.Duration
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

	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "start the bikeOS daemon",
		Run:   daemonCommand,
	}
	daemonCmd.Flags().StringVar(&flagOutDirPath, "outdir", "/media/sdcard", "directory for recorded data")
	rootCmd.AddCommand(daemonCmd)

	benchCmd := &cobra.Command{
		Use:   "bench",
		Short: "benchmark interfaces",
		Run:   benchCommand,
	}
	benchCmd.Flags().DurationVar(&flagBenchDur, "time", 30*time.Second, "duration of benchmark")
	rootCmd.AddCommand(benchCmd)

	ingestCmd := &cobra.Command{
		Use:   "ingest",
		Short: "ingest log data into json",
		Run:   ingestCommand,
	}
	ingestCmd.Flags().StringVar(&flagLogDirPath, "logdir", "/media/sdcard/log", "directory for log data")
	rootCmd.AddCommand(ingestCmd)

}

func gpsTimeCommand(cmd *cobra.Command, args []string) {
	g, err := gps.NewGPS(flagDevGPS)
	fatalIf(err)
	defer g.Close()
	for msg := range g.NMEA() {
		if t := msg.Fix(); !t.IsZero() {
			fmt.Println(t)
			if flagSetTime {
				setSysTime(t)
			}
			return
		}
	}
	panic("gps closed: " + g.Close().Error())
}

func daemonCommand(cmd *cobra.Command, args []string) {
	cfg := daemon.Config{
		OutDirPath: flagOutDirPath,
	}
	fatalIf(daemon.Run(cfg))
}

func benchCommand(cmd *cobra.Command, args []string) {
	fatalIf(bench.Run(flagBenchDur))
}

func ingestCommand(cmd *cobra.Command, args []string) {
	fatalIf(ingest.Run(flagLogDirPath))
}

func main() {
	rootCmd.SetHelpTemplate(`{{.UsageString}}`)
	fatalIf(rootCmd.Execute())
}

func fatalIf(err error) {
	if err != nil {
		log.Error(err)
		panic(err)
	}
}
