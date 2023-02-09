axios.defaults.headers.post['Content-Type'] = 'application/x-www-form-urlencoded; charset=UTF-8';
axios.defaults.headers.common['X-Requested-With'] = 'XMLHttpRequest';

axios.interceptors.request.use(
    config => {
        config.data = Qs.stringify(config.data, {
            arrayFormat: 'repeat'
        });
        return config;
    },
    error => Promise.reject(error)
);