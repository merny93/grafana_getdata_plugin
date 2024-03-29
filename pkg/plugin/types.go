package plugin

import "time"

type InitSettings struct {
	DatabaseLocation string `json:"path"` //this specifies how to unmarshal
}

type QueryModel struct {
	FieldName           string  `json:"fieldName"`
	TimeName            string  `json:"timeName"`
	StreamingBool       bool    `json:"streamingBool"`
	IndexTimeOffsetType string  `json:"indexTimeOffsetType"`
	IndexTimeOffset     int64   `json:"indexTimeOffset"`
	SampleRate          float64 `json:"sampleRate"`
	IndexByIndex        bool    `json:"indexByIndex"`
	TimeType            bool    `json:"timeType"`
}

type AutocompleteRequest struct {
	RegexString string `json:"regexString"`
}

type AutocompleteResponse struct {
	MatchList []string
}

type StreamRequest struct {
	fieldName     string
	timeNameField string
	timeName      string
	interval      time.Duration
	timeType      bool
	sampleRate    float64
}
