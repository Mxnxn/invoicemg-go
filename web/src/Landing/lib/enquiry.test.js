import { describe, expect, it } from "vitest";

import { enquiryMessage, greetingName, normalisePhone } from "./enquiry";

describe("normalisePhone", () => {
    it("prepends the country code to a local number", () => {
        expect(normalisePhone("99999 99999")).toBe("+919999999999");
    });

    it("strips punctuation people actually type", () => {
        expect(normalisePhone("(99999) 99-999")).toBe("+919999999999");
    });

    it("keeps an international number's own code", () => {
        expect(normalisePhone("+44 20 7946 0958")).toBe("+442079460958");
    });

    it("does not double the code when it is pasted without a plus", () => {
        expect(normalisePhone("919999999999")).toBe("+919999999999");
    });

    it("returns empty for nothing usable, so the caller can complain", () => {
        expect(normalisePhone("")).toBe("");
        expect(normalisePhone("   ")).toBe("");
        expect(normalisePhone("abc")).toBe("");
    });
});

describe("enquiryMessage", () => {
    const base = {
        name: "Ramesh Patel",
        email: "ramesh@example.com",
        phone: "+919999999999",
        companyName: "Acme Signs",
        note: "We raise 40 challans a week by hand.",
    };

    it("carries every field the form collected", () => {
        const message = enquiryMessage(base);
        expect(message).toContain("Ramesh Patel");
        expect(message).toContain("Acme Signs");
        expect(message).toContain("+919999999999");
        expect(message).toContain("ramesh@example.com");
        expect(message).toContain("40 challans");
    });

    it("omits the optional lines rather than printing empty ones", () => {
        const message = enquiryMessage({ ...base, companyName: "", note: "" });
        expect(message).not.toContain("Company:");
        expect(message).not.toContain("What I need:");
        expect(message).toContain("Ramesh Patel");
    });
});

describe("greetingName", () => {
    it("capitalises the first name, matching what the server stores", () => {
        expect(greetingName("sunil verma")).toBe("Sunil");
        expect(greetingName("RAMESH")).toBe("RAMESH");
        expect(greetingName("  kavita  r  ")).toBe("Kavita");
    });

    it("is empty when there is no name to greet", () => {
        expect(greetingName("")).toBe("");
        expect(greetingName(null)).toBe("");
    });
});
