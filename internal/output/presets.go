package output

// Project trims a full Envelope to the preset's declared field set, folding
// in kind-specific extras where the preset depends on kind. Today only the
// compare preset uses kind — agent/price/full are kind-agnostic.
func Project(env Envelope, p Preset, kind string) Envelope {
	switch p {
	case PresetFull:
		return env
	case PresetPrice:
		return Envelope{Price: env.Price}
	case PresetCompare:
		return projectCompare(env, kind)
	case PresetAgent, "":
		return trimForAgent(env)
	default:
		return trimForAgent(env)
	}
}

// trimForAgent keeps the fields spec §4 "Presets" declares for the agent
// preset: provider, service, resource.name, location.provider_region, price,
// terms.commitment.
func trimForAgent(env Envelope) Envelope {
	out := Envelope{
		Provider: env.Provider,
		Service:  env.Service,
		Price:    env.Price,
	}
	if env.Resource != nil {
		out.Resource = &Resource{Name: env.Resource.Name}
	}
	if env.Location != nil {
		out.Location = &Location{
			ProviderRegion: env.Location.ProviderRegion,
		}
	}
	if env.Terms != nil {
		out.Terms = &Terms{Commitment: env.Terms.Commitment}
	}
	return out
}

// projectCompare applies spec §4 compare-preset rules: base fields plus
// kind-specific extras for LLM and compute.vm rows. Unknown kinds fall back
// to the base set so new kinds don't silently break existing compare output.
func projectCompare(env Envelope, kind string) Envelope {
	out := Envelope{
		Provider: env.Provider,
		Price:    env.Price,
	}
	if env.Resource != nil {
		out.Resource = &Resource{Name: env.Resource.Name}
	}
	if env.Location != nil {
		out.Location = &Location{NormalizedRegion: env.Location.NormalizedRegion}
	}
	switch kind {
	case "llm.text", "llm.multimodal", "llm.embedding":
		if env.Resource != nil && out.Resource != nil {
			out.Resource.ContextLength = env.Resource.ContextLength
			out.Resource.Capabilities = env.Resource.Capabilities
		}
		if env.Health != nil {
			out.Health = &Health{
				Uptime30d:    env.Health.Uptime30d,
				LatencyP95Ms: env.Health.LatencyP95Ms,
			}
		}
	case "compute.vm":
		if env.Resource != nil && out.Resource != nil {
			out.Resource.VCPU = env.Resource.VCPU
			out.Resource.MemoryGB = env.Resource.MemoryGB
			out.Resource.GPUCount = env.Resource.GPUCount
		}
	}
	return out
}
