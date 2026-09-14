import { describe, it, expect } from "vitest";
import {
    supplierLabel,
    formatOwed,
    outstandingSuppliers,
    settledSupplierCount,
    invoiceDue,
    previewAutoAllocation,
    allocationError,
} from "./purchaseReportMath";

describe("supplierLabel", () => {
    it("prefers the firm", () => {
        expect(supplierLabel({ supplierFirm: "Tirupati Flex", supplierName: "Kano" })).toBe("Tirupati Flex");
    });

    it("falls back to the contact name", () => {
        expect(supplierLabel({ supplierFirm: "", supplierName: "Kano" })).toBe("Kano");
    });

    it("never renders blank", () => {
        expect(supplierLabel({})).toBe("Unknown supplier");
    });
});

describe("formatOwed", () => {
    it("formats what we owe", () => {
        expect(formatOwed(8850)).toBe("₹8850.00");
    });

    it("labels an overpayment as an advance", () => {
        expect(formatOwed(-500)).toBe("₹500.00 advance");
    });

    it("shows a settled supplier as zero", () => {
        expect(formatOwed(0)).toBe("₹0.00");
    });
});

describe("outstandingSuppliers / settledSupplierCount", () => {
    const rows = [{ due: 100 }, { due: 0 }, { due: -20 }];

    it("keeps only suppliers we still owe", () => {
        expect(outstandingSuppliers(rows)).toHaveLength(1);
    });

    it("counts settled and overpaid as hidden", () => {
        expect(settledSupplierCount(rows)).toBe(2);
    });
});

describe("invoiceDue", () => {
    it("subtracts what's been paid", () => {
        expect(invoiceDue({ total: 8850, amount: 3000 })).toBe(5850);
    });

    // The bug that shipped in the first cut of the API: pre-existing purchase invoices have
    // no `amount` field, and total - undefined is NaN.
    it("treats a missing amount as zero rather than NaN", () => {
        expect(invoiceDue({ total: 8850 })).toBe(8850);
    });

    it("handles paise", () => {
        expect(invoiceDue({ total: 100.55, amount: 0.05 })).toBe(100.5);
    });
});

describe("previewAutoAllocation", () => {
    it("fills oldest-first up to each invoice's due", () => {
        const { allocations, remaining } = previewAutoAllocation(
            [
                { _id: "i1", total: 300, amount: 100 },
                { _id: "i2", total: 500, amount: 0 },
            ],
            600
        );
        expect(allocations.map((a) => [a._id, a.applied])).toEqual([
            ["i1", 200],
            ["i2", 400],
        ]);
        expect(remaining).toBe(0);
    });

    it("reports the excess instead of overpaying", () => {
        const { allocations, remaining } = previewAutoAllocation([{ _id: "i1", total: 100, amount: 0 }], 250);
        expect(allocations).toHaveLength(1);
        expect(allocations[0].applied).toBe(100);
        expect(remaining).toBe(150);
    });

    it("skips fully-paid invoices", () => {
        const { allocations } = previewAutoAllocation(
            [
                { _id: "settled", total: 100, amount: 100 },
                { _id: "open", total: 100, amount: 0 },
            ],
            50
        );
        expect(allocations.map((a) => a._id)).toEqual(["open"]);
    });

    it("allocates nothing for a zero amount", () => {
        expect(previewAutoAllocation([{ _id: "i1", total: 100, amount: 0 }], 0).allocations).toEqual([]);
    });
});

describe("allocationError", () => {
    it("passes when the payment is fully allocated", () => {
        expect(allocationError(1000, 1000)).toBe("");
    });

    it("catches an under-allocated payment", () => {
        expect(allocationError(600, 1000)).toContain("₹400.00");
    });

    it("catches an over-allocated payment", () => {
        expect(allocationError(1200, 1000)).toContain("more than");
    });

    it("rejects a zero amount", () => {
        expect(allocationError(0, 0)).toBe("Enter an amount greater than 0.");
    });

    it("tolerates rounding noise within a paisa", () => {
        expect(allocationError(999.995, 1000)).toBe("");
    });
});
