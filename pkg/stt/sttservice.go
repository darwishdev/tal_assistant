package stt

import (
	"context"
	"errors"
	"fmt"
	"io"

	speech "cloud.google.com/go/speech/apiv2"
	"cloud.google.com/go/speech/apiv2/speechpb"
	"google.golang.org/api/option"
)

const (
	SampleRate = 16000
	ChunkSize  = 6400
)

type STTServiceInterface interface {
	StreamDiarized(ctx context.Context, audio io.Reader) (<-chan TranscriptResult, error)
	ProjectID() string
}

// TranscriptResult holds a single recognition result from the stream.
// SpeakerTag is populated when diarization is enabled (1 = first speaker, 2 = second, etc.)
type TranscriptResult struct {
	Text       string
	StartMs    int64
	EndMs      int64
	IsFinal    bool
	SpeakerTag string // "" = unknown, "1"/"2"/... = diarized speaker label
}

type STTService struct {
	projectID  string
	client     *speech.Client
	recognizer string
}

// NewSTTService creates a new STT service client.
// If credentialsPath is empty, it uses Application Default Credentials (ADC).
// For production, provide a service account key file path.
func NewSTTService(projectID string, credentialsPath string) (STTServiceInterface, error) {
	var client *speech.Client
	var err error
	
	if credentialsPath != "" {
		// Use service account credentials file
		client, err = speech.NewClient(context.Background(), option.WithCredentialsFile(credentialsPath))
		if err != nil {
			return nil, fmt.Errorf("speech client with credentials file: %w", err)
		}
	} else {
		// Fall back to Application Default Credentials (development)
		client, err = speech.NewClient(context.Background())
		if err != nil {
			return nil, fmt.Errorf("speech client with ADC: %w", err)
		}
	}
	
	recognizer := fmt.Sprintf("projects/%s/locations/global/recognizers/_", projectID)
	return &STTService{
		projectID:  projectID,
		client:     client,
		recognizer: recognizer,
	}, nil
}

// NewSTTServiceWithCredentials creates a new STT service client using credentials JSON bytes.
// This is useful for embedding credentials directly in the binary.
func NewSTTServiceWithCredentials(projectID string, credentialsJSON []byte) (STTServiceInterface, error) {
	var client *speech.Client
	var err error
	
	if len(credentialsJSON) > 0 {
		// Use credentials from JSON bytes
		client, err = speech.NewClient(context.Background(), option.WithCredentialsJSON(credentialsJSON))
		if err != nil {
			return nil, fmt.Errorf("speech client with credentials JSON: %w", err)
		}
	} else {
		// Fall back to Application Default Credentials (development)
		client, err = speech.NewClient(context.Background())
		if err != nil {
			return nil, fmt.Errorf("speech client with ADC: %w", err)
		}
	}
	
	recognizer := fmt.Sprintf("projects/%s/locations/global/recognizers/_", projectID)
	return &STTService{
		projectID:  projectID,
		client:     client,
		recognizer: recognizer,
	}, nil
}
func (s *STTService) ProjectID() string {
	return s.projectID
}

// StreamDiarized opens a single stereo (or mono) audio stream against Google Speech v2,
// with speaker diarization enabled. Results are pushed onto the returned channel;
// the channel is closed when the stream ends or ctx is cancelled.
//
// audio    – raw PCM s16le, 16 kHz, 2 channels interleaved
// Returns  – <-chan TranscriptResult (read-only to callers)
func (s *STTService) StreamDiarized(ctx context.Context, audio io.Reader) (<-chan TranscriptResult, error) {
	// client, err := speech.NewClient(ctx)
	// if err != nil {
	// 	return nil, fmt.Errorf("speech client: %w", err)
	// }

	stream, err := s.client.StreamingRecognize(ctx)
	if err != nil {
		s.client.Close()
		return nil, fmt.Errorf("streaming recognize: %w", err)
	}

	cfg := &speechpb.StreamingRecognizeRequest{
		Recognizer: s.recognizer,
		StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
			StreamingConfig: &speechpb.StreamingRecognitionConfig{
				Config: &speechpb.RecognitionConfig{
					DecodingConfig: &speechpb.RecognitionConfig_ExplicitDecodingConfig{
						ExplicitDecodingConfig: &speechpb.ExplicitDecodingConfig{
							Encoding:          speechpb.ExplicitDecodingConfig_LINEAR16,
							SampleRateHertz:   SampleRate,
							AudioChannelCount: 2, // stereo — one channel per speaker
						},
					},
					LanguageCodes: []string{"en-US"},
					Model:         "long",
					Features: &speechpb.RecognitionFeatures{
						EnableAutomaticPunctuation: true,
						EnableWordTimeOffsets:      true,
						MultiChannelMode:           1,
					},
				},
				StreamingFeatures: &speechpb.StreamingRecognitionFeatures{
					InterimResults: true,
				},
			},
		},
	}

	if err := stream.Send(cfg); err != nil {
		s.client.Close()
		return nil, fmt.Errorf("send config: %w", err)
	}

	results := make(chan TranscriptResult, 32)

	// Pump raw audio into the stream.
	go func() {
		buf := make([]byte, ChunkSize)
		for {
			n, readErr := io.ReadFull(audio, buf)
			if n > 0 {
				sendErr := stream.Send(&speechpb.StreamingRecognizeRequest{
					Recognizer: s.recognizer,
					StreamingRequest: &speechpb.StreamingRecognizeRequest_Audio{
						Audio: buf[:n],
					},
				})
				if sendErr != nil {
					break
				}
			}
			if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
				break
			}
			if readErr != nil {
				break
			}
		}
		stream.CloseSend()
	}()

	// Receive recognition responses and forward onto results channel.
	go func() {
		// defer s.client.Close()
		defer close(results)

		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				// Only surface non-cancellation errors — caller watches ctx themselves.
				if ctx.Err() == nil {
					// Non-blocking send so a stalled consumer can't block shutdown.
					select {
					case results <- TranscriptResult{}: // zero value signals unexpected close
					default:
					}
				}
				return
			}

			for _, result := range resp.Results {
				if len(result.Alternatives) == 0 {
					continue
				}
				alt := result.Alternatives[0]
				SpeakerTag := "Candidate"
				if result.ChannelTag == 2 {
					SpeakerTag = "You"
				}
				tr := TranscriptResult{
					Text:    alt.Transcript,
					IsFinal: result.IsFinal,
				}
				tr.SpeakerTag = SpeakerTag
				if result.IsFinal && len(alt.Words) > 0 {
					tr.StartMs = alt.Words[0].StartOffset.AsDuration().Milliseconds()
					tr.EndMs = alt.Words[len(alt.Words)-1].EndOffset.AsDuration().Milliseconds()
				} else if result.IsFinal {
					tr.EndMs = result.ResultEndOffset.AsDuration().Milliseconds()
				}

				select {
				case results <- tr:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return results, nil
}
