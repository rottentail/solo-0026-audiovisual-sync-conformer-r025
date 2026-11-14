package media

import (
	"fmt"
	"math"

	"github.com/rottentail/solo-0026-audiovisual-sync-conformer/internal/domain"
)

const (
	CalibrationSampleRate = 48000
	CalibrationFrames     = 24
	CalibrationAudioMS    = 1000
	CalibrationAudioEvent = 500
	CalibrationVideoEvent = 9
	CalibrationFPS        = 24
)

// MultichannelClapChannels is the channel count of the multichannel field
// recording built by MultichannelCalibrationPair.
const MultichannelClapChannels = 6

func CalibrationPair() ([]byte, []byte, error) {
	samples := calibrationMonoSamples()
	audio, err := EncodePCM16(CalibrationSampleRate, 1, samples)
	if err != nil {
		return nil, nil, err
	}
	video, err := calibrationVideo()
	if err != nil {
		return nil, nil, err
	}
	return audio, video, nil
}

// MultichannelCalibrationPair builds a field-recording style WAV with several
// loud dialogue transients on the non-reference channels and the slate clap
// recorded only on channels 1 and 2 (1-based). The clap stays at
// CalibrationAudioEvent ms while the dialogue transients are louder and happen
// earlier, so any detector that ignores the designated reference channel would
// measure the wrong offset. The Y4M picture is identical to the standard
// calibration pair.
func MultichannelCalibrationPair() ([]byte, []byte, error) {
	monoSamples := calibrationMonoSamples()
	channels := MultichannelClapChannels
	frameCount := len(monoSamples)
	samples := make([]int16, frameCount*channels)
	dialogueStart := CalibrationSampleRate * 150 / 1000
	for frame := 0; frame < frameCount; frame++ {
		clap := monoSamples[frame]
		for channel := 0; channel < channels; channel++ {
			value := int16(0)
			switch {
			// The slate clap is recorded only on the first two channels.
			case channel < 2:
				value = clap
			default:
				if frame >= dialogueStart {
					elapsed := float64(frame-dialogueStart) / CalibrationSampleRate
					envelope := math.Exp(-elapsed * 4)
					wave := math.Sin(2 * math.Pi * 300 * float64(frame-dialogueStart) / CalibrationSampleRate)
					value = int16(math.Round(wave * 18000 * envelope))
				}
			}
			samples[frame*channels+channel] = value
		}
	}
	audio, err := EncodePCM16(CalibrationSampleRate, channels, samples)
	if err != nil {
		return nil, nil, err
	}
	video, err := calibrationVideo()
	if err != nil {
		return nil, nil, err
	}
	return audio, video, nil
}

func calibrationMonoSamples() []int16 {
	sampleCount := CalibrationSampleRate * CalibrationAudioMS / 1000
	samples := make([]int16, sampleCount)
	eventSample := CalibrationSampleRate * CalibrationAudioEvent / 1000
	for index := eventSample; index < sampleCount; index++ {
		elapsed := float64(index-eventSample) / CalibrationSampleRate
		envelope := math.Exp(-elapsed * 7)
		value := math.Sin(2*math.Pi*440*float64(index-eventSample)/CalibrationSampleRate) * 12000 * envelope
		samples[index] = int16(math.Round(value))
	}
	return samples
}

func calibrationVideo() ([]byte, error) {
	width, height := 96, 54
	frameBytes := width*height + 2*(width/2)*(height/2)
	frames := make([][]byte, 0, CalibrationFrames)
	for frameIndex := 0; frameIndex < CalibrationFrames; frameIndex++ {
		frame := make([]byte, frameBytes)
		luma := byte(32)
		if frameIndex == CalibrationVideoEvent {
			luma = 220
		}
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				frame[y*width+x] = luma
			}
		}
		chromaSize := (width / 2) * (height / 2)
		for index := 0; index < chromaSize; index++ {
			frame[width*height+index] = 128
			frame[width*height+chromaSize+index] = 128
		}
		frames = append(frames, frame)
	}
	info := domain.VideoInfo{
		Width:      width,
		Height:     height,
		FPSNum:     CalibrationFPS,
		FPSDen:     1,
		FrameCount: len(frames),
		Chroma:     "C420jpeg",
	}
	return EncodeY4M(info, frames)
}

func MustCalibrationPair() ([]byte, []byte) {
	audio, video, err := CalibrationPair()
	if err != nil {
		panic(fmt.Sprintf("calibration media construction failed: %v", err))
	}
	return audio, video
}
