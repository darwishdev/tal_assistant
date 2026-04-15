package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"tal_assistant/pkg/recording"
)

func main() {
	svc := recording.NewRecordingService()
	ctx := context.Background()

	devices, err := svc.AudioDeviceList(ctx)
	if err != nil {
		log.Fatalf("Failed to list devices: %v", err)
	}

	fmt.Println("Available Audio Devices:")
	var micSources []recording.AudioSource
	var speakerSources []recording.AudioSource

	for _, d := range devices {
		if d.Type == "mic" {
			micSources = append(micSources, d)
		} else {
			speakerSources = append(speakerSources, d)
		}
	}

	fmt.Println("\nMicrophones:")
	for i, d := range micSources {
		defStr := ""
		if d.IsDefault {
			defStr = " [DEFAULT]"
		}
		fmt.Printf("[%d] %s (ID: %s)%s\n", i, d.Name, d.ID, defStr)
	}

	fmt.Println("\nSpeakers (Loopback):")
	for i, d := range speakerSources {
		defStr := ""
		if d.IsDefault {
			defStr = " [DEFAULT]"
		}
		fmt.Printf("[%d] %s (ID: %s)%s\n", i, d.Name, d.ID, defStr)
	}

	if len(micSources) == 0 || len(speakerSources) == 0 {
		log.Fatal("Need at least one mic and one speaker source")
	}

	var micIdx, speakerIdx int
	for i, d := range micSources {
		if d.IsDefault {
			micIdx = i
			break
		}
	}
	for i, d := range speakerSources {
		if d.IsDefault {
			speakerIdx = i
			break
		}
	}

	micID := micSources[micIdx].ID
	speakerID := speakerSources[speakerIdx].ID

	fmt.Printf("\nRecording from Mic: %s\n", micSources[micIdx].Name)
	fmt.Printf("Recording from Speaker: %s\n", speakerSources[speakerIdx].Name)

	audioPipe, err := svc.Start(micID, speakerID)
	if err != nil {
		log.Fatalf("Failed to start recording: %v", err)
	}
	defer svc.Stop()
	defer audioPipe.Close()

	if err := os.MkdirAll("tmp", 0755); err != nil {
		log.Fatalf("Failed to create tmp directory: %v", err)
	}

	micFile, err := os.Create("tmp/mic_debug.pcm")
	if err != nil {
		log.Fatalf("Failed to create tmp/mic_debug.pcm: %v", err)
	}
	defer micFile.Close()

	speakerFile, err := os.Create("tmp/speaker_debug.pcm")
	if err != nil {
		log.Fatalf("Failed to create tmp/speaker_debug.pcm: %v", err)
	}
	defer speakerFile.Close()

	mergedFile, err := os.Create("tmp/merged_debug.pcm")
	if err != nil {
		log.Fatalf("Failed to create tmp/merged_debug.pcm: %v", err)
	}
	defer mergedFile.Close()

	fmt.Println("\nRecording started. Output files in tmp/ directory:")
	fmt.Println("- tmp/mic_debug.pcm (Mic only)")
	fmt.Println("- tmp/speaker_debug.pcm (Speaker only)")
	fmt.Println("- tmp/merged_debug.pcm (Mixed single channel)")
	fmt.Println("\nPress Ctrl+C to stop recording.")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	done := make(chan bool)
	go func() {
		buf := make([]byte, 4096) // Multiple of 4 for s16le stereo
		for {
			n, err := audioPipe.Read(buf)
			if n > 0 {
				for i := 0; i < n; i += 4 {
					if i+3 < n {
						// Left channel is Speaker, Right channel is Mic
						spkBytes := buf[i : i+2]
						micBytes := buf[i+2 : i+4]
						
						_, _ = speakerFile.Write(spkBytes)
						_, _ = micFile.Write(micBytes)

						// Mix down to mono
						spkSample := int16(binary.LittleEndian.Uint16(spkBytes))
						micSample := int16(binary.LittleEndian.Uint16(micBytes))
						
						mergedSample := int16((int32(spkSample) + int32(micSample)) / 2)
						var mergedBytes [2]byte
						binary.LittleEndian.PutUint16(mergedBytes[:], uint16(mergedSample))
						_, _ = mergedFile.Write(mergedBytes[:])
					}
				}
			}
			if err != nil {
				if err != io.EOF {
					log.Printf("Read error: %v", err)
				}
				close(done)
				return
			}
		}
	}()

	select {
	case <-stop:
		fmt.Println("\nStop signal received.")
	case <-done:
		fmt.Println("\nRecording stream ended.")
	}

	svc.Stop()
	fmt.Println("Recording stopped. Saving WAV files for easy playback...")

	// Convert PCM to WAV using ffmpeg for easier debugging
	exec.Command("ffmpeg", "-y", "-f", "s16le", "-ar", "16000", "-ac", "1", "-i", "tmp/mic_debug.pcm", "tmp/mic_debug.wav").Run()
	exec.Command("ffmpeg", "-y", "-f", "s16le", "-ar", "16000", "-ac", "1", "-i", "tmp/speaker_debug.pcm", "tmp/speaker_debug.wav").Run()
	exec.Command("ffmpeg", "-y", "-f", "s16le", "-ar", "16000", "-ac", "1", "-i", "tmp/merged_debug.pcm", "tmp/merged_debug.wav").Run()

	fmt.Println("Files saved in tmp/:")
	fmt.Println("- mic_debug.wav / mic_debug.pcm")
	fmt.Println("- speaker_debug.wav / speaker_debug.pcm")
	fmt.Println("- merged_debug.wav / merged_debug.pcm")
}
