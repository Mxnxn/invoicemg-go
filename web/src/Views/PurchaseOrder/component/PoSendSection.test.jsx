import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const can = vi.fn();
vi.mock("../../../Common/access", () => ({ hasAccess: (...a) => can(...a) }));

import PoSendSection from "./PoSendSection";

const state = (over = {}) => ({
    status: "Created",
    canShare: true,
    canConfirm: true,
    isUpdate: false,
    sentAt: null,
    count: 0,
    confirmedAt: null,
    ...over,
});

beforeEach(() => {
    vi.clearAllMocks();
    can.mockReturnValue(true);
});

describe("PoSendSection", () => {
    it("shows the current status", () => {
        render(<PoSendSection send={state({ status: "Modified" })} onShare={vi.fn()} onConfirm={vi.fn()} busy="" />);
        expect(screen.getByText("Modified")).toBeTruthy();
    });

    it("names the order send plainly, and says it is an update the second time", () => {
        const { rerender } = render(<PoSendSection send={state()} onShare={vi.fn()} onConfirm={vi.fn()} busy="" />);
        expect(screen.getByRole("button", { name: /send order to supplier/i })).toBeTruthy();
        rerender(<PoSendSection send={state({ isUpdate: true, status: "Modified" })} onShare={vi.fn()} onConfirm={vi.fn()} busy="" />);
        expect(screen.getByRole("button", { name: /send updated order/i })).toBeTruthy();
    });

    it("disables sharing when the supplier already has this version, and says why", () => {
        render(<PoSendSection send={state({ status: "Sent", canShare: false, count: 1, isUpdate: true })} onShare={vi.fn()} onConfirm={vi.fn()} busy="" />);
        expect(screen.getByRole("button", { name: /send updated order/i }).disabled).toBe(true);
        // An inert button with no explanation is the thing to avoid.
        expect(screen.getByText(/already has this version/i)).toBeTruthy();
    });

    it("hands the clicks back", () => {
        const onShare = vi.fn();
        const onConfirm = vi.fn();
        render(<PoSendSection send={state()} onShare={onShare} onConfirm={onConfirm} busy="" />);
        fireEvent.click(screen.getByRole("button", { name: /send order to supplier/i }));
        fireEvent.click(screen.getByRole("button", { name: /confirm dispatch/i }));
        expect(onShare).toHaveBeenCalled();
        expect(onConfirm).toHaveBeenCalled();
    });

    it("hides both sends from anyone without the permission", () => {
        can.mockReturnValue(false);
        render(<PoSendSection send={state()} onShare={vi.fn()} onConfirm={vi.fn()} busy="" />);
        expect(screen.queryByRole("button", { name: /send order to supplier/i })).toBeNull();
        expect(screen.queryByRole("button", { name: /confirm dispatch/i })).toBeNull();
    });

    it("shows when it was last sent", () => {
        render(
            <PoSendSection
                send={state({ status: "Sent", canShare: false, count: 1, isUpdate: true, sentAt: "2026-09-12T10:00:00Z" })}
                onShare={vi.fn()}
                onConfirm={vi.fn()}
                busy=""
            />
        );
        expect(screen.getByText(/last sent/i)).toBeTruthy();
    });
});
