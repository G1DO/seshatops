package forecast

const (
	// ProtocolID is the frozen evaluation contract identifier.
	ProtocolID = "m4-stockout-eval-v1"

	// HistorySeed is the official Northstar M4 daily on-hand fixture seed.
	HistorySeed = "northstar-m4-stockout-v1"

	// HorizonDays is the frozen prediction horizon in UTC calendar days.
	HorizonDays = 7

	// CoverageFloorPercent is the qualifying coverage gate: |P|*100 >= |S|*this.
	CoverageFloorPercent = 80

	SplitTrain      = "train"
	SplitValidation = "validation"
	SplitTest       = "test"

	BaselineSeasonalNaive = "seasonal_naive"
	BaselineMovingAverage = "moving_average"

	TenantNS001 = "11111111-1111-4111-8111-111111111111"
	TenantNS002 = "22222222-2222-4222-8222-222222222222"

	ItemFlour = "item-flour-001"
	ItemYeast = "item-yeast-001"
	ItemSalt  = "item-salt-001"

	HistoryStartDate   = "2026-01-05"
	HistoryDayCount    = 112
	LabeledDayCount    = 105
	TrainDayCount      = 70
	ValidationDayCount = 14
	TestDayCount       = 21
)
