import React, {useState } from 'react';
import {InlineFormLabel, AsyncSelect, LoadOptionsCallback, Checkbox, Select, VerticalGroup, HorizontalGroup, DateTimePicker} from '@grafana/ui';
import { QueryEditorProps, SelectableValue, dateTime } from '@grafana/data';
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

  const indexTimeOffsetMapping: any = {
    "fromStart" : "From start",
    "fromEnd" : "From end",
    "fromEndNow" : "From end now"
  }

  const [timeName, setTimeName] = useState<SelectableValue<string>>({label: props.query.timeName, value: props.query.timeName});
  const [fieldName, setFieldName] = useState<SelectableValue<string>>({label: props.query.fieldName, value: props.query.fieldName});
  const [streamingBool, setStreamingBool] = useState<boolean>(props.query.streamingBool);
  const [indexTimeOffsetType, setIndexTimeOffsetType] = useState<SelectableValue<string>>({label: indexTimeOffsetMapping[props.query.indexTimeOffsetType], value: props.query.indexTimeOffsetType});
  const [indexByIndex, setIndexByIndex] = useState<boolean>(props.query.indexByIndex);
  const [indexTimeOffset, setIndexTimeOffset] = useState<number>(props.query.indexTimeOffset);
  return (
    <div className="gf-form">
      <VerticalGroup>
        <HorizontalGroup>
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
          label="Streaming" description="Enable streaming mode"
        />
          </HorizontalGroup>
          <HorizontalGroup>

          <Checkbox value={indexByIndex} onChange={(e) => 
          {e.currentTarget.checked ? setIndexByIndex(true) : setIndexByIndex(false); 
          props.onChange({ ...props.query, indexByIndex: e.currentTarget.checked });
          }} 
          label="INDEX" description="Index time by INDEX"
        />

      <InlineFormLabel width={12} tooltip="">
          Index time offset type
        </InlineFormLabel>
          <Select
            options={[
              {label: "From start", value: "fromStart"},
              {label: "From end", value: "fromEnd" },
              {label: "From end now", value: "fromEndNow" }
            ]}
            placeholder='How to offset the index time'
            value={indexTimeOffsetType}
            onChange={(v: SelectableValue) => {
              setIndexTimeOffsetType(v);
              props.onChange({ ...props.query, indexTimeOffsetType: v.value });
            }}
          />
      <InlineFormLabel width={12} tooltip="">
        sample rate
      </InlineFormLabel>
      <input
        type="number"
        value={props.query.sampleRate}
        onChange={(e) => {
          props.onChange({ ...props.query, sampleRate: parseFloat(e.currentTarget.value) });
        }}
      />
      {/* <InlineFormLabel width={12} tooltip="">
        Index time offset
      </InlineFormLabel> */}
      <DateTimePicker
      label="Index time offset"
      // minDate={minDateVal}
      // maxDate={maxDateVal}
      date={dateTime(indexTimeOffset*1000)}
      // showSeconds={true}
      onChange={(newValue) => {
        setIndexTimeOffset(newValue.unix());
        props.onChange({ ...props.query, indexTimeOffset: newValue.unix() });
      }}
    />
      </HorizontalGroup>
      </VerticalGroup>
      </div>
  );
}
