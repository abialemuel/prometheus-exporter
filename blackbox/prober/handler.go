// Copyright 2016 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prober

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/abialemuel/prometheus-exporter/blackbox/config"
	"github.com/abialemuel/prometheus-exporter/helper"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/common/expfmt"
	"gopkg.in/yaml.v2"
)

var (
	Probers = map[string]ProbeFn{
		"http":     ProbeHTTP,
		"tcp":      ProbeTCP,
		"icmp":     ProbeICMP,
		"icmp_qos": ProbeICMPQoS,
		"dns":      ProbeDNS,
		"grpc":     ProbeGRPC,
	}
	moduleUnknownCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "blackbox_module_unknown_total",
		Help: "Count of unknown modules requested by probes",
	})
)

// Call is a function that calls the prober
func Call(target string, moduleName string, c *config.Config, logger log.Logger, rh *ResultHistory, timeoutOffset float64) (helper.ProbeResult, error) {
	module, ok := c.Modules[moduleName]
	if !ok {
		level.Debug(logger).Log("msg", "Unknown module", "module", moduleName)
		moduleUnknownCounter.Add(1)
		return nil, fmt.Errorf("unknown module %q", moduleName)
	}

	timeoutSeconds, err := getTimeout(nil, module, timeoutOffset)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds*float64(time.Second)))
	defer cancel()

	prober, ok := Probers[module.Prober]
	if !ok {
		return nil, fmt.Errorf("unknown prober %q", module.Prober)
	}

	sl := newScrapeLogger(logger, moduleName, target)
	level.Info(sl).Log("msg", "Beginning probe", "probe", module.Prober, "timeout_seconds", timeoutSeconds)

	probeSuccessGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_success",
		Help: "Displays whether or not the probe was a success",
	})
	probeDurationGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_duration_seconds",
		Help: "Returns how long the probe took to complete in seconds",
	})

	start := time.Now()
	registry := prometheus.NewRegistry()
	registry.MustRegister(probeSuccessGauge)
	registry.MustRegister(probeDurationGauge)
	success := prober(ctx, target, module, registry, sl)
	duration := time.Since(start).Seconds()
	probeDurationGauge.Set(duration)
	if success {
		probeSuccessGauge.Set(1)
		level.Info(sl).Log("msg", "Probe succeeded", "duration_seconds", duration)
	} else {
		level.Error(sl).Log("msg", "Probe failed", "duration_seconds", duration)
	}

	debugOutput := DebugOutput(&module, &sl.buffer, registry)
	rh.Add(moduleName, target, debugOutput, success)

	// Gather metrics
	metricFamilies, err := registry.Gather()
	if err != nil {
		// handle error
		return nil, fmt.Errorf("failed to gather metrics: %s", err)
	}

	return helper.NewProbeResult(success, metricFamilies), nil
}

type scrapeLogger struct {
	next         log.Logger
	buffer       bytes.Buffer
	bufferLogger log.Logger
}

func newScrapeLogger(logger log.Logger, module string, target string) *scrapeLogger {
	logger = log.With(logger, "module", module, "target", target)
	sl := &scrapeLogger{
		next:   logger,
		buffer: bytes.Buffer{},
	}
	bl := log.NewLogfmtLogger(&sl.buffer)
	sl.bufferLogger = log.With(bl, "ts", log.DefaultTimestampUTC, "caller", log.Caller(6), "module", module, "target", target)
	return sl
}

func (sl scrapeLogger) Log(keyvals ...interface{}) error {
	sl.bufferLogger.Log(keyvals...)
	kvs := make([]interface{}, len(keyvals))
	copy(kvs, keyvals)
	// Switch level to debug for application output.
	for i := 0; i < len(kvs); i += 2 {
		if kvs[i] == level.Key() {
			kvs[i+1] = level.DebugValue()
		}
	}
	return sl.next.Log(kvs...)
}

// DebugOutput returns plaintext debug output for a probe.
func DebugOutput(module *config.Module, logBuffer *bytes.Buffer, registry *prometheus.Registry) string {
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "Logs for the probe:\n")
	logBuffer.WriteTo(buf)
	fmt.Fprintf(buf, "\n\n\nMetrics that would have been returned:\n")
	mfs, err := registry.Gather()
	if err != nil {
		fmt.Fprintf(buf, "Error gathering metrics: %s\n", err)
	}
	for _, mf := range mfs {
		expfmt.MetricFamilyToText(buf, mf)
	}
	fmt.Fprintf(buf, "\n\n\nModule configuration:\n")
	c, err := yaml.Marshal(module)
	if err != nil {
		fmt.Fprintf(buf, "Error marshalling config: %s\n", err)
	}
	buf.Write(c)

	return buf.String()
}

func getTimeout(r *http.Request, module config.Module, timeout float64) (timeoutSeconds float64, err error) {
	// If a timeout is configured via the Prometheus header, add it to the request.
	// if r != nil {
	// 	if v := r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds"); v != "" {
	// 		var err error
	// 		timeoutSeconds, err = strconv.ParseFloat(v, 64)
	// 		if err != nil {
	// 			return 0, err
	// 		}
	// 	}
	// }

	// if timeoutSeconds == 0 {
	// 	timeoutSeconds = 120
	// }

	// var maxTimeoutSeconds = timeoutSeconds - offset
	// if module.Timeout.Seconds() < maxTimeoutSeconds && module.Timeout.Seconds() > 0 || maxTimeoutSeconds < 0 {
	// 	timeoutSeconds = module.Timeout.Seconds()
	// } else {
	// 	timeoutSeconds = maxTimeoutSeconds
	// }

	if timeout <= 0 {
		timeoutSeconds = module.Timeout.Seconds()
	} else {
		timeoutSeconds = timeout
	}

	return timeoutSeconds, nil
}
