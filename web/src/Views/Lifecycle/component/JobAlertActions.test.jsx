import React from "react";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

const sendJobAlert = vi.fn();
const completeAllRows = vi.fn();
vi.mock("../lifecycle_backend", () => ({
    lifecycleBackend: {
        sendJobAlert: (...a) => sendJobAlert(...a),
        completeAllRows: (...a) => completeAllRows(...a),
    },
}));
vi.mock("../../../global/toast", () => ({ notifySuccess: () => {}, notifyError: () => {} }));

import JobAlertActions from "./JobAlertActions";

const job = (over = {}) => ({
    _id: "j1",
    client_id: { clientPhone: "9990001111" },
    rows: [{ queue: "Created" }],
    lock: { canEditQueue: true },
    alerts: {},
    ...over,
});

describe("JobAlertActions", () => {
    beforeEach(() => {
        sendJobAlert.mockReset().mockResolvedValue({ data: { job: { _id: "j1" } } });
        completeAllRows.mockReset().mockResolvedValue({ data: { _id: "j1" }, message: "Rows marked done." });
    });

    it("renders nothing at all when there is neither a status nor an action", () => {
        const { container } = render(
            <JobAlertActions job={job({ rows: [{ queue: "Done" }], alerts: {} })} showMarkAllDone={false} />
        );
        expect(container.firstChild).toBeNull();
    });

    it("offers the created send while no card is done yet", () => {
        render(<JobAlertActions job={job({ alerts: { created: { canSend: true } } })} />);
        expect(screen.getByText("Notify created")).toBeTruthy();
    });

    // Two sends offering to tell one customer two different things about one job is how the
    // wrong one gets pressed.
    it("withdraws the created send once any card is done", () => {
        render(
            <JobAlertActions
                job={job({ rows: [{ queue: "Done" }, { queue: "Printing" }], alerts: { created: { canSend: true } } })}
            />
        );
        expect(screen.queryByText("Notify created")).toBeNull();
    });

    it("sends the kind that was pressed", async () => {
        render(<JobAlertActions job={job({ alerts: { created: { canSend: true } } })} />);
        fireEvent.click(screen.getByText("Notify created"));

        await waitFor(() => expect(sendJobAlert).toHaveBeenCalled());
        const sent = sendJobAlert.mock.calls[0][0];
        expect(sent.get("kind")).toBe("created");
        expect(sent.get("job_id")).toBe("j1");
    });

    it("refuses to send with no phone number on file", () => {
        render(<JobAlertActions job={job({ client_id: {}, alerts: { created: { canSend: true } } })} />);
        expect(screen.getByText("Notify created").closest("button").disabled).toBe(true);
    });

    it("hides mark-all-done once every card is done", () => {
        const { rerender } = render(<JobAlertActions job={job()} />);
        expect(screen.getByText("Mark all done")).toBeTruthy();

        rerender(<JobAlertActions job={job({ rows: [{ queue: "Done" }] })} />);
        expect(screen.queryByText("Mark all done")).toBeNull();
    });

    it("hides mark-all-done on a locked job", () => {
        render(<JobAlertActions job={job({ lock: { canEditQueue: false } })} />);
        expect(screen.queryByText("Mark all done")).toBeNull();
    });

    it("hands the updated job back so the view it sits in can re-render", async () => {
        const onChange = vi.fn();
        render(<JobAlertActions job={job()} onChange={onChange} />);
        fireEvent.click(screen.getByText("Mark all done"));

        await waitFor(() => expect(onChange).toHaveBeenCalledWith({ _id: "j1" }));
    });

    // The reason the component exists: the two halves wrap as blocks, so a status chip can
    // never end up between two buttons.
    it("keeps what happened and what to do in separate groups", () => {
        const { container } = render(
            <JobAlertActions
                job={job({ alerts: { created: { canSend: true, sentBefore: true, status: "delivered" } } })}
            />
        );
        const status = container.querySelector(".job-action-status");
        const buttons = container.querySelector(".job-action-buttons");
        expect(status).toBeTruthy();
        expect(buttons).toBeTruthy();
        expect(status.contains(buttons)).toBe(false);
        expect(container.querySelector(".job-action-divider")).toBeTruthy();
    });

    it("drops the divider when only one half is present", () => {
        const { container } = render(<JobAlertActions job={job()} />);
        expect(container.querySelector(".job-action-divider")).toBeNull();
    });
});
