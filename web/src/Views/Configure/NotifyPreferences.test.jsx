import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const listNotifyPreferences = vi.fn();
const setNotifyPreference = vi.fn();
vi.mock("../Client/client_backend", () => ({
    clientsBackend: {
        listNotifyPreferences: (...a) => listNotifyPreferences(...a),
        setNotifyPreference: (...a) => setNotifyPreference(...a),
    },
}));
vi.mock("../../global/toast", () => ({ notifySuccess: vi.fn(), notifyError: vi.fn() }));

import NotifyPreferences from "./NotifyPreferences";

beforeEach(() => {
    vi.clearAllMocks();
    setNotifyPreference.mockResolvedValue({ code: 200 });
    listNotifyPreferences.mockResolvedValue({
        code: 200,
        data: [
            { _id: "c1", clientFirm: "Acme Signs", clientName: "Priya Nair", clientPhone: "919000000001", notifyOnCreate: true },
            { _id: "c2", clientFirm: "Bolt Print", clientName: "Ravi Kumar", clientPhone: "919000000002", notifyOnCreate: false },
        ],
    });
});

describe("NotifyPreferences", () => {
    // The two remembered answers are opposites and must not read alike - one means a message
    // goes out on its own, the other means one never does.
    it("distinguishes an opted-in customer from a silenced one", async () => {
        render(<NotifyPreferences />);
        await waitFor(() => expect(screen.getByText("Acme Signs")).toBeDefined());
        expect(screen.getByText("Told automatically")).toBeDefined();
        expect(screen.getByText("Never told")).toBeDefined();
    });

    // Reset must write "clear", not "false". Writing false would silence the customer forever,
    // which is the opposite of putting them back to being asked.
    it("resets to 'clear' and drops the row", async () => {
        render(<NotifyPreferences />);
        await waitFor(() => expect(screen.getByText("Acme Signs")).toBeDefined());
        fireEvent.click(screen.getAllByText("Reset")[0]);
        await waitFor(() => expect(setNotifyPreference).toHaveBeenCalled());
        expect(setNotifyPreference.mock.calls[0][0].get("notifyOnCreate")).toBe("clear");
        expect(setNotifyPreference.mock.calls[0][0].get("client_id")).toBe("c1");
        await waitFor(() => expect(screen.queryByText("Acme Signs")).toBeNull());
        expect(screen.getByText("Bolt Print")).toBeDefined();
    });

    // An empty panel has to say so. A blank area under a heading reads as a failed load.
    it("says so when nothing is remembered", async () => {
        listNotifyPreferences.mockResolvedValue({ code: 200, data: [] });
        render(<NotifyPreferences />);
        await waitFor(() => expect(screen.getByText(/every one of them is asked each time/)).toBeDefined());
    });
});

describe("NotifyPreferences table", () => {
    // Search has to match the firm AND the client name: a customer is filed under either
    // depending on who entered them, so matching one alone makes rows look deleted.
    it("searches on firm name", async () => {
        render(<NotifyPreferences />);
        await waitFor(() => expect(screen.getByText("Acme Signs")).toBeDefined());
        fireEvent.change(screen.getByPlaceholderText("Search client or firm"), { target: { value: "bolt" } });
        await waitFor(() => expect(screen.queryByText("Acme Signs")).toBeNull());
        expect(screen.getByText("Bolt Print")).toBeDefined();
    });

    it("searches on client name too", async () => {
        render(<NotifyPreferences />);
        await waitFor(() => expect(screen.getByText("Acme Signs")).toBeDefined());
        fireEvent.change(screen.getByPlaceholderText("Search client or firm"), { target: { value: "priya" } });
        await waitFor(() => expect(screen.queryByText("Bolt Print")).toBeNull());
        expect(screen.getByText("Acme Signs")).toBeDefined();
    });

    // A search that matches nothing must say so rather than render an empty table body, which
    // reads as a failed load.
    it("says when a search matches nothing", async () => {
        render(<NotifyPreferences />);
        await waitFor(() => expect(screen.getByText("Acme Signs")).toBeDefined());
        fireEvent.change(screen.getByPlaceholderText("Search client or firm"), { target: { value: "zzzz" } });
        await waitFor(() => expect(screen.getByText("No customer matches that search.")).toBeDefined());
    });

    // The row count is shown so the table says how much it is holding.
    it("reports how many customers there are", async () => {
        render(<NotifyPreferences />);
        await waitFor(() => expect(screen.getByText("2 customers")).toBeDefined());
    });
});
