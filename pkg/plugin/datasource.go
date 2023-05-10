package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
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
	for _, q := range req.Queries {
		res := d.query(ctx, req.PluginContext, q)

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}

func (d *Datasource) query(ctx context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {

	var response backend.DataResponse

	// Unmarshal the JSON into our queryModel.
	var qm QueryModel

	err := json.Unmarshal(query.JSON, &qm)
	if err != nil {
		//if it fails we really cant do much
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	// create data frame response.
	// For an overview on data frames and how grafana handles them:
	// https://grafana.com/docs/grafana/latest/developers/plugins/data-frames/
	frame := data.NewFrame("response")
	response.Frames = append(response.Frames, frame)

	//create the arrays which we will return eventually
	var timeSlice []time.Time
	var dataSlice []float64

	// add fields and frame to the response. Do this now so the response gets sent back correctly always
	defer func() {
		frame.Fields = append(frame.Fields,
			data.NewField("time", nil, timeSlice),
			data.NewField(qm.FieldName, nil, dataSlice),
		)
		// Add the "Channel" field to the frame metadata
		// this should convince grafana to stream
		// pCtx.DataSourceInstanceSettings.UID
		if qm.StreamingBool {
			channalName := "ds/" + pCtx.DataSourceInstanceSettings.UID + "/stream/" + qm.FieldName + "/" + query.Interval.String()
			backend.Logger.Info(fmt.Sprintf("Creating a steram on %s", channalName))
			frame.Meta = &data.FrameMeta{
				// Channel: "ds/fcfd8594-00f2-4cdb-8519-7ab60b5403b7/stream",
				// Channel: "ds/simon-myplugin-datasource/stream",
				Channel: channalName,
			}
		}
	}()

	backend.Logger.Info(fmt.Sprintf("Interpreted time: %s, and field: %s. Will initiate streaming: %t\nGoing from: %s to: %s", qm.TimeName, qm.FieldName, qm.StreamingBool, query.TimeRange.From.Format(time.ANSIC), query.TimeRange.To.Format(time.ANSIC)))

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

	//shoudl figure out the other stuff here like how to compute the number of frames and samples
	backend.Logger.Info(fmt.Sprintf("frames from: %v, %v", firstFrame, numFrames))

	//grab the data and error check
	dataSlice = GD_getdata(qm.FieldName, d.df, int(firstFrame), 0, numFrames, 0)
	errStr := GD_error(d.df)
	if errStr != "" {
		backend.Logger.Error(fmt.Sprintf("getdata error: %s", errStr))
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("getdata error: %s", errStr))
	}
	unixTimeSlice := GD_getdata(qm.TimeName, d.df, int(firstFrame), 0, numFrames, 0)
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

	//decimate the data
	//first decimate the data and then decimate time to match, sometimes time is sampled at higher freqeuncy so use sample rate
	var decimationFactor int = 1
	//the excess sampleRate is just the ratio of extra time stamps
	sampleRate := int(len(unixTimeSlice) / len(dataSlice))
	if query.MaxDataPoints < int64(len(dataSlice)) {
		decimationFactor = int(math.Ceil(float64(len(dataSlice)) / float64(query.MaxDataPoints)))
		dataSlice = decimate(dataSlice, decimationFactor)
	}
	if len(dataSlice) < len(unixTimeSlice) {
		unixTimeSlice = decimate(unixTimeSlice, decimationFactor*sampleRate)
	}

	timeSlice = unixSlice2TimeSlice(unixTimeSlice)

	backend.Logger.Info(fmt.Sprintf("Sending: %v values", len(timeSlice)))

	//dataSlice and timeSlice are added to the response by the defer call. This means that

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
	interval, err := time.ParseDuration(chunks[2])
	if err != nil {
		return err
	}

	backend.Logger.Info(fmt.Sprintf("FROM INSIDE THE STREAM and field: %s", fieldName))
	defer backend.Logger.Info(fmt.Sprintf("THE RUNSTREAM IS TERMINATED for endpoint: %s", request.Path))

	ticker := time.NewTicker(interval)
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
				dataSlice := GD_getdata(fieldName, d.df, newFrame-1, 0, 0, 1)
				errStr := GD_error(d.df)
				if errStr != "" {
					backend.Logger.Error(fmt.Sprintf("getdata error: %s", errStr))
					continue
				}
				unixTimeSlice := GD_getdata("TIME", d.df, newFrame-1, 0, 0, 1) //todo make time name a variable
				errStr = GD_error(d.df)
				if errStr != "" {
					backend.Logger.Error(fmt.Sprintf("getdata error: %s", errStr))
					continue
				}
				//if there was no error but there was also no data
				if dataSlice == nil || unixTimeSlice == nil {
					backend.Logger.Info("No data in selected time range")
					continue
				}

				timeSlice := unixSlice2TimeSlice(unixTimeSlice)

				frame := data.NewFrame("response")
				frame.Fields = append(frame.Fields,
					data.NewField("time", nil, timeSlice),
					data.NewField(fieldName, nil, dataSlice),
				)

				sender.SendFrame(frame, data.IncludeAll)

				//update the last frame
				d.lastFrame[request.Path] = newFrame

				//debuggg
				backend.Logger.Info(fmt.Sprintf("Sent frame on endpoint: %s", request.Path))
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

	res := GD_getdata("INDEX", d.df, 0, 0, 0, 1)
	errStr := GD_error(d.df)
	if errStr != "" || res == nil {
		status = backend.HealthStatusError
		message = fmt.Sprintf("getdata error: %s", errStr)
	}

	return &backend.CheckHealthResult{
		Status:  status,
		Message: message,
	}, nil
}
