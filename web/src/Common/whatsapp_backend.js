import axios from "axios";

const getHeader = () => ({
    headers: {
        "SESSION-TOKEN": window.localStorage.getItem("session_token"),
    },
});

class WhatsAppBackend {
    getConfig() {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/whatsapp/config`, {}, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    updateConfig(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/whatsapp/config/update`, formData, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    send(to, message) {
        return new Promise(async (resolve, reject) => {
            try {
                const formData = new FormData();
                formData.set("to", to);
                formData.set("message", message);
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/whatsapp/send`, formData, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    // The company's approved templates, read live from Meta by the API. Never cached here:
    // approval state changes on Meta's side without telling us.
    listTemplates() {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/whatsapp/templates`, new FormData(), getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }
}

export const whatsappBackend = new WhatsAppBackend();
