import { describe, it, expect } from "vitest";
import { displayName, formatDue, outstandingRows, settledCount, canRemind, remindedToday, formatLastRemind } from "./duesMath";

const row = (over = {}) => ({ clientId: "c1", clientName: "Arpan Shah", clientFirm: "Green OOH", clientPhone: "9638884736", billed: 1000, received: 400, due: 600, ...over });

describe("displayName", () => {
    it("prefers the firm, since that's what's on the invoice", () => {
        expect(displayName(row())).toBe("Green OOH");
    });

    it("falls back to the person's name when there's no firm", () => {
        expect(displayName(row({ clientFirm: "" }))).toBe("Arpan Shah");
    });

    it("falls back to a neutral greeting when neither is set", () => {
        expect(displayName(row({ clientFirm: "", clientName: "" }))).toBe("there");
    });
});

describe("formatDue", () => {
    it("formats a debt to two decimals with a rupee sign", () => {
        expect(formatDue(4515)).toBe("₹4515.00");
    });

    it("labels a negative balance as an advance rather than a minus sign", () => {
        expect(formatDue(-1500)).toBe("₹1500.00 advance");
    });

    it("shows a settled client as zero, not as an advance", () => {
        expect(formatDue(0)).toBe("₹0.00");
    });
});

describe("outstandingRows / settledCount", () => {
    const rows = [row({ clientId: "a", due: 600 }), row({ clientId: "b", due: 0 }), row({ clientId: "c", due: -200 })];

    it("keeps only clients who owe money", () => {
        expect(outstandingRows(rows).map((r) => r.clientId)).toEqual(["a"]);
    });

    it("counts settled and overpaid clients together as hidden", () => {
        expect(settledCount(rows)).toBe(2);
    });

    it("reports nothing hidden when everyone owes", () => {
        expect(settledCount([row({ due: 5 })])).toBe(0);
    });
});

describe("canRemind", () => {
    it("allows a reminder when there's a debt and a phone number", () => {
        expect(canRemind(row())).toBe(true);
    });

    it("blocks a reminder when no phone number is on file", () => {
        expect(canRemind(row({ clientPhone: "" }))).toBe(false);
    });

    it("blocks a reminder for a client who owes nothing", () => {
        expect(canRemind(row({ due: 0 }))).toBe(false);
    });

    it("blocks a reminder for a client holding an advance", () => {
        expect(canRemind(row({ due: -50 }))).toBe(false);
    });
});

describe("remindedToday", () => {
    const now = new Date("2026-09-01T18:00:00");

    it("is false when this customer has never been reminded", () => {
        expect(remindedToday({ lastRemindedAt: null }, now)).toBe(false);
        expect(remindedToday({}, now)).toBe(false);
    });

    // The case it exists for: a colleague pressed the button this morning and the button
    // itself gives no sign of it.
    it("is true for a reminder earlier the same day", () => {
        expect(remindedToday({ lastRemindedAt: "2026-09-01T09:30:00" }, now)).toBe(true);
    });

    it("is false for yesterday, however few hours ago", () => {
        expect(remindedToday({ lastRemindedAt: "2026-08-31T23:30:00" }, new Date("2026-09-01T00:30:00"))).toBe(false);
    });

    it("ignores a stored value that is not a date rather than blocking a real send", () => {
        expect(remindedToday({ lastRemindedAt: "nonsense" }, now)).toBe(false);
    });
});

describe("formatLastRemind", () => {
    const now = new Date("2026-09-01T18:00:00");

    it("says so plainly when nobody has been chased", () => {
        expect(formatLastRemind({}, now)).toBe("Never");
    });

    // "Recently?" is the question the column answers, so today and yesterday are named rather
    // than dated - a date makes the reader do the arithmetic.
    it("names today and yesterday", () => {
        expect(formatLastRemind({ lastRemindedAt: "2026-09-01T09:30:00" }, now)).toMatch(/^Today /);
        expect(formatLastRemind({ lastRemindedAt: "2026-08-31T09:30:00" }, now)).toBe("Yesterday");
    });

    it("dates anything older", () => {
        expect(formatLastRemind({ lastRemindedAt: "2026-08-20T09:30:00" }, now)).toMatch(/Aug/);
    });
});
