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

class BankReportBackend {
    // Per-bank opening / in / out / current plus the transactions behind it.
    report(formData) {
        return post("/bank/report", formData);
    }

    createExpense(formData) {
        return post("/expense/create", formData);
    }

    listExpenses(formData) {
        return post("/expense/list", formData);
    }

    removeExpense(formData) {
        return post("/expense/remove", formData);
    }
}

export const bankReportBackend = new BankReportBackend();
