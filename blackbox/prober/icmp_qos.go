// Copyright 2024 Arieditya Pr.dH [for netmonk.id]
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
	"context"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	ping "github.com/prometheus-community/pro-bing"
	"github.com/prometheus/client_golang/prometheus"
	"gitlab.playcourt.id/telkom-digital/dpe/std/impl/netmonk/prometheus-exporter/blackbox/config"
)

func ProbeICMPQoS(_ context.Context, target string, module config.Module, registry *prometheus.Registry, logger log.Logger) (success bool) {
	var (
		// durations
		startDuration time.Time
		endDuration   time.Time
		totalDuration time.Duration

		// latencies helper
		totalLatency time.Duration
		maxLatency   time.Duration
		minLatency   time.Duration
		avgLatency   float64

		// jitters
		prevTime  float64
		jitter    float64
		jitterSum float64
		jitterMax float64
		jitterMin float64

		// request counter
		count int64
		// probe_duration_seconds
		probeQoSDurationSecondsSecond float64

		//// Parsers to OpenMetrix data
		// probe_qos_latency_gauge : aggregate [total, max, min, avg, standard_deviation]
		probeQoSLatencyGaugeTotal             float64
		probeQoSLatencyGaugeMax               float64
		probeQoSLatencyGaugeMin               float64
		probeQoSLatencyGaugeAvg               float64
		probeQoSLatencyGaugeStandardDeviation float64

		// probe_qos_packet_loss : total
		probeQoSPacketLossSent           float64
		probeQoSPacketLossReceived       float64
		probeQoSPacketLossLossPercentage float64
		probeQoSPacketLossLossCount      int64

		// probe_qos_jitter_gauge : aggregate
		probeQoSJitterGaugeTotalDiff float64
		probeQoSJitterGaugeMaxDiff   float64
		probeQoSJitterGaugeMinDiff   float64

		// probe_qos_jitter
		probeQoSJitterµs float64

		// probe_qos_packet_count
		probeQoSPacketCountInt float64
	)
	// start duration benchmark
	startDuration = time.Now()

	// var probeResponseError = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	// 	Name: "probe_break_error_code",
	// 	Help: "Exception has been raised on probe",
	// }, []string{"error_code", "error_message", "error_value"})

	// Registering OpenMetrics Data
	var probeQoSDurationSeconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_qos_duration_seconds",
		Help: "Total Durations of the pinger executions (in seconds)",
	})

	var probeQosLatencyGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "probe_qos_latency_gauge",
		Help: "Probe QoS Latency Gauge (all are in milliseconds)",
	}, []string{"aggregate"})
	for _, lv := range []string{"total", "min", "max", "avg", "standard_deviation"} {
		probeQosLatencyGauge.WithLabelValues(lv)
	}

	var probeQoSPacketLoss = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "probe_qos_packet_loss",
		Help: "Probe QoS Latency Packet Loss",
	}, []string{"total"})
	for _, lv := range []string{"sent", "received", "loss", "loss_percentage"} {
		probeQoSPacketLoss.WithLabelValues(lv)
	}

	var probeQoSJitterGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "probe_qos_jitter_gauge",
		Help: "Probe QoS Jitter Gauge (all are in microseconds)",
	}, []string{"aggregate"})
	for _, lv := range []string{"total_diff", "max_diff", "min_diff"} {
		probeQoSJitterGauge.WithLabelValues(lv)
	}

	var probeQoSJitter = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_qos_jitter",
		Help: "Jitter Calculations and Aggregates (in microseconds)",
	})

	var probeQoSPacketCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "probe_qos_packet_count",
		Help: "Total number of tested data to be sent to target",
	})

	_ = level.Debug(logger).Log("msg", "Set Pinger")
	pinger, err := ping.NewPinger(target)
	if err != nil {
		_ = level.Error(logger).Log("msg", "set pinger failed", "err", err)
		return false
	}

	if onWindows() {
		pinger.SetPrivileged(true)
	}

	perThousand := float64(1000)

	pinger.Count = module.ICMPQOS.Count
	pinger.Size = module.ICMPQOS.PacketSize // in bytes
	pinger.Interval = time.Duration(module.ICMPQOS.Interval) * time.Millisecond
	pinger.Timeout = time.Duration(module.ICMPQOS.Timeout) * time.Millisecond
	pinger.TTL = module.ICMPQOS.TTL
	//maxTimeout = float64(pinger.Timeout) / perThousand

	// OVERRIDE with Query String

	registry.MustRegister(probeQoSDurationSeconds)
	registry.MustRegister(probeQosLatencyGauge)
	registry.MustRegister(probeQoSPacketLoss)
	registry.MustRegister(probeQoSJitter)
	registry.MustRegister(probeQoSJitterGauge)
	registry.MustRegister(probeQoSPacketCount)

	pinger.OnRecvError = func(_ error) {
		jitter = prevTime - (float64(pinger.Timeout.Nanoseconds()) / perThousand)
		if jitter < 0 {
			jitter = -jitter
		}
		jitterSum += jitter
		prevTime = float64(pinger.Timeout.Nanoseconds()) / perThousand
		jitterMax = prevTime
	}

	pinger.OnRecv = func(pkt *ping.Packet) {
		// jitter (in microseconds)
		jitter = prevTime - float64(pkt.Rtt.Nanoseconds())/perThousand
		if jitter < 0 {
			jitter = -jitter
		}
		if jitter > jitterMax {
			jitterMax = jitter
		}
		if jitterMin == 0 || jitter < jitterMin {
			jitterMin = jitter
		}
		jitterSum += jitter

		totalLatency += pkt.Rtt
		if maxLatency < pkt.Rtt {
			maxLatency = pkt.Rtt
		}
		if minLatency == 0 || minLatency > pkt.Rtt {
			minLatency = pkt.Rtt
		}
		prevTime = float64(pkt.Rtt.Nanoseconds()) / perThousand
	}

	pinger.OnFinish = func(s *ping.Statistics) {

		// probe_qos_packet_loss : total
		probeQoSPacketLossSent = float64(s.PacketsSent)
		probeQoSPacketLossReceived = float64(s.PacketsRecv)
		probeQoSPacketLossLossPercentage = s.PacketLoss
		probeQoSPacketLossLossCount = int64(s.PacketsSent - s.PacketsRecv)

		count = int64(pinger.Count)

		//// below line are added for making up the missing packets
		////
		totalLatency += (time.Duration(probeQoSPacketLossLossCount) * time.Microsecond) * pinger.Timeout

		// microsecond to millisecond
		if probeQoSPacketLossReceived > 0 {
			avgLatency = float64(totalLatency.Microseconds()) / probeQoSPacketLossReceived / perThousand
		} else {
			// this happened if packet is 100% loss???
			avgLatency = float64(totalLatency.Microseconds()/count) / perThousand
		}

		// probe_qos_latency_gauge : aggregate [total, max, min, avg, standard_deviation]
		probeQoSLatencyGaugeTotal = float64(totalLatency.Microseconds()) / perThousand
		probeQoSLatencyGaugeMax = float64(maxLatency.Microseconds()) / perThousand
		probeQoSLatencyGaugeMin = float64(minLatency.Microseconds()) / perThousand
		probeQoSLatencyGaugeAvg = avgLatency
		probeQoSLatencyGaugeStandardDeviation = float64(s.StdDevRtt.Microseconds()) / perThousand

		// probe_qos_jitter_gauge : aggregate
		//// below line are added for the missing packet jitters
		jitterSum += float64(pinger.Timeout.Microseconds() * probeQoSPacketLossLossCount)

		probeQoSJitterGaugeTotalDiff = jitterSum
		probeQoSJitterGaugeMaxDiff = jitterMax
		probeQoSJitterGaugeMinDiff = jitterMin

		// probe_qos_jitter
		if probeQoSPacketLossReceived > 1 {
			probeQoSJitterµs = jitterSum / (probeQoSPacketLossReceived - 1)
		} else {
			// only happen if 100% loss???
			probeQoSJitterµs = jitterSum / float64(count-1)
		}

		// probe_qos_packet_count
		probeQoSPacketCountInt = float64(pinger.Count)
		probeQoSPacketCount.Set(probeQoSPacketCountInt)

		probeQoSJitter.Set(probeQoSJitterµs)
		probeQoSJitterGauge.WithLabelValues("total_diff").Add(probeQoSJitterGaugeTotalDiff)
		probeQoSJitterGauge.WithLabelValues("max_diff").Add(probeQoSJitterGaugeMaxDiff)
		probeQoSJitterGauge.WithLabelValues("min_diff").Add(probeQoSJitterGaugeMinDiff)

		probeQoSPacketLoss.WithLabelValues("sent").Add(probeQoSPacketLossSent)
		probeQoSPacketLoss.WithLabelValues("received").Add(probeQoSPacketLossReceived)
		probeQoSPacketLoss.WithLabelValues("loss").Add(float64(probeQoSPacketLossLossCount))
		probeQoSPacketLoss.WithLabelValues("loss_percentage").Add(probeQoSPacketLossLossPercentage)

		probeQosLatencyGauge.WithLabelValues("total").Add(probeQoSLatencyGaugeTotal)
		probeQosLatencyGauge.WithLabelValues("max").Add(probeQoSLatencyGaugeMax)
		probeQosLatencyGauge.WithLabelValues("min").Add(probeQoSLatencyGaugeMin)
		probeQosLatencyGauge.WithLabelValues("avg").Add(probeQoSLatencyGaugeAvg)
		probeQosLatencyGauge.WithLabelValues("standard_deviation").Add(probeQoSLatencyGaugeStandardDeviation)

		// Logging the result
		_ = level.Info(logger).Log("msg", "ICMP Gauge summary",
			// packet loss
			"packet_sent", probeQoSPacketLossSent,
			"packet_received", probeQoSPacketLossReceived,
			"packet_loss", probeQoSPacketLossLossCount,
			"packet_loss_percentage", probeQoSPacketLossLossPercentage,

			// latency
			"latency_total", probeQoSLatencyGaugeTotal,
			"latency_max", probeQoSLatencyGaugeMax,
			"latency_min", probeQoSLatencyGaugeMin,
			"latency_avg", probeQoSLatencyGaugeAvg,
			"latency_std_deviation", probeQoSLatencyGaugeStandardDeviation,

			// jitters
			"jitters", probeQoSJitterµs,
			"jitter_max", probeQoSJitterGaugeMaxDiff,
			"jitter_min", probeQoSJitterGaugeMinDiff,
			"jitter_total", probeQoSJitterGaugeTotalDiff,
		)

		// probe_duration_seconds
		endDuration = time.Now()
		totalDuration = endDuration.Sub(startDuration)
		probeQoSDurationSecondsSecond = float64(totalDuration.Microseconds()) / (perThousand * perThousand)
		probeQoSDurationSeconds.Set(probeQoSDurationSecondsSecond)
		_ = level.Info(logger).Log("msg", "ICMP Execution duration", "duration", probeQoSDurationSecondsSecond)
	}

	err = pinger.Run()
	if err != nil {
		_ = level.Error(logger).Log("msg", "Pinger failed to run", "err", err)
		return false
	}

	return true
}
