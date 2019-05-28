package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/bikeos/bosd/gps"
	"github.com/bikeos/bosd/internal/bench"
	"github.com/bikeos/bosd/internal/daemon"
	"github.com/bikeos/bosd/internal/http"
	"github.com/bikeos/bosd/internal/ingest"
)

var (
	rootCmd = &cobra.Command{
		Use:   "bosd",
		Short: "The multi-purpose bikeOS binary and daemon.",
	}
	flagBenchDur        time.Duration
	flagDataDir         string
	flagDevGPS          string
	flagHttpRootDirPath string
	flagLogDirPath      string
	flagSetTime         bool
)

func init() {
	cobra.EnablePrefixMatching = true
	rootCmd.PersistentFlags().StringVar(&flagDataDir, "data-dir", "/media/sdcard", "bosd data directory")

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
	ingestCmd.Flags().StringVar(&flagLogDirPath, "logdir", "", "directory for log data")
	rootCmd.AddCommand(ingestCmd)

	httpCmd := &cobra.Command{
		Use:   "http",
		Short: "serve standalone http interface",
		Run:   httpCommand,
	}
	httpCmd.Flags().StringVar(&flagLogDirPath, "logdir", "", "directory for log data")
	httpCmd.Flags().StringVar(&flagHttpRootDirPath, "rootdir", "", "directory for http resources")
	rootCmd.AddCommand(httpCmd)
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

func dataDirExec(dir string) {
	bosd := filepath.Join(dir, "bosd-"+runtime.GOARCH)
	if os.Args[0] == bosd {
		// Already running from data directory.
		log.Infof("using custom bosd %q", bosd)
		return
	}
	args := []string{bosd}
	args = append(args, os.Args[1:]...)
	fmt.Println(bosd)
	fmt.Println(args)
	syscall.Exec(bosd, args, os.Environ())
	// Exec failed; stick to this binary.
	log.Infof("using system bosd; searched %q", bosd)
}

func daemonCommand(cmd *cobra.Command, args []string) {
	dataDirExec(flagDataDir)
	cfg := daemon.Config{
		OutDirPath: flagDataDir,
	}
	fatalIf(daemon.Run(cfg))
}

func benchCommand(cmd *cobra.Command, args []string) {
	fatalIf(bench.Run(flagBenchDur))
}

func ingestCommand(cmd *cobra.Command, args []string) {
	logdir := flagLogDirPath
	if len(logdir) == 0 {
		logdir = filepath.Join(flagDataDir, "log")
	}
	fatalIf(ingest.Run(logdir))
}

func httpCommand(cmd *cobra.Command, args []string) {
	cfg := http.ServerConfig{
		ListenAddr: "localhost:8800",
		RootDir:    flagHttpRootDirPath,
		LogDir:     flagLogDirPath,
	}
	if err := http.Serve(cfg); err != nil {
		panic(err)
	}
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
