import { RoundOff } from "../../Common/DateAndTime/RoundOff";

// Presentation logic for the Customer Dues table, kept out of the component so it can be
// tested (same split as Lifecycle's jobMath.js / queueConstants.js).

// Who the reminder is addressed to: the firm is what appears on the invoice, so it's the
// name the client recognises; their personal name is the fallback.
export const displayName = (row) => row.clientFirm || row.clientName || "there";

// A due below zero means the client has paid us more than we've billed - an advance we're
// holding, not a debt. Rendering a bare "-1500" against someone's name reads as an error.
export const formatDue = (due) => (due < 0 ? `₹${RoundOff(Math.abs(due))} advance` : `₹${RoundOff(due)}`);

// Only clients who actually owe money are worth chasing; settled and overpaid rows are
// hidden behind a toggle so the list stays scannable.
export const outstandingRows = (rows) => rows.filter((row) => row.due > 0);

export const settledCount = (rows) => rows.length - outstandingRows(rows).length;

// A reminder needs both a debt to chase and a number to reach - either missing disables
// the button rather than sending something meaningless or erroring at Meta.
export const canRemind = (row) => row.due > 0 && Boolean(row.clientPhone);

// Whether this customer has already been chased today, so the button can ask before sending a
// second one. The server enforces the same rule (Helpers/RemindWindow.js) for a page left open
// since yesterday, or a colleague who pressed it first; this copy exists so the common case
// asks the question without a wasted round trip that would also raise an error toast.
export const remindedToday = (row, now = new Date()) => {
    const raw = row && row.lastRemindedAt;
    if (!raw) return false;
    const last = new Date(raw);
    if (Number.isNaN(last.getTime())) return false;
    return (
        last.getFullYear() === now.getFullYear() && last.getMonth() === now.getMonth() && last.getDate() === now.getDate()
    );
};

// What the "Last remind" column shows. Today and yesterday are named rather than dated: the
// question the column answers is "recently?", and a date makes the reader do the arithmetic.
export const formatLastRemind = (row, now = new Date()) => {
    const raw = row && row.lastRemindedAt;
    if (!raw) return "Never";
    const last = new Date(raw);
    if (Number.isNaN(last.getTime())) return "Never";
    if (remindedToday(row, now)) {
        return `Today ${last.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })}`;
    }
    const yesterday = new Date(now);
    yesterday.setDate(yesterday.getDate() - 1);
    if (remindedToday(row, yesterday)) return "Yesterday";
    return last.toLocaleDateString([], { day: "2-digit", month: "short", year: "numeric" });
};
