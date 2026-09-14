import React from "react";
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import NumberBadge from "./NumberBadge";

// The accent marks the row's OWN document. Every document list now uses it, so this pins the
// two shapes the component can take rather than letting one drift.
describe("NumberBadge accent", () => {
    it("marks a plain number", () => {
        const { container } = render(<NumberBadge accent>INV/1</NumberBadge>);
        expect(container.querySelector(".xan-number-badge.is-accent")).toBeTruthy();
        expect(container.querySelector("button")).toBeNull();
    });

    it("marks a number that opens something, and stays a button", () => {
        const { container } = render(<NumberBadge accent onClick={() => {}}>INV/1</NumberBadge>);
        const el = container.querySelector("button.xan-number-badge");
        expect(el.classList.contains("is-accent")).toBe(true);
        expect(el.classList.contains("is-action")).toBe(true);
    });

    it("stays neutral when not asked", () => {
        const { container } = render(<NumberBadge onClick={() => {}}>INV/1</NumberBadge>);
        expect(container.querySelector(".is-accent")).toBeNull();
    });

    it("still renders nothing for an absent number", () => {
        const { container } = render(<NumberBadge accent>{""}</NumberBadge>);
        expect(container.firstChild).toBeNull();
    });
});
