FROM golang:1.25.0-bookworm@sha256:81dc45d05a7444ead8c92a389621fafabc8e40f8fd1a19d7e5df14e61e98bc1a AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/seshatops ./cmd/seshatops

FROM python:3.13.7-slim-bookworm@sha256:adafcc17694d715c905b4c7bebd96907a1fd5cf183395f0ebc4d3428bd22d92d

RUN groupadd --system --gid 10001 seshatops \
    && useradd --system --uid 10001 --gid seshatops --home-dir /nonexistent --shell /usr/sbin/nologin seshatops
WORKDIR /app
COPY --from=build /out/seshatops /app/seshatops
COPY forecast_candidate /app/forecast_candidate
COPY scripts/local-smoke.py /app/scripts/local-smoke.py
USER 10001:10001
ENTRYPOINT ["/app/seshatops"]
