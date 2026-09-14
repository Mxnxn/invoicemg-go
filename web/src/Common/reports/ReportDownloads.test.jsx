import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const activeCompany = vi.fn();
const exportWorkbook = vi.fn();
const triggerDownload = vi.fn();
const getUserInfo = vi.fn();
const notifyError = vi.fn();
const pdf = vi.fn();

vi.mock("../company_backend", () => ({ companyBackend: { activeCompany: (...a) => activeCompany(...a) } }));
vi.mock("./exportWorkbook", () => ({ exportWorkbook: (...a) => exportWorkbook(...a) }));
vi.mock("../../Views/BankReport/downloads", () => ({ triggerDownload: (...a) => triggerDownload(...a) }));
vi.mock("../../Views/UserProfile/user_backend", () => ({ userBackend: { getUserInfo: (...a) => getUserInfo(...a) } }));
vi.mock("../../global/toast", () => ({ notifyError: (...a) => notifyError(...a) }));
// @react-pdf/renderer's pdf(<Doc/>).toBlob() - mocked at the module boundary so the failure
// tests can make it reject without actually rendering a document.
vi.mock("@react-pdf/renderer", () => ({ pdf: (...a) => pdf(...a) }));
vi.mock("./ReportPDF", () => ({ default: () => null }));

import ReportDownloads from "./ReportDownloads";

const columns = [{ key: "a", label: "Party" }, { key: "b", label: "Amount", numeric: true }];
const rows = [["Sharma Packaging", 100]];

beforeEach(() => {
    vi.clearAllMocks();
    activeCompany.mockResolvedValue({
        code: 200,
        data: { firm: "Plain Firm", address: "Somewhere", gst: "24AAA", url: "logo.png", exportTemplate: { logo: true, firm: true } },
    });
    exportWorkbook.mockResolvedValue(new Blob(["x"]));
    pdf.mockReturnValue({ toBlob: () => Promise.resolve(new Blob(["pdf"])) });
});

describe("ReportDownloads", () => {
    it("takes the letterhead from the tab's company, not the user record", async () => {
        // getUserInfo is keyed on the user; two tabs can be two companies, so the company
        // is the only correct source for a letterhead.
        render(<ReportDownloads title="Ledger" columns={columns} rows={rows} />);
        await waitFor(() => expect(activeCompany).toHaveBeenCalled());
        expect(getUserInfo).not.toHaveBeenCalled();
    });

    it("passes that company through to the workbook", async () => {
        render(<ReportDownloads title="Ledger" columns={columns} rows={rows} />);
        await waitFor(() => expect(activeCompany).toHaveBeenCalled());

        fireEvent.click(screen.getByRole("button", { name: /XLSX/i }));
        await waitFor(() => expect(exportWorkbook).toHaveBeenCalled());
        expect(exportWorkbook.mock.calls[0][0].company.firm).toBe("Plain Firm");
        expect(exportWorkbook.mock.calls[0][0].template.logo).toBe(true);
    });

    it("still exports when the company cannot be loaded", async () => {
        // A letterhead is nice; losing the report because of it is not.
        activeCompany.mockRejectedValue(new Error("offline"));
        render(<ReportDownloads title="Ledger" columns={columns} rows={rows} />);

        await waitFor(() => expect(screen.getByRole("button", { name: /XLSX/i })).toBeDefined());
        fireEvent.click(screen.getByRole("button", { name: /XLSX/i }));
        await waitFor(() => expect(exportWorkbook).toHaveBeenCalled());
    });

    it("refuses to export nothing", async () => {
        render(<ReportDownloads title="Ledger" columns={columns} rows={[]} />);
        expect(screen.getByRole("button", { name: /XLSX/i })).toHaveProperty("disabled", true);
    });

    it("passes the company to the PDF too, not just the XLSX", async () => {
        // pdf() is invoked with the <ReportPDF/> element, not a rendered tree - React.createElement
        // never calls the mocked component, so the element's own props are what to inspect.
        // Reverting `user={company}` on ReportPDF would undo half of what this branch built,
        // and nothing else here would notice.
        render(<ReportDownloads title="Ledger" columns={columns} rows={rows} />);
        await waitFor(() => expect(activeCompany).toHaveBeenCalled());

        fireEvent.click(screen.getByRole("button", { name: /PDF/i }));
        await waitFor(() => expect(pdf).toHaveBeenCalled());
        expect(pdf.mock.calls[0][0].props.user).toEqual(
            expect.objectContaining({ firm: "Plain Firm" })
        );
    });

    it("forwards summary to the workbook, same as it does columns/rows/foot", async () => {
        const summary = [{ label: "Total Billed", value: "₹1,20,000" }];
        render(<ReportDownloads title="Ledger" columns={columns} rows={rows} summary={summary} />);
        await waitFor(() => expect(activeCompany).toHaveBeenCalled());

        fireEvent.click(screen.getByRole("button", { name: /XLSX/i }));
        await waitFor(() => expect(exportWorkbook).toHaveBeenCalled());
        expect(exportWorkbook.mock.calls[0][0].summary).toEqual(summary);
    });

    it("surfaces a failed spreadsheet instead of leaving the button dead", async () => {
        // ExcelJS failing to load, say - errorInterceptor only watches axios, so nothing else
        // would tell the user the download never happened.
        exportWorkbook.mockRejectedValue(new Error("boom"));
        render(<ReportDownloads title="Ledger" columns={columns} rows={rows} />);
        await waitFor(() => expect(activeCompany).toHaveBeenCalled());

        const button = screen.getByRole("button", { name: /XLSX/i });
        fireEvent.click(button);
        await waitFor(() => expect(notifyError).toHaveBeenCalled());
        expect(triggerDownload).not.toHaveBeenCalled();
        await waitFor(() => expect(button).toHaveProperty("disabled", false));
    });

    it("surfaces a failed PDF instead of leaving the button dead", async () => {
        pdf.mockReturnValue({ toBlob: () => Promise.reject(new Error("boom")) });
        render(<ReportDownloads title="Ledger" columns={columns} rows={rows} />);
        await waitFor(() => expect(activeCompany).toHaveBeenCalled());

        const button = screen.getByRole("button", { name: /PDF/i });
        fireEvent.click(button);
        await waitFor(() => expect(notifyError).toHaveBeenCalled());
        expect(triggerDownload).not.toHaveBeenCalled();
        await waitFor(() => expect(button).toHaveProperty("disabled", false));
    });
});

