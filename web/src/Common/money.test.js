import { describe, it, expect } from "vitest";
import { round2, formatAmount } from "./money";

describe("money", () => {
	it("trims float tails", () => {
		expect(round2(67.799999999)).toBe(67.8);
		expect(round2(69.999999997)).toBe(70);
		expect(round2(0.1 + 0.2)).toBe(0.3);
	});

	it("rounds half up the way a person expects", () => {
		expect(round2(1.005)).toBe(1.01);
		expect(round2(2.675)).toBe(2.68);
	});

	it("leaves clean values alone", () => {
		expect(round2(10)).toBe(10);
		expect(round2(10.5)).toBe(10.5);
	});

	it("handles junk without throwing", () => {
		expect(round2(null)).toBe(0);
		expect(round2(undefined)).toBe(0);
		expect(round2("abc")).toBe(0);
		expect(round2("12.349")).toBe(12.35);
	});

	it("formats to exactly two decimals", () => {
		expect(formatAmount(67.799999999)).toBe("67.80");
		expect(formatAmount(10)).toBe("10.00");
		expect(formatAmount(0)).toBe("0.00");
	});
});
