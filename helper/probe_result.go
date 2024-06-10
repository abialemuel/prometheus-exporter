package helper

import (
	"bytes"
	"encoding/json"
	"fmt"

	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

type ProbeResult interface {
	Success() bool
	Text() ([]byte, error)
	Json() ([]byte, error)
}

type probeResult struct {
	success        bool
	metricFamilies []*io_prometheus_client.MetricFamily
}

func (c probeResult) Success() bool {
	return c.success
}

func (c probeResult) Text() ([]byte, error) {
	// Convert metrics to output
	var buffer bytes.Buffer
	encoder := expfmt.NewEncoder(&buffer, expfmt.NewFormat(expfmt.TypeTextPlain))
	for _, mf := range c.metricFamilies {
		if err := encoder.Encode(mf); err != nil {
			// handle error
			return nil, fmt.Errorf("failed to encode metric family: %s", err)
		}
	}
	return buffer.Bytes(), nil
}

func (c probeResult) Json() ([]byte, error) {
	// Convert metrics to JSON
	jsonData, err := json.Marshal(c.metricFamilies)
	if err != nil {
		// handle error
		return nil, fmt.Errorf("failed to marshal metrics to JSON: %s", err)
	}
	// Print or send the JSON output
	return jsonData, nil
}

func NewProbeResult(success bool, metricFamilies []*io_prometheus_client.MetricFamily) ProbeResult {
	return &probeResult{success: success, metricFamilies: metricFamilies}
}
