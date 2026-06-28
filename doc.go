// Package elevenlabs provides a small Go client for the ElevenLabs REST API.
//
// The first public surface is intentionally focused on speech-to-text. The
// package exposes typed request and response values, streams multipart uploads,
// and returns API failures as *APIError so callers can inspect status and
// provider metadata with errors.As.
package elevenlabs
