package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifier).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	// create response struct
	response := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	for i, q := range req.Queries {
		appendString := ""
		if len(req.Queries) > 1 {
			appendString = fmt.Sprintf("%d", i)
		}
		res := d.query(ctx, req.PluginContext, q, appendString)

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}

func (d *Datasource) query(ctx context.Context, pCtx backend.PluginContext, query backend.DataQuery, timeAppend string) backend.DataResponse {

	//generatae response object
	var response backend.DataResponse

	// Unmarshal the JSON into our queryModel.
	var qm QueryModel

	err := json.Unmarshal(query.JSON, &qm)
	if err != nil {
		//if it fails we really cant do much
		response.Error = err
		return response
	}

	//grab the starting time and the end time
	var numFrames, firstFrame int

	//take a bit more data than u think you need for rounding reasons
	//this makes sure that the screen gets filled
	timeFrom := query.TimeRange.From.UnixMilli() / 1e3
	timeTo := query.TimeRange.To.UnixMilli() / 1e3

	if qm.IndexByIndex {
		// need to find first frame based on start time and num frames based on timerange * sample rate
		if qm.IndexTimeOffsetType == "fromStart" {
			firstFrame = int(float64(timeFrom-qm.IndexTimeOffset) * qm.SampleRate)
		} else if qm.IndexTimeOffsetType == "fromEnd" {
			nFrames := GD_nframes(d.df)
			firstFrame = nFrames - int(float64(qm.IndexTimeOffset-timeFrom)*qm.SampleRate)
			// backend.Logger.Info(fmt.Sprintf("index offset: %d, time from: %d, frames: %d, firstFrame: %d", qm.IndexTimeOffset, timeFrom, nFrames, firstFrame))
		} else if qm.IndexTimeOffsetType == "fromEndNow" {
			nFrames := GD_nframes(d.df)
			firstFrame = nFrames - int(float64(time.Now().Unix()-timeFrom)*qm.SampleRate)
			// backend.Logger.Info(fmt.Sprintf("Time now: %d, time from: %d, frames: %d, firstFrame: %d", time.Now().Unix(), timeFrom, nFrames, firstFrame))
		}
		//get data does not like negative frame numbers
		if firstFrame < 0 {
			firstFrame = 0
		}
		numFrames = int(float64(timeTo-timeFrom) * qm.SampleRate)

	} else {

		firstFrame_float := GD_framenum(d.df, qm.TimeName, float64(timeFrom))
		endFrame := GD_framenum(d.df, qm.TimeName, float64(timeTo))

		//get data does not like negative frame numbers
		if firstFrame_float < 0 {
			firstFrame_float = 0
		}

		numFrames = int(endFrame - firstFrame_float)
		firstFrame = int(firstFrame_float)
	}

	//send an extra frame just in case, never less than 2 frames
	numFrames++
	if numFrames < 2 {
		numFrames = 2
	}

	//block of code to make sure that we dont ask for more data than we have
	lastFrame := GD_nframes(d.df)
	if firstFrame+numFrames > lastFrame {
		numFrames = lastFrame - firstFrame
	}

	//shoudl figure out the other stuff here like how to compute the number of frames and samples
	backend.Logger.Info(fmt.Sprintf("frames from: %v, num frames: %v", firstFrame, numFrames))

	dataSlice, unixTimeSlice, err := getdata_double(d.df, qm.TimeName, qm.FieldName, firstFrame, numFrames)
	if err != nil {
		response.Error = err
		return response
	}

	spf := GD_spf(d.df, qm.FieldName)
	var decimationFactor int

	maxDataPoints := query.MaxDataPoints // 4 //send 4 times less data than u think u need to

	//do we need to decimate
	if maxDataPoints < int64(len(dataSlice)) {
		decimationFactor = int(math.Ceil(float64(len(dataSlice)) / float64(maxDataPoints)))
		decimationFactor = compatibleDecimationFactor(decimationFactor, spf)
		backend.Logger.Info(fmt.Sprintf("decimation factor: %v", decimationFactor))
		dataSlice = decimate(dataSlice, decimationFactor)
	}
	//we might need to do something with time
	if len(dataSlice) < len(unixTimeSlice) {
		//downsample to match
		unixTimeSlice = decimate(unixTimeSlice, len(unixTimeSlice)/len(dataSlice))
	} else if len(dataSlice) > len(unixTimeSlice) {
		//upsample to match
		unixTimeSlice, err = upsample(unixTimeSlice, len(dataSlice)/len(unixTimeSlice))
		if err != nil {
			backend.Logger.Error(fmt.Sprintf("Error upsampling time: %v", err))
			return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("Error upsampling time: %v", err))
		}
	}

	// decide if we are converting index to time object
	indexConvert := false
	sampleRateSend := 0.0
	if qm.TimeType && qm.TimeName == "INDEX" && qm.IndexByIndex && qm.IndexTimeOffsetType == "fromEndNow" {
		indexConvert = true
		sampleRateSend = qm.SampleRate
	}

	var timeSlice interface{}
	if indexConvert {
		// indexing by index from end now we can convert index into a time object
		timeSlice = indexSlice2TimeSlice(unixTimeSlice, qm.SampleRate, query.TimeRange.To)
	} else if qm.TimeType {
		timeSlice = unixSlice2TimeSlice(unixTimeSlice)
	} else {
		timeSlice = unixTimeSlice
	}

	// create data frame response.
	// For an overview on data frames and how grafana handles them:
	// https://grafana.com/docs/grafana/latest/developers/plugins/data-frames/
	frame := data.NewFrame("response")
	response.Frames = append(response.Frames, frame)

	// add fields and frame to the response. Do this now so the response gets sent back correctly always
	appendString := ""
	if timeAppend != "" {
		appendString = "__" + timeAppend
	}

	frame.Fields = append(frame.Fields,
		data.NewField(qm.TimeName+appendString, nil, timeSlice),
		data.NewField(qm.FieldName, nil, dataSlice),
	)
	// Add the "Channel" field to the frame metadata
	// this should convince grafana to stream
	// pCtx.DataSourceInstanceSettings.UID
	if qm.StreamingBool {
		//turns out the front end is "optimistic" in the interval calculation
		interval := time.Duration(math.Max(float64(query.Interval.Milliseconds()), float64(query.TimeRange.To.UnixMilli()-query.TimeRange.From.UnixMilli())/float64(query.MaxDataPoints)) * 1e6)
		channelName := encodeChan(pCtx.DataSourceInstanceSettings.UID, qm.FieldName, interval.String(), qm.TimeName+appendString, qm.TimeType, sampleRateSend)
		backend.Logger.Info(fmt.Sprintf("Requesting stream on hannel name: %s", channelName))
		frame.Meta = &data.FrameMeta{
			Channel: channelName,
		}
	}

	backend.Logger.Info(fmt.Sprintf("Sending: %v, %v values. For querry %+v", len(unixTimeSlice), len(dataSlice), qm))

	//dataSlice and timeSlice are added to the response by the defer call

	return response
}
