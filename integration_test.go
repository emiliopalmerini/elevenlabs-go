package elevenlabs

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestIntegrationSpeechToTextSourceURL(t *testing.T) {
	apiKey := os.Getenv("ELEVENLABS_API_KEY")
	sourceURL := os.Getenv("ELEVENLABS_TEST_SOURCE_URL")
	if apiKey == "" || sourceURL == "" {
		t.Skip("set ELEVENLABS_API_KEY and ELEVENLABS_TEST_SOURCE_URL to run live integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := NewClient(WithAPIKey(apiKey), WithRetryPolicy(DefaultRetryPolicy()))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	resp, err := client.SpeechToText.Convert(ctx, &SpeechToTextRequest{
		ModelID:       ModelScribeV2,
		SourceURL:     sourceURL,
		EnableLogging: Bool(false),
	})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	if resp.Text == "" && len(resp.Transcripts) == 0 {
		t.Fatalf("empty transcript response: %+v", resp)
	}
}
