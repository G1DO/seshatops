package forecast

import (
	"math"
	"sort"
)

// Prediction is one split row's score. A nil Score is an explicit abstention.
type Prediction struct {
	RowID string
	Score *float64
}

// Result is abstention-aware evaluation of one split.
type Result struct {
	Split     string
	N         int
	Predicted int
	Abstained int
	Coverage  float64
	AP        *float64
	Brier     *float64
	Defined   bool
	Reason    string
}

type scoredRow struct {
	rowID string
	score float64
	label int
}

// Evaluate scores one named split. Predictions must be exactly the split rows.
func Evaluate(ds Dataset, split string, preds []Prediction) (Result, error) {
	if split != SplitTrain && split != SplitValidation && split != SplitTest {
		return Result{}, wrapInvalid("split %s", split)
	}

	var examples []Example
	for _, e := range ds.Examples {
		if e.Split == split {
			examples = append(examples, e)
		}
	}
	byID, err := indexPredictions(preds)
	if err != nil {
		return Result{}, err
	}

	base := Result{Split: split, N: len(examples)}
	if len(examples) == 0 {
		if len(byID) != 0 {
			return Result{}, wrapInvalid("extra row_id")
		}
		return undefined(base, "empty split"), nil
	}
	if len(byID) != len(examples) {
		return Result{}, wrapInvalid("prediction count %d, want %d", len(byID), len(examples))
	}

	scored := make([]scoredRow, 0, len(examples))
	for _, e := range examples {
		score, ok := byID[e.RowID]
		if !ok {
			return Result{}, wrapInvalid("omitted row_id %s", e.RowID)
		}
		delete(byID, e.RowID)
		if score == nil {
			base.Abstained++
			continue
		}
		scored = append(scored, scoredRow{rowID: e.RowID, score: *score, label: e.Label})
	}
	if len(byID) != 0 {
		return Result{}, wrapInvalid("extra row_id")
	}

	base.Predicted = len(scored)
	if base.N > 0 {
		base.Coverage = float64(base.Predicted) / float64(base.N)
	}

	var pos, neg int
	for _, e := range examples {
		if e.Label == 1 {
			pos++
		} else {
			neg++
		}
	}
	if pos == 0 || neg == 0 {
		return undefined(base, "degenerate class"), nil
	}
	if base.Predicted == 0 {
		return undefined(base, "all abstain"), nil
	}

	brier := brierScore(scored)
	base.Brier = &brier

	var scoredPos int
	for _, r := range scored {
		if r.label == 1 {
			scoredPos++
		}
	}
	if scoredPos == 0 {
		return undefined(base, "undefined average precision"), nil
	}
	ap := averagePrecision(scored)
	base.AP = &ap
	base.Defined = true
	return base, nil
}

// Qualifies reports whether r meets the frozen coverage and metric gates.
func Qualifies(r Result) bool {
	if !r.Defined || r.AP == nil || r.Brier == nil {
		return false
	}
	return r.Predicted*100 >= r.N*CoverageFloorPercent
}

// QualifyingBaseline returns the winning declared baseline id among results
// that pass the quality gates.
func QualifyingBaseline(results map[string]Result) (string, bool) {
	ids := []string{BaselineMovingAverage, BaselineSeasonalNaive}
	var bestID string
	var best Result
	found := false
	for _, id := range ids {
		r, ok := results[id]
		if !ok || !Qualifies(r) {
			continue
		}
		if !found || baselineBetter(r, id, best, bestID) {
			bestID = id
			best = r
			found = true
		}
	}
	return bestID, found
}

// CandidatePromoted reports whether candidate beats the qualifying baseline.
func CandidatePromoted(candidate Result, baselineID string, baseline Result) (bool, string) {
	if baselineID == "" || !Qualifies(baseline) {
		return false, "no qualifying baseline"
	}
	if !Qualifies(candidate) {
		return false, "candidate failed quality gates"
	}
	if *candidate.AP <= *baseline.AP {
		return false, "candidate average precision does not beat baseline"
	}
	if *candidate.Brier > *baseline.Brier {
		return false, "candidate Brier is worse than baseline"
	}
	return true, "candidate beats qualifying baseline"
}

func indexPredictions(preds []Prediction) (map[string]*float64, error) {
	byID := make(map[string]*float64, len(preds))
	for _, p := range preds {
		if p.RowID == "" {
			return nil, wrapInvalid("empty row_id")
		}
		if _, dup := byID[p.RowID]; dup {
			return nil, wrapInvalid("duplicate row_id %s", p.RowID)
		}
		if p.Score != nil {
			s := *p.Score
			if math.IsNaN(s) || math.IsInf(s, 0) || s < 0 || s > 1 {
				return nil, wrapInvalid("score %v", s)
			}
		}
		byID[p.RowID] = p.Score
	}
	return byID, nil
}

func averagePrecision(rows []scoredRow) float64 {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].score != rows[j].score {
			return rows[i].score > rows[j].score
		}
		return rows[i].rowID < rows[j].rowID
	})
	var pos int
	for _, r := range rows {
		if r.label == 1 {
			pos++
		}
	}
	var seen int
	var sum float64
	for i, r := range rows {
		if r.label != 1 {
			continue
		}
		seen++
		sum += float64(seen) / float64(i+1)
	}
	return sum / float64(pos)
}

func brierScore(rows []scoredRow) float64 {
	var sum float64
	for _, r := range rows {
		y := float64(r.label)
		d := r.score - y
		sum += d * d
	}
	return sum / float64(len(rows))
}

func baselineBetter(a Result, idA string, b Result, idB string) bool {
	if *a.AP != *b.AP {
		return *a.AP > *b.AP
	}
	if *a.Brier != *b.Brier {
		return *a.Brier < *b.Brier
	}
	if a.Coverage != b.Coverage {
		return a.Coverage > b.Coverage
	}
	return idA < idB
}

func undefined(r Result, reason string) Result {
	r.Defined = false
	r.Reason = reason
	return r
}
