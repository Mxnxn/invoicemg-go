import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import AssigneeDropdown from "./AssigneeDropdown";

const options = [
    { id: "1", name: "Acme Signs" },
    { id: "2", name: "Bolt Print" },
    { id: "3", name: "Zenith Media" },
];

const open = (props = {}) => {
    render(<AssigneeDropdown value="" placeholder="Select client" options={options} onSelect={vi.fn()} {...props} />);
    fireEvent.click(screen.getByText("Select client"));
};

// The order of the menu, read off the DOM rather than assumed.
const menuOrder = () =>
    [...document.querySelectorAll(".xan-row-menu-item")].map((el) => el.textContent.trim()).filter(Boolean);

describe("AssigneeDropdown", () => {
    // Below the list it moved as you typed, and with forty customers it sat off the bottom of
    // the scroll - "add a new one" was only reachable by scrolling past every one that already
    // existed. At the top it is in the same place every time.
    it("offers Create above the results, not below them", () => {
        open({ onCreateNew: vi.fn() });
        const order = menuOrder();
        expect(order[0]).toMatch(/Create new/);
        expect(order[1]).toBe("Acme Signs");
    });

    it("keeps Create at the top once a search narrows the list", () => {
        open({ onCreateNew: vi.fn() });
        fireEvent.change(screen.getByPlaceholderText(/Search/i), { target: { value: "bolt" } });
        const order = menuOrder();
        expect(order[0]).toMatch(/Create "bolt"/);
        expect(order).toContain("Bolt Print");
        expect(order).not.toContain("Acme Signs");
    });

    // With nothing typed it still opens the form - it used to refuse and pulse the box red.
    it("opens the create form with an empty search box", () => {
        const onCreateNew = vi.fn();
        open({ onCreateNew });
        fireEvent.click(screen.getByText("Create new"));
        expect(onCreateNew).toHaveBeenCalledWith("");
    });

    // A dropdown that cannot create anything must not imply it can.
    it("shows no create row when the caller offers none", () => {
        open();
        expect(menuOrder()).not.toContain("Create new");
        expect(menuOrder()[0]).toBe("Acme Signs");
    });

    it("still selects an option", () => {
        const onSelect = vi.fn();
        open({ onSelect, onCreateNew: vi.fn() });
        fireEvent.click(screen.getByText("Bolt Print"));
        expect(onSelect).toHaveBeenCalledWith("2", "Bolt Print");
    });
});
