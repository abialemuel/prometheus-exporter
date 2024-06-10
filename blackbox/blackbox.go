package blackbox

import (
	"fmt"

	"github.com/go-kit/log"
	promCfg "github.com/prometheus/common/config"
	"github.com/prometheus/common/promlog"
	"gitlab.com/telkom/netmonk/prometheus-exporter/blackbox/config"
	"gitlab.com/telkom/netmonk/prometheus-exporter/blackbox/prober"
	"gitlab.com/telkom/netmonk/prometheus-exporter/helper"
	proto "gitlab.playcourt.id/telkom-digital/dpe/std/impl/netmonk/Proto/interfaces"
)

type blackbox struct {
	timeoutOffset float64
	rh            *prober.ResultHistory
	sc            *config.SafeConfig
	logger        log.Logger
}

type Blackbox interface {
	Call(target string, moduleName string, data *proto.WorkerProbe) (helper.ProbeResult, error)
}

func New(historyLimit uint, timeoutOffset float64, logLevel string) (Blackbox, error) {
	v := &promlog.AllowedLevel{}
	if err := v.Set(logLevel); err != nil {
		return nil, fmt.Errorf("error setting log level: %w", err)
	}
	logger := promlog.New(&promlog.Config{Level: v})
	rh := &prober.ResultHistory{MaxResults: historyLimit}
	sc := &config.SafeConfig{C: &config.Config{}}
	if err := sc.ReloadConfig("./blackbox/blackbox.yml", logger); err != nil {
		return nil, fmt.Errorf("error loading config: %w", err)
	}

	return &blackbox{
		timeoutOffset: timeoutOffset,
		rh:            rh,
		sc:            sc,
		logger:        logger,
	}, nil
}

func (c *blackbox) Call(target string, moduleName string, data *proto.WorkerProbe) (helper.ProbeResult, error) {
	module, ok := c.sc.C.Modules[moduleName]
	if !ok {
		return nil, fmt.Errorf("unknown module %q", moduleName)
	}

	webConfig := data.GetWebsite()
	if webConfig != nil {
		if webConfig.Authorization != nil {
			if webConfig.Authorization.Username != "" && webConfig.Authorization.Password != "" {
				// inject username and password for module
				basicAuth := &promCfg.BasicAuth{
					Username: webConfig.Authorization.Username,
					Password: promCfg.Secret(webConfig.Authorization.Password),
				}
				module.HTTP.HTTPClientConfig.BasicAuth = basicAuth
			}
		}

		// inject headers for module
		module.HTTP.Headers = webConfig.Headers

		// inject method & body for module
		module.HTTP.Method = webConfig.Method
		module.HTTP.Body = webConfig.Body
	}

	icmpQosConfig := data.GetICMPQOS()
	if icmpQosConfig != nil {
		// inject icmp qos for module
		module.ICMPQOS = config.ICMPQOSProbe{
			PacketSize: int(icmpQosConfig.PacketSize),
			Count:      int(icmpQosConfig.Count),
			Interval:   int(icmpQosConfig.Interval),
			Timeout:    config.DefaultICMPQoSProbe.Timeout,
			TTL:        config.DefaultICMPQoSProbe.TTL,
		}
	}

	// new config modules
	newModules := map[string]config.Module{
		moduleName: module,
	}
	config := &config.Config{
		Modules: newModules,
	}
	return prober.Call(target, moduleName, config, c.logger, c.rh, c.timeoutOffset)
}
