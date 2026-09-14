import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const createPerson = vi.fn();
vi.mock("../../../Common/person_backend", () => ({ personBackend: { create: (...a) => createPerson(...a) } }));
vi.mock("../../../global/toast", () => ({ notifySuccess: vi.fn(), notifyError: vi.fn() }));

import QuickCreateEmployeeModal from "./QuickCreateEmployeeModal";

const open = (props = {}) =>
    render(<QuickCreateEmployeeModal isOpen initialName="Sunil" toggle={vi.fn()} onCreated={vi.fn()} {...props} />);

beforeEach(() => {
    vi.clearAllMocks();
    createPerson.mockResolvedValue({ code: 200, data: { _id: "p9", name: "Sunil", type: "Employee" } });
});

describe("QuickCreateEmployeeModal", () => {
    it("opens on the name that was typed into the picker", () => {
        open();
        expect(screen.getByLabelText(/^name$/i).value).toBe("Sunil");
    });

    // Somebody created from a job board is created in order to work on jobs, and an employee
    // with no permissions signs in to an empty app - which looks like a broken account.
    it("grants the two job permissions without asking", async () => {
        open();
        fireEvent.change(screen.getByLabelText(/^email$/i), { target: { value: "sunil@shop.test" } });
        fireEvent.change(screen.getByLabelText(/^password$/i), { target: { value: "goodpassword" } });
        fireEvent.click(screen.getByRole("button", { name: /create employee/i }));

        await waitFor(() => expect(createPerson).toHaveBeenCalled());
        const sent = createPerson.mock.calls[0][0];
        expect(sent.get("type")).toBe("Employee");
        expect(JSON.parse(sent.get("permissions"))).toEqual(["lifecycle:view", "lifecycle:create"]);
    });

    // The API only sets a password when it has both, so a password beside an empty email is
    // silently discarded and the account exists with no way in.
    it("refuses a password with no email, and an email with no password", () => {
        open();
        fireEvent.change(screen.getByLabelText(/^password$/i), { target: { value: "x" } });
        fireEvent.click(screen.getByRole("button", { name: /create employee/i }));
        expect(screen.getByText(/needs an email address/i)).toBeTruthy();
        expect(createPerson).not.toHaveBeenCalled();

        fireEvent.change(screen.getByLabelText(/^password$/i), { target: { value: "" } });
        fireEvent.change(screen.getByLabelText(/^email$/i), { target: { value: "a@b.test" } });
        fireEvent.click(screen.getByRole("button", { name: /create employee/i }));
        expect(screen.getByText(/Set a password/i)).toBeTruthy();
        expect(createPerson).not.toHaveBeenCalled();
    });

    // A name is the one thing a Person cannot be created without.
    it("refuses an empty name", () => {
        open({ initialName: "" });
        fireEvent.click(screen.getByRole("button", { name: /create employee/i }));
        expect(screen.getByText(/Enter their name/i)).toBeTruthy();
        expect(createPerson).not.toHaveBeenCalled();
    });

    // Name only is allowed: someone who does the work but never signs in.
    it("allows a name with no credentials at all", async () => {
        open();
        fireEvent.click(screen.getByRole("button", { name: /create employee/i }));
        await waitFor(() => expect(createPerson).toHaveBeenCalled());
        expect(createPerson.mock.calls[0][0].get("email")).toBeNull();
    });
});
