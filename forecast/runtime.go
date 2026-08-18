package forecast

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	// RuntimeContractVersion identifies the narrow Go-to-forecaster contract.
	RuntimeContractVersion = 1

	RuntimePredictorCandidate = "candidate"
	RuntimePredictorBaseline  = "baseline"

	RuntimeUncertaintyDeterministic = "deterministic"

	RuntimeAbstentionInsufficientFeatureHistory = "insufficient_feature_history"
)

// RuntimeSelection is the only predictor choice accepted by the runtime
// boundary. It is derived from the frozen candidate evaluation outcome.
type RuntimeSelection struct {
	Predictor    string
	BaselineID   string
	ModelVersion string
	CodeVersion  string
}

// RuntimeRequest is the typed, read-only input sent to a learned forecaster.
// It contains one authorized feature row and immutable lineage, never labels,
// credentials, or mutable platform state.
type RuntimeRequest struct {
	ContractVersion          int                `json:"contract_version"`
	Predictor                string             `json:"predictor"`
	TenantID                 string             `json:"tenant_id"`
	ItemID                   string             `json:"item_id"`
	RowID                    string             `json:"row_id"`
	ObservationDate          string             `json:"observation_date"`
	Target                   string             `json:"target"`
	HorizonDays              int                `json:"horizon_days"`
	FeatureDefinitionVersion string             `json:"feature_definition_version"`
	FeatureSnapshotID        string             `json:"feature_snapshot_id"`
	FeatureSnapshotChecksum  string             `json:"feature_snapshot_checksum"`
	ModelVersion             string             `json:"model_version"`
	CodeVersion              string             `json:"code_version"`
	Feature                  FeatureRow         `json:"feature"`
	ModelArtifact            *CandidateArtifact `json:"model_artifact,omitempty"`
}

// RuntimeResponse is the typed result returned by a learned forecaster. A
// nil StockoutRisk is an explicit abstention and must carry a known reason.
type RuntimeResponse struct {
	ContractVersion          int                    `json:"contract_version"`
	Predictor                string                 `json:"predictor"`
	TenantID                 string                 `json:"tenant_id"`
	ItemID                   string                 `json:"item_id"`
	RowID                    string                 `json:"row_id"`
	ObservationDate          string                 `json:"observation_date"`
	SourceCutoffDate         string                 `json:"source_cutoff_date"`
	Target                   string                 `json:"target"`
	HorizonDays              int                    `json:"horizon_days"`
	FeatureDefinitionVersion string                 `json:"feature_definition_version"`
	FeatureSnapshotID        string                 `json:"feature_snapshot_id"`
	FeatureSnapshotChecksum  string                 `json:"feature_snapshot_checksum"`
	ModelVersion             string                 `json:"model_version"`
	CodeVersion              string                 `json:"code_version"`
	Status                   string                 `json:"status"`
	StockoutRisk             *float64               `json:"stockout_risk,omitempty"`
	Uncertainty              *PredictionUncertainty `json:"uncertainty,omitempty"`
	AbstentionReason         string                 `json:"abstention_reason,omitempty"`
}

// SelectRuntimePredictor derives the runtime choice from the frozen test
// outcome. It rejects incomplete or contradictory evaluation records rather
// than allowing a caller to force the learned candidate.
func SelectRuntimePredictor(evaluation CandidateEvaluation) (RuntimeSelection, error) {
	if evaluation.EvaluationProtocolVersion != ProtocolID || evaluation.PromotionSplit != CandidatePromotionSplit {
		return RuntimeSelection{}, wrapInvalid("runtime evaluation lineage")
	}
	var test *CandidateSplitEvaluation
	for i := range evaluation.Splits {
		if evaluation.Splits[i].Split == CandidatePromotionSplit {
			test = &evaluation.Splits[i]
			break
		}
	}
	if test == nil {
		return RuntimeSelection{}, wrapInvalid("runtime evaluation has no test outcome")
	}

	switch evaluation.Outcome {
	case CandidateOutcomeCandidate:
		if !evaluation.PromotionEligible || !test.CandidateBeatsBaseline || test.BaselineID == "" || evaluation.ModelVersion == "" || evaluation.CodeVersion == "" {
			return RuntimeSelection{}, wrapInvalid("candidate outcome is not promotion-eligible")
		}
		return RuntimeSelection{
			Predictor:    RuntimePredictorCandidate,
			ModelVersion: evaluation.ModelVersion,
			CodeVersion:  evaluation.CodeVersion,
		}, nil
	case CandidateOutcomeBaseline:
		if evaluation.PromotionEligible || test.CandidateBeatsBaseline || (test.BaselineID != BaselineMovingAverage && test.BaselineID != BaselineSeasonalNaive) || evaluation.BaselineEvaluation.CodeVersion == "" {
			return RuntimeSelection{}, wrapInvalid("baseline outcome is contradictory")
		}
		return RuntimeSelection{
			Predictor:    RuntimePredictorBaseline,
			BaselineID:   test.BaselineID,
			ModelVersion: test.BaselineID,
			CodeVersion:  evaluation.BaselineEvaluation.CodeVersion,
		}, nil
	default:
		return RuntimeSelection{}, wrapInvalid("runtime outcome %q has no predictor", evaluation.Outcome)
	}
}

