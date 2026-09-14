import { getDateForEntry } from "./DateAndTime/getDate";

// Shared start-end month filtering, used by the Client Entries feed, Purchase Invoices and
// both Bank Transfers tabs. Written once because the first version of this (a single-month
// modal on the Client page) compared only the month number out of the date - so picking
// August also matched every previous year's August.

export const EMPTY_MONTH_RANGE = { from: "", to: "" };

// "YYYY-MM" - the value an <input type="month"> produces, and the form these comparisons
// need. Zero-padded ISO prefixes order correctly as plain strings, so no Date parsing.
//
// Deliberately slices the string rather than going through getDateForEntry: that helper does
// `new Date(str).getDate()`, which parses "2026-06-01" as UTC midnight and then reads it back
// in local time - so west of UTC every ISO date shifts back a day and the 1st of a month lands
// in the previous one. Dates are stored as YYYY-MM-DD strings throughout this app, so the
// prefix is already exactly what's wanted. Date objects and other formats still fall back.
const ISO_DATE = /^\d{4}-\d{2}-\d{2}/;

export const monthOf = (date) => {
    if (!date) return "";
    const raw = String(date);
    if (ISO_DATE.test(raw)) return raw.slice(0, 7);
    const parsed = getDateForEntry(date);
    return parsed ? String(parsed).slice(0, 7) : "";
};

export const isMonthRangeActive = (range) => Boolean(range && (range.from || range.to));

// Both bounds are optional and inclusive: from-only is "this month onward", to-only is
// "up to and including this month".
export const inMonthRange = (date, range) => {
    if (!isMonthRangeActive(range)) return true;
    const month = monthOf(date);
    // A record with no usable date can't be shown to fall inside a range the user asked for.
    if (!month) return false;
    if (range.from && month < range.from) return false;
    if (range.to && month > range.to) return false;
    return true;
};

// `getDate` picks the field to compare - records name their date differently (date,
// receivedDate, invoiceDate).
export const filterByMonthRange = (rows = [], range, getDate = (row) => row.date) =>
    isMonthRangeActive(range) ? rows.filter((row) => inMonthRange(getDate(row), range)) : rows;

// The month range a filter opens on.
//
// Day-precision ranges are NOT here: Common/DateAndTime/defaultRange.js already owns that and
// six reports already use it. This is the month-precision counterpart, which had no
// equivalent.
//
// One definition, because "the default period" appearing in three views as three slightly
// different calculations is how they drift apart. Both take `now` rather than reading the
// clock, so they are testable and so a caller can pin them.
//
// Applied to REPORTS only. Lists that show money still owed - purchase invoices, supplier
// payments, batch receive - stay unfiltered on purpose: an unpaid bill from March must not
// disappear from view because the calendar moved on, which is exactly how one gets forgotten.

const pad = (n) => String(n).padStart(2, "0");
const asMonth = (d) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}`;

// Month precision: last month through this one.
export const defaultMonthRange = (now = new Date()) => {
    const previous = new Date(now.getFullYear(), now.getMonth() - 1, 1);
    return { from: asMonth(previous), to: asMonth(now) };
};
