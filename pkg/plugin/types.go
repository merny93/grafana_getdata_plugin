package plugin

type InitSettings struct {
	DatabaseLocation string `json:"path"` //this specifies how to unmarshal
}

type QueryModel struct {
	FieldName     string `json:"fieldName"`
	TimeName      string `json:"timeName"`
	StreamingBool bool   `json:"streamingBool"`
}

type AutocompleteRequest struct {
	RegexString string `json:"regexString"`
}

type AutocompleteResponse struct {
	MatchList []string
}
