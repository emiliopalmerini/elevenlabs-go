package elevenlabs

import "io"

// TranscriptFile is a file upload for multipart API requests.
type TranscriptFile struct {
	Name   string
	Reader io.Reader

	// SizeBytes is the total upload size reported in progress callbacks. When
	// it is not positive, the client attempts to infer the size from Reader.
	SizeBytes int64
}

// TranscriptUploadProgress reports uploaded file bytes for a transcript request.
type TranscriptUploadProgress struct {
	SentBytes int64
	// TotalBytes is -1 when the upload size cannot be determined.
	TotalBytes int64
	Done       bool
	// Attempt starts at 1 and increments when a replayable upload is retried.
	Attempt int
}

// CreateTranscriptRequest contains parameters for creating a speech-to-text
// transcript.
type CreateTranscriptRequest struct {
	ModelID string

	File            *TranscriptFile
	SourceURL       string
	CloudStorageURL string
	EnableLogging   *bool

	LanguageCode            string
	TimestampsGranularity   string
	AdditionalFormats       []TranscriptAdditionalFormatOptions
	Diarize                 *bool
	DiarizationThreshold    *float64
	NumSpeakers             int
	TagAudioEvents          *bool
	NoVerbatim              *bool
	Webhook                 *bool
	WebhookID               string
	WebhookMetadata         map[string]any
	FileFormat              string
	Temperature             *float64
	Seed                    *int
	UseMultiChannel         *bool
	MultichannelOutputStyle string
	EntityDetection         []string
	UseSpeakerLibrary       *bool
	DetectSpeakerRoles      *bool
	EntityRedaction         []string
	EntityRedactionMode     string
	Keyterms                []string
	ExtraFormFields         map[string][]string

	OnUploadProgress func(TranscriptUploadProgress)
}

// Transcript is a speech-to-text transcript response.
type Transcript struct {
	Text                string                       `json:"text,omitempty"`
	LanguageCode        string                       `json:"language_code,omitempty"`
	LanguageProbability float64                      `json:"language_probability,omitempty"`
	Words               []TranscriptWord             `json:"words,omitempty"`
	ChannelIndex        *int                         `json:"channel_index,omitempty"`
	AdditionalFormats   []TranscriptAdditionalFormat `json:"additional_formats,omitempty"`
	TranscriptionID     string                       `json:"transcription_id,omitempty"`
	AudioDurationSecs   *float64                     `json:"audio_duration_secs,omitempty"`
	Entities            []DetectedEntity             `json:"entities,omitempty"`
	Transcripts         []Transcript                 `json:"transcripts,omitempty"`
}

// Chunks returns the channel-level transcripts when present, otherwise the
// transcript itself as a single chunk.
func (t Transcript) Chunks() []Transcript {
	if len(t.Transcripts) > 0 {
		return t.Transcripts
	}
	return []Transcript{t}
}

// TranscriptWord is a word-level transcript segment.
type TranscriptWord struct {
	Text         string                `json:"text"`
	Type         string                `json:"type,omitempty"`
	Start        *float64              `json:"start,omitempty"`
	End          *float64              `json:"end,omitempty"`
	SpeakerID    string                `json:"speaker_id,omitempty"`
	Logprob      float64               `json:"logprob,omitempty"`
	Characters   []TranscriptCharacter `json:"characters,omitempty"`
	ChannelIndex *int                  `json:"channel_index,omitempty"`
}

// TranscriptCharacter is a character-level transcript segment.
type TranscriptCharacter struct {
	Text  string   `json:"text"`
	Start *float64 `json:"start,omitempty"`
	End   *float64 `json:"end,omitempty"`
}

// DetectedEntity is an entity detected in a transcript.
type DetectedEntity struct {
	Text       string `json:"text"`
	EntityType string `json:"entity_type"`
	StartChar  int    `json:"start_char"`
	EndChar    int    `json:"end_char"`
}

// TranscriptAdditionalFormatOptions configures an additional transcript export
// format.
type TranscriptAdditionalFormatOptions struct {
	Format                      string   `json:"format"`
	IncludeSpeakers             *bool    `json:"include_speakers,omitempty"`
	IncludeTimestamps           *bool    `json:"include_timestamps,omitempty"`
	SegmentOnSilenceLongerThanS *float64 `json:"segment_on_silence_longer_than_s,omitempty"`
	MaxSegmentDurationS         *float64 `json:"max_segment_duration_s,omitempty"`
	MaxSegmentChars             *int     `json:"max_segment_chars,omitempty"`
	MaxCharactersPerLine        *int     `json:"max_characters_per_line,omitempty"`
}

// TranscriptAdditionalFormat is an additional transcript export returned by
// the API.
type TranscriptAdditionalFormat struct {
	RequestedFormat string `json:"requested_format"`
	FileExtension   string `json:"file_extension"`
	ContentType     string `json:"content_type"`
	IsBase64Encoded bool   `json:"is_base64_encoded"`
	Content         string `json:"content"`
}

// TranscriptWebhookResponse is returned when a transcript is submitted for
// asynchronous webhook processing.
type TranscriptWebhookResponse struct {
	Message         string  `json:"message"`
	RequestID       string  `json:"request_id"`
	TranscriptionID *string `json:"transcription_id,omitempty"`
}
