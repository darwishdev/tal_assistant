//go:build windows

package go_recording

import (
	"fmt"
	"strings"

	"github.com/cyberxnomad/wasapi/audioclient"
	"github.com/cyberxnomad/wasapi/com"
	"github.com/cyberxnomad/wasapi/mmdevice"
	"golang.org/x/sys/windows"
)

// PKEY_AudioEndpoint_FormFactor = {1DA5D803-D492-4EDD-8C23-E0C0FFEE7F0E}, 0
// Not exposed by the wasapi library, defined manually here.
var pkeyFormFactor = com.PROPERTYKEY{
	Fmtid: windows.GUID{
		Data1: 0x1DA5D803,
		Data2: 0xD492,
		Data3: 0x4EDD,
		Data4: [8]byte{0x8C, 0x23, 0xE0, 0xC0, 0xFF, 0xEE, 0x7F, 0x0E},
	},
	Pid: 0,
}

// ListDevices returns all currently active audio endpoints (speakers + mics).
func ListDevices() ([]Device, error) {
	var out []Device
	err := withCOM(func() error {
		enum, done, err := newEnumerator()
		if err != nil {
			return err
		}
		defer done()

		speakers, err := listFlow(enum, mmdevice.ERender)
		if err != nil {
			return err
		}
		mics, err := listFlow(enum, mmdevice.ECapture)
		if err != nil {
			return err
		}
		out = append(speakers, mics...)
		return nil
	})
	return out, err
}

// ListSpeakers returns all active render (speaker / loopback) endpoints.
func ListSpeakers() ([]Device, error) {
	var out []Device
	err := withCOM(func() error {
		enum, done, err := newEnumerator()
		if err != nil {
			return err
		}
		defer done()
		out, err = listFlow(enum, mmdevice.ERender)
		return err
	})
	return out, err
}

// ListMics returns all active capture (microphone) endpoints.
func ListMics() ([]Device, error) {
	var out []Device
	err := withCOM(func() error {
		enum, done, err := newEnumerator()
		if err != nil {
			return err
		}
		defer done()
		out, err = listFlow(enum, mmdevice.ECapture)
		return err
	})
	return out, err
}

// DefaultSpeaker returns the current Windows default render device.
func DefaultSpeaker() (*Device, error) {
	return defaultDevice(mmdevice.ERender, FlowSpeaker)
}

// DefaultMic returns the current Windows default capture device.
func DefaultMic() (*Device, error) {
	return defaultDevice(mmdevice.ECapture, FlowMic)
}

// --- internal helpers -------------------------------------------------------

func defaultDevice(dataFlow mmdevice.EDataFlow, flow Flow) (*Device, error) {
	var out *Device
	err := withCOM(func() error {
		enum, done, err := newEnumerator()
		if err != nil {
			return err
		}
		defer done()

		raw, err := enum.GetDefaultAudioEndpoint(dataFlow, mmdevice.EConsole)
		if err != nil {
			return fmt.Errorf("GetDefaultAudioEndpoint: %w", err)
		}
		defer raw.Release()

		d, err := deviceFromRaw(raw, flow, true)
		if err != nil {
			return err
		}
		out = &d
		return nil
	})
	return out, err
}

// withCOM initialises COM for the current goroutine, runs fn, then
// uninitialises.  Safe to call from multiple goroutines independently.
func withCOM(fn func() error) error {
	if err := com.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); err != nil {
		return fmt.Errorf("CoInitializeEx: %w", err)
	}
	defer com.CoUninitialize()
	return fn()
}

func newEnumerator() (*mmdevice.IMMDeviceEnumerator, func(), error) {
	clsid := mmdevice.CLSID_MMDeviceEnumerator()
	iid := mmdevice.IID_IMMDeviceEnumerator()
	v, err := com.CoCreateInstance(&clsid, nil, clsctxAll, &iid)
	if err != nil {
		return nil, nil, fmt.Errorf("CoCreateInstance: %w", err)
	}
	e := mmdevice.ToType[mmdevice.IMMDeviceEnumerator](v)
	return e, func() { e.Release() }, nil
}

