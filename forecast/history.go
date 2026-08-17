package forecast

import (
	"time"
)

// Observation is one end-of-day on-hand quantity for a tenant item.
type Observation struct {
	TenantID       string
	ItemID         string
	AsOfDate       string
	QuantityOnHand int64
}

// History is a replayable daily on-hand log used to build evaluation examples.
type History struct {
	Seed         string
	Observations []Observation
}

type itemSpec struct {
	itemID         string
	initial        int64
	weekdayDemand  int64
	weekendDemand  int64
	restockWeekday time.Weekday
	restockAmount  int64
}

// GenerateHistory returns the frozen Northstar M4 daily on-hand fixture.
func GenerateHistory(seed string) (History, error) {
	if seed != HistorySeed {
		return History{}, unsupportedSeed(seed)
	}
	start, err := parseDate(HistoryStartDate)
	if err != nil {
		return History{}, err
	}

	var obs []Observation
	obs = append(obs, generateTenantSeries(TenantNS001, start, []itemSpec{
		{
			itemID:         ItemFlour,
			initial:        80,
			weekdayDemand:  7,
			weekendDemand:  2,
			restockWeekday: time.Monday,
			restockAmount:  30,
		},
		{
			itemID:         ItemYeast,
			initial:        6,
			weekdayDemand:  3,
			weekendDemand:  1,
			restockWeekday: time.Friday,
			restockAmount:  10,
		},
		{
			itemID:         ItemSalt,
			initial:        100,
			weekdayDemand:  1,
			weekendDemand:  1,
			restockWeekday: time.Monday,
			restockAmount:  10,
		},
	})...)
	obs = append(obs, generateTenantSeries(TenantNS002, start, []itemSpec{
		{
			itemID:         ItemFlour,
			initial:        400,
			weekdayDemand:  1,
			weekendDemand:  1,
			restockWeekday: time.Monday,
			restockAmount:  20,
		},
	})...)

	return History{Seed: seed, Observations: obs}, nil
}

func generateTenantSeries(tenantID string, start time.Time, specs []itemSpec) []Observation {
	out := make([]Observation, 0, len(specs)*HistoryDayCount)
	for _, spec := range specs {
		onHand := spec.initial
		for i := 0; i < HistoryDayCount; i++ {
			day := start.AddDate(0, 0, i)
			if day.Weekday() == spec.restockWeekday {
				onHand += spec.restockAmount
			}
			demand := spec.weekdayDemand
			if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
				demand = spec.weekendDemand
			}
			onHand -= demand
			if onHand < 0 {
				onHand = 0
			}
			out = append(out, Observation{
				TenantID:       tenantID,
				ItemID:         spec.itemID,
				AsOfDate:       day.Format(dateLayout),
				QuantityOnHand: onHand,
			})
		}
	}
	return out
}
