import { DataQuery, DataSourceJsonData } from '@grafana/data';

export interface MyQuery extends DataQuery {
  fieldName: string;
  timeName: string;
  indexByIndex: boolean;
  streamingBool: boolean;
  indexTimeOffsetType: string;
  indexTimeOffset: number;
  sampleRate: number;
  timeType: boolean;
}

export const DEFAULT_QUERY: Partial<MyQuery> = {
  timeName: "TIME",
  streamingBool: false,
  indexTimeOffsetType: "fromEndNow",
  sampleRate: 6,
  indexTimeOffset: new Date().getUTCSeconds(),
  indexByIndex: false,
  timeType: true,
};

/**
 * These are options configured for each DataSource instance
 */
export interface MyDataSourceOptions extends DataSourceJsonData {
  path?: string;
}

/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
export interface MySecureJsonData {
  apiKey?: string;
}
