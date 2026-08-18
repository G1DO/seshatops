package api_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/G1DO/seshatops/api"
	"github.com/G1DO/seshatops/forecast"
	"github.com/G1DO/seshatops/identity"
	"github.com/G1DO/seshatops/platform"
)

func forecastPredictionPath(tenantID, resourceID string) string {
	return "/v1/tenants/" + tenantID + "/forecast/predictions/" + resourceID
}

func TestForecastPredictionReturnsLatestLineageAndFreshness(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	seedCompactForecastHistory(t, db, fx)
	snapshot := currentFeatureSnapshot(t, db, fx.TenantID)

	older := validPredictionRecord(t, fx.TenantID, fx.ItemID, snapshot, "2026-01-05", forecast.CandidatePredictionStatusPredicted)
	older.FeatureSnapshotID = "older-feature-snapshot"
	older.FeatureSnapshotChecksum = "older-feature-checksum"
	older.RecordedAt = time.Date(2026, time.January, 11, 10, 0, 0, 0, time.UTC)
	persistPrediction(t, db, older)
	latest := validPredictionRecord(t, fx.TenantID, fx.ItemID, snapshot, "2026-01-05", forecast.CandidatePredictionStatusPredicted)
	latest.RecordedAt = time.Date(2026, time.January, 12, 10, 0, 0, 0, time.UTC)
	persistPrediction(t, db, latest)

	ts, sess := gatedSession(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))
	resp := getWithSession(t, ts.URL+forecastPredictionPath(fx.TenantID, fx.ItemID), sess, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}

	var prediction api.ForecastPredictionSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&prediction); err != nil {
		t.Fatal(err)
	}
	if prediction.TenantID != fx.TenantID || prediction.ResourceID != fx.ItemID || prediction.ObservationDate != "2026-01-05" || prediction.Status != forecast.CandidatePredictionStatusPredicted {
		t.Fatalf("prediction=%+v", prediction)
	}
	if prediction.Freshness.Status != "fresh" || prediction.Freshness.FreshAt == "" {
		t.Fatalf("freshness=%+v", prediction.Freshness)
	}
	if prediction.StockoutRisk == nil || prediction.Uncertainty == nil {
		t.Fatalf("prediction output=%+v", prediction)
	}
	if prediction.Lineage.FeatureSnapshotID != snapshot.SnapshotID || prediction.Lineage.FeatureSnapshotChecksum != snapshot.Checksum || prediction.Lineage.Predictor != forecast.RuntimePredictorBaseline || prediction.Lineage.ModelVersion != forecast.BaselineMovingAverage || prediction.Lineage.CodeVersion != forecast.EvaluationCodeVersion {
		t.Fatalf("lineage=%+v", prediction.Lineage)
	}
	if prediction.CorrelationID == "" || prediction.RecordedAt == "" || prediction.ObservedAt == "" {
		t.Fatalf("timestamps/correlation=%+v", prediction)
	}
	raw, err := json.Marshal(prediction)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "quantity_on_hand") || strings.Contains(string(raw), "model_artifact") {
		t.Fatalf("unsafe prediction response: %s", raw)
	}
}

func TestForecastPredictionRepresentsStaleAndAbstainedResults(t *testing.T) {
	t.Run("stale", func(t *testing.T) {
		db := openTestDB(t)
		fx := mustFixture(t)
		seedCompactForecastHistory(t, db, fx)
		snapshot := currentFeatureSnapshot(t, db, fx.TenantID)
		record := validPredictionRecord(t, fx.TenantID, fx.ItemID, snapshot, "2026-01-05", forecast.CandidatePredictionStatusPredicted)
		record.FeatureSnapshotID = "old-feature-snapshot"
		record.FeatureSnapshotChecksum = "old-feature-checksum"
		persistPrediction(t, db, record)

		ts, sess := gatedSession(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))
		resp := getWithSession(t, ts.URL+forecastPredictionPath(fx.TenantID, fx.ItemID), sess, nil)
		defer resp.Body.Close()
		var prediction api.ForecastPredictionSnapshot
		if err := json.NewDecoder(resp.Body).Decode(&prediction); err != nil {
			t.Fatal(err)
		}
		if prediction.Freshness.Status != "stale" || prediction.StockoutRisk == nil {
			t.Fatalf("prediction=%+v", prediction)
		}
	})

	t.Run("abstained", func(t *testing.T) {
		db := openTestDB(t)
		fx := mustFixture(t)
		record := validPredictionRecord(t, fx.TenantID, fx.ItemID, forecast.FeatureSnapshot{
			DatasetVersion:           forecast.ProtocolID,
			FeatureDefinitionVersion: forecast.FeatureDefinitionVersion,
			SnapshotID:               "abstained-snapshot",
			Checksum:                 "abstained-checksum",
		}, "2026-01-05", forecast.CandidatePredictionStatusAbstained)
		persistPrediction(t, db, record)

		ts, sess := gatedSession(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))
		resp := getWithSession(t, ts.URL+forecastPredictionPath(fx.TenantID, fx.ItemID), sess, nil)
		defer resp.Body.Close()
		var prediction api.ForecastPredictionSnapshot
		if err := json.NewDecoder(resp.Body).Decode(&prediction); err != nil {
			t.Fatal(err)
		}
		if prediction.Status != forecast.CandidatePredictionStatusAbstained || prediction.StockoutRisk != nil || prediction.Uncertainty != nil || prediction.AbstentionReason == "" {
			t.Fatalf("prediction=%+v", prediction)
		}
	})
}

