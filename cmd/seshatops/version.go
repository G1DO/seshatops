package main

import (
	"encoding/json"
	"net/http"
	goruntime "runtime"
	"runtime/debug"

	"github.com/G1DO/seshatops/forecast"
	"github.com/G1DO/seshatops/northstar"
)

// Version, Commit and BuildTime are set via -ldflags at build time:
//
//	go build -ldflags="-X main.Version=v0.1.0 -X main.Commit=$GITSHA -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var (
	Version   = "v0.1.0"
	Commit    = "unknown"
	BuildTime = "unknown"
)

type versionPayload struct {
	Version            string            `json:"version"`
	Commit             string            `json:"commit"`
	BuildTime          string            `json:"build_time"`
	GoVersion          string            `json:"go_version"`
	FixtureVersions    map[string]string `json:"fixture_versions"`
	ProtocolVersions   map[string]string `json:"protocol_versions"`
	ArtifactChecksums  map[string]string `json:"artifact_checksums"`
}

func buildVersionPayload() versionPayload {
	goVersion := goruntime.Version()
	if info, ok := debug.ReadBuildInfo(); ok && info.GoVersion != "" {
		goVersion = info.GoVersion
	}
	return versionPayload{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
		GoVersion: goVersion,
		FixtureVersions: map[string]string{
			"northstar-m3-lineage-v1":          northstar.LineageSeed,
			"northstar-m4-stockout-v1":         forecast.HistorySeed,
			"northstar-m5-poison-v1":           demoPoisonFixtureVersion,
			"northstar-m5-forecast-incomplete-v1": demoForecastIncompleteFixtureVersion,
		},
		ProtocolVersions: map[string]string{
			"m4-stockout-eval-v1":            forecast.ProtocolID,
			"m4-raw-onhand-v1":               forecast.FeatureDefinitionVersion,
			"m4-deterministic-baselines-v1":  forecast.EvaluationCodeVersion,
			"m4-python-stockout-candidate-v1": forecast.CandidateCodeVersion,
			"m4-onhand-bucket-rate-v1":       forecast.CandidateModelVersion,
			"event_schema_version":           "1",
		},
		ArtifactChecksums: map[string]string{
			"frozen_m4_dataset":          frozenM4DatasetChecksum,
			"frozen_m4_feature_snapshot": frozenM4FeatureSnapshotChecksum,
			"frozen_m4_snapshot_id":      frozenM4FeatureSnapshotID,
		},
	}
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeHealthJSON(w, http.StatusMethodNotAllowed, "not_ready")
		return
	}
	payload := buildVersionPayload()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
