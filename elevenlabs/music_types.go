package elevenlabs

import (
	"errors"
	"io"
)

// ComposeMusicRequest contains parameters for ElevenLabs music composition
// requests.
type ComposeMusicRequest struct {
	CompositionPlan         MusicCompositionPlan `json:"composition_plan,omitempty"`
	ForceInstrumental       *bool                `json:"force_instrumental,omitempty"`
	ModelID                 MusicModelID         `json:"model_id,omitempty"`
	MusicLengthMS           *int                 `json:"music_length_ms,omitempty"`
	OutputFormat            OutputFormat         `json:"-"`
	Prompt                  string               `json:"prompt,omitempty"`
	RespectSectionDurations *bool                `json:"respect_sections_durations,omitempty"`
	Seed                    *int                 `json:"seed,omitempty"`
	SignWithC2PA            *bool                `json:"sign_with_c2pa,omitempty"`
	StoreForInpainting      *bool                `json:"store_for_inpainting,omitempty"`
}

// ComposeDetailedMusicRequest contains parameters for ElevenLabs detailed music
// composition requests.
type ComposeDetailedMusicRequest struct {
	CompositionPlan         MusicCompositionPlan `json:"composition_plan,omitempty"`
	ForceInstrumental       *bool                `json:"force_instrumental,omitempty"`
	ModelID                 MusicModelID         `json:"model_id,omitempty"`
	MusicLengthMS           *int                 `json:"music_length_ms,omitempty"`
	OutputFormat            OutputFormat         `json:"-"`
	Prompt                  string               `json:"prompt,omitempty"`
	RespectSectionDurations *bool                `json:"respect_sections_durations,omitempty"`
	Seed                    *int                 `json:"seed,omitempty"`
	SignWithC2PA            *bool                `json:"sign_with_c2pa,omitempty"`
	StoreForInpainting      *bool                `json:"store_for_inpainting,omitempty"`
	WithTimestamps          *bool                `json:"with_timestamps,omitempty"`
}

// CreateCompositionPlanRequest contains parameters for generating a music
// composition plan.
type CreateCompositionPlanRequest struct {
	ModelID               MusicModelID         `json:"model_id,omitempty"`
	MusicLengthMS         *int                 `json:"music_length_ms,omitempty"`
	Prompt                string               `json:"prompt"`
	SourceCompositionPlan MusicCompositionPlan `json:"source_composition_plan,omitempty"`
}

// MusicModelID identifies the music generation model.
type MusicModelID string

const (
	MusicModelV1 MusicModelID = "music_v1"
	MusicModelV2 MusicModelID = "music_v2"
)

// MusicComposition is a generated music composition.
type MusicComposition struct {
	Audio []byte
	// SongID is read from the song-id response header.
	SongID string
}

// DetailedMusicComposition is a generated music composition with metadata.
type DetailedMusicComposition struct {
	Audio           []byte
	CompositionPlan MusicCompositionPlan
	// SongID is read from the song-id response header.
	SongID       string
	SongMetadata *MusicSongMetadata
}

// MusicSongMetadata contains metadata returned by detailed music composition.
type MusicSongMetadata struct {
	Description string   `json:"description,omitempty"`
	Genres      []string `json:"genres,omitempty"`
	IsExplicit  bool     `json:"is_explicit,omitempty"`
	Languages   []string `json:"languages,omitempty"`
	Title       string   `json:"title,omitempty"`
}

// StreamMusicRequest contains parameters for ElevenLabs streaming music
// composition requests.
type StreamMusicRequest struct {
	CompositionPlan    MusicCompositionPlan `json:"composition_plan,omitempty"`
	ForceInstrumental  *bool                `json:"force_instrumental,omitempty"`
	ModelID            MusicModelID         `json:"model_id,omitempty"`
	MusicLengthMS      *int                 `json:"music_length_ms,omitempty"`
	OutputFormat       OutputFormat         `json:"-"`
	Prompt             string               `json:"prompt,omitempty"`
	Seed               *int                 `json:"seed,omitempty"`
	StoreForInpainting *bool                `json:"store_for_inpainting,omitempty"`
}

// MusicStream is a streaming music composition response. The caller must close
// it when finished reading.
type MusicStream struct {
	Body io.ReadCloser
	// SongID is read from the song-id response header.
	SongID string
}

// Read reads audio bytes from the response stream.
func (s *MusicStream) Read(p []byte) (int, error) {
	if s == nil || s.Body == nil {
		return 0, errors.New("elevenlabs: nil music stream")
	}
	return s.Body.Read(p)
}

// Close closes the response stream.
func (s *MusicStream) Close() error {
	if s == nil || s.Body == nil {
		return nil
	}
	return s.Body.Close()
}

// MusicCompositionPlan is the composition_plan request union. Use MusicPrompt
// with music_v1 and CompositionPlan with music_v2.
type MusicCompositionPlan interface {
	isMusicCompositionPlan()
}

// AudioRefChunk references an existing song range for v2 composition plans.
type AudioRefChunk struct {
	Range  TimeRange `json:"range"`
	SongID string    `json:"song_id"`
}

func (AudioRefChunk) isCompositionPlanChunk() {}

// CompositionPlan is a chunk-based composition plan for the music_v2 model.
type CompositionPlan struct {
	Chunks []CompositionPlanChunk `json:"chunks"`
}

func (CompositionPlan) isMusicCompositionPlan() {}

// CompositionPlanChunk is a chunk entry in a music_v2 CompositionPlan.
type CompositionPlanChunk interface {
	isCompositionPlanChunk()
}

// GenerationChunkInput describes one generated chunk in a music_v2 composition
// plan.
type GenerationChunkInput struct {
	ConditionStrength string         `json:"condition_strength,omitempty"`
	ConditioningRef   *AudioRefChunk `json:"conditioning_ref,omitempty"`
	ContextAdherence  string         `json:"context_adherence,omitempty"`
	DurationMS        int            `json:"duration_ms"`
	NegativeStyles    []string       `json:"negative_styles,omitempty"`
	PositiveStyles    []string       `json:"positive_styles"`
	Text              string         `json:"text"`
}

func (GenerationChunkInput) isCompositionPlanChunk() {}

// MusicPrompt is a section-based composition plan for the music_v1 model.
type MusicPrompt struct {
	NegativeGlobalStyles []string      `json:"negative_global_styles"`
	PositiveGlobalStyles []string      `json:"positive_global_styles"`
	Sections             []SongSection `json:"sections"`
}

func (MusicPrompt) isMusicCompositionPlan() {}

// SectionSource references an existing song section for inpainting.
type SectionSource struct {
	NegativeRanges []TimeRange `json:"negative_ranges,omitempty"`
	Range          TimeRange   `json:"range"`
	SongID         string      `json:"song_id"`
}

// SongSection describes one section in a music_v1 MusicPrompt.
type SongSection struct {
	DurationMS          int            `json:"duration_ms"`
	Lines               []string       `json:"lines"`
	NegativeLocalStyles []string       `json:"negative_local_styles"`
	PositiveLocalStyles []string       `json:"positive_local_styles"`
	SectionName         string         `json:"section_name"`
	SourceFrom          *SectionSource `json:"source_from,omitempty"`
}

// TimeRange identifies a range within a generated song in milliseconds.
type TimeRange struct {
	EndMS   int `json:"end_ms"`
	StartMS int `json:"start_ms"`
}
