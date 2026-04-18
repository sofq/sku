package output

// Options configures the output Pipeline. Zero values are filled by
// WithDefaults: empty Preset becomes PresetAgent, empty Format becomes "json".
type Options struct {
	Preset            Preset
	Format            string
	Pretty            bool
	Fields            string
	JQ                string
	IncludeRaw        bool
	IncludeAggregated bool
	NoColor           bool
}

// WithDefaults returns a copy of o with zero fields filled in.
func (o Options) WithDefaults() Options {
	if o.Preset == "" {
		o.Preset = PresetAgent
	}
	if o.Format == "" {
		o.Format = "json"
	}
	return o
}
