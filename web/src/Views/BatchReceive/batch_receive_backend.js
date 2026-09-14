import axios from "axios";

const getHeader = () => ({
    headers: {
        "SESSION-TOKEN": window.localStorage.getItem("session_token"),
    },
});

class BatchReceiveBackend {
    listReceives(filter = {}) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/batch-receive/list`, filter, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    lookupOpenJobs(clientId) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/batch-receive/lookups/open-jobs`, { client_id: clientId }, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    createReceive(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/batch-receive/create`, formData, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    deleteReceive(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/batch-receive/delete`, formData, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }
}

export let batchReceiveBackend = new BatchReceiveBackend();