describe("PDF letterhead", () => {
    // The printed page ignored Configure > Exports entirely until this: it always showed the
    // firm and GST whatever the toggles said, and never showed the logo. The spreadsheet
    // honoured all of it, so one report produced two different letterheads.
    it("passes the export template to the PDF, not just the XLSX", async () => {
        render(<ReportDownloads title="Ledger" columns={columns} rows={rows} />);
        await waitFor(() => expect(activeCompany).toHaveBeenCalled());
        fireEvent.click(screen.getByRole("button", { name: /PDF/i }));
        await waitFor(() => expect(pdf).toHaveBeenCalled());
        expect(pdf.mock.calls[0][0].props.template).toBeDefined();
    });

    it("passes the logo url to the PDF", async () => {
        render(<ReportDownloads title="Ledger" columns={columns} rows={rows} />);
        await waitFor(() => expect(activeCompany).toHaveBeenCalled());
        fireEvent.click(screen.getByRole("button", { name: /PDF/i }));
        await waitFor(() => expect(pdf).toHaveBeenCalled());
        // Whatever the fixture company carries - present means the wiring exists.
        expect(pdf.mock.calls[0][0].props).toHaveProperty("logoUrl");
    });

    // A missing logo file must cost the logo, not the report. @react-pdf throws on an
    // <Image src> it cannot fetch and takes the whole document down with it.
    it("still produces the PDF when the logo cannot be rendered", async () => {
        let attempt = 0;
        pdf.mockImplementation(() => ({
            toBlob: async () => {
                attempt += 1;
                if (attempt === 1) throw new Error("could not fetch image");
                return new Blob(["ok"]);
            },
        }));
        render(<ReportDownloads title="Ledger" columns={columns} rows={rows} />);
        await waitFor(() => expect(activeCompany).toHaveBeenCalled());
        fireEvent.click(screen.getByRole("button", { name: /PDF/i }));
        await waitFor(() => expect(pdf).toHaveBeenCalledTimes(2));
        // The retry drops the logo and keeps everything else.
        expect(pdf.mock.calls[1][0].props.logoUrl).toBe("");
        expect(pdf.mock.calls[1][0].props.user).toEqual(expect.objectContaining({ firm: "Plain Firm" }));
    });
});
