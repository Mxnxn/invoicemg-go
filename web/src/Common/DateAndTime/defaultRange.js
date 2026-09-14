// The date range every report opens on: today, back to the same day last month.
//
// Four reports had four different answers - GST started on the 1st of the current month,
// Ledger left `from` blank with `to` set to today, and Purchase Report and Inventory opened
// with both blank, which asks the server for everything ever recorded. One helper so they
// agree, and so "last month" means the same thing on each.
//
// Dates are YYYY-MM-DD strings throughout this app (see Model/Wastage.date, BatchReceive.date
// and the Ledger comparisons), so these are built as strings from local date parts rather
// than through toISOString - that converts to UTC first, and west of UTC it hands back
// yesterday.

const pad = (n) => String(n).padStart(2, "0");

/** A Date as the YYYY-MM-DD string the rest of the app stores and compares. */
export function isoDay(date) {
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

/**
 * The same day one month earlier, clamped when that day does not exist.
 *
 * 31 March has no counterpart in February, and `setMonth(month - 1)` does NOT clamp - it
 * overflows forward, turning 31 March into 3 March, which would silently produce a 28-day
 * range labelled as a month. Setting the day explicitly against the target month's length is
 * what makes 31 March -> 28 February (or 29 in a leap year).
 */
export function sameDayLastMonth(from = new Date()) {
    const year = from.getFullYear();
    const month = from.getMonth();
    const day = from.getDate();

    // Day 0 of the following month is the last day of the month in question.
    const daysInPrevMonth = new Date(year, month, 0).getDate();
    return new Date(year, month - 1, Math.min(day, daysInPrevMonth));
}

/**
 * { from, to } for a report's initial state.
 *
 * `to` is today and `from` is the same day last month, so every report opens on a period that
 * is both recent and complete enough to read - rather than on everything ever recorded, which
 * is slow to fetch and tells you nothing about how the business is doing now.
 */
export function defaultDateRange(now = new Date()) {
    return { from: isoDay(sameDayLastMonth(now)), to: isoDay(now) };
}

export default defaultDateRange;
