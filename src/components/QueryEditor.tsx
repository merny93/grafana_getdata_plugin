import React, { ChangeEvent } from 'react';
import { InlineField, Input } from '@grafana/ui';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from '../datasource';
import { MyDataSourceOptions, MyQuery } from '../types';

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

export function QueryEditor({ query, onChange, onRunQuery }: Props) {

  const onFieldNameChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...query, fieldName: event.target.value });
    // do not execute
    // onRunQuery();
  };

  const onTimeFieldChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...query, timeName: event.target.value });
    // do not execute
    // onRunQuery();
  };

  

  const { fieldName, timeName} = query;

  return (
    <div className="gf-form">
      <InlineField label="Field Name" labelWidth={16} tooltip="Name of field">
        <Input onChange={onFieldNameChange} value={fieldName || ''} />
      </InlineField>
      <InlineField label="Time Field" labelWidth={16} tooltip="Name of field to interpret as time">
        <Input onChange={onTimeFieldChange} value={timeName || ''} />
      </InlineField>
    </div>
  );
}
