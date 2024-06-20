package prober

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"gitlab.playcourt.id/telkom-digital/dpe/std/impl/netmonk/prometheus-exporter/helper"
	"gitlab.playcourt.id/telkom-digital/dpe/std/impl/netmonk/prometheus-exporter/snmp/collector"
	"gitlab.playcourt.id/telkom-digital/dpe/std/impl/netmonk/prometheus-exporter/snmp/config"
)

const (
	namespace = "snmp"
)

var (
	// Metrics about the SNMP exporter itself.
	snmpRequestErrors = createCounter("request_errors_total", "Errors in SNMP requests.")
	exporterMetrics   = createExporterMetrics()
)

func createCounter(name string, help string) prometheus.Counter {
	return promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      name,
		Help:      help,
	})
}

func createExporterMetrics() collector.Metrics {
	buckets := prometheus.ExponentialBuckets(0.0001, 2, 15)
	return collector.Metrics{
		SNMPCollectionDuration: config.SnmpCollectionDuration,
		SNMPUnexpectedPduType:  createCounter("unexpected_pdu_type_total", "Unexpected Go types in a PDU."),
		SNMPDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "packet_duration_seconds",
			Help:      "A histogram of latencies for SNMP packets.",
			Buckets:   buckets,
		}),
		SNMPPackets: createCounter("packets_total", "Number of SNMP packet sent, including retries."),
		SNMPRetries: createCounter("packet_retries_total", "Number of SNMP packet retries."),
		SNMPInflight: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "request_in_flight",
			Help:      "Current number of SNMP scrapes being requested.",
		}),
	}
}

// Call is a function that calls the prober
func Call(target string, moduleNames []string, c *config.SafeConfig, auth config.Auth, logger log.Logger, timeoutOffset float64) (helper.ProbeResult, error) {
	if target == "" {
		level.Debug(logger).Log("msg", "parameter must be specified once", "target", target)
		snmpRequestErrors.Inc()
		return nil, fmt.Errorf("unknown target %q", target)
	}

	// if authName == nil {
	// 	authName = new(string)
	// 	*authName = "public_v2"
	// }

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutOffset*float64(time.Second)))
	defer cancel()

	c.RLock()
	var nmodules []*collector.NamedModule
	for _, m := range moduleNames {
		module, moduleOk := c.C.Modules[m]
		if !moduleOk {
			c.RUnlock()
			level.Debug(logger).Log("msg", "Unknown module", "module", m)
			snmpRequestErrors.Inc()
			return nil, fmt.Errorf("unknown module %q", m)
		}
		getTimeout(module, timeoutOffset) // Convert timeoutOffset to time.Duration
		nmodules = append(nmodules, collector.NewNamedModule(m, module))
	}

	// auth, authOk := c.C.Auths[*authName]
	// if !authOk {
	// 	c.RUnlock()
	// 	level.Debug(logger).Log("msg", "Unknown auth", "auth", *authName)
	// 	snmpRequestErrors.Inc()
	// 	return nil, fmt.Errorf("unknown auth %q", *authName)
	// }
	c.RUnlock()

	logger = log.With(logger, "auth", c.C.Auths, "target", target)
	registry := prometheus.NewRegistry()
	authName := fmt.Sprintf("version: %d, securityLevel: %s", auth.Version, auth.SecurityLevel)
	col := collector.New(ctx, target, authName, &auth, nmodules, logger, exporterMetrics, config.Concurrency)
	registry.MustRegister(col)

	// Gather metrics
	metricFamilies, err := registry.Gather()
	if err != nil {
		// handle error
		return nil, fmt.Errorf("failed to gather metrics: %s", err)
	}

	success := true

	return helper.NewProbeResult(success, metricFamilies), nil
}

func getTimeout(module *config.Module, timeout float64) (err error) {
	if timeout <= 0 {
		module.WalkParams.Timeout = config.DefaultWalkParams.Timeout
		return nil
	} else {
		module.WalkParams.Timeout = time.Duration(timeout * float64(time.Second))
	}

	return nil
}
