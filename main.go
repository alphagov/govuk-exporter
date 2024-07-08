package main

import (
	"fmt"
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
	MirrorFreshnessUrl    string        `env:"MIRROR_FRESHNESS_URL"`
	MirrorAvailabilityUrl string        `env:"MIRROR_AVAILABILITY_URL"`
	Backends              []string      `env:"BACKENDS" envSeparator:","`
	RefreshInterval       time.Duration `env:"REFRESH_INTERVAL" envDefault:"4h"`
}

type metrics struct {
	mirrorLastUpdatedGauge   *prometheus.GaugeVec
	mirrorResponseStatusCode *prometheus.GaugeVec
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
		mirrorResponseStatusCode: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "govuk_mirror_response_status_code",
			Help: "Response status code for the MIRROR_AVAILABILITY_URL probe",
		}, []string{"backend"}),
	}
	reg.MustRegister(m.mirrorLastUpdatedGauge)
	reg.MustRegister(m.mirrorResponseStatusCode)
	return m
}

func fetchMirrorAvailabilityMetric(backend string, url string) (httpStatus int, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("Backend-Override", backend)

	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}

	return resp.StatusCode, nil
}

func fetchMirrorFreshnessMetric(backend string, url string) (seconds float64, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("Backend-Override", backend)

	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}

	if resp.StatusCode != http.StatusOK {
		return -1, fmt.Errorf("request failed with status code: %s", resp.Status)
	}

	lastModified := resp.Header.Get("Last-Modified")

	t, err := time.Parse(time.RFC1123, lastModified)
	if err != nil {
		return
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

func updateMirrorResponseStatusCode(m *metrics, url string, backend string) error {
	statusCode, err := fetchMirrorAvailabilityMetric(backend, url)
	if err != nil {
		return err
	}

	m.mirrorResponseStatusCode.With(prometheus.Labels{"backend": backend}).Set(float64(statusCode))
	return nil
}

func updateMetrics(m *metrics, cfg *Config) {
	for {
		for _, backend := range cfg.Backends {
			err := updateMirrorLastUpdatedGauge(m, cfg.MirrorFreshnessUrl, backend)
			if err != nil {
				log.Error().Str("metric", "govuk_mirror_last_updated_time").Str("backend", backend).Err(err).Msg("Error updating metrics")
			}

			err = updateMirrorResponseStatusCode(m, cfg.MirrorAvailabilityUrl, backend)
			if err != nil {
				log.Error().Str("metric", "govuk_mirror_response_status_code").Str("backend", backend).Err(err).Msg("Error updating metrics")
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
