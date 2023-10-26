# GOV.UK Exporter

A Prometheus exporter that exposes the following metrics:

- `govuk_mirror_last_updated_time`: A unix timestamp representing the Last-Modified header of the page referenced by the MIRROR_FRESHNESS_URL. Has the label `backend` for each backend override being used.

## Usage

Configuration is handled through environment variables as listed below:

- MIRROR_FRESHNESS_URL: Specifies the URL to probe for Mirror freshness.
    - Example: `MIRROR_FRESHNESS_URL=https://www.gov.uk/last-updated.txt`
- BACKENDS: A comma-separated list of backend overrides to collect metrics for.
    - Example: `BACKENDS=mirrorS3,mirrorS3Replica,mirrorGCS`
- REFRESH_INTERVAL: The interval refresh the metrics. Defaults to `4h`.
    - Example: `REFRESH_INTERVAL=4h`


