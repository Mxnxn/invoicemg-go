import { describe, expect, it } from "vitest";
import { collectionRate, concentration, conversionRate, netPosition } from "./analyticsMath";

describe("concentration", () => {
    const sales = [{ total: 50 }, { total: 30 }, { total: 10 }, { total: 6 }, { total: 4 }, { total: 100 }];

    it("reports the top n share of the whole", () => {
        // 100+50+30+10+6 = 196 of 200
        expect(concentration(sales, 5)).toBe(98);
    });

    it("is 0 when there is no revenue, not NaN", () => {
        expect(concentration([], 5)).toBe(0);
        expect(concentration([{ total: 0 }], 5)).toBe(0);
        expect(concentration(null, 5)).toBe(0);
    });
});

describe("conversionRate", () => {
    it("reports jobs as a share of quotations", () => {
        expect(conversionRate(20, 5)).toBe(25);
    });

    it("does not divide by zero", () => {
        expect(conversionRate(0, 5)).toBe(0);
    });
});

describe("collectionRate", () => {
    const data = [
        { billed: 100, collected: 60 },
        { billed: 100, collected: 80 },
    ];

    it("reports collected against billed across the period", () => {
        expect(collectionRate(data)).toBe(70);
    });

    it("is 0 for a period that billed nothing", () => {
        expect(collectionRate([{ billed: 0, collected: 0 }])).toBe(0);
        expect(collectionRate([])).toBe(0);
    });
});

describe("netPosition", () => {
    it("is what is owed to us minus what we owe", () => {
        expect(netPosition(1000, 400)).toBe(600);
        // Negative is a real answer, not an error - we owe more than we are owed.
        expect(netPosition(200, 900)).toBe(-700);
    });
});
