package schema

// Kind enumerates the resource kinds understood by this binary.
// New kinds are appended per §5 and require a binary release; see spec
// §5 Kind taxonomy. M1 seed ships only LLM kinds because the only live
// shard is openrouter.
type Kind string

// Supported Kind values.
const (
	KindLLMText       Kind = "llm.text"
	KindLLMMultimodal Kind = "llm.multimodal"
	KindLLMEmbedding  Kind = "llm.embedding"
	KindLLMImage      Kind = "llm.image"
	KindLLMAudio      Kind = "llm.audio"
)

// IsLLM reports whether k is one of the llm.* kinds.
func (k Kind) IsLLM() bool {
	switch k {
	case KindLLMText, KindLLMMultimodal, KindLLMEmbedding, KindLLMImage, KindLLMAudio:
		return true
	}
	return false
}
