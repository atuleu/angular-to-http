package ath

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/jessevdk/go-flags"
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

func printRoutes(routes map[string]Route) {
	targets := make([]string, 0, len(routes))
	maxLength := 0
	wg := sync.WaitGroup{}
	start := time.Now()
	for target, route := range routes {
		targets = append(targets, target)
		maxLength = Max(maxLength, len(target))
		wg.Add(1)
		go func(route Route) {
			defer wg.Done()
			route.PreCache()
		}(route)
	}
	sort.Strings(targets)

	format := fmt.Sprintf("%%-%ds %%s\n", maxLength)
	for _, target := range targets {
		route := routes[target]
		fmt.Printf(format, target, route.Flags())
	}
	wg.Wait()
	end := time.Now()
	fmt.Printf("Pre-Caching done in %s\n", end.Sub(start).Round(time.Millisecond))
}

func Execute() error {
	config := Config{}
	if _, err := flags.Parse(&config); err != nil {
		if flags.WroteHelp(err) {
			return nil
		}
		return err
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

	return http.Serve(listen, NewHandler(routes, config.Verbose))
}
