// How close an account is to its paid-through date.
//
// activeUntil is the switch that actually locks a login out (see the API's Helpers/Tenancy.js),
// but it was only ever visible in the superadmin Dev panel - the person it affects could not
// see it at all, and found out by being unable to sign in. A null date means no expiry.
//
// Pure, so the thresholds are testable and the same everywhere they are shown.

export const DAY_MS = 24 * 60 * 60 * 1000;

// A month's notice to do something about it - long enough to reach someone and pay, which is
// the point of warning at all.
export const WARN_DAYS = 30;
// Inside a week it stops being a note and starts being urgent.
export const URGENT_DAYS = 7;

export function daysUntil(activeUntil, now = Date.now()) {
    if (!activeUntil) return null;
    const t = new Date(activeUntil).getTime();
    if (Number.isNaN(t)) return null;
    // Ceil, so "expires in 0 days" never shows for an account that is still valid today.
    return Math.ceil((t - now) / DAY_MS);
}

// "ok" | "warn" | "urgent" | "expired" | null (no expiry set)
export function expiryLevel(activeUntil, now = Date.now()) {
    const days = daysUntil(activeUntil, now);
    if (days === null) return null;
    if (days <= 0) return "expired";
    if (days <= URGENT_DAYS) return "urgent";
    if (days <= WARN_DAYS) return "warn";
    return "ok";
}

export function expiryLabel(activeUntil, now = Date.now()) {
    const days = daysUntil(activeUntil, now);
    if (days === null) return "No expiry";
    if (days <= 0) return "Expired";
    if (days === 1) return "Expires tomorrow";
    return `Expires in ${days} days`;
}
