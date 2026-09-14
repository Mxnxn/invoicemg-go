import axios from "axios";

const getHeader = () => ({
    headers: {
        "SESSION-TOKEN": window.localStorage.getItem("session_token"),
    },
});

// Read-only lists that may span every company the acting admin owns. Deliberately NOT the
// same endpoints the customer and product pickers use - see routes/Shared.js for why.
class SharedBackend {
    customers() {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.get(`${import.meta.env.VITE_API_URL}/shared/customers`, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }


    // Products entered more than once across the owner's companies, grouped, with each
    // company's own rate so a price mismatch is visible rather than averaged away.
    duplicateMaterials() {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.get(`${import.meta.env.VITE_API_URL}/shared/duplicate-materials`, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    // What a sharing change would take away, BEFORE writing it - the confirmation dialog names
    // the companies rather than asking a vague "are you sure".
    previewSharing(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/sharing/preview`, formData, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    setSharing(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/sharing/set`, formData, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    materials() {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.get(`${import.meta.env.VITE_API_URL}/shared/materials`, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }
}

export const sharedBackend = new SharedBackend();
