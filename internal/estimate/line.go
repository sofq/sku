package estimate

// LineItem is the estimator output for one parsed Item.
type LineItem struct {
	Item         Item
	Kind         string
	SKUID        string
	Provider     string
	Service      string
	Resource     string
	Region       string
	HourlyUSD    float64
	Quantity     float64
	QuantityUnit string
	MonthlyUSD   float64
	Notes        []string
}

// Result is the aggregate of all line items in one estimate run.
type Result struct {
	Items           []LineItem
	MonthlyTotalUSD float64
	Currency        string
}
