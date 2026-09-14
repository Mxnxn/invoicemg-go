import { describe, it, expect } from "vitest";
import { daysUntil, expiryLevel, expiryLabel, WARN_DAYS, URGENT_DAYS } from "./accountExpiry";

const NOW = new Date("2026-08-27T12:00:00Z").getTime();
const inDays = (n) => new Date(NOW + n * 24 * 60 * 60 * 1000).toISOString();

describe("daysUntil", () => {
    it("counts whole days ahead", () => {
        expect(daysUntil(inDays(45), NOW)).toBe(45);
    });

    it("is null when no expiry is set", () => {
        expect(daysUntil(null, NOW)).toBeNull();
        expect(daysUntil("", NOW)).toBeNull();
        expect(daysUntil("not a date", NOW)).toBeNull();
    });

    it("goes negative once past", () => {
        expect(daysUntil(inDays(-3), NOW)).toBeLessThan(0);
    });
});

describe("expiryLevel", () => {
    it("is null when the account never expires", () => {
        expect(expiryLevel(null, NOW)).toBeNull();
    });

    it("is ok comfortably ahead", () => {
        expect(expiryLevel(inDays(WARN_DAYS + 1), NOW)).toBe("ok");
    });

    // The whole point of the request: inside a month it has to look different.
    it("warns within a month", () => {
        expect(expiryLevel(inDays(WARN_DAYS), NOW)).toBe("warn");
        expect(expiryLevel(inDays(URGENT_DAYS + 1), NOW)).toBe("warn");
    });

    it("is urgent within a week", () => {
        expect(expiryLevel(inDays(URGENT_DAYS), NOW)).toBe("urgent");
        expect(expiryLevel(inDays(1), NOW)).toBe("urgent");
    });

    it("is expired on or after the date", () => {
        expect(expiryLevel(inDays(0), NOW)).toBe("expired");
        expect(expiryLevel(inDays(-1), NOW)).toBe("expired");
    });
});

describe("expiryLabel", () => {
    it("reads naturally at each boundary", () => {
        expect(expiryLabel(null, NOW)).toBe("No expiry");
        expect(expiryLabel(inDays(-1), NOW)).toBe("Expired");
        expect(expiryLabel(inDays(1), NOW)).toBe("Expires tomorrow");
        expect(expiryLabel(inDays(20), NOW)).toBe("Expires in 20 days");
    });
});
