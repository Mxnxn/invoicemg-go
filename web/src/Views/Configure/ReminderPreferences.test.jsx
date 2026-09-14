import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const load = vi.fn();
const setNotifyPreference = vi.fn();
vi.mock("../../global/toast", () => ({ notifySuccess: vi.fn(), notifyError: vi.fn() }));

import ReminderPreferences from "./ReminderPreferences";

const EVENTS = [
    { field: "notifyPoCreated", label: "Order sent", hint: "", fallback: true },
    { field: "notifyPoUpdated", label: "Order updated", hint: "", fallback: true },
    { field: "notifyPoConfirmed", label: "Dispatch", hint: "", fallback: true },
];

const renderIt = (extra = {}) =>
    render(
        <ReminderPreferences
            title="Suppliers"
            noun="supplier"
            events={EVENTS}
            load={load}
            save={setNotifyPreference}
            intro="who gets what"
            emptyHint="No suppliers yet."
            {...extra}
        />
    );

beforeEach(() => {
    vi.clearAllMocks();
    setNotifyPreference.mockResolvedValue({ code: 200 });
    load.mockResolvedValue([
        { _id: "p1", name: "Ravi Kumar", firm: "Bolt Vinyls", phone: "919000000001", notifyPoCreated: false },
        { _id: "p2", name: "Priya Nair", firm: "Acme Board", phone: "919000000002", notifyPoUpdated: true },
    ]);
});

describe("ReminderPreferences", () => {
    // The table knows about no backend: whatever its loader hands back is what it lists, which
    // is what lets one table serve both a Person and a Client.
    it("lists whatever its loader hands back", async () => {
        renderIt();
        await waitFor(() => expect(screen.getByText("Bolt Vinyls")).toBeDefined());
        expect(screen.getByText("Acme Board")).toBeDefined();
    });

    // A cell with no stored answer must not look like an answer. "Default" plus what it
    // resolves to is the difference between a row you can read and one you have to guess at.
    it("shows an unanswered setting as the default, and says what the default does", async () => {
        renderIt();
        await waitFor(() => expect(screen.getByText("Bolt Vinyls")).toBeDefined());
        // p1 answered only notifyPoCreated (false); its other two are inherited.
        expect(screen.getAllByText("Default: On").length).toBeGreaterThan(0);
        expect(screen.getAllByText("Off").length).toBeGreaterThan(0);
    });

    it("writes an explicit answer when a cell is pressed", async () => {
        renderIt();
        await waitFor(() => expect(screen.getByText("Bolt Vinyls")).toBeDefined());
        // p1's "Order sent" is an explicit false; pressing it turns it on.
        fireEvent.click(screen.getByRole("button", { name: /Ravi Kumar.*Order sent/i }));
        await waitFor(() => expect(setNotifyPreference).toHaveBeenCalledWith("p1", "notifyPoCreated", "true"));
    });

    // Reset must write "clear" on every field. Writing "false" would silence the supplier
    // permanently, which is the opposite of putting them back to the house rule.
    it("resets every field to clear, not to false", async () => {
        renderIt();
        await waitFor(() => expect(screen.getByText("Bolt Vinyls")).toBeDefined());
        fireEvent.click(screen.getAllByRole("button", { name: /^Reset/ })[0]);
        await waitFor(() => expect(setNotifyPreference).toHaveBeenCalledTimes(3));
        expect(setNotifyPreference.mock.calls.every((c) => c[2] === "clear")).toBe(true);
    });

    it("searches name, firm and phone", async () => {
        renderIt();
        await waitFor(() => expect(screen.getByText("Bolt Vinyls")).toBeDefined());
        const box = screen.getByPlaceholderText(/Search/i);

        fireEvent.change(box, { target: { value: "priya" } });
        expect(screen.queryByText("Bolt Vinyls")).toBeNull();

        fireEvent.change(box, { target: { value: "Bolt" } });
        expect(screen.getByText("Bolt Vinyls")).toBeDefined();

        fireEvent.change(box, { target: { value: "919000000002" } });
        expect(screen.getByText("Acme Board")).toBeDefined();
        expect(screen.queryByText("Bolt Vinyls")).toBeNull();
    });

    // "None of them match" and "there are none" are different facts, and only the second one
    // means the list needs setting up.
    it("distinguishes an empty list from a search that matched nothing", async () => {
        renderIt();
        await waitFor(() => expect(screen.getByText("Bolt Vinyls")).toBeDefined());
        fireEvent.change(screen.getByPlaceholderText(/Search/i), { target: { value: "zzzz" } });
        expect(screen.getByText(/No supplier matches that search/i)).toBeDefined();
    });
});

describe("ReminderPreferences for customers", () => {
    const asCustomers = { nullMeans: "ask", noun: "customer", title: "Customers" };

    // A customer with no answer is ASKED - the prompt after a job-id is raised does the
    // asking. Showing that as "Default · Off" would claim they had opted out of something
    // nobody ever put to them.
    it("calls an unanswered customer 'asks each time', not a default", async () => {
        load.mockResolvedValue([{ _id: "c1", name: "Ravi Kumar", firm: "Bolt Vinyls", phone: "919000000001" }]);
        renderIt(asCustomers);
        await waitFor(() => expect(screen.getByText("Bolt Vinyls")).toBeDefined());
        expect(screen.getAllByText("Asks each time").length).toBeGreaterThan(0);
        expect(screen.queryByText(/^Default/)).toBeNull();
    });

    // Three real states, so the cell cycles rather than flipping between two: ask -> on ->
    // off -> ask. Getting back to "ask" is the state a two-way toggle could never reach, and
    // it is the one that means "keep offering me the choice".
    it("cycles an explicit no back to being asked, not to yes", async () => {
        load.mockResolvedValue([
            { _id: "c1", name: "Ravi Kumar", firm: "Bolt Vinyls", phone: "919000000001", notifyPoCreated: false },
        ]);
        renderIt(asCustomers);
        await waitFor(() => expect(screen.getByText("Bolt Vinyls")).toBeDefined());
        fireEvent.click(screen.getByRole("button", { name: /Ravi Kumar.*Order sent/i }));
        await waitFor(() => expect(setNotifyPreference).toHaveBeenCalledWith("c1", "notifyPoCreated", "clear"));
    });

    it("still turns an unanswered customer on", async () => {
        load.mockResolvedValue([{ _id: "c1", name: "Ravi Kumar", firm: "Bolt Vinyls", phone: "919000000001" }]);
        renderIt(asCustomers);
        await waitFor(() => expect(screen.getByText("Bolt Vinyls")).toBeDefined());
        fireEvent.click(screen.getByRole("button", { name: /Ravi Kumar.*Order sent/i }));
        await waitFor(() => expect(setNotifyPreference).toHaveBeenCalledWith("c1", "notifyPoCreated", "true"));
    });
});
