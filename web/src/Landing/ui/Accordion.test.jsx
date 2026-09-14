import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Accordion } from "./Accordion";

const items = [
    { q: "Is InvoiceMG GST-ready?", a: "Yes, invoices carry HSN and tax splits." },
    { q: "Can I self-host it?", a: "Yes, it ships with docker-compose." },
];

describe("Accordion", () => {
    it("renders every question as a collapsed trigger", () => {
        render(<Accordion items={items} />);
        const triggers = screen.getAllByRole("button");
        expect(triggers).toHaveLength(2);
        expect(triggers[0].getAttribute("aria-expanded")).toBe("false");
    });

    it("opens the clicked panel", () => {
        render(<Accordion items={items} />);
        const trigger = screen.getByRole("button", { name: /GST-ready/ });
        fireEvent.click(trigger);
        expect(trigger.getAttribute("aria-expanded")).toBe("true");
        expect(screen.getByText(/HSN and tax splits/)).toBeDefined();
    });

    it("closes the open panel when its trigger is clicked again", () => {
        render(<Accordion items={items} />);
        const trigger = screen.getByRole("button", { name: /GST-ready/ });
        fireEvent.click(trigger);
        fireEvent.click(trigger);
        expect(trigger.getAttribute("aria-expanded")).toBe("false");
    });

    it("keeps only one panel open at a time", () => {
        render(<Accordion items={items} />);
        const [first, second] = screen.getAllByRole("button");
        fireEvent.click(first);
        fireEvent.click(second);
        expect(first.getAttribute("aria-expanded")).toBe("false");
        expect(second.getAttribute("aria-expanded")).toBe("true");
    });
});
