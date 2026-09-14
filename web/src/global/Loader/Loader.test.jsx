import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import Loader from "./Loader";
import LoaderComponent from "./LoaderComponent";

describe("Loader", () => {
    // A shape with no text announces nothing. Anyone not looking at the screen has
    // only this to tell them the page is working rather than broken.
    it("announces itself to a screen reader", () => {
        render(<Loader />);
        expect(screen.getByRole("status")).toBeDefined();
        expect(screen.getByText(/Loading/)).toBeDefined();
    });

    it("takes a more specific label when the caller has one", () => {
        render(<Loader label="Loading invoices" />);
        expect(screen.getByText(/Loading invoices/)).toBeDefined();
    });

    // Every existing call site is a bare <Loader />, so no props must throw.
    it("renders with no props at all", () => {
        expect(() => render(<Loader />)).not.toThrow();
        expect(() => render(<LoaderComponent />)).not.toThrow();
    });

    // The inline one goes inside forms and rows - a <div> there is invalid inside a
    // <p> or a <span>, and React will not warn about the resulting DOM nesting.
    it("renders the inline loader as a span", () => {
        const { container } = render(<LoaderComponent />);
        expect(container.firstChild.tagName).toBe("SPAN");
    });

    // The colour comes from a token via currentColor. A hardcoded hex - which is
    // what the old MoonLoader had - follows neither theme.
    it("carries no hardcoded colour", () => {
        const { container } = render(<Loader />);
        expect(container.firstChild.getAttribute("style")).toBeNull();
    });
});
