import axios from "axios";

const getHeader = () => ({
    headers: {
        "SESSION-TOKEN": window.localStorage.getItem("session_token"),
    },
});

const post = (path, formData = new FormData()) =>
    new Promise(async (resolve, reject) => {
        try {
            const res = await axios.post(`${import.meta.env.VITE_API_URL}${path}`, formData, getHeader());
            if (res.data.code !== 200) throw res.data;
            resolve(res.data);
        } catch (error) {
            reject(error);
        }
    });

// Money paid out to suppliers - the payables twin of batch_receive_backend.
class SupplierPaymentBackend {
    listPayments() {
        return post("/supplier-payment/list");
    }

    lookupOpenInvoices(supplierId) {
        const formData = new FormData();
        formData.set("supplier_id", supplierId);
        return post("/supplier-payment/lookups/open-invoices", formData);
    }

    createPayment(formData) {
        return post("/supplier-payment/create", formData);
    }

    deletePayment(formData) {
        return post("/supplier-payment/delete", formData);
    }
}

export const supplierPaymentBackend = new SupplierPaymentBackend();
