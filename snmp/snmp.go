package snmp

import (
	"fmt"

	"github.com/go-kit/log"
	"github.com/prometheus/common/promlog"
	proto "gitlab.playcourt.id/telkom-digital/dpe/std/impl/netmonk/Proto/interfaces"
	"gitlab.playcourt.id/telkom-digital/dpe/std/impl/netmonk/prometheus-exporter/helper"
	"gitlab.playcourt.id/telkom-digital/dpe/std/impl/netmonk/prometheus-exporter/snmp/config"
	"gitlab.playcourt.id/telkom-digital/dpe/std/impl/netmonk/prometheus-exporter/snmp/prober"
)

type snmp struct {
	timeoutOffset float64
	sc            *config.SafeConfig
	logger        log.Logger
}

type Snmp interface {
	Call(target string, moduleName []string, nodeConfig *proto.NodeConfig) (helper.ProbeResult, error)
}

var (
	path          = "snmp.yml"
	expandEnvVars = false
)

func New(historyLimit uint, timeoutOffset float64, logLevel string) (Snmp, error) {
	v := &promlog.AllowedLevel{}
	if err := v.Set(logLevel); err != nil {
		return nil, fmt.Errorf("error setting log level: %w", err)
	}
	logger := promlog.New(&promlog.Config{Level: v})
	sc := &config.SafeConfig{C: &config.Config{}}
	if err := sc.ReloadConfig(path, expandEnvVars); err != nil {
		return nil, fmt.Errorf("error loading config: %w", err)
	}

	return &snmp{
		timeoutOffset: timeoutOffset,
		sc:            sc,
		logger:        logger,
	}, nil
}

func (c *snmp) Call(target string, moduleName []string, nodeConfig *proto.NodeConfig) (helper.ProbeResult, error) {
	// inject auth to snmp config
	snmpAuth := config.Auth{
		Community:     config.Secret(nodeConfig.Community),
		SecurityLevel: nodeConfig.SecurityLevel,
		Username:      nodeConfig.Username,
		Password:      config.Secret(nodeConfig.Password),
		AuthProtocol:  nodeConfig.AuthProtocol,
		PrivProtocol:  nodeConfig.PrivProtocol,
		PrivPassword:  config.Secret(nodeConfig.PrivPassword),
		ContextName:   nodeConfig.ContextName,
		Version:       int(nodeConfig.Version),
	}

	// dynamic timeout offset for each node
	if nodeConfig.Timeout != 0 {
		c.timeoutOffset = float64(nodeConfig.Timeout)
	}

	return prober.Call(target, moduleName, c.sc, snmpAuth, c.logger, c.timeoutOffset)
}
