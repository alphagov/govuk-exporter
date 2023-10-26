package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func setup() (*metrics, *Config) {
	reg := prometheus.NewRegistry()
	m := newMetrics(reg)
	cfg := &Config{
		MirrorFreshnessUrl: "http://mirror.test/freshness",
		Backends:           []string{"backend1", "backend2"},
		RefreshInterval:    1 * time.Hour,
	}
	return m, cfg
}

func createTestServer(lastModified time.Time) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backend := r.Header.Get("Backend-Override")
		if backend == "backend1" || backend == "backend2" {
			if backend == "backend2" {
				lastModified = lastModified.AddDate(0, 0, -1)
			}

			w.Header().Set("Last-Modified", lastModified.Format(http.TimeFormat))
		} else {
			http.Error(w, "Backend-Override header not set to backend1 or backend2", http.StatusBadRequest)
		}
	}))
}

func TestFetchMirrorFreshnessMetric(t *testing.T) {
	timestamp := time.Date(2006, time.January, 2, 15, 4, 5, 0, time.UTC)
	ts := createTestServer(timestamp)
	defer ts.Close()

	freshness, err := fetchMirrorFreshnessMetric("backend1", ts.URL)
	assert.NoError(t, err)

	expectedFreshness := float64(timestamp.Unix())
	assert.Equal(t, expectedFreshness, freshness)
}

func TestUpdateMetrics(t *testing.T) {
	m, cfg := setup()

	timestamp := time.Date(2006, time.January, 2, 15, 4, 5, 0, time.UTC)
	ts := createTestServer(timestamp)
	defer ts.Close()

	cfg.MirrorFreshnessUrl = ts.URL

	go updateMetrics(m, cfg)

	time.Sleep(2 * time.Second)

	assert.Equal(t, 2, testutil.CollectAndCount(m.mirrorLastUpdatedGauge))
	assert.Equal(t, float64(timestamp.Unix()), testutil.ToFloat64(m.mirrorLastUpdatedGauge.WithLabelValues("backend1")))
	assert.Equal(t, float64(timestamp.AddDate(0, 0, -1).Unix()), testutil.ToFloat64(m.mirrorLastUpdatedGauge.WithLabelValues("backend2")))
}
