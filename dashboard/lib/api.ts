import axios from 'axios';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.codereview.ai';

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Add interceptor for Auth
api.interceptors.request.use((config) => {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : null;
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

export const orgApi = {
  getOrgs: () => api.get('/orgs'),
  getOrg: (id: string) => api.get(`/orgs/${id}`),
  createOrg: (data: any) => api.post('/orgs', data),
  getRepos: (orgId: string) => api.get(`/orgs/${orgId}/repos`),
  addRepo: (orgId: string, data: any) => api.post(`/orgs/${orgId}/repos`, data),
  updateSubscription: (orgId: string, data: any) => api.post(`/orgs/${orgId}/subscription`, data),
};

export default api;