package forecast

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

// Example is one labeled chronological evaluation row.
type Example struct {
	RowID            string
	TenantID         string
	ItemID           string
	AsOfDate         string
	Label            int
	Split            string
	SourceCutoffDate string
	HistoryHash      string
	ProtocolID       string
}

// Dataset is the labeled train/validation/test rows for one tenant.
type Dataset struct {
	ProtocolID string
	TenantID   string
	Examples   []Example
}

type seriesKey struct {
	tenant string
	item   string
}

// BuildDataset constructs labeled chronological examples for tenantID.
func BuildDataset(history History, tenantID string) (Dataset, error) {
	tenantID = strings.ToLower(strings.TrimSpace(tenantID))
	if tenantID == "" {
		return Dataset{}, wrapInvalid("empty tenant")
	}

	series, err := indexObservations(history.Observations, tenantID)
	if err != nil {
		return Dataset{}, err
	}
	if len(series) == 0 {
		return Dataset{}, wrapInvalid("no observations for tenant %s", tenantID)
	}

	labelableDates, err := uniqueLabelableDates(series)
	if err != nil {
		return Dataset{}, err
	}
	splits := assignSplits(labelableDates)

	var examples []Example
	for key, days := range series {
		dates := sortedDates(days)
		for _, asOf := range dates {
			end, ok := addDays(asOf, HorizonDays)
			if !ok {
				return Dataset{}, wrapInvalid("as_of_date %s", asOf)
			}
			if _, exists := days[end]; !exists {
				continue
			}
			label, err := labelAt(days, asOf)
			if err != nil {
				return Dataset{}, err
			}
			split, ok := splits[asOf]
			if !ok {
				continue
			}
			examples = append(examples, Example{
				RowID:            RowID(key.tenant, key.item, asOf),
				TenantID:         key.tenant,
				ItemID:           key.item,
				AsOfDate:         asOf,
				Label:            label,
				Split:            split,
				SourceCutoffDate: asOf,
				HistoryHash:      historyHash(days, asOf),
				ProtocolID:       ProtocolID,
			})
		}
	}
	sort.Slice(examples, func(i, j int) bool {
		return exampleLess(examples[i], examples[j])
	})
	return Dataset{
		ProtocolID: ProtocolID,
		TenantID:   tenantID,
		Examples:   examples,
	}, nil
}

// RowID is the stable identity of one observation unit under this protocol.
func RowID(tenantID, itemID, asOfDate string) string {
	sum := sha256.Sum256([]byte(ProtocolID + "\t" + tenantID + "\t" + itemID + "\t" + asOfDate + "\n"))
	return hex.EncodeToString(sum[:])
}

func indexObservations(obs []Observation, tenantID string) (map[seriesKey]map[string]int64, error) {
	series := make(map[seriesKey]map[string]int64)
	for _, o := range obs {
		tenant := strings.ToLower(strings.TrimSpace(o.TenantID))
		item := strings.ToLower(strings.TrimSpace(o.ItemID))
		if tenant != tenantID {
			continue
		}
		if item == "" {
			return nil, wrapInvalid("empty item_id")
		}
		if o.QuantityOnHand < 0 {
			return nil, wrapInvalid("negative quantity for %s %s", item, o.AsOfDate)
		}
		day, err := parseDate(o.AsOfDate)
		if err != nil {
			return nil, err
		}
		date := day.Format(dateLayout)
		if date != o.AsOfDate {
			return nil, wrapInvalid("as_of_date %q", o.AsOfDate)
		}
		key := seriesKey{tenant: tenant, item: item}
		days := series[key]
		if days == nil {
			days = make(map[string]int64)
			series[key] = days
		}
		if _, dup := days[date]; dup {
			return nil, wrapInvalid("duplicate observation %s %s %s", tenant, item, date)
		}
		days[date] = o.QuantityOnHand
	}
	for key, days := range series {
		if err := requireDense(key, days); err != nil {
			return nil, err
		}
	}
	return series, nil
}

func requireDense(key seriesKey, days map[string]int64) error {
	dates := sortedDates(days)
	if len(dates) == 0 {
		return wrapInvalid("empty series %s %s", key.tenant, key.item)
	}
	start, err := parseDate(dates[0])
	if err != nil {
		return err
	}
	end, err := parseDate(dates[len(dates)-1])
	if err != nil {
		return err
	}
	want := int(end.Sub(start).Hours()/24) + 1
	if want != len(dates) {
		return wrapInvalid("calendar gap in %s %s", key.tenant, key.item)
	}
	for i, d := range dates {
		got := start.AddDate(0, 0, i).Format(dateLayout)
		if d != got {
			return wrapInvalid("calendar gap in %s %s", key.tenant, key.item)
		}
	}
	return nil
}

func uniqueLabelableDates(series map[seriesKey]map[string]int64) ([]string, error) {
	seen := make(map[string]struct{})
	for _, days := range series {
		for asOf := range days {
			end, ok := addDays(asOf, HorizonDays)
			if !ok {
				return nil, wrapInvalid("as_of_date %s", asOf)
			}
			if _, exists := days[end]; !exists {
				continue
			}
			seen[asOf] = struct{}{}
		}
	}
	dates := make([]string, 0, len(seen))
	for d := range seen {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	return dates, nil
}

func assignSplits(labeledDates []string) map[string]string {
	out := make(map[string]string, len(labeledDates))
	for i, d := range labeledDates {
		idx := i + 1
		switch {
		case idx <= TrainDayCount:
			out[d] = SplitTrain
		case idx <= TrainDayCount+ValidationDayCount:
			out[d] = SplitValidation
		case idx <= TrainDayCount+ValidationDayCount+TestDayCount:
			out[d] = SplitTest
		}
	}
	return out
}

func labelAt(days map[string]int64, asOf string) (int, error) {
	for i := 1; i <= HorizonDays; i++ {
		day, ok := addDays(asOf, i)
		if !ok {
			return 0, wrapInvalid("as_of_date %s", asOf)
		}
		qty, exists := days[day]
		if !exists {
			return 0, wrapInvalid("missing label-window observation %s", day)
		}
		if qty == 0 {
			return 1, nil
		}
	}
	return 0, nil
}

func historyHash(days map[string]int64, cutoff string) string {
	dates := make([]string, 0, len(days))
	for d := range days {
		if d <= cutoff {
			dates = append(dates, d)
		}
	}
	sort.Strings(dates)
	var b strings.Builder
	for _, d := range dates {
		b.WriteString(d)
		b.WriteByte('\t')
		b.WriteString(formatInt(days[d]))
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func sortedDates(days map[string]int64) []string {
	dates := make([]string, 0, len(days))
	for d := range days {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	return dates
}

func exampleLess(a, b Example) bool {
	if a.TenantID != b.TenantID {
		return a.TenantID < b.TenantID
	}
	if a.ItemID != b.ItemID {
		return a.ItemID < b.ItemID
	}
	return a.AsOfDate < b.AsOfDate
}

func parseDate(s string) (time.Time, error) {
	if s != strings.TrimSpace(s) {
		return time.Time{}, wrapInvalid("as_of_date %q", s)
	}
	t, err := time.ParseInLocation(dateLayout, s, time.UTC)
	if err != nil {
		return time.Time{}, wrapInvalid("as_of_date %q", s)
	}
	if t.Format(dateLayout) != s {
		return time.Time{}, wrapInvalid("as_of_date %q", s)
	}
	return t, nil
}

func addDays(asOf string, days int) (string, bool) {
	t, err := parseDate(asOf)
	if err != nil {
		return "", false
	}
	return t.AddDate(0, 0, days).Format(dateLayout), true
}
