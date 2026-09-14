import axios from "axios";
const HEADER = {
    headers: {
        "SESSION-TOKEN": window.localStorage.getItem("session_token"),
    },
};
class ClientsBackend {
    /**
     * @param {string} entry
     * */

    // Customers and their answer to "tell them when a job-id is raised?".
    //
    // `all` returns every customer, answered or not - which is what the Customers tab in
    // Configure > WhatsApp wants, because that tab is for CHOOSING an answer rather than
    // reviewing the ones already given. Without it, only customers who have been asked.
    listNotifyPreferences(all = false) {
        return new Promise(async (resolve, reject) => {
            try {
                const url = `${import.meta.env.VITE_API_URL}/client/notify-preferences${all ? "?all=true" : ""}`;
                const res = await axios.get(url, {
                    headers: { "SESSION-TOKEN": window.localStorage.getItem("session_token") },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    // The remembered answer to "tell this customer when a job-id is raised?".
    // "true" / "false" / "clear" - clear puts them back to being asked each time.
    setNotifyPreference(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/client/notify-preference`, formData, {
                    // Read at call time, not at module load: HEADER above is captured once when
                    // this file is imported, which is before login on a cold start.
                    headers: { "SESSION-TOKEN": window.localStorage.getItem("session_token") },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    addNewClient(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/client/add`, formData, HEADER);
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    getOnlyClients(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/client/only`, formData, HEADER);
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    getAllClientWithData(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/client/getAll`, formData, HEADER);
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    editClientDetail(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/client/update`, formData, HEADER);
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }
    deleteClient(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/client/remove`, formData, HEADER);
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    getClientDetail(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/client/get`, formData, HEADER);
                // const res = await axios.post(`http://localhost:5000/client/get`, formData, HEADER);
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }
    saveInvoice(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/invoice/save`, formData, HEADER);
                // const res = await axios.post(`http://localhost:5000/invoice/save`, formData, HEADER);
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    batchUpdate(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/client/batchUpdate`, formData, HEADER);
                // const res = await axios.post(`http://localhost:5000/client/batchUpdate`, formData, HEADER);
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    updateBatchReceive(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/client/batchReceiveUpdate`, formData, HEADER);
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    deleteBatchReceive(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/client/batchReceiveDelete`, formData, HEADER);
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }
}

export let clientsBackend = new ClientsBackend();
