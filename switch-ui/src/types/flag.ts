export enum FeatureFlagType {
    BOOLEAN = 'boolean',
    STRING = 'string',
    INT = 'int',
    FLOAT = 'float',
    JSON = 'json',
    CEL = 'cel',
}

export interface FeatureFlag {
  key: string;
  type: FeatureFlagType;
  value: unknown;
  store: string;
}

export interface FlagContext {
  [key: string]: string | number | boolean;
}

export interface FlagResponse {
  value: unknown;
}