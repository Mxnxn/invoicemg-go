import { fireEvent, render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useRef } from "react";
import { useSmoothAnchors } from "./useSmoothAnchors";

const scrollIntoView = vi.fn();
const focus = vi.fn();

function Page({ href = "#contact" }) {
    const ref = useRef(null);
    useSmoothAnchors(ref);
    return (
        <div ref={ref}>
            <a href={href}>Request a demo</a>
            <section id="contact">Contact</section>
        </div>
    );
}

beforeEach(() => {
    vi.clearAllMocks();
    Element.prototype.scrollIntoView = scrollIntoView;
    HTMLElement.prototype.focus = focus;
    window.matchMedia = vi.fn().mockReturnValue({ matches: false });
});

describe("useSmoothAnchors", () => {
    it("glides to the section instead of jumping", () => {
        const { getByText } = render(<Page />);
        fireEvent.click(getByText("Request a demo"));
        expect(scrollIntoView).toHaveBeenCalledWith({ behavior: "smooth", block: "start" });
    });

    // "smooth" ignores prefers-reduced-motion in every engine, so it has to be honoured here
    // or a reader who asked for less movement gets a long glide anyway.
    it("jumps instantly when the reader asked for reduced motion", () => {
        window.matchMedia = vi.fn().mockReturnValue({ matches: true });
        const { getByText } = render(<Page />);
        fireEvent.click(getByText("Request a demo"));
        expect(scrollIntoView).toHaveBeenCalledWith({ behavior: "auto", block: "start" });
    });

    // The scroll moves the viewport, not focus - without this the next Tab continues from
    // where the reader was, and the page appears to move for everyone except them.
    it("moves focus to the section", () => {
        const { getByText } = render(<Page />);
        fireEvent.click(getByText("Request a demo"));
        expect(focus).toHaveBeenCalledWith({ preventScroll: true });
    });

    // A link to a section that is not on the page must behave as it always did.
    it("leaves an unknown anchor alone", () => {
        const { getByText } = render(<Page href="#nowhere" />);
        fireEvent.click(getByText("Request a demo"));
        expect(scrollIntoView).not.toHaveBeenCalled();
    });

    // Modified clicks belong to the browser - hijacking them breaks "open in new tab".
    it("does not hijack a ctrl-click", () => {
        const { getByText } = render(<Page />);
        fireEvent.click(getByText("Request a demo"), { ctrlKey: true });
        expect(scrollIntoView).not.toHaveBeenCalled();
    });
});
