package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// Make sure Datasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. In this example datasource instance implements backend.QueryDataHandler,
// backend.CheckHealthHandler interfaces. Plugin should not implement all these
// interfaces- only those which are required for a particular task.
var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
	_ backend.StreamHandler         = (*Datasource)(nil) //streaming implementation
)

type Datasource struct {
	df        Dirfile
	lastFrame map[string]int
}

// NewDatasource creates a new datasource instance.
func NewDatasource(settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {

	// chat gpt thinks this will get the database location from the settings

	var params InitSettings
	err := json.Unmarshal(settings.JSONData, &params)

	if err != nil {
		return nil, err
	}
	backend.Logger.Info("Attempting to open database located at: " + fmt.Sprint(params.DatabaseLocation))
	df := GD_open(params.DatabaseLocation)
	return &Datasource{df: df, lastFrame: make(map[string]int)}, nil
}

// Datasource is an example datasource which can respond to data queries, reports
// its health and has streaming skills.

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as datasource settings change detected by SDK old datasource instance will
// be disposed and a new one will be created using NewSampleDatasource factory function.
func (d *Datasource) Dispose() {
	// Clean up datasource instance resources.

	//close the dirfile, probably a good idea
	GD_close(d.df)
}

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

	var response backend.DataResponse

	// Unmarshal the JSON into our queryModel.
	var qm QueryModel

	err := json.Unmarshal(query.JSON, &qm)
	if err != nil {
		//if it fails we really cant do much
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	backend.Logger.Info(fmt.Sprintf("QueryModel: %+v\n", qm))

	// create data frame response.
	// For an overview on data frames and how grafana handles them:
	// https://grafana.com/docs/grafana/latest/developers/plugins/data-frames/
	frame := data.NewFrame("response")
	response.Frames = append(response.Frames, frame)

	//create the arrays which we will return eventually}
	var timeSlice interface{}
	var dataSlice []float64

	// add fields and frame to the response. Do this now so the response gets sent back correctly always
	appendString := ""
	if timeAppend != "" {
		appendString = "__" + timeAppend
	}
	defer func() {
		if dataSlice == nil || timeSlice == nil {
			dataSlice = make([]float64, 0)
			if qm.TimeType {
				timeSlice = make([]time.Time, 0)
			} else {
				timeSlice = make([]float64, 0)
			}
		}
		frame.Fields = append(frame.Fields,
			data.NewField(qm.TimeName+appendString, nil, timeSlice),
			data.NewField(qm.FieldName, nil, dataSlice),
		)
		// Add the "Channel" field to the frame metadata
		// this should convince grafana to stream
		// pCtx.DataSourceInstanceSettings.UID
		if qm.StreamingBool {
			channalName := "ds/" + pCtx.DataSourceInstanceSettings.UID + "/stream/" + qm.FieldName + "/" + query.Interval.String() + "/" + qm.TimeName + appendString + "/" + fmt.Sprintf("%t", qm.TimeType)
			frame.Meta = &data.FrameMeta{
				// Channel: "ds/fcfd8594-00f2-4cdb-8519-7ab60b5403b7/stream",
				// Channel: "ds/simon-myplugin-datasource/stream",
				Channel: channalName,
			}
		}
	}()

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

		backend.Logger.Info(fmt.Sprintf("first frame: %v, end frame: %v", firstFrame_float, endFrame))
		backend.Logger.Info(fmt.Sprintf("time from: %v, time to: %v", timeFrom, timeTo))

		//get data does not like negative frame numbers
		if firstFrame_float < 0 {
			firstFrame_float = 0
		}

		numFrames = int(endFrame - firstFrame_float)
		firstFrame = int(firstFrame_float)
	}

	numFrames++ //send an extra frame just in case
	if numFrames < 2 {
		numFrames = 2
	}

	//shoudl figure out the other stuff here like how to compute the number of frames and samples
	backend.Logger.Info(fmt.Sprintf("frames from: %v, num frames: %v", firstFrame, numFrames))

	//grab the data and error check
	dataSlice = GD_getdata(qm.FieldName, d.df, int(firstFrame), numFrames)
	errStr := GD_error(d.df)
	if errStr != "" {
		backend.Logger.Error(fmt.Sprintf("getdata error: %s", errStr))
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("getdata error: %s", errStr))
	}
	unixTimeSlice := GD_getdata(qm.TimeName, d.df, int(firstFrame), numFrames)
	errStr = GD_error(d.df)
	if errStr != "" {
		backend.Logger.Error(fmt.Sprintf("getdata error: %s", errStr))
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("getdata error: %s", errStr))
	}
	//if there was no error but there was also no data
	if dataSlice == nil || unixTimeSlice == nil {
		backend.Logger.Info("No data in selected time range")
		return response
	}

	backend.Logger.Info(fmt.Sprintf("FIRST len data: %v, len time: %v", len(dataSlice), len(unixTimeSlice)))

	//decimate the data
	// currently assuming that the time field has 1 sample per frame
	//will decimate the data field by either a divisor or a multiple of samples per frame
	// this means that the upsample for the time field will be clean and will work properly

	spf := GD_spf(d.df, qm.FieldName)
	var decimationFactor int

	maxDataPoints := query.MaxDataPoints // 4 //send 4 times less data than u think u need to

	if maxDataPoints < int64(len(dataSlice)) {
		backend.Logger.Info("Decimating data")
		backend.Logger.Info(fmt.Sprintf("len data: %v, len time: %v", len(dataSlice), len(unixTimeSlice)))
		//decimate the data by a factor which is either a divisor or a multiple of spf
		decimationFactor = int(math.Ceil(float64(len(dataSlice)) / float64(maxDataPoints)))
		decimationFactor = compatibleDecimationFactor(decimationFactor, spf)
		backend.Logger.Info(fmt.Sprintf("decimation factor: %v", decimationFactor))
		dataSlice = decimate(dataSlice, decimationFactor)
	}
	if len(dataSlice) < len(unixTimeSlice) {
		backend.Logger.Info("decimating time")
		backend.Logger.Info(fmt.Sprintf("len data: %v, len time: %v", len(dataSlice), len(unixTimeSlice)))
		unixTimeSlice = decimate(unixTimeSlice, len(unixTimeSlice)/len(dataSlice))
	} else if len(dataSlice) > len(unixTimeSlice) {
		backend.Logger.Info("upsampling time")
		backend.Logger.Info(fmt.Sprintf("len data: %v, len time: %v", len(dataSlice), len(unixTimeSlice)))
		unixTimeSlice = upsample(unixTimeSlice, len(dataSlice)/len(unixTimeSlice))
		backend.Logger.Info("made it throught")
	}
	if qm.TimeType {
		timeSlice = unixSlice2TimeSlice(unixTimeSlice)
	} else {
		timeSlice = unixTimeSlice
	}
	backend.Logger.Info(fmt.Sprintf("Sending: %v values", len(dataSlice)))

	//dataSlice and timeSlice are added to the response by the defer call

	return response
}

