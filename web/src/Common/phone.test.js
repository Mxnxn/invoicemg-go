import { describe, it, expect } from "vitest";
import { normalizePhone, isValidPhone, phoneRule } from "./phone";

describe("normalizePhone", () => {
    it("keeps a bare 10-digit number", () => {
        expect(normalizePhone("9876543210")).toBe("9876543210");
    });

    it("accepts the format the fields advertise in their placeholder", () => {
        // The bug: this exact string failed /^\d{10}$/, disabling the submit button.
        expect(normalizePhone("+91 98765 43210")).toBe("9876543210");
    });

    it("strips separators people actually type", () => {
        expect(normalizePhone("98765-43210")).toBe("9876543210");
        expect(normalizePhone("(98765) 43210")).toBe("9876543210");
        expect(normalizePhone(" 98765 43210 ")).toBe("9876543210");
    });

    it("drops a leading zero", () => {
        expect(normalizePhone("09876543210")).toBe("9876543210");
    });

    it("leaves a real 10-digit number that happens to start with 91", () => {
        expect(normalizePhone("9198765432")).toBe("9198765432");
    });

    it("handles empty and nullish input without throwing", () => {
        expect(normalizePhone("")).toBe("");
        expect(normalizePhone(null)).toBe("");
        expect(normalizePhone(undefined)).toBe("");
    });
});

describe("isValidPhone / phoneRule", () => {
    it("accepts a 10-digit number however it was typed", () => {
        expect(isValidPhone("+91 98765 43210")).toBe(true);
        expect(phoneRule("+91 98765 43210")).toBe(null);
    });

    it("rejects too few and too many digits", () => {
        expect(isValidPhone("98765")).toBe(false);
        expect(isValidPhone("98765432101234")).toBe(false);
        expect(phoneRule("98765")).toMatch(/10-digit/);
    });

    it("rejects a blank value", () => {
        expect(isValidPhone("")).toBe(false);
    });
});
