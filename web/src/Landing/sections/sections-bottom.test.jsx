import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";

import { Workflow } from "./Workflow";
import { Activity } from "./Activity";
import { Differentiators } from "./Differentiators";
import { Faq } from "./Faq";
import { Cta } from "./Cta";

describe("bottom-of-page sections", () => {
    it("Workflow is anchored for the header nav", () => {
        const { container } = render(<Workflow />);
        expect(container.querySelector("#workflow")).not.toBeNull();
    });

    it("Activity describes the job lifecycle", () => {
        const { container } = render(<Activity />);
        expect(container.textContent).toMatch(/lifecycle/i);
    });

    it("Differentiators names the three product strengths", () => {
        const { container } = render(<Differentiators />);
        expect(container.textContent).toContain("Multi-company, per tab");
        expect(container.textContent).toContain("Roles and permissions");
    });

    it("Faq renders collapsed questions and is anchored", () => {
        const { container } = render(<Faq />);
        expect(container.querySelector("#faq")).not.toBeNull();
        expect(screen.getAllByRole("button").length).toBeGreaterThanOrEqual(4);
    });

    it("Cta links through to the admin panel", () => {
        render(
            <MemoryRouter>
                <Cta />
            </MemoryRouter>
        );
        const links = screen.getAllByRole("link");
        expect(links.some((link) => link.getAttribute("href") === "/admin")).toBe(true);
    });
});
