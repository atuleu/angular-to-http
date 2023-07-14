package ath

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jessevdk/go-flags"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/exp/constraints"
	"golang.org/x/sys/unix"
)

func IsAtty(file *os.File) bool {
	_, err := unix.IoctlGetWinsize(int(file.Fd()), unix.TIOCGWINSZ)
	return err == nil
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
	size          int64
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
		format = fmt.Sprintf("%%s%%-%ds %%-%ds ....\n%%s", e.targetSize, e.flagSize)
	} else if ellapsed > 0 {
		format = fmt.Sprintf("%%s%%-%ds %%-%ds %%8sB cached in %%5.2f ms\n%%s", e.targetSize, e.flagSize)
		moveUp = fmt.Sprintf("\033[%dA\033[2K", len(e.entries)-i)
		moveDown = fmt.Sprintf("\033[%dB", len(e.entries)-i-1)
	}
	if ellapsed > 0 {
		fmt.Printf(format, moveUp,
			e.entries[i].target, e.entries[i].flags,
			ByteSize(e.entries[i].size), ellapsed.Seconds()*1000.0,
			moveDown)
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
	var totalSize atomic.Int64
	for i, entry := range entries.entries {
		entries.printEntry(i, 0)
		wg.Add(1)
		go func(i int, route Route) {
			defer wg.Done()
			size := route.PreCache()
			ellapsed := time.Now().Sub(start)
			totalSize.Add(size)
			mx.Lock()
			defer mx.Unlock()
			entries.entries[i].size = size
			entries.printEntry(i, ellapsed)
		}(i, routes[entry.target])
	}
	mx.Unlock()
	wg.Wait()
	end := time.Now()
	fmt.Printf("Pre-Cached %sB in %s\n", ByteSize(totalSize.Load()),
		end.Sub(start).Round(time.Millisecond))
}

func printRoutesNoTTY(routes map[string]Route) {
	entries := buildEntries(routes)
	wg := sync.WaitGroup{}
	start := time.Now()
	var totalSize atomic.Int64
	for i, entry := range entries.entries {
		entries.printEntry(i, -1)
		wg.Add(1)
		go func(route Route) {
			defer wg.Done()
			size := route.PreCache()
			totalSize.Add(size)
		}(routes[entry.target])
	}
	wg.Wait()
	end := time.Now()
	fmt.Printf("Pre-Cached %sB in %s\n", ByteSize(totalSize.Load()),
		end.Sub(start).Round(time.Millisecond))
}

func printRoutes(routes map[string]Route) {
	if IsAtty(os.Stdout) == true {
		zap.L().Debug("using a TTY")
		printRouteTTY(routes)
	} else {
		zap.L().Debug("not using a TTY")
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

	if err := setLogger(len(config.Verbose)); err != nil {
		return err
	}
	defer zap.L().Sync()

	if config.Otel.Endpoint != "" {
		shutdown, err := setTelemetry(config)
		if err == nil {
			defer shutdown(context.Background())
		} else {
			zap.L().Error("could not setup telemetry",
				zap.String("endpoint", config.Otel.Endpoint),
				zap.String("serviceName", config.Otel.ServiceName),
				zap.String("serviceInstanceID", config.Otel.ServiceInstanceID),
				zap.Error(err))
		}
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

	handler := otelhttp.NewHandler(NewHandler(routes), "",
		otelhttp.WithSpanNameFormatter(
			func(operation string, req *http.Request) string {
				return req.RequestURI
			}),
	)
	return http.Serve(listen, handler)
}

func setTelemetry(config Config) (func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }

	exporter, err := otlptracegrpc.New(context.Background(),
		otlptracegrpc.WithEndpoint(config.Otel.Endpoint),
		otlptracegrpc.WithInsecure(),
	)

	if err != nil {
		return noop, nil
	}

	instanceID := config.Otel.ServiceInstanceID
	if len(instanceID) == 0 {
		var err error
		instanceID, err = os.Hostname()
		if err != nil {
			return noop, err
		}
	}

	resource, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(config.Otel.ServiceName),
			semconv.ServiceVersion("0.2.0"),
			semconv.ServiceInstanceID(instanceID),
		),
	)
	if err != nil {
		return noop, err
	}

	provider := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resource),
	)

	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))

	return exporter.Shutdown, nil
}

func mapLogLevel(level int) zapcore.Level {
	if level <= 0 {
		return zapcore.WarnLevel
	}
	if level == 1 {
		return zapcore.InfoLevel
	}
	return zapcore.DebugLevel
}

func setLogger(level int) error {
	lvlThreshold := mapLogLevel(level)

	config := zap.NewProductionConfig()

	config.Level.SetLevel(lvlThreshold)
	log, err := config.Build(
		zap.WrapCore(func(zapcore.Core) zapcore.Core {
			return zapcore.NewCore(
				zapcore.NewConsoleEncoder(zap.NewProductionEncoderConfig()),
				zapcore.Lock(os.Stderr),
				zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
					return lvl >= lvlThreshold
				}),
			)
		}),
		zap.WithCaller(false),
	)

	if err != nil {
		return err
	}
	zap.ReplaceGlobals(log)
	return nil
}
