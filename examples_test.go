package elevenlabs_test

import (
	"context"
	"errors"
	"fmt"
	"os"

	elevenlabs "github.com/emiliopalmerini/elevenlabs-go"
)

func ExampleSpeechToTextService_ConvertFile() {
	client, err := elevenlabs.NewClient(
		elevenlabs.WithAPIKey("test-key"),
	)
	if err != nil {
		panic(err)
	}

	_, err = client.SpeechToText.ConvertFile(context.Background(), "episode.mp3", &elevenlabs.SpeechToTextRequest{
		ModelID:               elevenlabs.ModelScribeV2,
		TimestampsGranularity: elevenlabs.TimestampsWord,
		Diarize:               elevenlabs.Bool(true),
	})
	if err != nil {
		var apiErr *elevenlabs.APIError
		if errors.As(err, &apiErr) {
			fmt.Println(apiErr.StatusCode, apiErr.ProviderCode, apiErr.RequestID)
		}
	}
}

func ExampleNewClient() {
	client, err := elevenlabs.NewClient(
		elevenlabs.WithAPIKey(os.Getenv("ELEVENLABS_API_KEY")),
	)
	if err != nil {
		panic(err)
	}
	_ = client
}
