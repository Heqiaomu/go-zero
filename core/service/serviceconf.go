package service

import (
	"github.com/zeromicro/go-zero/core/interceptor"
	"log"

	"github.com/zeromicro/go-zero/core/load"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/prometheus"
	"github.com/zeromicro/go-zero/core/stat"
	"github.com/zeromicro/go-zero/core/trace"
)

const (
	// DevMode means development mode.
	DevMode = "dev"
	// TestMode means test mode.
	TestMode = "test"
	// RtMode means regression test mode.
	RtMode = "rt"
	// PreMode means pre-release mode.
	PreMode = "pre"
	// ProMode means production mode.
	ProMode = "pro"
)

// A ServiceConf is a service config.
type ServiceConf struct {
	Name            string                          `json:"name,optional"`
	Log             logx.LogConf                    `json:"log"`
	Mode            string                          `json:"mode,default=pro,options=dev|test|rt|pre|pro"`
	MetricsUrl      string                          `json:"metricsUrl,optional"`
	Prometheus      prometheus.Config               `json:"prometheus,optional"`
	Telemetry       trace.Config                    `json:"trace,optional"`
	StatInterceptor interceptor.StatInterceptorConf `json:"stat_interceptor,optional"`
}

// MustSetUp sets up the service, exits on error.
func (sc ServiceConf) MustSetUp() {
	if err := sc.SetUp(); err != nil {
		log.Fatal(err)
	}
}

// SetUp sets up the service.
func (sc ServiceConf) SetUp() error {
	if len(sc.Log.ServiceName) == 0 {
		sc.Log.ServiceName = sc.Name
	}
	if err := logx.SetUp(sc.Log); err != nil {
		return err
	}

	sc.initMode()

	if len(sc.Telemetry.Name) == 0 {
		sc.Telemetry.Name = sc.Name
	}
	trace.StartAgent(sc.Telemetry)

	if len(sc.MetricsUrl) > 0 {
		stat.SetReportWriter(stat.NewRemoteWriter(sc.MetricsUrl))
	}

	return nil
}

func (sc ServiceConf) initMode() {
	switch sc.Mode {
	case DevMode, TestMode, RtMode, PreMode:
		load.Disable()
		stat.SetReporter(nil)
	}
}
