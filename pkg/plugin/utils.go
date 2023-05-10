package plugin

import (
	"math"
	"time"
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
		timeSlice[i] = time.Unix(int64(c_time), int64(math.Mod(c_time, 1)/1e9))
	}
	return timeSlice
}
