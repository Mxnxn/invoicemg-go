import axios from "axios";

// Per-table layout - which columns are shown, and in what order.
//
// Same split as the appearance settings, for the same reason: localStorage is the cache so
// the table can render its arrangement on the first paint without waiting for a request,
// and the server copy is what makes the arrangement survive a session expiring, a cleared
// browser, or logging in somewhere else. Losing a hand-arranged table because a session
// timed out is exactly the kind of small annoyance nobody reports and everybody feels.
//
// Stored under the same UserSetting document as the appearance, keyed per table id, so a
// second table can adopt this without another endpoint.

const getHeader = () => ({
    headers: {
        "SESSION-TOKEN": window.localStorage.getItem("session_token"),
    },
});

// Local first - synchronous, so a component can seed useState from it.
export function readTableSetting(key, fallback) {
    try {
        const raw = window.localStorage.getItem(key);
        if (!raw) return fallback;
        const parsed = JSON.parse(raw);
        return Array.isArray(parsed) && parsed.length ? parsed : fallback;
    } catch (error) {
        return fallback;
    }
}

export function writeTableSetting(key, value) {
    try {
        window.localStorage.setItem(key, JSON.stringify(value));
    } catch (error) {
        // Private mode - the arrangement just will not survive a reload locally. The server
        // copy below still carries it.
    }
    pushTableSettings();
}

let pushTimer = null;

// What the server is believed to already hold. Compared against before every push, because a
// push that changes nothing is not free: the API answers "Settings saved." and the global
// interceptor toasts any successful /update, so an unnecessary request tells the user they
// saved something they never touched.
//
// That is exactly what merely opening the board did. LifecycleIndex writes its column state
// from a mount effect - the effect runs with the values it just read - so visiting
// /admin/lifecycle pushed the current arrangement straight back and announced it.
//
// Seeded at import from whatever localStorage already holds, which is either what the server
// sent at login (pullTableSettings) or the defaults this client would send anyway. So the
// mount write matches and is skipped, while a real change does not.
let lastPushed = null;

// Every table key this build knows about. Sent as one object rather than per-table calls:
// dragging a column fires repeatedly, and one debounced write is enough for all of them.
const TABLE_KEYS = ["lifecycle_visible_columns", "lifecycle_column_order"];

function collect() {
    const tables = {};
    TABLE_KEYS.forEach((key) => {
        const value = readTableSetting(key, null);
        if (value) tables[key] = value;
    });
    return tables;
}

// Records the current arrangement as already saved, without sending anything.
export function markTableSettingsSynced() {
    try {
        lastPushed = JSON.stringify(collect());
    } catch (error) {
        lastPushed = null;
    }
}

export function pushTableSettings(delay = 800) {
    if (!window.localStorage.getItem("session_token")) return;
    if (pushTimer) clearTimeout(pushTimer);
    pushTimer = setTimeout(async () => {
        const payload = JSON.stringify(collect());
        // Nothing the server does not already have. Saying so out loud would be a toast for
        // an action the user did not take.
        if (payload === lastPushed) return;
        try {
            const formData = new FormData();
            formData.set("tables", payload);
            await axios.post(`${import.meta.env.VITE_API_URL}/settings/tables/update`, formData, getHeader());
            lastPushed = payload;
        } catch (error) {
            // The local copy is already correct and the user can see their change; a failed
            // sync is not worth interrupting them for. lastPushed is deliberately NOT updated,
            // so the next change retries this arrangement too.
        }
    }, delay);
}

// The snapshot taken when this module loads - before any view mounts, so it reflects what the
// server sent at login rather than anything a mount effect has written.
markTableSettingsSynced();

// Pulled at login, before the first table renders. The server copy wins there because it is
// the one that followed this person to whatever browser they are now sitting at.
export async function pullTableSettings() {
    try {
        const res = await axios.post(`${import.meta.env.VITE_API_URL}/settings/tables`, new FormData(), getHeader());
        if (res.data?.code !== 200 || !res.data.data) return null;
        const tables = res.data.data;
        Object.keys(tables).forEach((key) => {
            // Only keys this build understands - a newer client's table must not be written
            // into an older one's storage where nothing will ever read or clear it.
            if (!TABLE_KEYS.includes(key)) return;
            if (!Array.isArray(tables[key])) return;
            try {
                window.localStorage.setItem(key, JSON.stringify(tables[key]));
            } catch (error) {
                // Nothing to do - the table falls back to its defaults.
            }
        });
        // The server's own copy is now in localStorage, so nothing needs pushing back.
        markTableSettingsSynced();
        return tables;
    } catch (error) {
        return null;
    }
}
