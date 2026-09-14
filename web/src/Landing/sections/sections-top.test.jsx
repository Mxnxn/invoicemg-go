import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it } from "vitest";

import { Hero } from "./Hero";
import { Stats } from "./Stats";
import { Features } from "./Features";

afterEach(() => window.localStorage.clear());

describe("top-of-page sections", () => {
    it("Hero states what the product does", () => {
        const { container } = render(
            <MemoryRouter>
                <Hero />
            </MemoryRouter>
        );
        expect(container.textContent).toMatch(/print/i);
    });

    it("Hero's primary call to action adapts to the signed-in state", () => {
        window.localStorage.setItem("uid", "abc123");
        render(
            <MemoryRouter>
                <Hero />
            </MemoryRouter>
        );
        expect(screen.getByRole("link", { name: /Go to dashboard/i })).toBeDefined();
    });

    it("Stats renders four figures", () => {
        const { container } = render(<Stats />);
        expect(container.querySelectorAll("[data-stat]")).toHaveLength(4);
    });

    it("Features names the four product areas", () => {
        const { container } = render(<Features />);
        expect(container.textContent).toContain("Invoices & challans");
        expect(container.textContent).toContain("Job lifecycle queue");
        expect(container.textContent).toContain("Ledger & GST reports");
        expect(container.textContent).toContain("Analytics");
    });

    it("Features is anchored for the header nav", () => {
        const { container } = render(<Features />);
        expect(container.querySelector("#features")).not.toBeNull();
    });
});
