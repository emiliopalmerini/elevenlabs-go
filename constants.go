package elevenlabs

const (
	// DefaultBaseURL is the default ElevenLabs REST API origin.
	DefaultBaseURL = "https://api.elevenlabs.io"

	ModelScribeV1 = "scribe_v1"
	ModelScribeV2 = "scribe_v2"

	TimestampsNone      = "none"
	TimestampsWord      = "word"
	TimestampsCharacter = "character"

	FileFormatOther      = "other"
	FileFormatPCMS16LE16 = "pcm_s16le_16"

	MultichannelSeparate = "separate"
	MultichannelCombined = "combined"

	EntityAll               = "all"
	EntityCategoryPII       = "pii"
	EntityCategoryPHI       = "phi"
	EntityCategoryPCI       = "pci"
	EntityCategoryOther     = "other"
	EntityOffensiveLanguage = "offensive_language"

	EntityRedactionRedacted             = "redacted"
	EntityRedactionEntityType           = "entity_type"
	EntityRedactionEnumeratedEntityType = "enumerated_entity_type"
)
