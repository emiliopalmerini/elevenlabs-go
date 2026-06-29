# elevenlabs-go

A small Go client for the ElevenLabs API.

This module currently focuses on speech-to-text workflows and a few account
metadata endpoints:

- create, retrieve, and delete transcripts
- submit asynchronous transcript webhook jobs
- stream audio to the realtime transcription WebSocket API
- list models
- read authenticated user metadata
- inspect API errors and raw HTTP response metadata
- retry replayable transient failures

## Installation

```sh
go get github.com/emiliopalmerini/elevenlabs-go@v0.1.0
```

Import the package as:

```go
import elevenlabs "github.com/emiliopalmerini/elevenlabs-go"
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	elevenlabs "github.com/emiliopalmerini/elevenlabs-go"
)

func main() {
	ctx := context.Background()
	client := elevenlabs.NewClient(os.Getenv("ELEVENLABS_API_KEY"))

	file, err := os.Open("audio.mp3")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	transcript, err := client.CreateTranscript(ctx, elevenlabs.CreateTranscriptRequest{
		ModelID: "scribe_v1",
		File: &elevenlabs.File{
			Name:   "audio.mp3",
			Reader: file,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(transcript.Text)
}
```

You can also transcribe by URL:

```go
transcript, err := client.CreateTranscript(ctx, elevenlabs.CreateTranscriptRequest{
	ModelID:   "scribe_v1",
	SourceURL: "https://example.com/audio.mp3",
})
```

## Advanced Transcript Options

`CreateTranscriptRequest` exposes ElevenLabs speech-to-text options such as
language code, diarization, speaker count, keyterms, multichannel output,
entity detection, redaction, additional formats, webhook metadata, and upload
progress callbacks.

```go
diarize := true

transcript, err := client.CreateTranscript(ctx, elevenlabs.CreateTranscriptRequest{
	ModelID:      "scribe_v1",
	SourceURL:    "https://example.com/interview.mp3",
	LanguageCode: "en",
	Diarize:      &diarize,
	Keyterms:     []string{"ElevenLabs", "speech-to-text"},
})
```

For multichannel responses, `Transcript.Chunks()` returns the channel-level
transcripts when present, otherwise a single chunk containing the transcript
itself.

## Webhook Transcripts

Use `SubmitTranscriptWebhook` when the transcript should be processed
asynchronously by ElevenLabs:

```go
resp, err := client.SubmitTranscriptWebhook(ctx, elevenlabs.CreateTranscriptRequest{
	ModelID:   "scribe_v1",
	SourceURL: "https://example.com/audio.mp3",
	WebhookID: "your-webhook-id",
	WebhookMetadata: map[string]any{
		"job_id": "123",
	},
})
if err != nil {
	return err
}

fmt.Println(resp.TranscriptionID)
```

## Realtime Transcription

Realtime transcription uses the package WebSocket session type:

```go
session, err := client.ConnectRealtimeTranscript(ctx, elevenlabs.RealtimeTranscriptRequest{
	ModelID:     "scribe_v1",
	AudioFormat: "pcm_16000",
})
if err != nil {
	return err
}
defer session.Close()

if err := session.SendAudioChunk(elevenlabs.RealtimeAudioChunk{
	Audio:      pcmBytes,
	Commit:     true,
	SampleRate: 16000,
}); err != nil {
	return err
}

event, err := session.Receive()
if err != nil {
	return err
}

fmt.Println(event.Text)
```

The session can authenticate with the client API key or with a realtime token
passed on `RealtimeTranscriptRequest.Token`.

## Response Metadata

Methods ending in `WithResponse` return parsed data plus raw HTTP metadata:

```go
resp, err := client.GetTranscriptWithResponse(ctx, "transcript-id")
if err != nil {
	return err
}

fmt.Println(resp.RawResponse.StatusCode)
fmt.Println(resp.RawResponse.Header.Get("request-id"))
fmt.Println(resp.Data.Text)
```

## Error Handling

Non-2xx API responses return `*elevenlabs.APIError` when the response can be
read:

```go
transcript, err := client.GetTranscript(ctx, "transcript-id")
if err != nil {
	var apiErr *elevenlabs.APIError
	if errors.As(err, &apiErr) {
		fmt.Println(apiErr.StatusCode)
		fmt.Println(apiErr.Message)
		fmt.Println(apiErr.RequestID)
		return err
	}
	return err
}

fmt.Println(transcript.Text)
```

`APIError` keeps provider error fields, validation details, retry headers, and
the raw response metadata.

## Retries

Replayable requests retry transient status codes by default:

- `429 Too Many Requests`
- `500 Internal Server Error`
- `502 Bad Gateway`
- `503 Service Unavailable`
- `504 Gateway Timeout`

Customize or disable retries with client options:

```go
client := elevenlabs.NewClient(
	os.Getenv("ELEVENLABS_API_KEY"),
	elevenlabs.WithRetryConfig(elevenlabs.RetryConfig{
		MaxAttempts: 5,
	}),
)

noRetryClient := elevenlabs.NewClient(
	os.Getenv("ELEVENLABS_API_KEY"),
	elevenlabs.WithoutRetries(),
)
```

File uploads are retried only when the upload body can be replayed.

## Client Options

```go
client := elevenlabs.NewClient(
	os.Getenv("ELEVENLABS_API_KEY"),
	elevenlabs.WithHTTPClient(customHTTPClient),
	elevenlabs.WithBaseURL("https://api.elevenlabs.io"),
)
```

`WithBaseURL` is mainly useful for tests or custom API routing.

## Development

Run the package checks with:

```sh
go test ./...
go vet ./...
```

Tagged releases are standard Go module versions:

```sh
git tag v0.1.0
git push origin v0.1.0
```

Consumers can then depend on the module with:

```sh
go get github.com/emiliopalmerini/elevenlabs-go@v0.1.0
```
