import { render } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";

import LandingPage from "./index";

function renderPage() {
    return render(
        <MemoryRouter>
            <LandingPage />
        </MemoryRouter>
    );
}

describe("LandingPage", () => {
    it("renders the whole page without crashing", () => {
        const { container } = renderPage();
        expect(container.querySelector(".velora-root")).not.toBeNull();
    });

    it("renders every anchored section the nav points at", () => {
        const { container } = renderPage();
        expect(container.querySelector("#features")).not.toBeNull();
        expect(container.querySelector("#workflow")).not.toBeNull();
        expect(container.querySelector("#faq")).not.toBeNull();
    });

    it("uses no unprefixed tailwind utility classes", () => {
        const { container } = renderPage();
        const offenders = [];
        container.querySelectorAll("[class]").forEach((node) => {
            const value = typeof node.className === "string" ? node.className : node.className.baseVal;
            value
                .split(/\s+/)
                .filter(Boolean)
                // Ours: the tw: prefix and the one wrapper class. lucide-react
                // stamps its own `lucide lucide-<icon>` classes on every svg -
                // not Tailwind utilities, so they cannot collide with Bootstrap.
                .filter((c) => !c.startsWith("tw:") && c !== "velora-root" && !c.startsWith("lucide"))
                .forEach((c) => offenders.push(c));
        });
        expect(offenders).toEqual([]);
    });
});
