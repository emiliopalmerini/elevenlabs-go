package elevenlabs

// OutputFormat identifies the encoded audio format requested from APIs that
// accept the output_format query parameter.
type OutputFormat string

const (
	OutputFormatMP3_22050_32   OutputFormat = "mp3_22050_32"
	OutputFormatMP3_24000_48   OutputFormat = "mp3_24000_48"
	OutputFormatMP3_44100_32   OutputFormat = "mp3_44100_32"
	OutputFormatMP3_44100_64   OutputFormat = "mp3_44100_64"
	OutputFormatMP3_44100_96   OutputFormat = "mp3_44100_96"
	OutputFormatMP3_44100_128  OutputFormat = "mp3_44100_128"
	OutputFormatMP3_44100_192  OutputFormat = "mp3_44100_192"
	OutputFormatPCM_8000       OutputFormat = "pcm_8000"
	OutputFormatPCM_16000      OutputFormat = "pcm_16000"
	OutputFormatPCM_22050      OutputFormat = "pcm_22050"
	OutputFormatPCM_24000      OutputFormat = "pcm_24000"
	OutputFormatPCM_32000      OutputFormat = "pcm_32000"
	OutputFormatPCM_44100      OutputFormat = "pcm_44100"
	OutputFormatPCM_48000      OutputFormat = "pcm_48000"
	OutputFormatULaw_8000      OutputFormat = "ulaw_8000"
	OutputFormatALaw_8000      OutputFormat = "alaw_8000"
	OutputFormatOpus_48000_32  OutputFormat = "opus_48000_32"
	OutputFormatOpus_48000_64  OutputFormat = "opus_48000_64"
	OutputFormatOpus_48000_96  OutputFormat = "opus_48000_96"
	OutputFormatOpus_48000_128 OutputFormat = "opus_48000_128"
	OutputFormatOpus_48000_192 OutputFormat = "opus_48000_192"
)
