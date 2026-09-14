import { describe, expect, it } from "vitest";
import { invoiceTotals } from "./invoiceTotals";

const row = (over = {}) => ({ qty: 1, rate: 100, length: "", width: "", byQuantity: true, ...over });

describe("invoiceTotals", () => {
    it("sums the lines into a subtotal", () => {
        const t = invoiceTotals([row({ qty: 2, rate: 100 }), row({ qty: 1, rate: 50 })], {});
        expect(t.subtotal).toBe(250);
    });

    it("nets discount and charges against the subtotal before tax", () => {
        const t = invoiceTotals([row({ qty: 1, rate: 1000, discount: 100, charges: 50 })], {});
        expect(t.subtotal).toBe(1000);
        expect(t.discount).toBe(100);
        expect(t.charges).toBe(50);
        expect(t.net).toBe(950);
    });

    // The bug this module was extracted to stop repeating: a service line with blank
    // dimensions made the whole subtotal NaN and printed "NaN" on a customer's invoice.
    it("treats a blank dimension as a factor of one rather than NaN", () => {
        const t = invoiceTotals([row({ qty: 1, rate: 500, length: "", width: "" })], {});
        expect(t.subtotal).toBe(500);
        expect(Number.isNaN(t.grandTotal)).toBe(false);
    });

    it("coerces junk quantities and rates to zero instead of spreading NaN", () => {
        const t = invoiceTotals([row({ qty: "abc", rate: null }), row({ qty: 2, rate: 100 })], {});
        expect(t.subtotal).toBe(200);
    });

    it("taxes at the rate recorded on the line, not a hardcoded 18%", () => {
        const t = invoiceTotals([row({ qty: 1, rate: 1000, cgst: 2.5, sgst: 2.5 })], {});
        expect(t.tax).toBeCloseTo(50, 5);
    });

    it("adds tax to the net and rounds off to the grand total", () => {
        const t = invoiceTotals([row({ qty: 1, rate: 1000, cgst: 9, sgst: 9 })], {});
        expect(t.net).toBe(1000);
        expect(t.tax).toBeCloseTo(180, 5);
        expect(t.grandTotal).toBe(1180);
    });

    it("reports received and the balance still owed", () => {
        const t = invoiceTotals([row({ qty: 1, rate: 1000 })], { receivedAmount: 400 });
        expect(t.received).toBe(400);
        expect(t.balance).toBe(600);
        expect(t.showBalance).toBe(true);
    });

    // Both lines are omitted entirely when nothing has been received, so an unpaid invoice
    // does not carry a "Received 0.00" line.
    it("hides the received and balance lines when nothing has been paid", () => {
        const t = invoiceTotals([row({ qty: 1, rate: 1000 })], {});
        expect(t.received).toBe(0);
        expect(t.showBalance).toBe(false);
    });

    it("survives an invoice with no lines at all", () => {
        const t = invoiceTotals([], {});
        expect(t.subtotal).toBe(0);
        expect(t.grandTotal).toBe(0);
        expect(Number.isNaN(t.grandTotal)).toBe(false);
    });

    it("survives undefined entries", () => {
        expect(invoiceTotals(undefined, undefined).grandTotal).toBe(0);
    });

    it("hides the discount and charges lines when there are none", () => {
        const t = invoiceTotals([row()], {});
        expect(t.showDiscount).toBe(false);
        expect(t.showCharges).toBe(false);
    });
});
