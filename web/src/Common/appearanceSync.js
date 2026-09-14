import axios from "axios";
import { readAppearance, writeAppearance, applyAppearance } from "./appearance";

// Keeping the appearance settings on the server as well as on the device.
//
// localStorage is NOT replaced by this, it is the cache. index.js applies the appearance
// before first paint and cannot await a network round trip, so without a local copy every
// page load would flash the default font and size before settling. The server copy is what
// makes the preference survive a new browser, a cleared cache, or a different machine.
//
// The server wins on login, because that is the copy that followed the person here. After
// that the device leads and pushes changes up.

const getHeader = () => ({
    headers: {
        "SESSION-TOKEN": window.localStorage.getItem("session_token"),
    },
});

// Pulled after login. Failure is silent by design: not being able to reach the settings
// endpoint is no reason to block someone from using the app, and the device copy is a
// perfectly good answer.
export async function pullAppearance() {
    try {
        const res = await axios.post(`${import.meta.env.VITE_API_URL}/settings/appearance`, new FormData(), getHeader());
        if (res.data?.code !== 200 || !res.data.data) return null;
        // Through readAppearance's sanitiser rather than trusted as-is - it clamps every
        // value and drops ids this build does not know, so an older or newer client cannot
        // be handed something it will render badly.
        const merged = { ...readAppearance(), ...res.data.data };
        const clean = sanitise(merged);
        writeAppearance(clean);
        applyAppearance(clean);
        return clean;
    } catch (error) {
        return null;
    }
}

// Round-trips a value object through the same validation readAppearance applies to stored
// JSON, without disturbing what is currently saved.
function sanitise(value) {
    const backup = window.localStorage.getItem("appearance");
    try {
        window.localStorage.setItem("appearance", JSON.stringify(value));
        return readAppearance();
    } catch (error) {
        return value;
    } finally {
        if (backup === null) window.localStorage.removeItem("appearance");
        else window.localStorage.setItem("appearance", backup);
    }
}

let pushTimer = null;

// Debounced: the size boxes fire on every keystroke and the accent swatches on every click,
// and none of that needs its own request.
export function pushAppearance(appearance, delay = 800) {
    // Immediate, not debounced: tables already on screen should re-page the moment the
    // setting changes, not when the network call happens to land.
    window.dispatchEvent(new CustomEvent(APPEARANCE_EVENT));
    if (!window.localStorage.getItem("session_token")) return;
    if (pushTimer) clearTimeout(pushTimer);
    pushTimer = setTimeout(async () => {
        try {
            const formData = new FormData();
            formData.set("appearance", JSON.stringify(appearance));
            await axios.post(`${import.meta.env.VITE_API_URL}/settings/appearance/update`, formData, getHeader());
        } catch (error) {
            // Same reasoning as the pull: the device copy is already correct and the user
            // can see their change. A failed sync is not worth a toast.
        }
    }, delay);
}

// Fired whenever the appearance is saved, so views already on screen (the tables reading
// rowsPerPage) re-read it without a reload. The `storage` event does not fire in the tab
// that made the change, which is precisely the tab that needs to know.
export const APPEARANCE_EVENT = "appearance:changed";
