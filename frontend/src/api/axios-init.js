import axios from 'axios';
import qs from 'qs';

// Apply the panel's axios defaults + interceptors. Call once at app
// startup before any HTTP call goes out.
export function setupAxios() {
  axios.defaults.headers.post['Content-Type'] = 'application/x-www-form-urlencoded; charset=UTF-8';
  axios.defaults.headers.common['X-Requested-With'] = 'XMLHttpRequest';

  axios.interceptors.request.use(
    (config) => {
      config.headers = config.headers || {};
      const csrfToken = document.querySelector('meta[name="csrf-token"]')?.getAttribute('content');
      const method = (config.method || 'get').toUpperCase();
      if (csrfToken && !['GET', 'HEAD', 'OPTIONS', 'TRACE'].includes(method)) {
        config.headers['X-CSRF-Token'] = csrfToken;
      }
      if (config.data instanceof FormData) {
        config.headers['Content-Type'] = 'multipart/form-data';
      } else {
        config.data = qs.stringify(config.data, { arrayFormat: 'repeat' });
      }
      return config;
    },
    (error) => Promise.reject(error),
  );

  axios.interceptors.response.use(
    (response) => response,
    (error) => {
      if (error.response && error.response.status === 401) {
        return window.location.reload();
      }
      return Promise.reject(error);
    },
  );
}
