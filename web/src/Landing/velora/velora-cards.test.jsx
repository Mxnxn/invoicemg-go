import { createRef } from "react";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { BentoCard, BentoGrid } from "./bento-grid";
import { BorderBeam } from "./border-beam";
import { SpotlightCard } from "./spotlight-card";
import { TiltCard } from "./tilt-card";
import { Dock, DockIcon } from "./dock";
import { AnimatedBeam } from "./animated-beam";

describe("card and layout velora primitives", () => {
    it("BentoCard renders its name and description", () => {
        render(
            <BentoGrid>
                <BentoCard
                    name="Job lifecycle queue"
                    description="Track every job from received to dispatched."
                    background={<div data-testid="bg" />}
                />
            </BentoGrid>
        );
        expect(screen.getByText("Job lifecycle queue")).toBeDefined();
        expect(screen.getByText(/received to dispatched/)).toBeDefined();
        expect(screen.getByTestId("bg")).toBeDefined();
    });

    it("BentoGrid forwards a ref (React 18 forwardRef conversion)", () => {
        const ref = createRef();
        render(<BentoGrid ref={ref}>{null}</BentoGrid>);
        expect(ref.current).not.toBeNull();
    });

    it("BorderBeam renders without crashing", () => {
        const { container } = render(<BorderBeam size={72} duration={7} />);
        expect(container.firstChild).not.toBeNull();
    });

    it("SpotlightCard renders its children", () => {
        render(<SpotlightCard>Multi-company, per tab</SpotlightCard>);
        expect(screen.getByText("Multi-company, per tab")).toBeDefined();
    });

    it("TiltCard renders its children", () => {
        render(<TiltCard>Ledger</TiltCard>);
        expect(screen.getByText("Ledger")).toBeDefined();
    });

    it("DockIcon exposes its label to assistive tech", () => {
        render(
            <Dock>
                <DockIcon label="Invoices">
                    <span>icon</span>
                </DockIcon>
            </Dock>
        );
        // The label is an aria-label, not visible text.
        expect(screen.getByLabelText("Invoices")).toBeDefined();
    });

    it("AnimatedBeam renders an svg without measuring in jsdom", () => {
        const container = createRef();
        const from = createRef();
        const to = createRef();
        const { container: dom } = render(
            <div ref={container}>
                <div ref={from} />
                <div ref={to} />
                <AnimatedBeam containerRef={container} fromRef={from} toRef={to} />
            </div>
        );
        expect(dom.querySelector("svg")).not.toBeNull();
    });
});
