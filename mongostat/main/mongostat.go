package main

import (
	"fmt"
	"github.com/mongodb/mongo-tools/common/log"
	commonopts "github.com/mongodb/mongo-tools/common/options"
	"github.com/mongodb/mongo-tools/mongostat"
	"github.com/mongodb/mongo-tools/mongostat/options"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	// initialize command-line opts
	opts := commonopts.New(
		"mongostat",
		"[options] <polling interval in seconds>",
		commonopts.EnabledOptions{Connection: true, Auth: true, Namespace: false})

	// add mongotop-specific options
	statOpts := &options.StatOptions{}
	opts.AddOptions(statOpts)

	extra, err := opts.Parse()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid options: %v\n", err)
		opts.PrintHelp(true)
		os.Exit(-1)
	}

	log.SetVerbosity(opts.Verbosity)

	sleepInterval := 1
	if len(extra) > 0 {
		if len(extra) != 1 {
			fmt.Fprintf(os.Stderr, "Too many positional operators\n")
			opts.PrintHelp(true)
			os.Exit(-1)
		}
		sleepInterval, err = strconv.Atoi(extra[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Bad sleep interval: %v\n", extra[0])
			os.Exit(-1)
		}
		if sleepInterval < 1 {
			fmt.Fprintf(os.Stderr, "Sleep interval must be at least 1 second\n")
			os.Exit(-1)
		}
	}

	// print help, if specified
	if opts.PrintHelp(false) {
		return
	}

	// print version, if specified
	if opts.PrintVersion() {
		return
	}

	var formatter mongostat.LineFormatter
	formatter = &mongostat.GridLineFormatter{!statOpts.NoHeaders, 10}
	if statOpts.Json {
		formatter = &mongostat.JSONLineFormatter{}
	}

	seedHosts := []string{}
	hostOption := strings.Split(opts.Host, ",")
	for _, seedHost := range hostOption {
		if opts.Port != "" {
			seedHost = fmt.Sprintf("%s:%s", seedHost, opts.Port)
		}
		seedHosts = append(seedHosts, seedHost)
	}

	var cluster mongostat.ClusterMonitor
	if statOpts.Discover || len(seedHosts) > 1 {
		cluster = &mongostat.AsyncClusterMonitor{
			ReportChan:    make(chan mongostat.StatLine),
			LastStatLines: map[string]*mongostat.StatLine{},
			Formatter:     formatter,
		}
	} else {
		cluster = &mongostat.SyncClusterMonitor{
			ReportChan: make(chan mongostat.StatLine),
			Formatter:  formatter,
		}
	}

	var discoverChan chan string
	if statOpts.Discover {
		discoverChan = make(chan string, 128)
	}

	opts.Direct = true
	stat := &mongostat.MongoStat{
		Options:       opts,
		StatOptions:   statOpts,
		Nodes:         map[string]*mongostat.NodeMonitor{},
		Discovered:    discoverChan,
		SleepInterval: time.Duration(sleepInterval) * time.Second,
		Cluster:       cluster,
	}

	for _, v := range seedHosts {
		stat.AddNewNode(v)
	}

	// kick it off
	err = stat.Run()
	if err != nil {
		log.Logf(log.Always, "Error: %v", err)
		os.Exit(-1)
	}
}
