// Client-side mirror of Helpers/Permissions.js on the API.
//
// Permissions are stored as "<feature>:<action>" (view / create / delete). A flat legacy key
// like "invoices" still grants all three, so employees created before this existed keep
// working without a migration.
//
// This is a UI convenience only - it hides controls the user can't use. The API enforces the
// same rules independently (requireFeature / requireCreate / requireDelete), so a hidden
// button is not the security boundary.

export const ACTIONS = ["view", "create", "delete"];

const readPermissions = () => {
    try {
        const raw = JSON.parse(window.localStorage.getItem("permissions") || "[]");
        return Array.isArray(raw) ? raw : [];
    } catch (error) {
        return [];
    }
};

const isEmployee = () => (window.localStorage.getItem("role") || "admin") === "employee";

// `can("invoices", "create")`. Admins always pass. A null/undefined feature key means
// "admin only" (People, Suppliers, Templates use this convention in features.js).
export const can = (feature, action = "view") => {
    if (!isEmployee()) return true;
    if (!feature) return false;
    return readPermissions().some((entry) => {
        const [entryFeature, entryAction] = String(entry).split(":");
        if (entryFeature !== feature) return false;
        return !entryAction || entryAction === action;
    });
};

// Existing call sites use hasAccess(key) to mean "can open this screen".
export const hasAccess = (featureKey) => can(featureKey, "view");
