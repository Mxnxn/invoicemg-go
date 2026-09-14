import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const getClientInvoices = vi.fn();
vi.mock("../invoice_backend", () => ({ invoiceBackend: { getClientInvoices: (...a) => getClientInvoices(...a) } }));
vi.mock("./RecivedHistory", () => ({ default: ({ cname }) => <div data-testid="history">History for {cname}</div> }));
vi.mock("../../Lifecycle/component/JobDetailModal", () => ({ default: () => <div data-testid="job-modal" /> }));
vi.mock("./InvoiceJobGroup", () => ({
    default: ({ jobs }) => (
        <tr data-testid="job-tree">
            <td>{jobs.map((j) => j.challanNumber).join(", ")}</td>
        </tr>
    ),
}));

import ClientInvoicesPanel from "./ClientInvoicesPanel";

const payload = {
    data: [
        {
            _id: "inv1",
            invoiceId: "MG/26-27/1",
            date: "2026-04-02",
            taxedValue: 1180,
            nonTaxedValue: 1000,
            entryReceived: 1180,
            job_ids: [{ _id: "j1", challanNumber: "JOB-1" }, { _id: "j2", challanNumber: "JOB-2" }],
        },
        { _id: "inv2", invoiceId: "MG/26-27/2", date: "2026-04-09", taxedValue: 500, nonTaxedValue: 424, entryReceived: 0, job_ids: [] },
    ],
    clientName: "Priya Nair",
    clientFirm: "Northwind Signage",
    totalDue: 500,
    totalReceived: 1180,
    receivedHistory: [{ _id: "r1", amount: 1180 }],
};

beforeEach(() => {
    vi.clearAllMocks();
    getClientInvoices.mockResolvedValue(payload);
});

describe("ClientInvoicesPanel", () => {
    it("lists the customer's invoices", async () => {
        render(<ClientInvoicesPanel cid="c1" />);
        await waitFor(() => expect(screen.getByText("MG/26-27/1")).toBeDefined());
        expect(screen.getByText("MG/26-27/2")).toBeDefined();
    });

    // Paid and pending must not read alike - the whole point of the column.
    it("marks paid and pending apart", async () => {
        render(<ClientInvoicesPanel cid="c1" />);
        await waitFor(() => expect(screen.getByText("paid")).toBeDefined());
        expect(screen.getByText("pending")).toBeDefined();
    });

    // The tree is the invoice's contents. It stays shut until asked for, or a customer with
    // forty invoices opens onto hundreds of job rows.
    it("expands an invoice into its job-ids only when clicked", async () => {
        render(<ClientInvoicesPanel cid="c1" />);
        await waitFor(() => expect(screen.getByText("MG/26-27/1")).toBeDefined());
        expect(screen.queryByTestId("job-tree")).toBeNull();

        fireEvent.click(screen.getByText("MG/26-27/1").closest("tr"));
        expect(screen.getByTestId("job-tree")).toBeDefined();
        expect(screen.getByText("JOB-1, JOB-2")).toBeDefined();
    });

    // An invoice with no jobs has nothing to expand, so it must not offer to.
    it("does not expand an invoice with no job-ids", async () => {
        render(<ClientInvoicesPanel cid="c1" />);
        await waitFor(() => expect(screen.getByText("MG/26-27/2")).toBeDefined());
        fireEvent.click(screen.getByText("MG/26-27/2").closest("tr"));
        expect(screen.queryByTestId("job-tree")).toBeNull();
    });

    it("opens the payment history in a side panel", async () => {
        render(<ClientInvoicesPanel cid="c1" />);
        await waitFor(() => expect(screen.getByLabelText("Open payment history")).toBeDefined());
        expect(screen.queryByTestId("history")).toBeNull();

        fireEvent.click(screen.getByLabelText("Open payment history"));
        expect(screen.getByText(/History for Priya Nair/)).toBeDefined();
    });

    // It used to portal to document.body, which put it OUTSIDE the modal it belongs to and
    // over the whole viewport. Rendering it within the panel's own tree is what keeps it in
    // the dialog, so that is what this asserts - not merely that it appears somewhere.
    it("renders the history inside the panel, not portalled to the body", async () => {
        const { container } = render(<ClientInvoicesPanel cid="c1" />);
        await waitFor(() => expect(screen.getByLabelText("Open payment history")).toBeDefined());
        fireEvent.click(screen.getByLabelText("Open payment history"));

        expect(container.querySelector('[data-testid="history"]')).not.toBeNull();
    });

    // Beside the invoices, so the two can be read against each other.
    it("puts the history alongside the invoice table", async () => {
        const { container } = render(<ClientInvoicesPanel cid="c1" />);
        await waitFor(() => expect(screen.getByLabelText("Open payment history")).toBeDefined());
        fireEvent.click(screen.getByLabelText("Open payment history"));

        const body = container.querySelector(".client-invoices-body");
        expect(body).not.toBeNull();
        expect(body.querySelector(".client-invoices-main")).not.toBeNull();
        expect(body.querySelector('[data-testid="history"]')).not.toBeNull();
    });

    it("tells the caller who this is, so a modal can title itself", async () => {
        const onLoaded = vi.fn();
        render(<ClientInvoicesPanel cid="c1" onLoaded={onLoaded} />);
        await waitFor(() => expect(onLoaded).toHaveBeenCalled());
        expect(onLoaded.mock.calls[0][0].clientFirm).toBe("Northwind Signage");
    });

    // A customer with no invoices is a normal state, not a broken panel.
    it("says so when there are no invoices", async () => {
        getClientInvoices.mockResolvedValue({ ...payload, data: [] });
        render(<ClientInvoicesPanel cid="c1" />);
        await waitFor(() => expect(screen.getByText(/No invoices for this customer yet/)).toBeDefined());
    });

    // A failed load must not leave a permanent "Loading…".
    it("stops loading when the request fails", async () => {
        getClientInvoices.mockRejectedValue(new Error("down"));
        render(<ClientInvoicesPanel cid="c1" />);
        await waitFor(() => expect(screen.queryByText("Loading…")).toBeNull());
    });
});
