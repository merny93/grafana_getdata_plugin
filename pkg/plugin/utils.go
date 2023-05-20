package plugin

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func decimate(data []float64, decimationFactor int) []float64 {
	if decimationFactor > len(data) {
		decimationFactor = len(data)
	}
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

func encodeChan(UID, fieldName, interval, timeName string, timeType bool, sampleRateSend float64) string {
	channelName := fmt.Sprintf("ds/%s/steam/%s/%s/%s/%t/%.3f", UID, fieldName, interval, timeName, timeType, sampleRateSend)
	return channelName
}

func decodeChan(path string) (sr StreamRequest, err error) {
	// var sr StreamRequest
	// var err error
	// get the name of the name of the field, held in path as stream/NameOfField
	chunks := strings.Split(path, "/")
	sr.fieldName = chunks[1]
	sr.timeNameField = chunks[3]
	sr.timeName = strings.Split(sr.timeNameField, "__")[0]
	sr.interval, err = time.ParseDuration(chunks[2])
	if err != nil {
		return
	}
	sr.timeType, err = strconv.ParseBool(chunks[4])
	if err != nil {
		return
	}
	sr.sampleRate, err = strconv.ParseFloat(chunks[5], 64)
	if err != nil {
		return
	}
	return
}

func getdata_double(df Dirfile, timeName string, fieldName string, firstFrame int, numFrames int) ([]float64, []float64, error) {
	// grab the data and error check
	dataSlice, err := GD_getdata(fieldName, df, int(firstFrame), numFrames)
	if err != nil {
		return nil, nil, err
	}
	unixTimeSlice, err := GD_getdata(timeName, df, int(firstFrame), numFrames)
	if err != nil {
		return nil, nil, err
	}
	return dataSlice, unixTimeSlice, nil
}
