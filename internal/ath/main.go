package ath

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/constraints"
	"golang.org/x/sys/unix"
)

func IsAtty(file *os.File) bool {
	_, err := unix.IoctlGetWinsize(int(file.Fd()), unix.TIOCGWINSZ)
	return err != nil
}

func Max[T constraints.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

type printEntry struct {
	target, flags string
	ellapsed      time.Duration
}

type printEntries struct {
	entries              []printEntry
	targetSize, flagSize int
}

func buildEntries(routes map[string]Route) (entries printEntries) {
	targets := make([]string, 0, len(routes))

	for target, route := range routes {
		targets = append(targets, target)
		entries.targetSize = Max(entries.targetSize, len(target))
		entries.flagSize = Max(entries.flagSize, len(route.Flags().String()))
	}
	sort.Strings(targets)

	entries.entries = make([]printEntry, len(routes))
	for i, target := range targets {
		entries.entries[i] = printEntry{
			target: target,
			flags:  routes[target].Flags().String(),
		}
	}

	return
}

func (e printEntries) printEntry(i int, ellapsed time.Duration) {
	moveUp := ""
	moveDown := ""
	format := fmt.Sprintf("%%s%%-%ds %%s\n%%s", e.targetSize)
	if ellapsed == 0 {
		format = fmt.Sprintf("%%s%%-%ds %%%ds ....\n%%s", e.targetSize, e.flagSize)
	} else if ellapsed > 0 {
		format = fmt.Sprintf("%%s%%-%ds %%%ds DONE in %%s\n%%s", e.targetSize, e.flagSize)
		moveUp = fmt.Sprintf("\033[%dA\033[2K", len(e.entries)-i)
		moveDown = fmt.Sprintf("\033[%dB", len(e.entries)-i-1)
	}
	if ellapsed > 0 {
		fmt.Printf(format, moveUp, e.entries[i].target, e.entries[i].flags, ellapsed, moveDown)
	} else {
		fmt.Printf(format, moveUp, e.entries[i].target, e.entries[i].flags, moveDown)
	}
}

func printRouteTTY(routes map[string]Route) {
	entries := buildEntries(routes)
	start := time.Now()
	wg := sync.WaitGroup{}
	mx := sync.Mutex{}
	mx.Lock()
	for i, entry := range entries.entries {
		entries.printEntry(i, 0)
		wg.Add(1)
		go func(i int, route Route) {
			defer wg.Done()
			route.PreCache()
			ellapsed := time.Now().Sub(start)
			mx.Lock()
			defer mx.Unlock()
			entries.printEntry(i, ellapsed)
		}(i, routes[entry.target])
	}
	mx.Unlock()
	wg.Wait()
	end := time.Now()
	fmt.Printf("Pre-Caching done in %s\n", end.Sub(start).Round(time.Millisecond))
}

func printRoutesNoTTY(routes map[string]Route) {
	entries := buildEntries(routes)
	wg := sync.WaitGroup{}
	start := time.Now()
	for i, entry := range entries.entries {
		entries.printEntry(i, -1)
		wg.Add(1)
		go func(route Route) {
			defer wg.Done()
			route.PreCache()
		}(routes[entry.target])
	}
	wg.Wait()
	end := time.Now()
	fmt.Printf("Pre-Caching done in %s\n", end.Sub(start).Round(time.Millisecond))
}

func printRoutes(routes map[string]Route) {
	if IsAtty(os.Stdout) {
		logrus.Debug("using a TTY")
		printRouteTTY(routes)
	} else {
		logrus.Debug("not using a TTY")
		printRoutesNoTTY(routes)
	}
}

func Execute() error {
	config := Config{}
	if _, err := flags.Parse(&config); err != nil {
		if flags.WroteHelp(err) {
			return nil
		}
		return err
	}

	if config.Otel.Endpoint != "" {
		shutdown := setTelemetry(config)
		defer shutdown(context.Background())
	}

	routes, err := BuildRoutes(config)
	if err != nil {
		return err
	}

	go printRoutes(routes)

	listen, err := net.Listen("tcp", fmt.Sprintf("%s:%d", config.Address, config.Port))
	if err != nil {
		return err
	}

	setLogrusLevel(len(config.Verbose))

	return http.Serve(listen, NewHandler(routes))
}

func setTelemetry(config Config) func(context.Context) error {

	return func(context.Context) error { return errors.New("Not Yet Implemented") }
}

func setLogrusLevel(level int) {
	if level <= 0 {
		return
	}
	switch level {
	case 1:
		logrus.SetLevel(logrus.InfoLevel)
	case 2:
		logrus.SetLevel(logrus.DebugLevel)
	default:
		logrus.SetLevel(logrus.TraceLevel)
	}
}
