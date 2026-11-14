package media

import (
	"fmt"
	"math"

	"github.com/rottentail/solo-0026-audiovisual-sync-conformer/internal/domain"
)

func AlignAudio(audioData []byte, measuredOffsetMS int, maxCorrectionMS int) (domain.AlignedAudio, error) {
	clip, err := InspectAudio(audioData)
	if err != nil {
		return domain.AlignedAudio{}, fmt.Errorf("audio probe failed: %w", err)
	}
	if absInt(measuredOffsetMS) > maxCorrectionMS {
		return domain.AlignedAudio{}, domain.NewError(
			"correction_limit_exceeded",
			422,
			"measured correction of %d ms exceeds the ceiling of %d ms",
			absInt(measuredOffsetMS),
			maxCorrectionMS,
		)
	}
	frameDelta := int(math.Round(float64(measuredOffsetMS) * float64(clip.Info.SampleRate) / 1000))
	channels := clip.Info.Channels
	sampleDelta := frameDelta * channels
	output := make([]int16, len(clip.Samples))
	// The same frame shift is applied to every interleaved channel, so the
	// original channel order and inter-channel timing are preserved. No
	// reference channel is mixed in and no channel is dropped or summed.
	for outputIndex := range output {
		sourceIndex := outputIndex + sampleDelta
		if sourceIndex >= 0 && sourceIndex < len(clip.Samples) {
			output[outputIndex] = clip.Samples[sourceIndex]
		}
	}
	wav, err := EncodePCM16(clip.Info.SampleRate, channels, output)
	if err != nil {
		return domain.AlignedAudio{}, err
	}
	return domain.AlignedAudio{
		WAV:            wav,
		AppliedSamples: frameDelta,
		Digest:         DigestBytes(wav),
	}, nil
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