// ValidateFeatureSnapshot validates the immutable metadata and row identity
// required before a snapshot can enter runtime inference.
func ValidateFeatureSnapshot(snapshot FeatureSnapshot) error {
	if snapshot.ContractVersion != SnapshotContractVersion || snapshot.Status != SnapshotStatusComplete {
		return wrapInvalid("runtime feature snapshot status or contract")
	}
	tenant := strings.ToLower(strings.TrimSpace(snapshot.TenantID))
	if !featureTenantUUIDv4.MatchString(tenant) || tenant != snapshot.TenantID {
		return wrapInvalid("runtime feature snapshot tenant")
	}
	if snapshot.DatasetVersion != ProtocolID || snapshot.FeatureDefinitionVersion != FeatureDefinitionVersion {
		return wrapInvalid("runtime feature snapshot version")
	}
	if snapshot.Checksum == "" || snapshot.Checksum != FeatureChecksum(snapshot) || snapshot.SnapshotID == "" || snapshot.SnapshotID != snapshotIdentity(snapshot) {
		return wrapInvalid("runtime feature snapshot identity")
	}
	seen := make(map[string]struct{}, len(snapshot.Rows))
	for _, row := range snapshot.Rows {
		if row.TenantID != tenant || row.ItemID == "" || row.RowID != RowID(row.TenantID, row.ItemID, row.AsOfDate) {
			return wrapInvalid("runtime feature row identity")
		}
		if _, err := parseDate(row.AsOfDate); err != nil {
			return err
		}
		if row.SourceCutoffDate != row.AsOfDate || row.QuantityOnHand < 0 || !isEvaluationSplit(row.Split) || row.HistoryHash == "" {
			return wrapInvalid("runtime feature row %s", row.RowID)
		}
		if _, duplicate := seen[row.RowID]; duplicate {
			return wrapInvalid("runtime duplicate feature row %s", row.RowID)
		}
		seen[row.RowID] = struct{}{}
	}
	if len(snapshot.Rows) == 0 {
		return wrapInvalid("runtime feature snapshot has no rows")
	}
	return nil
}

// NewRuntimeRequest creates the one-row request sent to a learned predictor.
func NewRuntimeRequest(snapshot FeatureSnapshot, tenantID, itemID, observationDate string, selection RuntimeSelection, artifact *CandidateArtifact) (RuntimeRequest, error) {
	if err := ValidateFeatureSnapshot(snapshot); err != nil {
		return RuntimeRequest{}, err
	}
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	itemID = strings.ToLower(strings.TrimSpace(itemID))
	if tenantID != snapshot.TenantID || itemID == "" {
		return RuntimeRequest{}, wrapInvalid("runtime feature tenant or item")
	}
	if _, err := parseDate(observationDate); err != nil {
		return RuntimeRequest{}, err
	}
	var row FeatureRow
	found := false
	for _, candidate := range snapshot.Rows {
		if candidate.TenantID == tenantID && candidate.ItemID == itemID && candidate.AsOfDate == observationDate {
			row = candidate
			found = true
			break
		}
	}
	if !found {
		return RuntimeRequest{}, wrapInvalid("runtime feature row not found")
	}
	request := RuntimeRequest{
		ContractVersion:          RuntimeContractVersion,
		Predictor:                selection.Predictor,
		TenantID:                 tenantID,
		ItemID:                   itemID,
		RowID:                    row.RowID,
		ObservationDate:          observationDate,
		Target:                   CandidateTarget,
		HorizonDays:              HorizonDays,
		FeatureDefinitionVersion: snapshot.FeatureDefinitionVersion,
		FeatureSnapshotID:        snapshot.SnapshotID,
		FeatureSnapshotChecksum:  snapshot.Checksum,
		ModelVersion:             selection.ModelVersion,
		CodeVersion:              selection.CodeVersion,
		Feature:                  row,
		ModelArtifact:            artifact,
	}
	if err := ValidateRuntimeRequest(request); err != nil {
		return RuntimeRequest{}, err
	}
	return request, nil
}

