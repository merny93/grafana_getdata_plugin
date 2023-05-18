package plugin

import (
	"errors"
	"math"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func decimate(data []float64, decimationFactor int) []float64 {
	resultSize := int(len(data) / int(decimationFactor))
	dataTmp := make([]float64, resultSize)

	for i := 0; i < resultSize*decimationFactor; i++ {
		if i%int(decimationFactor) == 0 {
			dataTmp[int(i/int(decimationFactor))] = data[i]
		}
	}
	return dataTmp
}

func unixSlice2TimeSlice(unixTimeSlice []float64) []time.Time {
	timeSlice := make([]time.Time, len(unixTimeSlice))

	//loop through the ctimes and turn them into time objects
	for i, c_time := range unixTimeSlice {
		timeSlice[i] = time.Unix(int64(c_time), int64(math.Mod(c_time, 1)*1e9))
	}
	return timeSlice
}

func indexSlice2TimeSlice(indexSlice []float64, sampleRate float64, lastTime time.Time) []time.Time {
	timeSlice := make([]time.Time, len(indexSlice))
	if len(indexSlice) == 0 {
		return timeSlice
	}
	lastIndex := indexSlice[len(indexSlice)-1]
	for i := 0; i < len(indexSlice); i++ {
		timeFloat := (indexSlice[i]-lastIndex)/sampleRate + (float64(lastTime.UnixMilli()) / 1e3)
		timeSlice[i] = time.Unix(int64(timeFloat), int64(math.Mod(timeFloat, 1)*1e9))
	}

	return timeSlice

}

func upsample(data []float64, upsampleFactor int) ([]float64, error) {
	if len(data) < 2 {
		backend.Logger.Info("data length must be at least 2 to upsample")
		return nil, errors.New("data length must be at least 2 to upsample")
	}
	dataUpsampled := make([]float64, len(data)*upsampleFactor)
	jdx := -1
	for idx := 0; idx < len(dataUpsampled); idx++ {
		i := idx % upsampleFactor
		if i == 0 {
			jdx++
			dataUpsampled[idx] = data[jdx]
		} else {
			t := float64(i) / float64(upsampleFactor)
			if jdx == len(data)-1 {
				// backend.Logger.Info("upsampling last point")
				// backend.Logger.Info(fmt.Sprintf("len(dataUpsampled): %v, len(data): %v, jdx: %v, idx: %v", len(dataUpsampled), len(data), jdx, idx))
				dataUpsampled[idx] = data[jdx] + (data[jdx]-data[jdx-1])*t
			} else {
				dataUpsampled[idx] = data[jdx]*(1-t) + data[jdx+1]*(t)
			}
		}
	}
	return dataUpsampled, nil
}

func compatibleDecimationFactor(decimationFactor int, spf int) int {
	if decimationFactor > spf {
		decimationFactor = int(math.Ceil(float64(decimationFactor)/float64(spf))) * spf
	} else {
		// get the smallest number larger than decimationFactor which is a divisor of spf
		for ; decimationFactor <= spf; decimationFactor++ {
			if spf%decimationFactor == 0 {
				break
			}
		}
	}
	return decimationFactor

}