/// stubs for streaming implementation *****************************************

func (d *Datasource) SubscribeStream(ctx context.Context, request *backend.SubscribeStreamRequest) (*backend.SubscribeStreamResponse, error) {
	// Implement subscription logic here
	backend.Logger.Info("SubscribeStream called")
	status := backend.SubscribeStreamStatusOK

	//write down the last frame
	d.lastFrame[request.Path] = GD_nframes(d.df) - 1

	return &backend.SubscribeStreamResponse{Status: status}, nil
}

func (d *Datasource) RunStream(ctx context.Context, request *backend.RunStreamRequest, sender *backend.StreamSender) error {
	// request.Data
	// Implement data retrieval and streaming logic here
	backend.Logger.Info("RunStream called")

	// get the name of the name of the field, held in path as stream/NameOfField
	chunks := strings.Split(request.Path, "/")
	fieldName := chunks[1]
	timeNameField := chunks[3]
	timeName := strings.Split(timeNameField, "__")[0]
	interval, err := time.ParseDuration(chunks[2])
	if err != nil {
		return err
	}
	timeType, err := strconv.ParseBool(chunks[4])
	if err != nil {
		return err
	}

	backend.Logger.Info(fmt.Sprintf("FROM INSIDE THE STREAM and field: %s", fieldName))
	defer backend.Logger.Info(fmt.Sprintf("Look for a context done above for endpoint: %s", request.Path))
	tickerInterval := time.Duration(interval)

	//limit the ticker interval to n second, right now set it to 3 cause why not
	if tickerInterval < 1*time.Second {
		tickerInterval = 3 * time.Second
	}
	ticker := time.NewTicker(tickerInterval)

	var newFrame int
	for {
		select {
		case <-ctx.Done():
			backend.Logger.Info("Context done")
			ticker.Stop()
			return nil
		case <-ticker.C:
			newFrame = GD_nframes(d.df)
			if newFrame > d.lastFrame[request.Path] {
				//new data
				//grab the data and error check
				dataSlice := GD_getdata(fieldName, d.df, d.lastFrame[request.Path], newFrame-d.lastFrame[request.Path])
				errStr := GD_error(d.df)
				if errStr != "" {
					backend.Logger.Error(fmt.Sprintf("getdata error: %s", errStr))
					continue
				}

				unixTimeSlice := GD_getdata(timeName, d.df, d.lastFrame[request.Path], newFrame-d.lastFrame[request.Path])
				errStr = GD_error(d.df)
				if errStr != "" {
					backend.Logger.Error(fmt.Sprintf("getdata error: %s", errStr))
					continue
				}

				//if there was no error but there was also no data, dunoo how this would happen
				if dataSlice == nil || unixTimeSlice == nil {
					backend.Logger.Info("No new data, odd")
					continue
				}

				//decimate the data
				// currently assuming that the time field has 1 sample per frame
				if len(dataSlice) > 1 {
					// check what the interval is
					// if it is less than the interval of the stream, then we need to decimate

					spf := GD_spf(d.df, fieldName)

					dataInterval := tickerInterval.Seconds() / float64(len(dataSlice))
					// dataInterval = dataInterval / 4 //just to be safe
					if dataInterval < interval.Seconds() {
						//decimate the data by a factor which is either a divisor or a multiple of spf
						decimationFactor := int(math.Ceil(interval.Seconds() / dataInterval))
						backend.Logger.Info(fmt.Sprintf("inetrvals: %v, %v", interval.Seconds(), dataInterval))
						decimationFactor = compatibleDecimationFactor(decimationFactor, spf)
						//if the decimation factor is larger than the data slice, then just send one value
						if decimationFactor > len(dataSlice) {
							decimationFactor = len(dataSlice)
						}
						backend.Logger.Info(fmt.Sprintf("decimation factor in stream of data: %v", decimationFactor))
						dataSlice = decimate(dataSlice, decimationFactor)
					}
					if len(dataSlice) < len(unixTimeSlice) {
						backend.Logger.Info(fmt.Sprintf("decimating time in stream factor: %v", len(unixTimeSlice)/len(dataSlice)))
						unixTimeSlice = decimate(unixTimeSlice, len(unixTimeSlice)/len(dataSlice))
					} else if len(dataSlice) > len(unixTimeSlice) {
						backend.Logger.Info(fmt.Sprintf("upsampling time in stream factor: %v", len(dataSlice)/len(unixTimeSlice)))
						if len(unixTimeSlice) == 1 {
							//hard to upsample with just one data point, lets grab another one from the past
							unixTimeSlice = GD_getdata(timeName, d.df, newFrame-2, 2)
						}
						unixTimeSlice = upsample(unixTimeSlice, len(dataSlice)/len(unixTimeSlice))
					}
				}
				var timeSlice interface{}
				if timeType {
					timeSlice = unixSlice2TimeSlice(unixTimeSlice)
				} else {
					timeSlice = unixTimeSlice
				}

				frame := data.NewFrame("response")
				frame.Fields = append(frame.Fields,
					data.NewField(timeNameField, nil, timeSlice),
					data.NewField(fieldName, nil, dataSlice),
				)

				sender.SendFrame(frame, data.IncludeAll)

				//update the last frame
				d.lastFrame[request.Path] = newFrame

				//debuggg
				backend.Logger.Info(fmt.Sprintf("Sent frame on endpoint: %s with %v values", request.Path, len(dataSlice)))
			}
		}
	}
}