// ValidateRuntimeRequest validates all values crossing into a learned
// forecaster. It deliberately contains no database or authorization logic.
func ValidateRuntimeRequest(request RuntimeRequest) error {
	if request.ContractVersion != RuntimeContractVersion || (request.Predictor != RuntimePredictorCandidate && request.Predictor != RuntimePredictorBaseline) {
		return wrapInvalid("runtime request contract or predictor")
	}
	if request.TenantID == "" || request.ItemID == "" || request.RowID != RowID(request.TenantID, request.ItemID, request.ObservationDate) {
		return wrapInvalid("runtime request identity")
	}
	if _, err := parseDate(request.ObservationDate); err != nil {
		return err
	}
	if request.Target != CandidateTarget || request.HorizonDays != HorizonDays || request.FeatureDefinitionVersion != FeatureDefinitionVersion || request.FeatureSnapshotID == "" || request.FeatureSnapshotChecksum == "" || request.ModelVersion == "" || request.CodeVersion == "" {
		return wrapInvalid("runtime request lineage")
	}
	if request.Feature.TenantID != request.TenantID || request.Feature.ItemID != request.ItemID || request.Feature.RowID != request.RowID || request.Feature.AsOfDate != request.ObservationDate || request.Feature.SourceCutoffDate != request.ObservationDate || request.Feature.QuantityOnHand < 0 {
		return wrapInvalid("runtime request feature")
	}
	if request.ModelArtifact != nil {
		artifact := request.ModelArtifact
		if artifact.ModelVersion != request.ModelVersion || artifact.CodeVersion != request.CodeVersion || artifact.EvaluationProtocolVersion != ProtocolID || artifact.FeatureDefinitionVersion != request.FeatureDefinitionVersion || artifact.FeatureSnapshotID != request.FeatureSnapshotID || artifact.FeatureSnapshotChecksum != request.FeatureSnapshotChecksum {
			return wrapInvalid("runtime request model artifact lineage")
		}
	}
	return nil
}

// ValidateRuntimeResponse checks a response against the exact request that
// produced it. Unknown JSON fields are rejected by the subprocess adapter.
func ValidateRuntimeResponse(request RuntimeRequest, response RuntimeResponse) error {
	if err := ValidateRuntimeRequest(request); err != nil {
		return err
	}
	if response.ContractVersion != RuntimeContractVersion || response.Predictor != request.Predictor || response.TenantID != request.TenantID || response.ItemID != request.ItemID || response.RowID != request.RowID || response.ObservationDate != request.ObservationDate || response.SourceCutoffDate != request.ObservationDate || response.Target != request.Target || response.HorizonDays != request.HorizonDays || response.FeatureDefinitionVersion != request.FeatureDefinitionVersion || response.FeatureSnapshotID != request.FeatureSnapshotID || response.FeatureSnapshotChecksum != request.FeatureSnapshotChecksum || response.ModelVersion != request.ModelVersion || response.CodeVersion != request.CodeVersion {
		return wrapInvalid("runtime response lineage")
	}
	switch response.Status {
	case CandidatePredictionStatusPredicted:
		if response.StockoutRisk == nil || response.Uncertainty == nil || response.AbstentionReason != "" {
			return wrapInvalid("runtime predicted response")
		}
		if err := validateCandidateScore(*response.StockoutRisk); err != nil {
			return err
		}
		if err := validateCandidateUncertainty(*response.Uncertainty, *response.StockoutRisk); err != nil {
			return err
		}
	case CandidatePredictionStatusAbstained:
		if response.StockoutRisk != nil || response.Uncertainty != nil || !isRuntimeAbstentionReason(response.AbstentionReason) {
			return wrapInvalid("runtime abstained response")
		}
	default:
		return wrapInvalid("runtime response status %q", response.Status)
	}
	return nil
}

func isRuntimeAbstentionReason(reason string) bool {
	return isCandidateAbstentionReason(reason) || reason == RuntimeAbstentionInsufficientFeatureHistory
}

