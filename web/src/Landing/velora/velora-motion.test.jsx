import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { BlurFade } from "./blur-fade";
import { TextReveal } from "./text-reveal";
import { Typewriter } from "./typewriter";
import { NumberTicker } from "./number-ticker";
import { ScrollProgress } from "./scroll-progress";
import { AnimatedList } from "./animated-list";

describe("motion-driven velora primitives", () => {
    it("BlurFade renders its children immediately", () => {
        render(
            <BlurFade>
                <p>Run your print shop on one system</p>
            </BlurFade>
        );
        expect(screen.getByText(/print shop/)).toBeDefined();
    });

    it("TextReveal renders the full text content", () => {
        const { container } = render(<TextReveal text="quote to dispatch" />);
        expect(container.textContent.replace(/\s+/g, " ")).toContain("quote to dispatch");
    });

    it("Typewriter renders at least the first word", () => {
        const { container } = render(<Typewriter words={["invoices.", "challans."]} />);
        expect(container.textContent.length).toBeGreaterThan(0);
    });

    it("NumberTicker renders prefix and suffix around the value", () => {
        const { container } = render(<NumberTicker value={5000} prefix="" suffix="+" />);
        expect(container.textContent).toContain("+");
    });

    it("ScrollProgress renders a progress bar element", () => {
        const { container } = render(<ScrollProgress />);
        expect(container.firstChild).not.toBeNull();
    });

    it("AnimatedList renders its children", () => {
        render(
            <AnimatedList>
                <li>Invoice paid</li>
                <li>Challan received</li>
            </AnimatedList>
        );
        expect(screen.getByText("Invoice paid")).toBeDefined();
    });
});
