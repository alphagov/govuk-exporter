package main

import (
	"net/http"
	"os"
	"time"

	"github.com/caarlos0/env/v9"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var httpClient = &http.Client{}

type Config struct {
	MirrorFreshnessUrl string        `env:"MIRROR_FRESHNESS_URL"`
	Backends           []string      `env:"BACKENDS" envSeparator:","`
	RefreshInterval    time.Duration `env:"REFRESH_INTERVAL" envDefault:"4h"`
}

type metrics struct {
	mirrorLastUpdatedGauge *prometheus.GaugeVec
}

func newConfig() (*Config, error) {
	cfg := Config{}

	err := env.Parse(&cfg)

	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		mirrorLastUpdatedGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "govuk_mirror_last_updated_time",
			Help: "Last time the mirror was updated",
		}, []string{"backend"}),
	}
	reg.MustRegister(m.mirrorLastUpdatedGauge)
	return m
}

func fetchMirrorFreshnessMetric(backend string, url string) (float64, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return -1, err
	}
	req.Header.Set("Backend-Override", backend)

	resp, err := httpClient.Do(req)
	if err != nil {
		return -1, err
	}

	lastModified := resp.Header.Get("Last-Modified")

	t, err := time.Parse(time.RFC1123, lastModified)
	if err != nil {
		return -1, err
	}

	return float64(t.Unix()), nil
}

func updateMirrorLastUpdatedGauge(m *metrics, url string, backend string) error {
	freshness, err := fetchMirrorFreshnessMetric(backend, url)
	if err != nil {
		return err
	}

	m.mirrorLastUpdatedGauge.With(prometheus.Labels{"backend": backend}).Set(freshness)
	return nil
}

func updateMetrics(m *metrics, cfg *Config) {
	for {
		for _, backend := range cfg.Backends {
			err := updateMirrorLastUpdatedGauge(m, cfg.MirrorFreshnessUrl, backend)
			if err != nil {
				log.Error().Str("metric", "govuk_mirror_last_updated_time").Str("backend", backend).Err(err).Msg("Error updating metrics")
			}
		}
		time.Sleep(cfg.RefreshInterval)
	}
}

func initLogger() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logLevel, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		logLevel = "INFO"
	}
	level, err := zerolog.ParseLevel(logLevel)
	checkError(err, "Error parsing log level")
	zerolog.SetGlobalLevel(level)
}

func checkError(err error, message string) {
	if err != nil {
		log.Fatal().Err(err).Msg(message)
	}
}

func main() {
	initLogger()

	cfg, err := newConfig()
	checkError(err, "Error parsing configuration")

	// Create a non-global registry.
	reg := prometheus.NewRegistry()
	m := newMetrics(reg)

	go updateMetrics(m, cfg)

	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))
	checkError(http.ListenAndServe(":9090", nil), "Server error")
}
