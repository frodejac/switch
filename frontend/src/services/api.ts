import axios from 'axios';
import { FeatureFlag, FlagContext, FlagResponse } from '../types/flag';
import { config } from '../config';

const api = axios.create({
  baseURL: config.apiBaseUrl,
  withCredentials: false, // Set to true if you need to send cookies
  headers: {
    'Content-Type': 'application/json',
    'Accept': 'application/json',
  },
});

// Add a response interceptor to handle CORS errors
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response) {
      // The request was made and the server responded with a status code
      // that falls out of the range of 2xx
      console.error('API Error:', error.response.status, error.response.data);
    } else if (error.request) {
      // The request was made but no response was received
      console.error('Network Error:', error.request);
    } else {
      // Something happened in setting up the request that triggered an Error
      console.error('Error:', error.message);
    }
    return Promise.reject(error);
  }
);

export const getFlag = async (store: string, key: string, context?: FlagContext): Promise<FlagResponse> => {
  const params = new URLSearchParams();
  if (context) {
    Object.entries(context).forEach(([k, v]) => {
      params.append(k, String(v));
    });
  }
  
  const response = await api.get(`/${store}/${key}`, { params });
  return response.data;
};

export const setFlag = async (store: string, key: string, flag: Partial<FeatureFlag>): Promise<void> => {
  await api.put(`/${store}/${key}`, flag);
};

export const listFlags = async (store: string): Promise<FeatureFlag[]> => {
  const response = await api.get(`/${store}`);
  // Transform the map response into an array of flags
  const flagsMap = response.data;
  return Object.entries(flagsMap)
    .filter(([fullKey]) => {
      // Extract just the key part (after the store name)
      const key = fullKey.split('/')[1];
      return !key.startsWith('__');
    })
    .map(([fullKey, value]) => {
      const key = fullKey.split('/')[1];
      return {
        key,
        ...value as Omit<FeatureFlag, 'key'>
      };
    });
};

export const listStores = async (): Promise<string[]> => {
  const response = await api.get('/stores');
  return response.data;
}; 