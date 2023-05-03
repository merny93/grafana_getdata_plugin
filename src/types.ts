import { DataQuery, DataSourceJsonData } from '@grafana/data';

export interface MyQuery extends DataQuery {
  fieldName: string;
  startIndex: number;
  frameNumber: number;
}

export const DEFAULT_QUERY: Partial<MyQuery> = {
  fieldName: "TIME",
  startIndex: 0,
  frameNumber: 1,
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
