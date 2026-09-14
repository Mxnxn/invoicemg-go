import axios from "axios";
const HEADER = {
    headers: {
        "SESSION-TOKEN": window.localStorage.getItem("session_token"),
    },
};
class InvoiceBackend {
    nextInvoiceNumber() {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/invoice/next-invoice-number`, {}, HEADER);
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    getInvoices(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/invoice/getAll`, formData, HEADER);
                //     const res = await axios.post(`http://localhost:5000/invoice/getAll`, formData, HEADER);
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
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/invoice/remove`, formData, HEADER);
                // const res = await axios.post(`http://localhost:5000/invoice/remove`, formData, HEADER);
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    paidInvoice(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/invoice/paid`, formData, HEADER);
                //     const res = await axios.post(`http://localhost:5000/invoice/paid`, formData, HEADER);
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    getInvoiceReceives(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/invoice/getReceived`, formData, HEADER);
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    entriesJobs(entryIds) {
        return new Promise(async (resolve, reject) => {
            try {
                const formData = new FormData();
                formData.set("entry_ids", JSON.stringify(entryIds));
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/invoice/entries-jobs`, formData, HEADER);
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    getClientInvoices(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(
                    `${import.meta.env.VITE_API_URL}/invoice/getClientInvoices`,
                    formData,
                    HEADER
                );
                //   const res = await axios.post(
                //     `http://localhost:5000/invoice/getClientInvoices`,
                //     formData,
                //     HEADER
                //   );
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }
    // Job-ids a client can still be invoiced for, with their billable entry ids resolved.
    invoiceableJobs(clientId) {
        return new Promise(async (resolve, reject) => {
            try {
                const formData = new FormData();
                formData.set("client_id", clientId);
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/invoice/invoiceable-jobs`, formData, HEADER);
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    // entry_ids is sent as a repeated field, matching what /invoice/save already reads.
    // Either job_ids (the current path - the server converts and links the jobs itself) or
    // entry_ids (still accepted for callers that already hold entries).
    saveInvoice({ entry_ids, job_ids, client_id, date, invNo }) {
        return new Promise(async (resolve, reject) => {
            try {
                const formData = new FormData();
                if (job_ids && job_ids.length) formData.set("job_ids", JSON.stringify(job_ids));
                (entry_ids || []).forEach((id) => formData.append("entry_ids", id));
                formData.set("client_id", client_id);
                formData.set("date", date);
                formData.set("invNo", invNo);
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/invoice/save`, formData, HEADER);
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }
}

export let invoiceBackend = new InvoiceBackend();
