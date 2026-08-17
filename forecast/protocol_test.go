package forecast_test

import (
	"errors"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/G1DO/seshatops/forecast"
)

func officialHistory(t *testing.T) forecast.History {
	t.Helper()
	h, err := forecast.GenerateHistory(forecast.HistorySeed)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func officialNS001(t *testing.T) forecast.Dataset {
	t.Helper()
	ds, err := forecast.BuildDataset(officialHistory(t), forecast.TenantNS001)
	if err != nil {
		t.Fatal(err)
	}
	return ds
}

func dense(tenant, item, start string, qtys []int64) []forecast.Observation {
	day, err := time.ParseInLocation("2006-01-02", start, time.UTC)
	if err != nil {
		panic(err)
	}
	out := make([]forecast.Observation, len(qtys))
	for i, q := range qtys {
		out[i] = forecast.Observation{
			TenantID:       tenant,
			ItemID:         item,
			AsOfDate:       day.AddDate(0, 0, i).Format("2006-01-02"),
			QuantityOnHand: q,
		}
	}
	return out
}

func exampleBy(ds forecast.Dataset, item, asOf string) (forecast.Example, bool) {
	for _, e := range ds.Examples {
		if e.ItemID == item && e.AsOfDate == asOf {
			return e, true
		}
	}
	return forecast.Example{}, false
}

func TestGenerateHistoryUnsupportedSeed(t *testing.T) {
	for _, seed := range []string{"", "other-seed", "northstar-m1-order-line-v1"} {
		if _, err := forecast.GenerateHistory(seed); !errors.Is(err, forecast.ErrUnsupportedSeed) {
			t.Fatalf("seed %q err=%v", seed, err)
		}
	}
}

func TestGenerateHistoryDeterministic(t *testing.T) {
	a := officialHistory(t)
	b := officialHistory(t)
	if a.Seed != forecast.HistorySeed || b.Seed != a.Seed {
		t.Fatalf("seed a=%s b=%s", a.Seed, b.Seed)
	}
	if len(a.Observations) != 4*forecast.HistoryDayCount {
		t.Fatalf("observations = %d", len(a.Observations))
	}
	if len(a.Observations) != len(b.Observations) {
		t.Fatal("observation counts differ")
	}
	for i := range a.Observations {
		if a.Observations[i] != b.Observations[i] {
			t.Fatalf("observation %d differs: %+v vs %+v", i, a.Observations[i], b.Observations[i])
		}
	}
}

func TestBuildDatasetEmptyTenant(t *testing.T) {
	h := officialHistory(t)
	for _, tenant := range []string{"", "  "} {
		if _, err := forecast.BuildDataset(h, tenant); !errors.Is(err, forecast.ErrInvalidInput) {
			t.Fatalf("tenant %q err=%v", tenant, err)
		}
	}
}

func TestBuildDatasetTenantIsolation(t *testing.T) {
	ds := officialNS001(t)
	if ds.TenantID != forecast.TenantNS001 || ds.ProtocolID != forecast.ProtocolID {
		t.Fatalf("dataset ids: %+v", ds)
	}
	for _, e := range ds.Examples {
		if e.TenantID != forecast.TenantNS001 {
			t.Fatalf("leaked tenant %s", e.TenantID)
		}
		if e.ProtocolID != forecast.ProtocolID {
			t.Fatalf("protocol %s", e.ProtocolID)
		}
		if e.SourceCutoffDate != e.AsOfDate {
			t.Fatalf("cutoff %s as_of %s", e.SourceCutoffDate, e.AsOfDate)
		}
		if e.RowID != forecast.RowID(e.TenantID, e.ItemID, e.AsOfDate) {
			t.Fatalf("row_id mismatch %s", e.AsOfDate)
		}
	}

	ns2, err := forecast.BuildDataset(officialHistory(t), forecast.TenantNS002)
	if err != nil {
		t.Fatal(err)
	}
	if len(ns2.Examples) == 0 {
		t.Fatal("expected NS-002 examples")
	}
	for _, e := range ns2.Examples {
		if e.TenantID != forecast.TenantNS002 {
			t.Fatalf("ns2 tenant %s", e.TenantID)
		}
		if e.ItemID != forecast.ItemFlour {
			t.Fatalf("ns2 item %s", e.ItemID)
		}
	}
	if forecast.Checksum(ds) == forecast.Checksum(ns2) {
		t.Fatal("tenants must not share dataset checksum")
	}
}

func TestOfficialDatasetShapeAndSplits(t *testing.T) {
	ds := officialNS001(t)
	if len(ds.Examples) != 3*forecast.LabeledDayCount {
		t.Fatalf("examples = %d", len(ds.Examples))
	}

	dates := map[string]map[string]struct{}{
		forecast.SplitTrain:      {},
		forecast.SplitValidation: {},
		forecast.SplitTest:       {},
	}
	class := map[string][2]int{}
	items := map[string]struct{}{}
	for _, e := range ds.Examples {
		items[e.ItemID] = struct{}{}
		dates[e.Split][e.AsOfDate] = struct{}{}
		c := class[e.Split]
		if e.Label == 1 {
			c[1]++
		} else {
			c[0]++
		}
		class[e.Split] = c
	}
	if _, ok := items[forecast.ItemFlour]; !ok {
		t.Fatal("missing flour")
	}
	if _, ok := items[forecast.ItemYeast]; !ok {
		t.Fatal("missing yeast")
	}
	if _, ok := items[forecast.ItemSalt]; !ok {
		t.Fatal("missing salt")
	}
	if len(dates[forecast.SplitTrain]) != forecast.TrainDayCount {
		t.Fatalf("train dates = %d", len(dates[forecast.SplitTrain]))
	}
	if len(dates[forecast.SplitValidation]) != forecast.ValidationDayCount {
		t.Fatalf("validation dates = %d", len(dates[forecast.SplitValidation]))
	}
	if len(dates[forecast.SplitTest]) != forecast.TestDayCount {
		t.Fatalf("test dates = %d", len(dates[forecast.SplitTest]))
	}

	maxTrain := maxDate(dates[forecast.SplitTrain])
	minVal := minDate(dates[forecast.SplitValidation])
	maxVal := maxDate(dates[forecast.SplitValidation])
	minTest := minDate(dates[forecast.SplitTest])
	maxTest := maxDate(dates[forecast.SplitTest])
	if !(maxTrain < minVal && minVal <= maxVal && maxVal < minTest) {
		t.Fatalf("split order train=%s val=%s..%s test=%s", maxTrain, minVal, maxVal, minTest)
	}
	if minDate(dates[forecast.SplitTrain]) != forecast.HistoryStartDate {
		t.Fatalf("train start %s", minDate(dates[forecast.SplitTrain]))
	}
	if maxTrain != "2026-03-15" || minVal != "2026-03-16" || maxVal != "2026-03-29" {
		t.Fatalf("train/val bounds %s %s %s", maxTrain, minVal, maxVal)
	}
	if minTest != "2026-03-30" || maxTest != "2026-04-19" {
		t.Fatalf("test bounds %s %s", minTest, maxTest)
	}

	for _, split := range []string{forecast.SplitTrain, forecast.SplitValidation, forecast.SplitTest} {
		c := class[split]
		if c[0] == 0 || c[1] == 0 {
			t.Fatalf("split %s class counts neg=%d pos=%d", split, c[0], c[1])
		}
	}

	unlabeled := []string{
		"2026-04-20", "2026-04-21", "2026-04-22", "2026-04-23",
		"2026-04-24", "2026-04-25", "2026-04-26",
	}
	for _, d := range unlabeled {
		for _, item := range []string{forecast.ItemFlour, forecast.ItemYeast, forecast.ItemSalt} {
			if _, ok := exampleBy(ds, item, d); ok {
				t.Fatalf("unlabeled date %s present for %s", d, item)
			}
		}
	}
}

func TestOfficialChecksumMatchesGoldenAndRebuild(t *testing.T) {
	ds1 := officialNS001(t)
	ds2 := officialNS001(t)
	got := forecast.Checksum(ds1)
	if got != forecast.Checksum(ds2) {
		t.Fatal("rebuild checksum differs")
	}
	want, err := os.ReadFile(filepath.Join("testdata", "dataset.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	if got != strings.TrimSpace(string(want)) {
		t.Fatalf("checksum = %s, want %s", got, strings.TrimSpace(string(want)))
	}
}

func TestChecksumEmpty(t *testing.T) {
	got := forecast.Checksum(forecast.Dataset{})
	if got != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("empty checksum = %s", got)
	}
}

func TestChecksumLowercasesIdentifiers(t *testing.T) {
	ds := officialNS001(t)
	want := forecast.Checksum(ds)
	cased := ds
	cased.Examples = append([]forecast.Example(nil), ds.Examples...)
	cased.Examples[0].TenantID = strings.ToUpper(cased.Examples[0].TenantID)
	cased.Examples[0].ItemID = strings.ToUpper(cased.Examples[0].ItemID)
	cased.Examples[0].Split = strings.ToUpper(cased.Examples[0].Split)
	if forecast.Checksum(cased) != want {
		t.Fatal("checksum changed with identifier case")
	}
}

func TestShuffleDoesNotChangeChecksum(t *testing.T) {
	h := officialHistory(t)
	ds, err := forecast.BuildDataset(h, forecast.TenantNS001)
	if err != nil {
		t.Fatal(err)
	}
	want := forecast.Checksum(ds)

	shuffled := append([]forecast.Observation(nil), h.Observations...)
	rng := rand.New(rand.NewSource(85))
	rng.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	got, err := forecast.BuildDataset(forecast.History{Observations: shuffled}, forecast.TenantNS001)
	if err != nil {
		t.Fatal(err)
	}
	if forecast.Checksum(got) != want {
		t.Fatal("shuffled observations changed checksum")
	}
}

func TestBuildDatasetRejectsBadObservations(t *testing.T) {
	base := dense(forecast.TenantNS001, forecast.ItemFlour, "2026-01-05", []int64{5, 5, 5, 5, 5, 5, 5, 5})
	t.Run("negative", func(t *testing.T) {
		obs := append([]forecast.Observation(nil), base...)
		obs[1].QuantityOnHand = -1
		if _, err := forecast.BuildDataset(forecast.History{Observations: obs}, forecast.TenantNS001); !errors.Is(err, forecast.ErrInvalidInput) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		obs := append([]forecast.Observation(nil), base...)
		obs = append(obs, obs[0])
		if _, err := forecast.BuildDataset(forecast.History{Observations: obs}, forecast.TenantNS001); !errors.Is(err, forecast.ErrInvalidInput) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("gap", func(t *testing.T) {
		obs := append([]forecast.Observation(nil), base[:3]...)
		obs = append(obs, base[4:]...)
		if _, err := forecast.BuildDataset(forecast.History{Observations: obs}, forecast.TenantNS001); !errors.Is(err, forecast.ErrInvalidInput) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("bad_date", func(t *testing.T) {
		obs := append([]forecast.Observation(nil), base...)
		obs[0].AsOfDate = "2026-01-05T00:00:00Z"
		if _, err := forecast.BuildDataset(forecast.History{Observations: obs}, forecast.TenantNS001); !errors.Is(err, forecast.ErrInvalidInput) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("unknown_tenant", func(t *testing.T) {
		if _, err := forecast.BuildDataset(forecast.History{Observations: base}, forecast.TenantNS002); !errors.Is(err, forecast.ErrInvalidInput) {
			t.Fatalf("err=%v", err)
		}
	})
}

func minDate(dates map[string]struct{}) string {
	min := ""
	for d := range dates {
		if min == "" || d < min {
			min = d
		}
	}
	return min
}

func maxDate(dates map[string]struct{}) string {
	max := ""
	for d := range dates {
		if d > max {
			max = d
		}
	}
	return max
}

func TestLabelWindowExcludesAsOfAndIncludesHorizonZero(t *testing.T) {
	start := "2026-01-05"
	t.Run("zero_on_as_of_only", func(t *testing.T) {
		qtys := []int64{0, 10, 10, 10, 10, 10, 10, 10}
		ds, err := forecast.BuildDataset(forecast.History{
			Observations: dense(forecast.TenantNS001, forecast.ItemFlour, start, qtys),
		}, forecast.TenantNS001)
		if err != nil {
			t.Fatal(err)
		}
		ex, ok := exampleBy(ds, forecast.ItemFlour, start)
		if !ok {
			t.Fatal("missing example")
		}
		if ex.Label != 0 {
			t.Fatalf("label = %d, want 0", ex.Label)
		}
	})
	t.Run("zero_through_horizon", func(t *testing.T) {
		qtys := []int64{0, 0, 0, 0, 0, 0, 0, 0}
		ds, err := forecast.BuildDataset(forecast.History{
			Observations: dense(forecast.TenantNS001, forecast.ItemFlour, start, qtys),
		}, forecast.TenantNS001)
		if err != nil {
			t.Fatal(err)
		}
		ex, ok := exampleBy(ds, forecast.ItemFlour, start)
		if !ok {
			t.Fatal("missing example")
		}
		if ex.Label != 1 {
			t.Fatalf("label = %d, want 1", ex.Label)
		}
	})
	t.Run("zero_on_horizon_day", func(t *testing.T) {
		qtys := []int64{10, 10, 10, 10, 10, 10, 10, 0}
		ds, err := forecast.BuildDataset(forecast.History{
			Observations: dense(forecast.TenantNS001, forecast.ItemFlour, start, qtys),
		}, forecast.TenantNS001)
		if err != nil {
			t.Fatal(err)
		}
		ex, ok := exampleBy(ds, forecast.ItemFlour, start)
		if !ok {
			t.Fatal("missing example")
		}
		if ex.Label != 1 {
			t.Fatalf("label = %d, want 1 when as_of+7 is zero", ex.Label)
		}
	})
}

func TestFutureMutationLeakageGuards(t *testing.T) {
	start := "2026-01-05"
	baseQtys := []int64{5, 5, 5, 5, 5, 5, 5, 5, 5, 5}
	base, err := forecast.BuildDataset(forecast.History{
		Observations: dense(forecast.TenantNS001, forecast.ItemFlour, start, baseQtys),
	}, forecast.TenantNS001)
	if err != nil {
		t.Fatal(err)
	}
	baseRow, ok := exampleBy(base, forecast.ItemFlour, start)
	if !ok {
		t.Fatal("missing base row")
	}
	if baseRow.Label != 0 {
		t.Fatalf("base label = %d", baseRow.Label)
	}

	within := append([]int64(nil), baseQtys...)
	within[3] = 0
	withinDS, err := forecast.BuildDataset(forecast.History{
		Observations: dense(forecast.TenantNS001, forecast.ItemFlour, start, within),
	}, forecast.TenantNS001)
	if err != nil {
		t.Fatal(err)
	}
	withinRow, ok := exampleBy(withinDS, forecast.ItemFlour, start)
	if !ok {
		t.Fatal("missing within row")
	}
	if withinRow.HistoryHash != baseRow.HistoryHash {
		t.Fatal("history_hash changed for mutation after as_of")
	}
	if withinRow.RowID != baseRow.RowID {
		t.Fatal("row_id changed")
	}
	if withinRow.Label != 1 {
		t.Fatalf("label = %d, want 1 after in-window zero", withinRow.Label)
	}

	after := append([]int64(nil), baseQtys...)
	after[8] = 0
	afterDS, err := forecast.BuildDataset(forecast.History{
		Observations: dense(forecast.TenantNS001, forecast.ItemFlour, start, after),
	}, forecast.TenantNS001)
	if err != nil {
		t.Fatal(err)
	}
	afterRow, ok := exampleBy(afterDS, forecast.ItemFlour, start)
	if !ok {
		t.Fatal("missing after row")
	}
	if afterRow.HistoryHash != baseRow.HistoryHash || afterRow.Label != baseRow.Label {
		t.Fatalf("mutation after horizon changed hash=%s label=%d", afterRow.HistoryHash, afterRow.Label)
	}

	atCutoff := append([]int64(nil), baseQtys...)
	atCutoff[0] = 0
	atDS, err := forecast.BuildDataset(forecast.History{
		Observations: dense(forecast.TenantNS001, forecast.ItemFlour, start, atCutoff),
	}, forecast.TenantNS001)
	if err != nil {
		t.Fatal(err)
	}
	atRow, ok := exampleBy(atDS, forecast.ItemFlour, start)
	if !ok {
		t.Fatal("missing cutoff row")
	}
	if atRow.HistoryHash == baseRow.HistoryHash {
		t.Fatal("history_hash must change when cutoff-day quantity changes")
	}
	if atRow.Label != baseRow.Label {
		t.Fatalf("label changed when only as_of quantity changed: %d", atRow.Label)
	}
}

func mixedEvalDataset(t *testing.T) forecast.Dataset {
	t.Helper()
	start := "2026-01-05"
	obs := dense(forecast.TenantNS001, forecast.ItemFlour, start, []int64{8, 8, 8, 8, 8, 8, 8, 8, 8})
	obs = append(obs, dense(forecast.TenantNS001, forecast.ItemYeast, start, []int64{8, 8, 0, 8, 8, 8, 8, 8, 8})...)
	ds, err := forecast.BuildDataset(forecast.History{Observations: obs}, forecast.TenantNS001)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.Examples) != 4 {
		t.Fatalf("examples = %d", len(ds.Examples))
	}
	return ds
}

func predsFor(ds forecast.Dataset, split string, scoreFn func(forecast.Example) *float64) []forecast.Prediction {
	var out []forecast.Prediction
	for _, e := range ds.Examples {
		if e.Split != split {
			continue
		}
		out = append(out, forecast.Prediction{RowID: e.RowID, Score: scoreFn(e)})
	}
	return out
}

func ptr(v float64) *float64 { return &v }

func TestEvaluateFailClosed(t *testing.T) {
	ds := mixedEvalDataset(t)
	split := forecast.SplitTrain
	valid := predsFor(ds, split, func(e forecast.Example) *float64 {
		if e.Label == 1 {
			return ptr(0.9)
		}
		return ptr(0.1)
	})

	t.Run("omitted", func(t *testing.T) {
		if _, err := forecast.Evaluate(ds, split, valid[:len(valid)-1]); !errors.Is(err, forecast.ErrInvalidInput) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		bad := append([]forecast.Prediction(nil), valid...)
		bad = append(bad, valid[0])
		if _, err := forecast.Evaluate(ds, split, bad); !errors.Is(err, forecast.ErrInvalidInput) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("score_high", func(t *testing.T) {
		bad := append([]forecast.Prediction(nil), valid...)
		bad[0].Score = ptr(1.01)
		if _, err := forecast.Evaluate(ds, split, bad); !errors.Is(err, forecast.ErrInvalidInput) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("score_nan", func(t *testing.T) {
		bad := append([]forecast.Prediction(nil), valid...)
		bad[0].Score = ptr(math.NaN())
		if _, err := forecast.Evaluate(ds, split, bad); !errors.Is(err, forecast.ErrInvalidInput) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("unknown_split", func(t *testing.T) {
		if _, err := forecast.Evaluate(ds, "holdout", valid); !errors.Is(err, forecast.ErrInvalidInput) {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestEvaluateAbstentionAndAP(t *testing.T) {
	ds := mixedEvalDataset(t)
	split := forecast.SplitTrain

	t.Run("all_abstain", func(t *testing.T) {
		preds := predsFor(ds, split, func(forecast.Example) *float64 { return nil })
		res, err := forecast.Evaluate(ds, split, preds)
		if err != nil {
			t.Fatal(err)
		}
		if res.Defined || res.Predicted != 0 || res.Abstained != res.N {
			t.Fatalf("result %+v", res)
		}
		if forecast.Qualifies(res) {
			t.Fatal("all abstain must not qualify")
		}
	})

	t.Run("empty_split", func(t *testing.T) {
		res, err := forecast.Evaluate(ds, forecast.SplitTest, nil)
		if err != nil {
			t.Fatal(err)
		}
		if res.Defined || res.Reason != "empty split" {
			t.Fatalf("result %+v", res)
		}
	})

	t.Run("empty_split_extra_pred", func(t *testing.T) {
		if _, err := forecast.Evaluate(ds, forecast.SplitTest, []forecast.Prediction{
			{RowID: "extra", Score: ptr(0.1)},
		}); !errors.Is(err, forecast.ErrInvalidInput) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("average_precision", func(t *testing.T) {
		preds := predsFor(ds, split, func(e forecast.Example) *float64 {
			switch {
			case e.ItemID == forecast.ItemYeast && e.AsOfDate == "2026-01-05":
				return ptr(0.9)
			case e.ItemID == forecast.ItemYeast && e.AsOfDate == "2026-01-06":
				return ptr(0.1)
			case e.ItemID == forecast.ItemFlour && e.AsOfDate == "2026-01-05":
				return ptr(0.5)
			default:
				return ptr(0.2)
			}
		})
		res, err := forecast.Evaluate(ds, split, preds)
		if err != nil {
			t.Fatal(err)
		}
		if !res.Defined || res.AP == nil || res.Brier == nil {
			t.Fatalf("result %+v", res)
		}
		// ranks: 0.9 yeast/01-05 pos, 0.5 flour/01-05 neg, 0.2 flour/01-06 neg, 0.1 yeast/01-06 pos
		// AP = (1/1 + 2/4) / 2 = 0.75
		if math.Abs(*res.AP-0.75) > 1e-12 {
			t.Fatalf("AP = %v, want 0.75", *res.AP)
		}
		if res.Predicted != 4 || res.Coverage != 1 {
			t.Fatalf("coverage %+v", res)
		}
	})

	t.Run("ap_tie_row_id", func(t *testing.T) {
		preds := predsFor(ds, split, func(forecast.Example) *float64 { return ptr(0.5) })
		res, err := forecast.Evaluate(ds, split, preds)
		if err != nil {
			t.Fatal(err)
		}
		var rows []forecast.Example
		for _, e := range ds.Examples {
			if e.Split == split {
				rows = append(rows, e)
			}
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].RowID < rows[j].RowID })
		var pos, seen int
		var sum float64
		for i, e := range rows {
			if e.Label != 1 {
				continue
			}
			pos++
			seen++
			sum += float64(seen) / float64(i+1)
		}
		want := sum / float64(pos)
		if !res.Defined || math.Abs(*res.AP-want) > 1e-12 {
			t.Fatalf("tie AP = %v, want %v", res.AP, want)
		}
	})

	t.Run("undefined_ap_no_predicted_positives", func(t *testing.T) {
		preds := predsFor(ds, split, func(e forecast.Example) *float64 {
			if e.Label == 1 {
				return nil
			}
			return ptr(0.1)
		})
		res, err := forecast.Evaluate(ds, split, preds)
		if err != nil {
			t.Fatal(err)
		}
		if res.Defined || res.Reason != "undefined average precision" || res.Brier == nil {
			t.Fatalf("result %+v", res)
		}
	})
}

func TestEvaluateDegenerateClass(t *testing.T) {
	qtys := []int64{8, 8, 8, 8, 8, 8, 8, 8, 8}
	ds, err := forecast.BuildDataset(forecast.History{
		Observations: dense(forecast.TenantNS001, forecast.ItemSalt, "2026-01-05", qtys),
	}, forecast.TenantNS001)
	if err != nil {
		t.Fatal(err)
	}
	valid := predsFor(ds, forecast.SplitTrain, func(forecast.Example) *float64 { return ptr(0.2) })
	res, err := forecast.Evaluate(ds, forecast.SplitTrain, valid)
	if err != nil {
		t.Fatal(err)
	}
	if res.Defined || res.Reason != "degenerate class" {
		t.Fatalf("result %+v", res)
	}
	if _, err := forecast.Evaluate(ds, forecast.SplitTrain, nil); !errors.Is(err, forecast.ErrInvalidInput) {
		t.Fatalf("omitted degenerate preds err=%v", err)
	}
	bad := append([]forecast.Prediction(nil), valid...)
	bad[0].Score = ptr(math.NaN())
	if _, err := forecast.Evaluate(ds, forecast.SplitTrain, bad); !errors.Is(err, forecast.ErrInvalidInput) {
		t.Fatalf("nan degenerate preds err=%v", err)
	}
}

func TestPromotionRules(t *testing.T) {
	base := qualified(0.70, 0.20, 10, 10)
	better := qualified(0.80, 0.20, 10, 10)
	equalAP := qualified(0.70, 0.10, 10, 10)
	worseBrier := qualified(0.80, 0.30, 10, 10)
	lowCoverage := qualified(0.90, 0.10, 10, 7)

	if ok, _ := forecast.CandidatePromoted(better, forecast.BaselineMovingAverage, base); !ok {
		t.Fatal("expected promotion")
	}
	if ok, reason := forecast.CandidatePromoted(equalAP, forecast.BaselineMovingAverage, base); ok {
		t.Fatalf("equal AP promoted: %s", reason)
	}
	if ok, _ := forecast.CandidatePromoted(worseBrier, forecast.BaselineMovingAverage, base); ok {
		t.Fatal("worse Brier promoted")
	}
	if ok, reason := forecast.CandidatePromoted(better, "", base); ok || reason != "no qualifying baseline" {
		t.Fatalf("missing baseline id: ok=%v reason=%s", ok, reason)
	}
	if ok, _ := forecast.CandidatePromoted(lowCoverage, forecast.BaselineMovingAverage, base); ok {
		t.Fatal("low coverage promoted")
	}

	id, ok := forecast.QualifyingBaseline(map[string]forecast.Result{
		forecast.BaselineSeasonalNaive: qualified(0.50, 0.2, 10, 10),
		forecast.BaselineMovingAverage: qualified(0.60, 0.2, 10, 10),
	})
	if !ok || id != forecast.BaselineMovingAverage {
		t.Fatalf("qualifying id=%s ok=%v", id, ok)
	}

	id, ok = forecast.QualifyingBaseline(map[string]forecast.Result{
		forecast.BaselineSeasonalNaive: qualified(0.50, 0.2, 10, 10),
		forecast.BaselineMovingAverage: qualified(0.50, 0.2, 10, 10),
	})
	if !ok || id != forecast.BaselineMovingAverage {
		t.Fatalf("lex tie id=%s ok=%v", id, ok)
	}

	if _, ok := forecast.QualifyingBaseline(map[string]forecast.Result{
		forecast.BaselineSeasonalNaive: lowCoverage,
	}); ok {
		t.Fatal("non-qualifying baseline selected")
	}
}

func qualified(ap, brier float64, n, predicted int) forecast.Result {
	apv, bv := ap, brier
	return forecast.Result{
		Defined:   true,
		AP:        &apv,
		Brier:     &bv,
		N:         n,
		Predicted: predicted,
		Coverage:  float64(predicted) / float64(n),
	}
}
