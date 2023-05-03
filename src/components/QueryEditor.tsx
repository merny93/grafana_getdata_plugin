import React, { ChangeEvent } from 'react';
import { InlineField, Input } from '@grafana/ui';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from '../datasource';
import { MyDataSourceOptions, MyQuery } from '../types';

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

export function QueryEditor({ query, onChange, onRunQuery }: Props) {
  const onStartIndexChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...query, startIndex: parseInt(event.target.value) });
    // executes the query
    onRunQuery();
  };

  const onFieldNameChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...query, fieldName: event.target.value });
    // do not execute
    // onRunQuery();
  };

  const onFrameNumberChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...query, frameNumber: parseInt(event.target.value) });
    // executes the query
    onRunQuery();
  };
  

  const { fieldName, startIndex,frameNumber } = query;

  return (
    <div className="gf-form">
      <InlineField label="Field Name" labelWidth={16} tooltip="Name of field">
        <Input onChange={onFieldNameChange} value={fieldName || ''} />
      </InlineField>
      <InlineField label="Start Index">
        <Input onChange={onStartIndexChange} value={startIndex} width={8} type="number" step="1" />
      </InlineField>
      <InlineField label="number of frames">
        <Input onChange={onFrameNumberChange} value={frameNumber} width={8} type="number" step="1" />
      </InlineField>
    </div>
  );
}
