package elevenlabs

type TranscriptResponse struct {
	LanguageCode        string            `json:"language_code,omitempty"`
	LanguageProbability float64           `json:"language_probability,omitempty"`
	Text                string            `json:"text,omitempty"`
	Words               []Word            `json:"words,omitempty"`
	ChannelIndex        *int              `json:"channel_index,omitempty"`
	TranscriptionID     string            `json:"transcription_id,omitempty"`
	AudioDurationSecs   *float64          `json:"audio_duration_secs,omitempty"`
	Entities            []DetectedEntity  `json:"entities,omitempty"`
	Transcripts         []TranscriptChunk `json:"transcripts,omitempty"`
}

type WebhookResponse struct {
	Message         string         `json:"message"`
	RequestID       string         `json:"request_id"`
	TranscriptionID *string        `json:"transcription_id,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type TranscriptChunk struct {
	LanguageCode        string           `json:"language_code,omitempty"`
	LanguageProbability float64          `json:"language_probability,omitempty"`
	Text                string           `json:"text,omitempty"`
	Words               []Word           `json:"words,omitempty"`
	ChannelIndex        *int             `json:"channel_index,omitempty"`
	TranscriptionID     string           `json:"transcription_id,omitempty"`
	AudioDurationSecs   *float64         `json:"audio_duration_secs,omitempty"`
	Entities            []DetectedEntity `json:"entities,omitempty"`
}

type Word struct {
	Text         string      `json:"text"`
	Start        *float64    `json:"start,omitempty"`
	End          *float64    `json:"end,omitempty"`
	Type         string      `json:"type,omitempty"`
	SpeakerID    string      `json:"speaker_id,omitempty"`
	Logprob      float64     `json:"logprob,omitempty"`
	Characters   []Character `json:"characters,omitempty"`
	ChannelIndex *int        `json:"channel_index,omitempty"`
}

type Character struct {
	Text  string   `json:"text"`
	Start *float64 `json:"start,omitempty"`
	End   *float64 `json:"end,omitempty"`
}

type DetectedEntity struct {
	Text       string `json:"text"`
	EntityType string `json:"entity_type"`
	StartChar  int    `json:"start_char"`
	EndChar    int    `json:"end_char"`
}

// Chunks returns channel transcripts when present, otherwise it adapts the
// top-level transcript into a single chunk.
func (r TranscriptResponse) Chunks() []TranscriptChunk {
	if len(r.Transcripts) > 0 {
		return r.Transcripts
	}
	return []TranscriptChunk{{
		LanguageCode:        r.LanguageCode,
		LanguageProbability: r.LanguageProbability,
		Text:                r.Text,
		Words:               r.Words,
		ChannelIndex:        r.ChannelIndex,
		TranscriptionID:     r.TranscriptionID,
		AudioDurationSecs:   r.AudioDurationSecs,
		Entities:            r.Entities,
	}}
}
