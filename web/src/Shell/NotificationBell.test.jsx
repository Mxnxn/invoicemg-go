import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import NotificationBell from "./NotificationBell";

const items = [
    { _id: "po1", poNumber: "PO/26-27/000001", supplierName: "Ravi Papers", total: 5000, date: "2026-09-01", ageDays: 3 },
    { _id: "po2", poNumber: "PO/26-27/000002", supplierName: "Sunil Inks", total: 1200, date: "2026-09-02", ageDays: 0 },
];

const open = () => fireEvent.click(screen.getByRole("button", { name: /pending approvals/i }));

describe("NotificationBell", () => {
    it("shows no badge when nothing is pending", () => {
        render(<NotificationBell count={0} items={[]} onSelect={vi.fn()} onOpen={vi.fn()} />);
        // A badge reading "0" is noise - the absence of the badge IS the message.
        expect(screen.queryByTestId("bell-badge")).toBeNull();
    });

    it("shows the pending count", () => {
        render(<NotificationBell count={7} items={items} onSelect={vi.fn()} onOpen={vi.fn()} />);
        expect(screen.getByTestId("bell-badge").textContent).toBe("7");
    });

    it("caps a large count so the dot keeps its shape", () => {
        render(<NotificationBell count={143} items={items} onSelect={vi.fn()} onOpen={vi.fn()} />);
        expect(screen.getByTestId("bell-badge").textContent).toBe("99+");
    });

    it("lists the pending orders with supplier and age once opened", () => {
        render(<NotificationBell count={2} items={items} onSelect={vi.fn()} onOpen={vi.fn()} />);
        open();
        expect(screen.getByText("PO/26-27/000001")).toBeTruthy();
        expect(screen.getByText(/Ravi Papers/)).toBeTruthy();
        expect(screen.getByText(/3 days/)).toBeTruthy();
        // Raised today should not read "0 days".
        expect(screen.getByText(/today/i)).toBeTruthy();
    });

    it("hands back the id of the order picked", () => {
        const onSelect = vi.fn();
        render(<NotificationBell count={2} items={items} onSelect={onSelect} onOpen={vi.fn()} />);
        open();
        fireEvent.click(screen.getByText("PO/26-27/000002"));
        expect(onSelect).toHaveBeenCalledWith("po2");
    });

    it("says so when nothing is waiting", () => {
        render(<NotificationBell count={0} items={[]} onSelect={vi.fn()} onOpen={vi.fn()} />);
        open();
        expect(screen.getByText(/nothing waiting for approval/i)).toBeTruthy();
    });

    it("refreshes when opened, so a stale badge corrects itself on the click", () => {
        const onOpen = vi.fn();
        render(<NotificationBell count={2} items={items} onSelect={vi.fn()} onOpen={onOpen} />);
        open();
        expect(onOpen).toHaveBeenCalled();
    });

    it("closes on Escape", () => {
        render(<NotificationBell count={2} items={items} onSelect={vi.fn()} onOpen={vi.fn()} />);
        open();
        expect(screen.getByText("PO/26-27/000001")).toBeTruthy();
        fireEvent.keyDown(document, { key: "Escape" });
        expect(screen.queryByText("PO/26-27/000001")).toBeNull();
    });
});

describe("NotificationBell mark as read", () => {
    it("marks one order as read", () => {
        const onDismiss = vi.fn();
        render(<NotificationBell count={2} totalPending={2} items={items} onDismiss={onDismiss} onOpen={vi.fn()} />);
        open();
        fireEvent.click(screen.getByRole("button", { name: /Mark PO\/26-27\/000001 as read/i }));
        expect(onDismiss).toHaveBeenCalledWith("po1");
    });

    // Dismissing must not also open the order. The X is a sibling of the row button rather
    // than nested inside it, which is what keeps one click from doing both.
    it("does not open the order it is dismissing", () => {
        const onSelect = vi.fn();
        render(
            <NotificationBell count={2} totalPending={2} items={items} onDismiss={vi.fn()} onSelect={onSelect} onOpen={vi.fn()} />
        );
        open();
        fireEvent.click(screen.getByRole("button", { name: /Mark PO\/26-27\/000001 as read/i }));
        expect(onSelect).not.toHaveBeenCalled();
    });

    // No id means "all of them" - the panel's footer button.
    it("marks everything as read at once", () => {
        const onDismiss = vi.fn();
        render(<NotificationBell count={2} totalPending={2} items={items} onDismiss={onDismiss} onOpen={vi.fn()} />);
        open();
        fireEvent.click(screen.getByRole("button", { name: /Mark all as read/i }));
        expect(onDismiss).toHaveBeenCalledWith(null);
    });

    // A button that does nothing reads as broken, and there is nothing to mark on an empty
    // list.
    it("offers neither control when there is nothing listed", () => {
        render(<NotificationBell count={0} totalPending={0} items={[]} onDismiss={vi.fn()} onOpen={vi.fn()} />);
        open();
        expect(screen.queryByRole("button", { name: /Mark all as read/i })).toBeNull();
        expect(screen.queryByRole("button", { name: /as read/i })).toBeNull();
    });

    // The important one. "Nothing new" and "nothing to do" are different facts, and a bell
    // that said the second while orders sat unapproved would be lying about its whole job.
    it("says what is still waiting once everything has been marked read", () => {
        render(<NotificationBell count={0} totalPending={3} items={[]} onDismiss={vi.fn()} onOpen={vi.fn()} />);
        open();
        expect(screen.getByText(/Nothing new/i)).toBeDefined();
        expect(screen.getByText(/3 orders are still waiting for approval/i)).toBeDefined();
        expect(screen.queryByText(/Nothing waiting for approval/i)).toBeNull();
    });

    it("says nothing is waiting when nothing genuinely is", () => {
        render(<NotificationBell count={0} totalPending={0} items={[]} onDismiss={vi.fn()} onOpen={vi.fn()} />);
        open();
        expect(screen.getByText(/Nothing waiting for approval/i)).toBeDefined();
    });

    // No id: the list, not one order - otherwise the link navigates to ?open=null.
    it("links to the list without an order id", () => {
        const onSelect = vi.fn();
        render(<NotificationBell count={0} totalPending={3} items={[]} onSelect={onSelect} onDismiss={vi.fn()} onOpen={vi.fn()} />);
        open();
        fireEvent.click(screen.getByRole("button", { name: /Open purchase orders/i }));
        expect(onSelect).toHaveBeenCalledWith(null);
    });
});
