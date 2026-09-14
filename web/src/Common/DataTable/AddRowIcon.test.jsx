import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import AddRowIcon from "./AddRowIcon";

// The icon carries no state and no timer: it maps `open` onto a class, and one CSS rule
// rotates it. So what is worth pinning is that mapping, and that the three strokes the
// rotation depends on are all present - a missing one still renders something plus-shaped
// when flat, and a broken cross only when rotated.
const iconIn = (container) => container.querySelector(".addrow-icon");

describe("AddRowIcon", () => {
    it("is a plus when the menu is closed", () => {
        const { container } = render(<AddRowIcon open={false} />);
        expect(iconIn(container).classList.contains("is-plus")).toBe(true);
        expect(iconIn(container).classList.contains("is-x")).toBe(false);
    });

    it("is a cross when the menu is open", () => {
        const { container } = render(<AddRowIcon open />);
        expect(iconIn(container).classList.contains("is-x")).toBe(true);
        expect(iconIn(container).classList.contains("is-plus")).toBe(false);
    });

    it("draws all three strokes", () => {
        const { container } = render(<AddRowIcon open={false} />);
        expect(container.querySelector(".addrow-icon-shaft")).not.toBeNull();
        expect(container.querySelector(".addrow-icon-left")).not.toBeNull();
        expect(container.querySelector(".addrow-icon-right")).not.toBeNull();
    });

    // The label beside it is what names the button; the icon repeating it would be read out
    // twice by a screen reader.
    it("is hidden from assistive technology", () => {
        const { container } = render(<AddRowIcon open={false} />);
        expect(iconIn(container).getAttribute("aria-hidden")).toBe("true");
        expect(iconIn(container).getAttribute("focusable")).toBe("false");
    });

    it("honours the size it is given", () => {
        const { container } = render(<AddRowIcon open={false} size={20} />);
        expect(iconIn(container).getAttribute("width")).toBe("20");
        expect(iconIn(container).getAttribute("height")).toBe("20");
    });
});
