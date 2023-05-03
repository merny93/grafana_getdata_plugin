package plugin

type InitSettings struct {
	DatabaseLocation string `json:"path"` //this specifies how to unmarshal
}

type QueryModel struct {
	FieldName string `json:"fieldName"`
	TimeName  string `json:"timeName"`
}