// listFlow enumerates all active devices for the given data flow.
func listFlow(enum *mmdevice.IMMDeviceEnumerator, dataFlow mmdevice.EDataFlow) ([]Device, error) {
	flow := FlowSpeaker
	if dataFlow == mmdevice.ECapture {
		flow = FlowMic
	}

	// Determine the default device ID so we can mark it.
	defaultID := ""
	if d, err := enum.GetDefaultAudioEndpoint(dataFlow, mmdevice.EConsole); err == nil {
		defaultID, _ = d.GetId()
		d.Release()
	}

	col, err := enum.EnumAudioEndpoints(dataFlow, mmdevice.DEVICE_STATE_ACTIVE)
	if err != nil {
		return nil, fmt.Errorf("EnumAudioEndpoints: %w", err)
	}
	defer col.Release()

	cnt, err := col.GetCount()
	if err != nil {
		return nil, fmt.Errorf("GetCount: %w", err)
	}

	out := make([]Device, 0, cnt)
	for i := uint32(0); i < uint32(cnt); i++ {
		raw, err := col.Item(i)
		if err != nil {
			continue
		}
		id, _ := raw.GetId()
		d, err := deviceFromRaw(raw, flow, id == defaultID)
		raw.Release()
		if err != nil {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

// deviceFromRaw reads all metadata from a WASAPI IMMDevice pointer.
func deviceFromRaw(dev *mmdevice.IMMDevice, flow Flow, isDefault bool) (Device, error) {
	d := Device{Flow: flow, IsDefault: isDefault}

	d.ID, _ = dev.GetId()

	// Friendly name and form factor from the property store.
	if props, err := dev.OpenPropertyStore(com.STGM_READ); err == nil {
		if pv, err := props.GetValue(com.PKEY_Device_FriendlyName()); err == nil {
			d.Name = pv.PwszValString()
		}
		if pv, err := props.GetValue(pkeyFormFactor); err == nil {
			// PROPVARIANT.Val holds the uint32 form-factor value (VT_UI4).
			d.FormFactor = formFactorFromUint32(uint32(pv.Val))
		}
		props.Release()
	}

	if d.Name == "" {
		d.Name = d.ID
	}
	if d.FormFactor == "" {
		d.FormFactor = FormFactorUnknown
	}
	d.IsPersonal = personalFormFactor(d.FormFactor) || strings.Contains(strings.ToLower(d.Name), "usb")

	// Activate an IAudioClient briefly to read the device mix format.
	iidAC := audioclient.IID_IAudioClient()
	if v, err := dev.Activate(iidAC, clsctxAll, nil); err == nil {
		client := audioclient.ToType[audioclient.IAudioClient](v)
		if f, err := client.GetMixFormat(); err == nil {
			d.Channels = f.Format.Channels
			d.SampleRate = f.Format.SamplesPerSec
			d.BitDepth = f.Format.BitsPerSample
		}
		client.Release()
	}

	return d, nil
}

// formFactorFromUint32 maps the Windows EndpointFormFactor enum to our type.
func formFactorFromUint32(v uint32) FormFactor {
	switch mmdevice.EndpointFormFactor(v) {
	case mmdevice.RemoteNetworkDevice:
		return FormFactorNetwork
	case mmdevice.Speakers:
		return FormFactorSpeakers
	case mmdevice.LineLevel:
		return FormFactorLine
	case mmdevice.Headphones:
		return FormFactorHeadphones
	case mmdevice.Microphone:
		return FormFactorMicrophone
	case mmdevice.Headset:
		return FormFactorHeadset
	case mmdevice.Handset:
		return FormFactorHandset
	case mmdevice.SPDIF:
		return FormFactorSPDIF
	case mmdevice.DigitalAudioDisplayDevice:
		return FormFactorDigitalDisplay
	default:
		return FormFactorUnknown
	}
}

// personalFormFactor returns true for close-range endpoints that do not
// radiate sound into the room (and therefore won't bleed into a mic).
func personalFormFactor(ff FormFactor) bool {
	return ff == FormFactorHeadphones || ff == FormFactorHeadset || ff == FormFactorHandset
}
