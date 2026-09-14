import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { AuroraBackground } from "./aurora-background";
import { DotPattern, GridPattern } from "./grid-pattern";
import { Marquee } from "./marquee";
import { Meteors } from "./meteors";
import { RetroGrid } from "./retro-grid";
import { ShimmerButton } from "./shimmer-button";
import { AnimatedGradientText } from "./animated-gradient-text";
import { OrbitingCircles } from "./orbiting-circles";

/** Every utility class in src/Landing must carry the tw: prefix. */
function assertPrefixed(element) {
    const seen = [];
    element.querySelectorAll("[class]").forEach((node) => {
        // SVG className is an SVGAnimatedString, not a string.
        const value = typeof node.className === "string" ? node.className : node.className.baseVal;
        value
            .split(/\s+/)
            .filter(Boolean)
            .forEach((c) => seen.push(c));
    });
    const unprefixed = seen.filter((c) => !c.startsWith("tw:") && c !== "velora-root");
    expect(unprefixed).toEqual([]);
}

describe("CSS-driven velora primitives", () => {
    it("AuroraBackground renders decorative layers", () => {
        const { container } = render(<AuroraBackground />);
        expect(container.firstChild).not.toBeNull();
        assertPrefixed(container);
    });

    it("GridPattern and DotPattern render svgs", () => {
        const { container } = render(
            <div>
                <GridPattern />
                <DotPattern />
            </div>
        );
        expect(container.querySelectorAll("svg")).toHaveLength(2);
        assertPrefixed(container);
    });

    it("Marquee duplicates its children so the loop is seamless", () => {
        render(
            <Marquee>
                <span>ledger</span>
            </Marquee>
        );
        expect(screen.getAllByText("ledger").length).toBeGreaterThan(1);
    });

    it("Meteors renders the requested number of streaks", () => {
        const { container } = render(<Meteors number={7} />);
        expect(container.querySelectorAll("span").length).toBeGreaterThanOrEqual(7);
    });

    it("RetroGrid renders without crashing", () => {
        const { container } = render(<RetroGrid />);
        assertPrefixed(container);
    });

    it("ShimmerButton renders a button with its label", () => {
        render(<ShimmerButton>Sign in</ShimmerButton>);
        expect(screen.getByRole("button", { name: /Sign in/ })).toBeDefined();
    });

    it("AnimatedGradientText renders its children", () => {
        render(<AnimatedGradientText>challans.</AnimatedGradientText>);
        expect(screen.getByText("challans.")).toBeDefined();
    });

    it("OrbitingCircles renders one orbit per child", () => {
        const { container } = render(
            <OrbitingCircles radius={80}>
                <span>a</span>
                <span>b</span>
            </OrbitingCircles>
        );
        expect(container.textContent).toContain("a");
        expect(container.textContent).toContain("b");
    });
});
