package plugin

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

/// stubs for streaming implementation *****************************************

func (d *Datasource) SubscribeStream(ctx context.Context, request *backend.SubscribeStreamRequest) (*backend.SubscribeStreamResponse, error) {
	// Implement subscription logic here
	backend.Logger.Info("SubscribeStream called")
	status := backend.SubscribeStreamStatusOK

	//write down the last frame
	d.lastFrame.Store(request.Path, GD_nframes(d.df)-1)

	return &backend.SubscribeStreamResponse{Status: status}, nil
}

func (d *Datasource) RunStream(ctx context.Context, request *backend.RunStreamRequest, sender *backend.StreamSender) error {

	var err error

	sr, err := decodeChan(request.Path)
	if err != nil {
		return err
	}

	//limit the ticker interval to n second, right now set it to 3 cause why not
	tickerInterval := time.Duration(sr.interval)
	if tickerInterval < 1*time.Second {
		tickerInterval = 3 * time.Second
	}
	ticker := time.NewTicker(tickerInterval)

	var newFrame int
	for {
		select {
		case <-ctx.Done():
			backend.Logger.Info(fmt.Sprintf("Context done on stream %s", request.Path))
			ticker.Stop()
			return err
		case <-ticker.C:
			//check if there is new data
			newFrame = GD_nframes(d.df)
			lastFrameInterface, found := d.lastFrame.Load(request.Path)
			if !found {
				backend.Logger.Info("odd, did not subscribe properly")
				d.lastFrame.Store(request.Path, newFrame)
				continue
			}
			lastFrame := lastFrameInterface.(int)

			//if there is no new data just continue
			if newFrame <= lastFrame {
				backend.Logger.Info(fmt.Sprintf("No new data on channel %s", request.Path))
				continue
			}

			//new data if we got here
			//grab the data and error check
			dataSlice, unixTimeSlice, err := getdata_double(d.df, sr.timeName, sr.fieldName, lastFrame, newFrame-lastFrame)
			if err != nil {
				backend.Logger.Error(err.Error())
				return err
			}

			//lets do a check to see that there is actually data, if not update the lastframe so it does not happen again
			if dataSlice == nil && unixTimeSlice == nil {
				backend.Logger.Info("No data, but frame number went up, strange...")
				d.lastFrame.Store(request.Path, newFrame)
				continue
			}

			// check what the interval is
			// if it is less than the interval of the stream, then we need to decimate

			spf := GD_spf(d.df, sr.fieldName)

			dataInterval := tickerInterval.Seconds() / float64(len(dataSlice))
			// dataInterval = dataInterval / 4 //just to be safe
			if dataInterval < sr.interval.Seconds() {
				//decimate the data by a factor which is either a divisor or a multiple of spf
				decimationFactor := int(math.Ceil(sr.interval.Seconds() / dataInterval))
				decimationFactor = compatibleDecimationFactor(decimationFactor, spf)
				//if the decimation factor is larger than the data slice, then just send one value
				dataSlice = decimate(dataSlice, decimationFactor)
			}
			//adjust x axis to match
			if len(dataSlice) < len(unixTimeSlice) {
				//etiher downsample
				unixTimeSlice = decimate(unixTimeSlice, len(unixTimeSlice)/len(dataSlice))
			} else if len(dataSlice) > len(unixTimeSlice) {
				//or upsample
				if len(unixTimeSlice) == 1 {
					//hard to upsample with just one data point, lets grab another one from the past
					//this call is guaranteed to give 2 data points  since calling it with 1 gave exactly 1
					unixTimeSlice, err = GD_getdata(sr.timeName, d.df, newFrame-2, 2)
					if err != nil {
						backend.Logger.Error(err.Error())
						return err
					}
				}
				unixTimeSlice, err = upsample(unixTimeSlice, len(dataSlice)/len(unixTimeSlice))
				if err != nil {
					backend.Logger.Error(fmt.Sprintf("Error upsampling time in stream: %s", err))
					return err
				}
			}

			//create the response objects out here

			var timeSlice interface{}
			if sr.sampleRate != 0 {
				// indexing by index from end now we can convert index into a time object
				// backend.Logger.Info("comparing a float to an int worked shockingly", sampleRate)
				timeSlice = indexSlice2TimeSlice(unixTimeSlice, sr.sampleRate, time.Now())
			} else if sr.timeType {
				timeSlice = unixSlice2TimeSlice(unixTimeSlice)
			} else {
				timeSlice = unixTimeSlice
			}

			//create frame object
			frame := data.NewFrame("response")
			frame.Fields = append(frame.Fields,
				data.NewField(sr.timeNameField, nil, timeSlice),
				data.NewField(sr.fieldName, nil, dataSlice),
			)

			d.senderLock.Lock()
			err = sender.SendFrame(frame, data.IncludeAll)
			d.senderLock.Unlock()
			if err != nil {
				backend.Logger.Info(fmt.Sprintf("Error sending frame: %v", err))
				return err
			}
			backend.Logger.Info(fmt.Sprintf("Sending frame on endpoint: %s with %v values", request.Path, len(dataSlice)))

			//update the last frame
			d.lastFrame.Store(request.Path, newFrame)

		}
	}
}

func (d *Datasource) PublishStream(ctx context.Context, request *backend.PublishStreamRequest) (*backend.PublishStreamResponse, error) {
	// Implement data publishing logic here
	backend.Logger.Info("PublishStream called")
	status := backend.PublishStreamStatusPermissionDenied
	return &backend.PublishStreamResponse{Status: status}, nil
}
