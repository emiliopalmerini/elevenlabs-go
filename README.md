# elevenlabs-go

Community Go SDK for the ElevenLabs REST API.

This is intentionally small and handwritten. Version 0.1 focuses on the speech-to-text API, with reusable transport, typed responses, streamed uploads, and inspectable API errors.

## Install

```sh
go get github.com/emiliopalmerini/elevenlabs-go
```

## Transcribe a file

```go
client, err := elevenlabs.NewClient(
	elevenlabs.WithAPIKey(os.Getenv("ELEVENLABS_API_KEY")),
)
if err != nil {
	log.Fatal(err)
}

transcript, err := client.SpeechToText.ConvertFile(ctx, "episode.mp3", &elevenlabs.SpeechToTextRequest{
	ModelID:               elevenlabs.ModelScribeV2,
	TimestampsGranularity: elevenlabs.TimestampsWord,
	Diarize:               elevenlabs.Bool(true),
})
if err != nil {
	log.Fatal(err)
}
fmt.Println(transcript.Text)
```

## Use an HTTPS source URL

```go
transcript, err := client.SpeechToText.Convert(ctx, &elevenlabs.SpeechToTextRequest{
	ModelID:   elevenlabs.ModelScribeV2,
	SourceURL: "https://example.com/audio.mp3",
})
```

## Handle API errors

```go
var apiErr *elevenlabs.APIError
if errors.As(err, &apiErr) {
	fmt.Println(apiErr.StatusCode, apiErr.ProviderCode, apiErr.RequestID)
}
```

## Development

```sh
make check
```

Live integration tests are skipped by default. Set `ELEVENLABS_API_KEY` and `ELEVENLABS_TEST_SOURCE_URL` to run the source URL smoke test.
