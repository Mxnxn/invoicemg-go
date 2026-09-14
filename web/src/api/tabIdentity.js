// Per-tab identifier. sessionStorage is per-tab by spec - a new tab starts empty and gets a
// fresh id - which is what lets one login sit on two companies at once. localStorage would
// be shared across tabs and defeat the whole point.
const TAB_ID_KEY = "tab_id";

export function getTabId() {
    let tabId = window.sessionStorage.getItem(TAB_ID_KEY);
    if (!tabId) {
        tabId =
            typeof crypto !== "undefined" && crypto.randomUUID
                ? crypto.randomUUID()
                : `tab-${Date.now()}-${Math.random().toString(36).slice(2)}`;
        window.sessionStorage.setItem(TAB_ID_KEY, tabId);
    }
    return tabId;
}

// Namespaces a localStorage key to one company, so per-company state (e.g. the invoice
// counter) does not leak between profiles.
export function companyScopedKey(base, companyId) {
    return companyId ? `${base}:${companyId}` : base;
}
