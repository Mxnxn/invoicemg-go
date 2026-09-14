import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const sendJobAlert = vi.fn();
const setNotifyPreference = vi.fn();
vi.mock("../lifecycle_backend", () => ({ lifecycleBackend: { sendJobAlert: (...a) => sendJobAlert(...a) } }));
vi.mock("../../Client/client_backend", () => ({
    clientsBackend: { setNotifyPreference: (...a) => setNotifyPreference(...a) },
}));
vi.mock("../../../global/toast", () => ({ notifySuccess: vi.fn(), notifyError: vi.fn() }));

import JobCreatedNotifyPrompt from "./JobCreatedNotifyPrompt";

const client = { _id: "c1", clientFirm: "Acme Signs", clientPhone: "919000000000", notifyOnCreate: null };
const job = (over) => ({ _id: "j1", challanNumber: "JOB-1", client_id: client, ...over });

beforeEach(() => {
    vi.clearAllMocks();
    sendJobAlert.mockResolvedValue({ data: { code: 200, job: {} } });
    setNotifyPreference.mockResolvedValue({ data: { code: 200 } });
});

describe("JobCreatedNotifyPrompt", () => {
    // The create case, unchanged - the wording that shipped.
    it("announces a new job-id as raised", () => {
        render(<JobCreatedNotifyPrompt job={job()} autoSend={false} onClose={() => {}} />);
        expect(screen.getByText("JOB-1 created")).toBeDefined();
        expect(screen.getByText(/this job-id is raised/)).toBeDefined();
    });

    // The update case. Announcing an edit with the create wording is the specific thing the
    // update template exists to avoid - telling a customer their job "is raised" for the
    // second time reads as a system that lost track of them.
    it("announces an edited job-id as changed, not as new", () => {
        const edited = job({ alerts: { created: { isUpdate: true, changed: true, canSend: true } } });
        render(<JobCreatedNotifyPrompt job={edited} autoSend={false} onClose={() => {}} />);
        expect(screen.getByText("JOB-1 updated")).toBeDefined();
        expect(screen.getByText(/has changed/)).toBeDefined();
        expect(screen.queryByText(/is raised/)).toBeNull();
    });

    // The two answers are remembered separately: wanting to hear that a job was raised is not
    // the same as wanting a message every time a line is edited.
    it("counts down on an update for a customer who opted into UPDATES", async () => {
        const edited = job({
            client_id: { ...client, notifyOnUpdate: true },
            alerts: { created: { isUpdate: true, changed: true, canSend: true } },
        });
        render(<JobCreatedNotifyPrompt job={edited} autoSend={false} onClose={() => {}} />);
        await waitFor(() => expect(screen.getByText(/Telling Acme Signs on WhatsApp in/)).toBeDefined());
    });

    // The separation, stated: opting into creation messages must NOT silently opt the customer
    // into an update message they never agreed to.
    it("does not auto-send an update to a customer who only opted into creations", async () => {
        const edited = job({
            client_id: { ...client, notifyOnCreate: true, notifyOnUpdate: null },
            alerts: { created: { isUpdate: true, changed: true, canSend: true } },
        });
        render(<JobCreatedNotifyPrompt job={edited} autoSend={false} onClose={() => {}} />);
        await waitFor(() => expect(screen.getByText(/has changed/)).toBeDefined());
        expect(screen.queryByText(/Telling Acme Signs on WhatsApp in/)).toBeNull();
    });

    // And the write says which answer it is recording.
    it("records the answer against the update channel", async () => {
        const edited = job({
            client_id: { ...client, notifyOnUpdate: true },
            alerts: { created: { isUpdate: true, changed: true, canSend: true } },
        });
        render(<JobCreatedNotifyPrompt job={edited} autoSend={false} onClose={() => {}} />);
        await waitFor(() => expect(screen.getByText("Cancel")).toBeDefined());
        fireEvent.click(screen.getByText("Cancel"));
        await waitFor(() => expect(setNotifyPreference).toHaveBeenCalled());
        expect(setNotifyPreference.mock.calls[0][0].get("kind")).toBe("updated");
    });
});
