import { describe, it, expect } from "vitest";
import { relativeDayLabel, dayParts, initials, cardDelay, CARD_STAGGER_CAP, CARD_STAGGER_MS } from "./dashboardCards";

describe("relativeDayLabel", () => {
    const now = new Date(2026, 8, 13, 14, 30); // 13 Sep 2026, 2:30pm local

    it("names today and yesterday", () => {
        expect(relativeDayLabel(new Date(2026, 8, 13, 9, 0), now)).toBe("Today");
        expect(relativeDayLabel(new Date(2026, 8, 12, 23, 59), now)).toBe("Yesterday");
    });

    // Calendar days, not 24-hour windows. 11pm last night is "yesterday" however few hours ago
    // it was, and 1am this morning is "today".
    it("counts calendar days rather than elapsed hours", () => {
        expect(relativeDayLabel(new Date(2026, 8, 13, 0, 1), now)).toBe("Today");
        expect(relativeDayLabel(new Date(2026, 8, 12, 22, 0), new Date(2026, 8, 13, 1, 0))).toBe("Yesterday");
    });

    it("says nothing for anything older, and for the future", () => {
        expect(relativeDayLabel(new Date(2026, 8, 11), now)).toBe("");
        expect(relativeDayLabel(new Date(2026, 8, 14), now)).toBe("");
    });

    it("survives junk rather than printing it", () => {
        expect(relativeDayLabel(undefined, now)).toBe("");
        expect(relativeDayLabel("not a date", now)).toBe("");
    });

    it("works across a month and a year boundary", () => {
        expect(relativeDayLabel(new Date(2026, 7, 31), new Date(2026, 8, 1))).toBe("Yesterday");
        expect(relativeDayLabel(new Date(2025, 11, 31), new Date(2026, 0, 1))).toBe("Yesterday");
    });
});

describe("dayParts", () => {
    it("splits a date into the pieces the card shows", () => {
        expect(dayParts(new Date(2026, 8, 13))).toEqual({ weekday: "Sun", day: "13", month: "Sep", year: "2026" });
    });

    it("does not pad the day - the card sets its own size", () => {
        expect(dayParts(new Date(2026, 8, 3)).day).toBe("3");
    });

    // A blank corner is survivable; "NaN" on a customer-facing dashboard is not.
    it("blanks rather than printing Invalid Date", () => {
        expect(dayParts("rubbish")).toEqual({ weekday: "", day: "", month: "", year: "" });
    });
});

describe("initials", () => {
    it("takes one letter from each of the first two words", () => {
        expect(initials("Himani Wallpapers")).toBe("HW");
        expect(initials("Acme Signs Pvt Ltd")).toBe("AS");
    });

    // One letter would make every "H..." firm look identical in a grid.
    it("takes two letters from a single word", () => {
        expect(initials("Himani")).toBe("HI");
    });

    it("copes with nothing at all", () => {
        expect(initials("")).toBe("?");
        expect(initials(null)).toBe("?");
        expect(initials("   ")).toBe("?");
    });

    it("ignores the extra spaces people paste in", () => {
        expect(initials("  Acme   Signs  ")).toBe("AS");
    });
});

describe("cardDelay", () => {
    it("staggers the first cards", () => {
        expect(cardDelay(0)).toBe(0);
        expect(cardDelay(3)).toBe(3 * CARD_STAGGER_MS);
    });

    // Past the cap they arrive together. Without it the last card of a long page lands most of
    // a second late and the page feels slow.
    it("stops growing after the cap", () => {
        expect(cardDelay(CARD_STAGGER_CAP)).toBe(CARD_STAGGER_CAP * CARD_STAGGER_MS);
        expect(cardDelay(200)).toBe(CARD_STAGGER_CAP * CARD_STAGGER_MS);
        expect(cardDelay(200)).toBeLessThanOrEqual(400);
    });
});

// --- "YYYY-MM-DD" is a local day -----------------------------------------------------------
// new Date("2026-09-13") is UTC midnight, which every timezone behind UTC reads back as the
// 12th. The sheet header is where that showed: a day's work filed under the day before. These
// pass whatever the machine's timezone is, which is the whole point.
describe("ISO date strings", () => {
    it("reads the day that was written, not the one UTC makes of it", () => {
        expect(dayParts("2026-09-13")).toEqual({ weekday: "Sun", day: "13", month: "Sep", year: "2026" });
        expect(dayParts("2026-01-01")).toEqual({ weekday: "Thu", day: "1", month: "Jan", year: "2026" });
    });

    it("labels an ISO string today when it is today", () => {
        const now = new Date(2026, 8, 13, 2, 0);
        expect(relativeDayLabel("2026-09-13", now)).toBe("Today");
        expect(relativeDayLabel("2026-09-12", now)).toBe("Yesterday");
    });

    it("still takes a real Date, which is what the rest of the app passes", () => {
        expect(dayParts(new Date(2026, 8, 13)).day).toBe("13");
    });

    it("still blanks on junk", () => {
        expect(dayParts("2026-13-45")).toEqual({ weekday: "", day: "", month: "", year: "" });
    });
});
