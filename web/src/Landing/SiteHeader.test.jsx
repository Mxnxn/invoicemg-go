import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it } from "vitest";

import { SiteHeader } from "./SiteHeader";

function renderHeader() {
    return render(
        <MemoryRouter>
            <SiteHeader />
        </MemoryRouter>
    );
}

afterEach(() => {
    window.localStorage.clear();
    document.documentElement.removeAttribute("data-theme");
});

describe("SiteHeader", () => {
    it("invites a signed-out visitor to sign in", () => {
        renderHeader();
        expect(screen.getByRole("link", { name: /Sign in/i })).toBeDefined();
    });

    it("sends a signed-in visitor to the dashboard", () => {
        window.localStorage.setItem("uid", "abc123");
        renderHeader();
        expect(screen.getByRole("link", { name: /Go to dashboard/i })).toBeDefined();
    });

    it("offers a demo request that points at the form", () => {
        renderHeader();
        const link = screen.getByRole("link", { name: /Request a demo/i });
        expect(link.getAttribute("href")).toBe("#contact");
    });

    it("toggles the document theme attribute and persists it", () => {
        document.documentElement.setAttribute("data-theme", "light");
        renderHeader();
        fireEvent.click(screen.getByRole("button", { name: /toggle theme/i }));
        expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
        expect(window.localStorage.getItem("theme")).toBe("dark");
    });
});
