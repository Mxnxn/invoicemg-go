import { describe, it, expect } from "vitest";
import { rowAmount, rowNetAmount, rowTotal, quotationGrandTotal, emptyQuotationRow } from "./quotationMath";

// A quotation row is taxed on one side or the other. rowTotal read only cgst + sgst, so an
// interstate row - the whole rate on igst, nothing on cgst/sgst - priced as though it were
// untaxed, both in the modal's Grand Total and in the PDF, which share this function.
describe("rowTotal", () => {
    const base = { qty: 2, rate: 100, hasDimensions: false, discount: 0, charges: 0 };

    it("adds CGST and SGST on an intrastate row", () => {
        expect(rowTotal({ ...base, cgst: 9, sgst: 9, igst: 0 })).toBe(236);
    });

    it("adds IGST on an interstate row", () => {
        expect(rowTotal({ ...base, cgst: 0, sgst: 0, igst: 18 })).toBe(236);
    });

    it("taxes discount and charges, not the raw amount", () => {
        // 200 - 50 + 30 = 180, +18% = 212.4
        expect(rowTotal({ ...base, discount: 50, charges: 30, igst: 18 })).toBeCloseTo(212.4, 6);
    });

    it("prices a dimensional row by area", () => {
        expect(rowAmount({ qty: 2, rate: 100, hasDimensions: true, length: "3", width: "0.5" })).toBe(300);
        expect(rowNetAmount({ qty: 1, rate: 100, hasDimensions: false, discount: 10, charges: 5 })).toBe(95);
    });
});

describe("quotationGrandTotal", () => {
    it("sums rows on either side of the tax", () => {
        const rows = [
            { qty: 1, rate: 100, hasDimensions: false, cgst: 9, sgst: 9 },
            { qty: 1, rate: 100, hasDimensions: false, igst: 18 },
        ];
        expect(quotationGrandTotal(rows)).toBe(236);
    });

    it("is 0 with no rows", () => {
        expect(quotationGrandTotal(null)).toBe(0);
    });
});

describe("emptyQuotationRow", () => {
    // Without igst on the blank row, switching a fresh quotation to IGST wrote a field the
    // row did not have and React's controlled input fell back to undefined.
    it("starts untaxed on both sides", () => {
        const row = emptyQuotationRow();
        expect(row.cgst).toBe(0);
        expect(row.sgst).toBe(0);
        expect(row.igst).toBe(0);
    });
});
