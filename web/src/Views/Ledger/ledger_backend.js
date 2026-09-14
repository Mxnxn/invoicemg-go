import axios from "axios";

const getHeader = () => ({
    headers: {
        "SESSION-TOKEN": window.localStorage.getItem("session_token"),
    },
});

class LedgerBackend {
    clientLedger(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/ledger/client`, formData, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    // Every client's outstanding balance in one shot (see routes/Ledger.js /dues). No
    // arguments - the active company comes from the SESSION-TOKEN/TAB-ID headers.
    // The payment reminder behind the Remind button. A template send, computed and dispatched
    // server-side (routes/Ledger.js /dues/remind) so the amount in the message cannot disagree
    // with the report it was sent from.
    remindDue(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/ledger/dues/remind`, formData, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    dues() {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/ledger/dues`, new FormData(), getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }
}

export const ledgerBackend = new LedgerBackend();
