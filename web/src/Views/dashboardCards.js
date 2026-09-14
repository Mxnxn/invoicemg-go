// The dashboard's two card shapes, as plain functions.
//
// A date has to be readable by someone who is not going to parse "2026-09-13" in their head,
// and a customer has to be recognisable before it is read. Both of those are decisions about
// wording rather than about layout, which is why they live here where they can be tested
// rather than inside the markup where they cannot.

import { parseISO } from "../Common/calendarMath";

const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

// Midnight local, so "yesterday" means the previous CALENDAR day rather than 24 hours ago.
// Comparing timestamps would call 11pm-to-1am "the same day" and 9am-to-9am "yesterday",
// which is not how anybody reads a date on a card.
// A "YYYY-MM-DD" string is read as a LOCAL day, never through new Date("2026-09-13") - that
// parses as UTC midnight, so every timezone behind UTC reads it back as the 12th. It is the
// same bug calendarMath.js was written to avoid, and the sheet header is exactly where it
// showed: a day's work filed under the day before.
const toLocalDate = (value) => {
    const parts = parseISO(value);
    if (parts) return new Date(parts.year, parts.month, parts.day);
    const d = new Date(value);
    return Number.isNaN(d.getTime()) ? null : d;
};

const startOfDay = (value) => {
    const d = toLocalDate(value);
    if (!d) return null;
    return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
};

const DAY_MS = 24 * 60 * 60 * 1000;

/**
 * "Today", "Yesterday", or nothing.
 *
 * Only those two. "3 days ago" is slower to place than the date already printed beside it -
 * the label earns its space precisely while it saves someone working out what today is, and
 * stops earning it immediately afterwards.
 */
export const relativeDayLabel = (value, now = new Date()) => {
    const day = startOfDay(value);
    const today = startOfDay(now);
    if (day === null || today === null) return "";
    const diff = Math.round((today - day) / DAY_MS);
    if (diff === 0) return "Today";
    if (diff === 1) return "Yesterday";
    return "";
};

/**
 * A date split into the pieces a card shows separately, so the day can be big and the rest
 * quiet. Returns empty strings rather than "Invalid Date" for junk, because a card with a
 * blank corner is survivable and one reading "NaN" is not.
 */
export const dayParts = (value) => {
    const d = toLocalDate(value);
    if (!d) return { weekday: "", day: "", month: "", year: "" };
    return {
        weekday: WEEKDAYS[d.getDay()],
        day: String(d.getDate()),
        month: MONTHS[d.getMonth()],
        year: String(d.getFullYear()),
    };
};

/**
 * Up to two letters standing in for a firm, so a customer is recognisable at a glance before
 * the name is read. Scanning forty cards for one you already know is a shape-matching job,
 * not a reading one.
 *
 * Two words give their first letters; one word gives its first two, which still distinguishes
 * "Himani" from "Hitesh" where a single letter would not.
 */
export const initials = (name) => {
    const words = String(name || "")
        .trim()
        .split(/\s+/)
        .filter(Boolean);
    if (words.length === 0) return "?";
    if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
    return (words[0][0] + words[1][0]).toUpperCase();
};

/**
 * The stagger a card animates in on, in milliseconds.
 *
 * Capped deliberately. A per-card delay that keeps growing means the last card of a
 * twenty-four card page arrives most of a second after the first, and the page feels slow
 * precisely because someone tried to make it feel considered. After the cap they all arrive
 * together, which nobody notices.
 */
export const CARD_STAGGER_MS = 26;
export const CARD_STAGGER_CAP = 12;
export const cardDelay = (index) => Math.min(index, CARD_STAGGER_CAP) * CARD_STAGGER_MS;
