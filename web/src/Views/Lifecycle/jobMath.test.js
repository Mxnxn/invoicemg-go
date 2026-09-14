import { describe, it, expect } from "vitest";
import { rowAmount, rowNetAmount, rowTotal, jobGrandTotal, rowNeedsField, missingRowFields, rowDimensions, emptyJobRow } from "./jobMath";

// A row that produces a float tail: 1.1 * 3 * 1 * 20.55 = 67.81499999999998
const TAILY = { qty: "1.1", length: "3", width: "1", rate: "20.55", cgst: 9, sgst: 9 };

describe("jobMath rounding", () => {
	it("rounds the row amount to paise", () => {
		const a = rowAmount(TAILY);
		expect(a).toBe(Number(a.toFixed(2)));
	});

	it("rounds net and total to paise", () => {
		expect(rowNetAmount(TAILY)).toBe(Number(rowNetAmount(TAILY).toFixed(2)));
		expect(rowTotal(TAILY)).toBe(Number(rowTotal(TAILY).toFixed(2)));
	});

	it("rounds the grand total, not just the parts", () => {
		const rows = [TAILY, TAILY, TAILY];
		const t = jobGrandTotal(rows);
		expect(t).toBe(Number(t.toFixed(2)));
		expect(String(t)).not.toMatch(/\.\d{3,}/);
	});

	it("keeps clean arithmetic exact", () => {
		expect(rowAmount({ qty: 2, length: 1, width: 1, rate: 50 })).toBe(100);
		expect(jobGrandTotal([])).toBe(0);
	});
});

describe("conditionally required row fields", () => {
	const row = (over = {}) => ({ material: "", length: "1", width: "1", qty: 1, rate: 0, ...over });

	it("asks for nothing while no material is chosen", () => {
		expect(missingRowFields([row()])).toEqual([]);
		expect(rowNeedsField(row(), "rate")).toBe(false);
	});

	it("requires the amount factors once a material is chosen", () => {
		expect(rowNeedsField(row({ material: "Ply" }), "rate")).toBe(true);
		expect(rowNeedsField(row({ material: "Ply", rate: 50 }), "rate")).toBe(false);
	});

	it("treats zero and blank as missing, since either zeroes the line", () => {
		expect(rowNeedsField(row({ material: "Ply", qty: 0 }), "qty")).toBe(true);
		expect(rowNeedsField(row({ material: "Ply", length: "" }), "length")).toBe(true);
	});

	it("exempts rows already converted to an Entry", () => {
		expect(rowNeedsField(row({ material: "Ply", rate: 0, entry_id: "e1" }), "rate")).toBe(false);
	});

	it("reports each missing field once across all rows", () => {
		const rows = [row({ material: "Ply", rate: 0 }), row({ material: "Board", rate: 0, qty: 0 })];
		expect(missingRowFields(rows)).toEqual(["Qty", "Rate"]);
	});

	it("says nothing when every started row is complete", () => {
		expect(missingRowFields([row({ material: "Ply", rate: 10 }), row()])).toEqual([]);
	});
});

describe("rowDimensions", () => {
	it("formats length x width", () => {
		expect(rowDimensions({ length: "2", width: "4" })).toBe("2 x 4");
	});

	// Shown even when both sides are the schema default: the column reads as a uniform
	// spec line, and a blank there looks like missing data rather than "no dimensions".
	it("shows 1 x 1 rather than hiding it", () => {
		expect(rowDimensions({ length: "1", width: "1" })).toBe("1 x 1");
	});

	// length/width are free-text strings on JobRow and default to "1", but rows written
	// before those fields existed have neither - fall back rather than print "undefined".
	it("falls back to 1 for missing or blank sides", () => {
		expect(rowDimensions({})).toBe("1 x 1");
		expect(rowDimensions({ length: "", width: "  " })).toBe("1 x 1");
	});

	it("keeps decimal sides as written", () => {
		expect(rowDimensions({ length: "2.5", width: "0.75" })).toBe("2.5 x 0.75");
	});
});

describe("by-quantity rows", () => {
    it("prices by count, ignoring whatever the sides hold", () => {
        expect(rowAmount({ qty: 3, rate: 100, hasDimensions: false, length: "9", width: "9" })).toBe(300);
        expect(rowTotal({ qty: 1, rate: 1000, hasDimensions: false, cgst: 9, sgst: 9 })).toBe(1180);
    });

    it("does not require a length or a width", () => {
        // The row is complete without them; demanding them would disable submit on a row the
        // user cannot possibly finish, locking them out of the flow.
        const row = { material: "Design", hasDimensions: false, length: "0", width: "0", qty: 1, rate: 500 };
        expect(rowNeedsField(row, "length")).toBe(false);
        expect(rowNeedsField(row, "width")).toBe(false);
        expect(missingRowFields([row])).toEqual([]);
    });

    it("still requires qty and rate", () => {
        const row = { material: "Design", hasDimensions: false, qty: 0, rate: 0 };
        expect(rowNeedsField(row, "qty")).toBe(true);
        expect(rowNeedsField(row, "rate")).toBe(true);
        expect(missingRowFields([row])).toEqual(["Qty", "Rate"]);
    });

    it("renders no dimension line at all", () => {
        expect(rowDimensions({ hasDimensions: false })).toBe("");
        expect(rowDimensions({ hasDimensions: false, length: "2", width: "3" })).toBe("");
    });

    it("leaves by-dimension rows exactly as they were", () => {
        // The flag is absent on every row written before this existed.
        expect(rowAmount({ qty: 4, rate: 120, length: "2", width: "3" })).toBe(2880);
        expect(rowDimensions({ length: "2", width: "4" })).toBe("2 x 4");
        expect(rowNeedsField({ material: "Vinyl", length: "0" }, "length")).toBe(true);
    });

    it("mixes both kinds in one job total", () => {
        const rows = [
            { qty: 4, rate: 120, length: "2", width: "3" },
            { qty: 1, rate: 2500, hasDimensions: false },
        ];
        expect(jobGrandTotal(rows)).toBe(2880 + 2500);
    });
});

describe("emptyJobRow", () => {
    it("defaults to by-dimension, and can be made by-quantity", () => {
        expect(emptyJobRow().hasDimensions).toBe(true);
        expect(emptyJobRow(true).hasDimensions).toBe(true);
        expect(emptyJobRow(false).hasDimensions).toBe(false);
    });
});
