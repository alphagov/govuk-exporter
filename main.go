package main

import (
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var httpClient = &http.Client{}

type metrics struct {
	mirrorLastUpdatedGauge *prometheus.GaugeVec
}

func NewMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		mirrorLastUpdatedGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "govuk_mirror_last_updated_time",
			Help: "Last time the mirror was updated",
		}, []string{"backend"}),
	}
	reg.MustRegister(m.mirrorLastUpdatedGauge)
	return m
}

func fetchMirrorFreshnessMetric(backend string) (float64, error) {
	req, err := http.NewRequest("GET", "https://www.gov.uk/last-updated.txt", nil)
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

func updateMirrorLastUpdatedGauge(backend string, m *metrics) error {
	freshness, err := fetchMirrorFreshnessMetric(backend)
	if err != nil {
		return err
	}

	m.mirrorLastUpdatedGauge.With(prometheus.Labels{"backend": backend}).Set(freshness)
	return nil
}

func updateMetrics(m *metrics) {
	backends := []string{"mirrorS3", "mirrorS3Replica", "mirrorGCS"}

	for {
		time.Sleep(5 * time.Second)
		for _, backend := range backends {
			err := updateMirrorLastUpdatedGauge(backend, m)
			if err != nil {
				log.Error().Str("metric", "govuk_mirror_last_updated_time").Str("backend", backend).Err(err).Msg("Error updating metrics")
			}
		}
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

	// Create a non-global registry.
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	go updateMetrics(m)

	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))
	checkError(http.ListenAndServe(":8000", nil), "Server error")
}
