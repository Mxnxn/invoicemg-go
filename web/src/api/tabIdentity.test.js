import { describe, it, expect, beforeEach } from "vitest";
import { getTabId, companyScopedKey } from "./tabIdentity";

describe("tabIdentity", () => {
    beforeEach(() => {
        window.sessionStorage.clear();
    });

    it("returns a stable id across calls within one tab", () => {
        const first = getTabId();
        const second = getTabId();
        expect(first).toBeTruthy();
        expect(second).toBe(first);
    });

    it("persists the id in sessionStorage, not localStorage", () => {
        const id = getTabId();
        expect(window.sessionStorage.getItem("tab_id")).toBe(id);
        expect(window.localStorage.getItem("tab_id")).toBeNull();
    });

    it("mints a new id when sessionStorage is empty, as a fresh tab would", () => {
        const first = getTabId();
        window.sessionStorage.clear();
        expect(getTabId()).not.toBe(first);
    });

    it("namespaces storage keys per company", () => {
        expect(companyScopedKey("inv_number", "abc123")).toBe("inv_number:abc123");
    });

    it("falls back to the bare key when no company is active yet", () => {
        expect(companyScopedKey("inv_number", null)).toBe("inv_number");
    });
});
