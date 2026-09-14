import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";

const listConversations = vi.fn();
const listMessages = vi.fn();
const sendMessage = vi.fn();

vi.mock("../dev_backend", () => ({
    devBackend: {
        listConversations: (...a) => listConversations(...a),
        listMessages: (...a) => listMessages(...a),
        sendMessage: (...a) => sendMessage(...a),
    },
}));
vi.mock("../../../global/toast", () => ({ notifySuccess: vi.fn(), notifyError: vi.fn() }));

import InboxPanel, { relativeTime } from "./InboxPanel";

const OPEN = {
    _id: "c1",
    waId: "919000000001",
    phone: "+919000000001",
    profileName: "Ramesh",
    connected: true,
    clientFirm: "Acme Signs",
    company: "Manan",
    lastMessageAt: new Date().toISOString(),
    lastMessagePreview: "is my job ready?",
    lastDirection: "in",
    unread: 2,
    windowOpen: true,
};

const STALE = { ...OPEN, _id: "c2", profileName: "Stranger", connected: false, clientFirm: "", unread: 0, windowOpen: false };

beforeEach(() => {
    vi.clearAllMocks();
    listConversations.mockResolvedValue({ code: 200, data: [OPEN, STALE] });
    listMessages.mockResolvedValue({ code: 200, data: [] });
    sendMessage.mockResolvedValue({ code: 200, data: {} });
});

describe("relativeTime", () => {
    const now = Date.UTC(2026, 0, 2, 12, 0, 0);
    it("reads as an age, not a date", () => {
        expect(relativeTime(new Date(now - 30 * 1000), now)).toBe("now");
        expect(relativeTime(new Date(now - 5 * 60 * 1000), now)).toBe("5m");
        expect(relativeTime(new Date(now - 3 * 3600 * 1000), now)).toBe("3h");
        expect(relativeTime(new Date(now - 2 * 86400 * 1000), now)).toBe("2d");
        expect(relativeTime(null, now)).toBe("");
    });
});

describe("InboxPanel", () => {
    it("separates a customer from a stranger", async () => {
        render(<InboxPanel />);
        await waitFor(() => expect(screen.getByText("Acme Signs")).toBeDefined());
        expect(screen.getByText("Not a customer")).toBeDefined();
    });

    it("says plainly when there is nothing, and why", async () => {
        listConversations.mockResolvedValue({ code: 200, data: [] });
        render(<InboxPanel />);
        await waitFor(() => expect(screen.getByText(/no history from before this was switched on/)).toBeDefined());
    });

    it("opens a thread when a conversation is clicked", async () => {
        listMessages.mockResolvedValue({
            code: 200,
            data: [{ _id: "m1", direction: "in", body: "is my job ready?", createdAt: new Date().toISOString() }],
        });
        render(<InboxPanel />);
        await waitFor(() => expect(screen.getByText("Ramesh")).toBeDefined());
        fireEvent.click(screen.getByText("Ramesh"));

        await waitFor(() => expect(listMessages).toHaveBeenCalledWith("c1"));
        await waitFor(() => expect(screen.getByLabelText("Reply")).toBeDefined());
    });

    it("sends the draft and clears it", async () => {
        render(<InboxPanel />);
        await waitFor(() => expect(screen.getByText("Ramesh")).toBeDefined());
        fireEvent.click(screen.getByText("Ramesh"));

        const input = await screen.findByLabelText("Reply");
        fireEvent.change(input, { target: { value: "Ready tomorrow" } });
        fireEvent.click(screen.getByRole("button", { name: /Send/ }));

        await waitFor(() => expect(sendMessage).toHaveBeenCalledWith("c1", "Ready tomorrow"));
        await waitFor(() => expect(screen.getByLabelText("Reply").value).toBe(""));
    });

    it("replaces the composer with the reason once the window has closed", async () => {
        render(<InboxPanel />);
        await waitFor(() => expect(screen.getByText("Stranger")).toBeDefined());
        fireEvent.click(screen.getByText("Stranger"));

        await waitFor(() => expect(screen.getByText(/older than 24 hours/)).toBeDefined());
        // Not a disabled box that looks sendable - there is no input at all.
        expect(screen.queryByLabelText("Reply")).toBeNull();
    });
});
