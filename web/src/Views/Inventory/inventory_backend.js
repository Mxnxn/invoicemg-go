import axios from "axios";

const getHeader = () => ({
    headers: {
        // Read at call time, not at module load: a module-level header is captured before
        // login on a cold start. The TAB-ID header is attached globally by the interceptor.
        "SESSION-TOKEN": window.localStorage.getItem("session_token"),
    },
});

class InventoryBackend {
    // Stock per product. `from`/`to` are optional; without them the report covers everything.
    report(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/inventory/report`, formData, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    // Sets just the stocked unit, from the report's "Not counted" panel. A focused route
    // because /material/edit rewrites the whole product - see routes/Material.js.
    setUnit(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/material/set-unit`, formData, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    // The units this company offers, to populate the picker in that panel.
    units() {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/unit/list`, {}, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    // The admin-only cross-company reporting toggle. Lives on Company, not on the report, so
    // every shared view agrees about whether sharing is on.
    setSharing(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/company/sharing`, formData, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }
}

export const inventoryBackend = new InventoryBackend();
