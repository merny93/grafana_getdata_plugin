import React, {useState } from 'react';
import {InlineFormLabel, AsyncSelect, LoadOptionsCallback, Checkbox} from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { DataSource } from '../datasource';
import { MyDataSourceOptions, MyQuery } from '../types';
type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

export function QueryEditor(props: Props) {

  const onFieldNameChange = (v: SelectableValue) => {
    props.onChange({ ...props.query, fieldName: v.value });
    console.log(v.value)
    setFieldName(v);
    props.onRunQuery();
  };

  const onTimeFieldChange = (v: SelectableValue) => {
    props.onChange({ ...props.query, timeName: v.value });
    console.log(v.value)
    setTimeName(v);
    props.onRunQuery();
  };


  const handleOptionFetch = async (value: string, cb?: LoadOptionsCallback<string>): Promise<Array<SelectableValue<string>>> => {
    //function checks with the backend to find all the regex matches of the fields in the open dirfile

    // get the options from the datasrouce backend. Nobody is checking on the autocoplete bit
    // will return an object which contains MatchList cointinaing all the regex matches
    const response = await props.datasource.postResource("autocomplete",{regexString: value});

    //response.MatchList is an array of strings, convert it to an array which has the same size duplicating the values and adding a label with the same value
    const options = response.MatchList.map((value: string) => {
      return {label: value, value: value}
    })
    return options;
  };

  const [timeName, setTimeName] = useState<SelectableValue<string>>({label: props.query.timeName, value: props.query.timeName});
  const [fieldName, setFieldName] = useState<SelectableValue<string>>({label: props.query.fieldName, value: props.query.fieldName});
  const [streamingBool, setStreamingBool] = useState<boolean>(props.query.streamingBool);

  return (
    <div className="gf-form">
      <InlineFormLabel width={7} tooltip="Enter field name">
          Field Name
        </InlineFormLabel>
        <AsyncSelect
          loadOptions={handleOptionFetch}
          defaultOptions
          value={fieldName}
          onChange={onFieldNameChange}
          allowCreateWhileLoading
          openMenuOnFocus
        />
      <InlineFormLabel width={12} tooltip="Enter time field name">
          Time Field Name
        </InlineFormLabel>
        <AsyncSelect
          loadOptions={handleOptionFetch}
          defaultOptions
          value={timeName}
          onChange={onTimeFieldChange}
          allowCreateWhileLoading
          openMenuOnFocus
        />
        <Checkbox value={streamingBool} onChange={(e) => 
          {e.currentTarget.checked ? setStreamingBool(true) : setStreamingBool(false); 
          props.onChange({ ...props.query, streamingBool: e.currentTarget.checked });
          }} 
          label="Streaming" description="Enable streaming mode"/>
    </div>
  );
}
