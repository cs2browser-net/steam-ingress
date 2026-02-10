import axios, { type AxiosInstance } from 'axios';

const http: AxiosInstance = axios.create({
    timeout: 120000,
    headers: {
        'X-Requested-With': 'XMLHttpRequest',
        Accept: 'application/json',
        'Content-Type': 'application/json',
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36"
    }
});

export default http;

export function httpErrorToHuman(error: any): string {
    if (error.response && error.response.data) {
        let { data } = error.response;

        if (typeof data === 'string') {
            try {
                data = JSON.parse(data);
            } catch (e) {
            }
        }

        if (data.errors && data.errors[0] && data.errors[0].detail) {
            return data.errors[0].detail;
        }

        if (data.error && typeof data.error === 'string') {
            return data.error;
        }
    }

    return error.message;
}