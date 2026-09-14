import { describe, it, expect } from "vitest";
import { parseISO, toISO, monthGrid, addMonths, daysInMonth, formatDisplay, isSameDay, yearPage, parseMonthISO, toMonthISO, formatMonthDisplay } from "./calendarMath";

describe("parseISO", () => {
    it("reads a valid date without shifting it", () => {
        expect(parseISO("2026-08-26")).toEqual({ year: 2026, month: 7, day: 26 });
    });

    it("does not lose a day the way new Date(string) does in a timezone behind UTC", () => {
        // The bug this exists to avoid: new Date("2026-01-01") is UTC midnight.
        expect(parseISO("2026-01-01").day).toBe(1);
        expect(parseISO("2026-01-01").month).toBe(0);
    });

    it("rejects nonsense and impossible dates", () => {
        expect(parseISO("")).toBeNull();
        expect(parseISO("26-08-2026")).toBeNull();
        expect(parseISO("2026-13-01")).toBeNull();
        expect(parseISO("2026-02-30")).toBeNull();
    });
});

describe("toISO", () => {
    it("pads month and day", () => {
        expect(toISO({ year: 2026, month: 0, day: 5 })).toBe("2026-01-05");
    });

    it("round-trips with parseISO", () => {
        expect(toISO(parseISO("2026-12-31"))).toBe("2026-12-31");
    });
});

describe("daysInMonth", () => {
    it("knows February in a leap year", () => {
        expect(daysInMonth(2024, 1)).toBe(29);
        expect(daysInMonth(2026, 1)).toBe(28);
    });
});

describe("addMonths", () => {
    it("rolls forward over a year boundary", () => {
        expect(addMonths({ year: 2026, month: 11 }, 1)).toEqual({ year: 2027, month: 0 });
    });

    it("rolls backward over a year boundary", () => {
        expect(addMonths({ year: 2026, month: 0 }, -1)).toEqual({ year: 2025, month: 11 });
    });

    it("handles a jump of more than a year", () => {
        expect(addMonths({ year: 2026, month: 5 }, 14)).toEqual({ year: 2027, month: 7 });
    });
});

describe("monthGrid", () => {
    it("is always six rows of seven", () => {
        expect(monthGrid(2026, 7)).toHaveLength(42);
    });

    it("starts on Monday - 1 Aug 2026 is a Saturday, so five blanks lead", () => {
        const cells = monthGrid(2026, 7);
        expect(cells.slice(0, 5)).toEqual([null, null, null, null, null]);
        expect(cells[5]).toBe(1);
    });

    it("pads the tail with nulls rather than next month's days", () => {
        const cells = monthGrid(2026, 7);
        // 1 Aug 2026 is a Saturday, so five blanks lead and the 31st lands at index 35.
        expect(cells[35]).toBe(31);
        expect(cells.slice(36)).toEqual([null, null, null, null, null, null]);
    });
});

describe("formatDisplay", () => {
    it("spells the month so the order is never ambiguous", () => {
        expect(formatDisplay("2026-08-09")).toBe("09 Aug 2026");
    });

    it("is empty for an unset or invalid value", () => {
        expect(formatDisplay("")).toBe("");
        expect(formatDisplay("nope")).toBe("");
    });
});

describe("isSameDay", () => {
    it("compares parts, and is false against null", () => {
        expect(isSameDay({ year: 2026, month: 7, day: 1 }, { year: 2026, month: 7, day: 1 })).toBe(true);
        expect(isSameDay({ year: 2026, month: 7, day: 1 }, { year: 2026, month: 6, day: 1 })).toBe(false);
        expect(isSameDay(null, { year: 2026, month: 7, day: 1 })).toBe(false);
    });
});

describe("yearPage", () => {
    it("returns a stable block of twelve for any year in it", () => {
        const fromStart = yearPage(2016);
        const fromMiddle = yearPage(2026);
        expect(fromStart).toEqual(fromMiddle);
        expect(fromStart).toHaveLength(12);
        expect(fromStart[0]).toBe(2016);
        expect(fromStart[11]).toBe(2027);
    });

    it("moves to the next block past the boundary", () => {
        expect(yearPage(2028)[0]).toBe(2028);
    });
});

describe("month helpers", () => {
    it("round-trips a month without touching Date parsing", () => {
        expect(parseMonthISO("2026-08")).toEqual({ year: 2026, month: 7 });
        expect(toMonthISO({ year: 2026, month: 7 })).toBe("2026-08");
    });

    it("rejects a full date or an impossible month", () => {
        expect(parseMonthISO("2026-08-26")).toBeNull();
        expect(parseMonthISO("2026-13")).toBeNull();
        expect(parseMonthISO("")).toBeNull();
    });

    it("spells the month for display", () => {
        expect(formatMonthDisplay("2026-01")).toBe("Jan 2026");
        expect(formatMonthDisplay("")).toBe("");
    });
});
