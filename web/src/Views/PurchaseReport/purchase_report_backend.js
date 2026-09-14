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

class PurchaseReportBackend {
    // Every supplier's outstanding balance plus totals.
    dues() {
        return post("/purchase-report/dues");
    }

    // One supplier's ledger. `from`/`to` are optional YYYY-MM-DD bounds; without them the
    // whole history is returned and no opening-balance row is produced.
    supplierLedger({ supplierId, from, to }) {
        const formData = new FormData();
        formData.set("supplier_id", supplierId);
        if (from) formData.set("from", from);
        if (to) formData.set("to", to);
        return post("/purchase-report/supplier", formData);
    }
}

export const purchaseReportBackend = new PurchaseReportBackend();
