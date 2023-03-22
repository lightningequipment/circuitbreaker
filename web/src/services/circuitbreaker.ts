import axios from 'axios';

const circuitbreakerApi = axios.create({
  baseURL: '/api',
});

export const getInfo = async () =>
  (await circuitbreakerApi.get<Info>('/info')).data;

export const getLimits = async () =>
  (await circuitbreakerApi.get<Limits>('/limits')).data;

interface UpdateLimitsParams {
  limits: {
    [key: string]: Limit;
  };
}
export const updateLimits = async (params: UpdateLimitsParams) =>
  (await circuitbreakerApi.post<{}>('/updatelimits', params)).data;

interface ClearLimitsParams {
  nodes: string[];
}
export const clearLimits = async (params: ClearLimitsParams) =>
  (await circuitbreakerApi.post<{}>('/clearlimits', params)).data;

interface UpdateDefaultLimitParams {
  limit: Limit;
}
export const updateDefaultLimit = async (params: UpdateDefaultLimitParams) =>
  (await circuitbreakerApi.post<{}>('/updatedefaultlimit', params)).data;
