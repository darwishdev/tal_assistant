//go:build windows

package go_recording

import "fmt"

// FormFactor describes the physical form of an audio endpoint.
// Use IsPersonal to decide whether a speaker bleeds into the microphone.
type FormFactor string

const (
	// FormFactorSpeakers — room / desk speakers.
	// The microphone WILL pick up audio from these when recording simultaneously.
	FormFactorSpeakers FormFactor = "speakers"
	// FormFactorHeadphones — closed-back, no bleed into mic.
	FormFactorHeadphones FormFactor = "headphones"
	// FormFactorHeadset — headphones with built-in mic combo.
	FormFactorHeadset FormFactor = "headset"
	// FormFactorHandset — phone handset style (earpiece + mouth mic).
	FormFactorHandset FormFactor = "handset"
	// FormFactorMicrophone — standalone capture device.
	FormFactorMicrophone FormFactor = "microphone"
	// FormFactorLine — line-level input/output.
	FormFactorLine FormFactor = "line"
	// FormFactorSPDIF — S/PDIF digital passthrough.
	FormFactorSPDIF FormFactor = "spdif"
	// FormFactorDigitalDisplay — HDMI / DisplayPort monitor speakers.
	FormFactorDigitalDisplay FormFactor = "digital_display"
	// FormFactorNetwork — remote / network audio device.
	FormFactorNetwork FormFactor = "network"
	// FormFactorUnknown — unrecognised form factor.
	FormFactorUnknown FormFactor = "unknown"
)

// Flow indicates whether the device renders (speaker / loopback source)
// or captures (microphone) audio.
type Flow string

const (
	FlowSpeaker Flow = "speaker"
	FlowMic     Flow = "mic"
)

// Device is a fully described WASAPI audio endpoint.
type Device struct {
	// ID is the WASAPI endpoint ID string; use it to select a specific device
	// in Options rather than relying on the friendly name.
	ID string
	// Name is the friendly name shown in Windows Sound settings.
	Name string
	// Flow is FlowSpeaker or FlowMic.
	Flow Flow
	// FormFactor describes the physical type of the endpoint.
	FormFactor FormFactor
	// Channels is the native channel count reported by the driver (1, 2, 6, …).
	Channels uint16
	// SampleRate is the native mix rate in Hz (typically 44100 or 48000).
	SampleRate uint32
	// BitDepth is the native sample size in bits (typically 16 or 32).
	BitDepth uint16
	// IsDefault is true when this is the current Windows default for its flow.
	IsDefault bool
	// IsPersonal is true for close-range endpoints (headphones, headset, handset).
	//
	// Practical meaning for recording:
	//   IsPersonal = true  → the mic will NOT hear what the speaker plays.
	//                        Recording speaker + mic together is safe.
	//   IsPersonal = false → room speakers will bleed into the mic.
	//                        Prefer ModeSpeakerOnly or ModeMicOnly, or warn the user.
	IsPersonal bool
}

// String returns a single-line human-readable summary.
func (d Device) String() string {
	def := ""
	if d.IsDefault {
		def = " [default]"
	}
	personal := ""
	if d.IsPersonal {
		personal = ", personal"
	}
	return fmt.Sprintf("%s  (%s%s, %dch %dHz %dbit%s)",
		d.Name, d.FormFactor, personal, d.Channels, d.SampleRate, d.BitDepth, def)
}