// BaselineRuntimeResponse computes the selected deterministic baseline from
// raw feature rows at or before the observation date. Missing history becomes
// an explicit abstention instead of an inferred value.
func BaselineRuntimeResponse(snapshot FeatureSnapshot, request RuntimeRequest) (RuntimeResponse, error) {
	if request.Predictor != RuntimePredictorBaseline {
		return RuntimeResponse{}, wrapInvalid("baseline request predictor")
	}
	if err := ValidateFeatureSnapshot(snapshot); err != nil {
		return RuntimeResponse{}, err
	}
	if request.TenantID != snapshot.TenantID || request.FeatureSnapshotID != snapshot.SnapshotID || request.FeatureSnapshotChecksum != snapshot.Checksum {
		return RuntimeResponse{}, wrapInvalid("baseline request snapshot lineage")
	}
	if request.ModelVersion != BaselineSeasonalNaive && request.ModelVersion != BaselineMovingAverage {
		return RuntimeResponse{}, wrapInvalid("unsupported baseline %q", request.ModelVersion)
	}
	rows := make(map[string]FeatureRow)
	for _, row := range snapshot.Rows {
		if row.TenantID == request.TenantID && row.ItemID == request.ItemID && row.AsOfDate <= request.ObservationDate {
			rows[row.AsOfDate] = row
		}
	}
	response := RuntimeResponse{
		ContractVersion:          RuntimeContractVersion,
		Predictor:                RuntimePredictorBaseline,
		TenantID:                 request.TenantID,
		ItemID:                   request.ItemID,
		RowID:                    request.RowID,
		ObservationDate:          request.ObservationDate,
		SourceCutoffDate:         request.ObservationDate,
		Target:                   request.Target,
		HorizonDays:              request.HorizonDays,
		FeatureDefinitionVersion: request.FeatureDefinitionVersion,
		FeatureSnapshotID:        request.FeatureSnapshotID,
		FeatureSnapshotChecksum:  request.FeatureSnapshotChecksum,
		ModelVersion:             request.ModelVersion,
		CodeVersion:              request.CodeVersion,
		Status:                   CandidatePredictionStatusPredicted,
	}
	var score float64
	var complete bool
	switch request.ModelVersion {
	case BaselineMovingAverage:
		complete = true
		var zeroDays int
		for offset := 0; offset < HorizonDays; offset++ {
			date, ok := addDays(request.ObservationDate, -offset)
			if !ok {
				complete = false
				break
			}
			row, ok := rows[date]
			if !ok {
				complete = false
				break
			}
			if row.QuantityOnHand == 0 {
				zeroDays++
			}
		}
		if complete {
			score = float64(zeroDays) / float64(HorizonDays)
		}
	case BaselineSeasonalNaive:
		lookback, ok := addDays(request.ObservationDate, -HorizonDays)
		if ok {
			complete = true
			for offset := 1; offset <= HorizonDays; offset++ {
				date, dateOK := addDays(lookback, offset)
				if !dateOK {
					complete = false
					break
				}
				row, rowOK := rows[date]
				if !rowOK {
					complete = false
					break
				}
				if row.QuantityOnHand == 0 {
					score = 1
					break
				}
			}
		}
	}
	if !complete {
		response.Status = CandidatePredictionStatusAbstained
		response.AbstentionReason = RuntimeAbstentionInsufficientFeatureHistory
		return response, nil
	}
	response.StockoutRisk = &score
	response.Uncertainty = &PredictionUncertainty{Method: RuntimeUncertaintyDeterministic, Lower: score, Upper: score}
	return response, nil
}

// PredictionIdentity returns the stable identity for one tenant-scoped
// prediction input. Predictor choice is intentionally excluded: a changed
// predictor for the same immutable input is a persistence conflict, not a
// second current-state effect.
func PredictionIdentity(tenantID, itemID, observationDate string, horizonDays int, datasetVersion, featureDefinitionVersion, snapshotID, snapshotChecksum string) string {
	canonical := strings.Join([]string{
		strings.ToLower(strings.TrimSpace(tenantID)),
		strings.ToLower(strings.TrimSpace(itemID)),
		observationDate,
		fmt.Sprintf("%d", horizonDays),
		datasetVersion,
		featureDefinitionVersion,
		snapshotID,
		snapshotChecksum,
	}, "\t") + "\n"
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}
