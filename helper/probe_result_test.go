package helper_test

import (
	"testing"

	"github.com/abialemuel/prometheus-exporter/helper"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func TestProbeResult(t *testing.T) {
	// Create a new ProbeResult
	pr := helper.NewProbeResult(true, []*io_prometheus_client.MetricFamily{})

	// Test the Success method
	assert.Equal(t, true, pr.Success(), "Success method did not return expected result")

	// Test the Text method
	text, err := pr.Text()
	assert.NoError(t, err, "Text method returned an error")
	assert.Equal(t, []byte(nil), text, "Text method did not return expected result")

	// Test the Json method
	json, err := pr.Json()
	assert.NoError(t, err, "Json method returned an error")
	assert.Equal(t, []byte("[]"), json, "Json method did not return expected result")
}
