export interface FeatureFlag {
  key: string;
  value: any;
  expression?: string;
  store: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface FlagContext {
  [key: string]: string | number | boolean;
}

export interface FlagResponse {
  value: any;
  evaluated: boolean;
} 