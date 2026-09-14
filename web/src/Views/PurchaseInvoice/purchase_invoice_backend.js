import axios from "axios";

const getHeader = () => ({
    headers: {
        "SESSION-TOKEN": window.localStorage.getItem("session_token"),
    },
});

class PurchaseInvoiceBackend {
    listInvoices() {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/purchase-invoice/list`, {}, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    createInvoice(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/purchase-invoice/create`, formData, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    updateInvoice(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/purchase-invoice/update`, formData, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    deleteInvoice(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/purchase-invoice/delete`, formData, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }
}

export let purchaseInvoiceBackend = new PurchaseInvoiceBackend();