func (d *Datasource) PublishStream(ctx context.Context, request *backend.PublishStreamRequest) (*backend.PublishStreamResponse, error) {
	// Implement data publishing logic here
	backend.Logger.Info("PublishStream called")
	status := backend.PublishStreamStatusPermissionDenied
	return &backend.PublishStreamResponse{Status: status}, nil
}

/// ***************************************************************************

/// Resrouce handler which serves the autocomplete endpoint. will autocomplete queries ************************

func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	var reqGo AutocompleteRequest

	json.Unmarshal(req.Body, &reqGo)

	backend.Logger.Info(fmt.Sprintf("interpreted %s", reqGo.RegexString))

	matchList := GD_match_entries(d.df, reqGo.RegexString)

	response := AutocompleteResponse{MatchList: matchList}
	responseBytes, _ := json.Marshal(response)

	return sender.Send(&backend.CallResourceResponse{
		Status: http.StatusOK,
		Body:   responseBytes,
	})
}

/// ***************************************************************************

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (d *Datasource) CheckHealth(_ context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	//lets try reading the time field as it should always be there

	var status = backend.HealthStatusOk
	var message = "Data source is working"
	dummyArray := make([]float64, 1)
	res := GD_getdata_c("INDEX", d.df, 0, 0, 0, 1, dummyArray)
	errStr := GD_error(d.df)
	if errStr != "" || res == 0 {
		status = backend.HealthStatusError
		message = fmt.Sprintf("getdata error: %s", errStr)
	}

	return &backend.CheckHealthResult{
		Status:  status,
		Message: message,
	}, nil
}