func TestForecastPredictionUnavailableAndAuthorizationFailClosed(t *testing.T) {
	db := openTestDB(t)
	fx := mustFixture(t)
	record := validPredictionRecord(t, fx.TenantID, fx.ItemID, forecast.FeatureSnapshot{
		DatasetVersion:           forecast.ProtocolID,
		FeatureDefinitionVersion: forecast.FeatureDefinitionVersion,
		SnapshotID:               "unavailable-snapshot",
		Checksum:                 "unavailable-checksum",
	}, "2026-01-05", forecast.CandidatePredictionStatusPredicted)
	persistPrediction(t, db, record)

	ts, sess := gatedSession(t, db, "operator-northstar", northstarReaderPolicy("operator-northstar"))
	resp := getWithSession(t, ts.URL+forecastPredictionPath(fx.TenantID, fx.ItemID), sess, nil)
	defer resp.Body.Close()
	var unavailable api.ForecastPredictionSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&unavailable); err != nil {
		t.Fatal(err)
	}
	if unavailable.Freshness.Status != "unavailable" {
		t.Fatalf("freshness=%+v", unavailable.Freshness)
	}

	resp = getWithSession(t, ts.URL+forecastPredictionPath(fx.TenantID, "missing-item"), sess, nil)
	if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("missing prediction status=%d body=%s", resp.StatusCode, body)
	}
	_ = resp.Body.Close()
	resp = getWithSession(t, ts.URL+forecastPredictionPath(fx.TenantID, fx.ItemID)+"?horizon=7", sess, nil)
	if resp.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("unsupported query status=%d body=%s", resp.StatusCode, body)
	}
	_ = resp.Body.Close()

	resp = getWithSession(t, ts.URL+forecastPredictionPath(identity.TenantNS002UUID, fx.ItemID), sess, nil)
	assertForbiddenNoProjection(t, resp)

	operatorTS, operatorSession := gatedSession(t, db, "platform-operator", northstarOperatorPolicy("platform-operator"))
	resp = getWithSession(t, operatorTS.URL+forecastPredictionPath(fx.TenantID, fx.ItemID), operatorSession, nil)
	assertForbiddenNoProjection(t, resp)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		req := httptest.NewRequest(method, ts.URL+forecastPredictionPath(fx.TenantID, fx.ItemID), nil)
		req.AddCookie(&http.Cookie{Name: identity.DefaultCookieName, Value: sess.ID})
		recorder := httptest.NewRecorder()
		ts.Config.Handler.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != http.MethodGet {
			t.Fatalf("%s status=%d allow=%q body=%s", method, recorder.Code, recorder.Header().Get("Allow"), recorder.Body.String())
		}
	}

	noSession := api.NewServer(db, nil, nil, northstarReaderPolicy())
	request := httptest.NewRequest(http.MethodGet, forecastPredictionPath(fx.TenantID, fx.ItemID), nil)
	recorder := httptest.NewRecorder()
	noSession.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("missing session status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func currentFeatureSnapshot(t *testing.T, db *sql.DB, tenantID string) forecast.FeatureSnapshot {
	t.Helper()
	source, err := platform.LoadTenantForecastSource(context.Background(), db, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := forecast.BuildFeatureSnapshot(source.History, tenantID, source.Boundary)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func validPredictionRecord(t *testing.T, tenantID, resourceID string, snapshot forecast.FeatureSnapshot, observationDate, status string) platform.PredictionRecord {
	t.Helper()
	score := 0.25
	record := platform.PredictionRecord{
		TenantID:                  tenantID,
		ResourceType:              "inventory_item",
		ResourceID:                resourceID,
		ObservationDate:           observationDate,
		ForecastHorizonDays:       forecast.HorizonDays,
		Target:                    forecast.CandidateTarget,
		Status:                    status,
		StockoutRisk:              &score,
		Uncertainty:               &forecast.PredictionUncertainty{Method: forecast.RuntimeUncertaintyDeterministic, Lower: 0, Upper: 1, SampleCount: 0},
		EvaluationProtocolVersion: forecast.ProtocolID,
		DatasetVersion:            snapshot.DatasetVersion,
		FeatureDefinitionVersion:  snapshot.FeatureDefinitionVersion,
		FeatureSnapshotID:         snapshot.SnapshotID,
		FeatureSnapshotChecksum:   snapshot.Checksum,
		SourceCutoffDate:          observationDate,
		Predictor:                 forecast.RuntimePredictorBaseline,
		ModelVersion:              forecast.BaselineMovingAverage,
		CodeVersion:               forecast.EvaluationCodeVersion,
		FreshAt:                   time.Date(2026, time.January, 12, 9, 0, 0, 0, time.UTC),
		RecordedAt:                time.Date(2026, time.January, 12, 10, 0, 0, 0, time.UTC),
		CorrelationID:             "018f5d78-6e64-4f5f-bd16-8e9f7c4a2999",
	}
	if status == forecast.CandidatePredictionStatusAbstained {
		record.StockoutRisk = nil
		record.Uncertainty = nil
		record.AbstentionReason = forecast.CandidateAbstentionInsufficientSupport
	}
	return record
}

func persistPrediction(t *testing.T, db *sql.DB, record platform.PredictionRecord) {
	t.Helper()
	if _, err := platform.PersistPrediction(context.Background(), db, record); err != nil {
		t.Fatal(err)
	}
}
