# Prometheus Exporter Library

This repository contains two types of Prometheus exporters: Blackbox and SNMP. These exporters are used to monitor network devices and services, providing valuable metrics for Prometheus, a powerful open-source monitoring and alerting toolkit.

## Blackbox Exporter

The Blackbox exporter allows probing of endpoints over HTTP, HTTPS, DNS, TCP and ICMP protocols. It is configured via a configuration file `blackbox.yml`.

## SNMP Exporter
The SNMP (Simple Network Management Protocol) exporter is a powerful tool that allows you to extract metrics from SNMP-enabled network devices. SNMP is a protocol for managing devices on IP networks. Devices that typically support SNMP include routers, switches, servers, workstations, printers, modem racks, and more.


## Getting Started
To give Git credentials, you’ll need to have a .netrc that includes gitlab.playcourt.id in your home directory.
```shell
export GOPROXY=https://proxy.golang.org,direct
export GOPRIVATE=gitlab.playcourt.id/telkom-digital/dpe/std/impl/netmonk/*
echo "machine gitlab.playcourt.id login <USER> password <PASSWORD OR TOKEN>" > ~/.netrc
```


## Usage

### Blackbox Exporter
To use the Blackbox exporter as a library, you need to import it in your Go application:

```go
import "gitlab.playcourt.id/telkom-digital/dpe/std/impl/netmonk/prometheus-exporter/blackbox"

historyLimit := uint(100)
timeoutOffset := 0.5
logLevel := "info"

blackbox, err := blackbox.New(historyLimit, timeoutOffset, logLevel)
if err != nil {
    log.Fatalf("Error creating blackbox: %v", err)
}
```

Then, you can use the Call method to probe a target:

```go
target := "http://example.com"
moduleName := "http_2xx"

// Create a WorkerProbe struct
data := &proto.WorkerProbe{
    Website: &proto.Website{
        Authorization: &proto.Authorization{
            Username: "username",
            Password: "password",
        },
    },
}

result, err := blackbox.Call(target, moduleName, data)
if err != nil {
    log.Fatalf("Error probing target: %v", err)
}

fmt.Printf("Probe result: %v\n", result)
```

### SNMP Exporter
To use the Blackbox exporter as a library, you need to import it in your Go application:

```go
import "gitlab.playcourt.id/telkom-digital/dpe/std/impl/netmonk/prometheus-exporter/snmp"