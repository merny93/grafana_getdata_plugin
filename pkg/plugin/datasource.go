package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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
	// _ backend.StreamHandler         = (*Datasource)(nil) //streaming implementation
)

type Datasource struct {
	df GD_dirfile //this is a pointer to a dirfile
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
	return &Datasource{df: df}, nil
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
	for _, q := range req.Queries {
		res := d.query(ctx, req.PluginContext, q)

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}

func (d *Datasource) query(_ context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {

	var response backend.DataResponse

	// Unmarshal the JSON into our queryModel.
	var qm QueryModel

	err := json.Unmarshal(query.JSON, &qm)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	backend.Logger.Info(fmt.Sprintf("Interpreted time: %s, and field: %s", qm.TimeName, qm.FieldName))

	//grab the starting time and the end time
	//using the default TIME field here... might be wrong
	firstFrame_float := GD_framenum(d.df, qm.TimeName, float64(query.TimeRange.From.Unix()))
	if firstFrame_float < 0 {
		//if the start frame is negative, set it to 0
		firstFrame_float = 0
	}
	endFrame := GD_framenum(d.df, qm.TimeName, float64(query.TimeRange.To.Unix()))
	if endFrame < 0 {
		//if last requested data is also in the past nothing to do just return the empty response
		return response
	}

	//this is what will be passed to getdata
	numFrames := int(endFrame - firstFrame_float)
	firstFrame := int(firstFrame_float)

	//figure out decimation

	//shoudl figure out the other stuff here like how to compute the number of frames and samples
	backend.Logger.Info(fmt.Sprintf("frames from: %v, %v", firstFrame, numFrames))

	//grab the data
	dataSlice := GD_getdata(qm.FieldName, d.df, int(firstFrame), 0, numFrames, 0)
	unixTimeSlice := GD_getdata(qm.TimeName, d.df, int(firstFrame), 0, numFrames, 0)

	//the excess sampleRate is just the ratio of extra time stamps
	sampleRate := int(len(unixTimeSlice) / len(dataSlice))

	var decimationFactor int = 1
	var resultSize int = len(dataSlice)
	//decimate the data
	if query.MaxDataPoints < int64(len(dataSlice)) {
		decimationFactor = int(math.Ceil(float64(len(dataSlice)) / float64(query.MaxDataPoints)))
		resultSize = int(len(dataSlice) / int(decimationFactor))
		dataSlice_tmp := make([]float64, resultSize)

		for i := 0; i < resultSize*decimationFactor; i++ {
			if i%int(decimationFactor) == 0 {
				dataSlice_tmp[int(i/int(decimationFactor))] = dataSlice[i]
			}
		}
		dataSlice = dataSlice_tmp
	}
	if len(dataSlice) < len(unixTimeSlice) {
		decimationFactor = decimationFactor * sampleRate
		unixTimeSlice_tmp := make([]float64, resultSize)

		for i := 0; i < resultSize*decimationFactor; i++ {
			if i%int(decimationFactor) == 0 {
				unixTimeSlice_tmp[int(i/int(decimationFactor))] = unixTimeSlice[i]
			}
		}
		unixTimeSlice = unixTimeSlice_tmp
	}

	//create the time slice which will hold proper time objects
	timeSlice := make([]time.Time, len(unixTimeSlice))

	//loop through the ctimes and turn them into time objects
	for i, c_time := range unixTimeSlice {
		timeSlice[i] = time.Unix(int64(c_time), int64(math.Mod(c_time, 1)/1e9))
	}

	// create data frame response.
	// For an overview on data frames and how grafana handles them:
	// https://grafana.com/docs/grafana/latest/developers/plugins/data-frames/
	frame := data.NewFrame("response")

	// add fields.
	frame.Fields = append(frame.Fields,
		data.NewField("time", nil, timeSlice),
		data.NewField("values", nil, dataSlice),
	)

	// add the frames to the response.
	response.Frames = append(response.Frames, frame)

	return response
}

/// stubs for streaming implementation *****************************************

// func (d *Datasource) SubscribeStream(ctx context.Context, request *backend.SubscribeStreamRequest) (*backend.SubscribeStreamResponse, error) {
// 	// Implement subscription logic here
// 	return nil, nil
// }

// func (d *Datasource) RunStream(ctx context.Context, request *backend.RunStreamRequest, sender *backend.StreamSender) error {
// 	// Implement data retrieval and streaming logic here
// 	return nil

// }

// func (d *Datasource) PublishStream(ctx context.Context, request *backend.PublishStreamRequest) (*backend.PublishStreamResponse, error) {
// 	// Implement data publishing logic here
// 	return nil, nil
// }

/// ***************************************************************************

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (d *Datasource) CheckHealth(_ context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	//lets try reading the time field as it should always be there

	res := GD_getdata("TIME", d.df, 0, 0, 0, 1)

	var status = backend.HealthStatusOk
	var message = "Data source is working"
	if res == nil {
		status = backend.HealthStatusError
		message = "Was not able to find the TIME field in specified dirfile location"
	}

	return &backend.CheckHealthResult{
		Status:  status,
		Message: message,
	}, nil
}
