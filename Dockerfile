ARG go_registry=""
ARG go_version=1.22
ARG go_tag_suffix=-alpine


FROM --platform=$TARGETPLATFORM ${go_registry}golang:${go_version}${go_tag_suffix} AS builder
ARG TARGETARCH TARGETOS
ARG GOARCH=$TARGETARCH GOOS=$TARGETOS
ARG CGO_ENABLED=0

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN go build -o /bin/govuk-exporter main.go


FROM --platform=$TARGETPLATFORM scratch
COPY --from=builder /bin/govuk-exporter /bin/govuk-exporter
COPY --from=builder /usr/share/ca-certificates /usr/share/ca-certificates
COPY --from=builder /etc/ssl /etc/ssl
USER 1001
CMD ["/bin/govuk-exporter"]

LABEL org.opencontainers.image.source="https://github.com/alphagov/govuk-exporter"
LABEL org.opencontainers.image.license=MIT
