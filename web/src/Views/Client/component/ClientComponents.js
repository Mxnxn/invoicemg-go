import React, { useState, useEffect, useCallback } from "react";
// import Liteheader from "../../../Common/Header/LiteHeader";
import ClientJobsTab from "./ClientJobsTab";
import Footer from "../../../Common/Footers/AdminFooter";

import { Row, Container, Col } from "reactstrap";
import { Menu } from "react-feather";
import ThemeToggleButton from "../../../Common/ThemeToggleButton";
import { initials } from "../../dashboardCards";
import "../client.css";
import { getDate } from "../../../Common/DateAndTime/getDate";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { lifecycleBackend } from "../../Lifecycle/lifecycle_backend";
import { ledgerBackend } from "../../Ledger/ledger_backend";
import { notifyError } from "../../../global/toast";
import { triggerDownload } from "../../BankReport/downloads";
import { downloadName } from "../../../Common/downloadName";
import { exportWorkbook } from "../../../Common/reports/exportWorkbook";
import { companyBackend } from "../../../Common/company_backend";
import GenrateInvoiceNumber from "../../Invoice/Component/GenerateInvoiceModal";
import InvoicePreviewModal from "../../Invoice/Component/InvoicePreviewModal";
import ConfirmModal from "./ConfirmModal";
import { clientsBackend } from "../client_backend";

// A curated list, not every field on the entry - the same curation the Reports exports do.
const ENTRY_COLUMNS = [
    { key: "date", label: "Date" },
    { key: "material", label: "Material" },
    { key: "description", label: "Description" },
    { key: "length", label: "Length", numeric: true },
    { key: "width", label: "Width", numeric: true },
    { key: "qty", label: "Qty", numeric: true },
    { key: "rate", label: "Rate", numeric: true },
    { key: "amount", label: "Amount", numeric: true },
    { key: "cgst", label: "CGST", numeric: true },
    { key: "sgst", label: "SGST", numeric: true },
    { key: "igst", label: "IGST", numeric: true },
    { key: "total", label: "Total", numeric: true },
    { key: "advance", label: "Advance", numeric: true },
    { key: "invoiced", label: "Invoiced" },
];

// Entries are no longer written by hand.
//
// This page once carried an Add/Edit/Delete Entry form. The interface for it went in
// e29a2ac ("remove entries from the interface") because invoicing converts a job's rows into
// Entries itself, so authoring one by hand offered a step nobody needs to take - but the
// handlers, the form state and the three modals stayed behind, unreachable: nothing rendered
// a control that could set addModal, editModal or deleteModal true. They are gone now, along
// with ModalAddEntry, ClientEntry/clientEntry_backend.js and the five /entry routes they
// called.
//
// The Entry COLLECTION is untouched and still load-bearing - an invoice is its entries (see
// routes/Invoice.js), and Statistics, Ledger and Analytics all read them. Only the hand
// authoring of one is gone.
const ClientComponents = ({ uid, darkModeFlag, user, customer, setOpen, cid, setCustomer }) => {
    const [due, setDue] = useState(0);
    // Work done for this client that no invoice covers yet - distinct from `due`, which is
    // money already billed and still unpaid. See Helpers/UninvoicedJobs.js.
    const [pending, setPending] = useState({ pending: 0, jobIds: [] });
    const [jobs, setJobs] = useState([]);
    // Download/Invoice used to live on the Entries tab, scoped to checked entries there -
    // now they're triggered from the Jobs tab (see Refactor.md "Features from Entry like
    // Download and Invoice should be on Job table"), scoped to the entries billed under
    // whichever jobs are selected there. The generation flow itself (number/date pick ->
    // preview -> confirm/save) is unchanged, just re-homed here so both tabs could reach it
    // during the transition.
    const [invoiceModal, setInvoiceModal] = useState(false);
    const [confirmSaveModal, setConfirmSaveModal] = useState(false);
    const [dataForInvoice, setDataForInvoice] = useState({ ready: false, view: false });

    useEffect(() => {
        if (!cid) return;
        lifecycleBackend
            .listJobs({ client_id: cid })
            .then((res) => setJobs(res.data))
            .catch(() => {});
    }, [cid]);

    // Pending Amount used to sum entries with `total > 0 && !has_issued` - i.e. it excluded
    // every invoiced entry, which is precisely when the client starts owing us. Once the Jobs
    // flow began issuing invoices for everything, the filter matched nothing and the card sat
    // at zero permanently.
    //
    // Ask the ledger instead. /ledger/client is the same computation behind the Client Ledger
    // and the Customer Dues report (Helpers/ClientDues.js), so this card, that report and that
    // ledger can't show three different numbers for one client.
    const dueCount = useCallback(() => {
        if (!cid) return;
        const formData = new FormData();
        formData.set("client_id", cid);
        ledgerBackend
            .clientLedger(formData)
            .then((res) => {
                setDue(res.data.currentBalance);
                setPending({ pending: res.data.pending || 0, jobIds: res.data.pendingJobIds || [] });
            })
            .catch(() => {});
    }, [cid]);

    useEffect(() => {
        dueCount();
    }, [dueCount]);

    // amount = qty * length * width * rate; discount/charges apply to amount before tax;
    // total = (amount - discount + charges) * (1 + tax%) - advance.

    const downloadXLSX = async (entries, clientLabel) => {
        // The company, not `user` (a plain User record) - two tabs can act as two companies
        // at once, so a letterhead cached off the user would carry the wrong firm. Fetched
        // fresh here rather than reused, same reasoning as ReportDownloads.
        try {
            const res = await companyBackend.activeCompany().catch(() => null);
            const company = res?.data || null;
            const blob = await exportWorkbook({
                title: "Entries",
                subtitle: clientLabel,
                columns: ENTRY_COLUMNS,
                rows: entries.map((entry) => [
                    getDate(entry.date),
                    entry.material,
                    entry.description,
                    entry.length,
                    entry.width,
                    entry.qty,
                    entry.rate,
                    entry.amount,
                    entry.cgst,
                    entry.sgst,
                    entry.igst,
                    entry.total,
                    entry.advance,
                    entry.has_issued ? "Yes" : "No",
                ]),
                company,
                template: company?.exportTemplate || {},
                logoUrl: company?.url ? `${import.meta.env.VITE_API_URL}/uploads/${company.url}` : "",
            });
            triggerDownload(blob, downloadName({ firm: company?.firm, ext: "xlsx", suffix: clientLabel }));
        } catch {
            notifyError("Couldn't generate the download - please try again.");
        }
    };

    // Opens the invoice-number/date picker for a given set of entries (Jobs tab passes the
    // entries billed under whichever jobs are selected there) - same flow the Entries tab
    // used to drive off its own checkbox selection.
    const openInvoiceFromEntries = (entries) => {
        setDataForInvoice({ entries, ready: false, view: false });
        setInvoiceModal(true);
    };

    // Invoicing straight from job-ids. The preview's line items are built from the job's own
    // rows rather than from Entries, because the rows may not have been converted yet - the
    // server converts them on save (/invoice/save takes job_ids). Shaped like an Entry so the
    // templates, which read qty/rate/length/width/tax off each line, need no special case.
    const openInvoiceFromJobs = (readyJobs) => {
        const entries = readyJobs.flatMap((job) =>
            (job.rows || []).map((row) => ({
                _id: row._id,
                description: row.description || row.material || job.challanNumber,
                material: row.material || row.description || job.challanNumber,
                hsn: row.hsn || "",
                qty: row.qty,
                rate: row.rate,
                length: row.length || "0",
                width: row.width || "0",
                cgst: row.cgst,
                sgst: row.sgst,
                igst: row.igst,
                discount: row.discount || 0,
                charges: row.charges || 0,
                date: job.receivedDate,
            }))
        );
        setDataForInvoice({ entries, job_ids: readyJobs.map((j) => j._id), ready: false, view: false });
        setInvoiceModal(true);
    };

    const onGenerateInvoice = (date, invNo) => {
        setInvoiceModal(false);
        const preparedData = {
            ...dataForInvoice,
            date,
            invoiceNumber: invNo,
            account: user.account,
            bank_name: user.bank_name,
            email: user.email,
            firm: user.firm,
            gst: user.gst,
            ifsc: user.ifsc,
            url: user.url,
            name: user.name,
            phone: user.phone,
            client_id: customer._id,
            uid: customer.uid,
            clientAddress: customer.clientAddress,
            clientFirm: customer.clientFirm,
            clientGST: customer.clientGST,
            clientName: customer.clientName,
            clientPhone: customer.clientPhone,
            ready: true,
            view: true,
        };
        setDataForInvoice(preparedData);
    };

    const onSaveInvoice = async () => {
        try {
            const formData = new FormData();
            // job_ids when the invoice came from the Jobs tab - the server converts the rows
            // and records which jobs it bills. entry_ids stays for the older entry-driven path.
            if (dataForInvoice.job_ids && dataForInvoice.job_ids.length > 0) {
                formData.set("job_ids", JSON.stringify(dataForInvoice.job_ids));
            } else {
                dataForInvoice.entries.forEach((elem, index) => formData.set(`entry_ids[${index}]`, elem._id));
            }
            formData.set("uid", dataForInvoice.uid);
            formData.set("date", new Date(dataForInvoice.date));
            formData.set("client_id", dataForInvoice.client_id);
            formData.set("invNo", dataForInvoice.invoiceNumber);
            const res = await clientsBackend.saveInvoice(formData);
            const copy = [...customer.entries];
            copy.forEach((entry) => {
                for (let i = 0; i < dataForInvoice.entries.length; i++) {
                    const elem = dataForInvoice.entries[i];
                    if (elem._id === entry._id) {
                        entry.has_issued = true;
                        entry.issued = res.data;
                        entry.checked = false;
                    }
                }
            });
            setCustomer({ ...customer, entries: copy });
            setConfirmSaveModal(false);
            setDataForInvoice({ ...dataForInvoice, ready: false });
        } catch (err) {
            console.log(err);
        }
    };

    return (
        <div style={{ minHeight: "100vh", display: "flex", flexDirection: "column", background: "var(--bg-canvas)" }}>
            {/* <Liteheader bg="primary" /> */}
            <Container fluid style={{ paddingTop: "20px", flex: "1 0 auto" }}>
                <Row>
                    <Col xl="12" className="d-flex align-items-center justify-content-between" style={{ flexWrap: "wrap", gap: 12 }}>
                        <div className="d-flex align-items-center">
                            <button
                                className="shell-icon-btn"
                                type="button"
                                aria-label="Open menu"
                                onClick={(e) =>
                                    setOpen((prev) => {
                                        return { ...prev, menuPanel: true };
                                    })
                                }
                                style={{ color: "var(--text-primary, #17191f)" }}
                            >
                                <Menu size={22} />
                            </button>
                        </div>
                        <ThemeToggleButton fixed={false} />
                    </Col>
                </Row>
                {/* Whose page this is, and what they owe.
                    Three stat cards used to sit here and the customer's own NAME appeared
                    nowhere on the screen - you could read the whole page and not know whose
                    work you were looking at. The sheet says who at the top of every section;
                    this page is one customer, so it says it once, here. */}
                <header className="client-head">
                    <span className="client-head-who">
                        <span className="client-head-monogram" aria-hidden="true">
                            {initials(customer.clientFirm || customer.clientName)}
                        </span>
                        <span className="client-head-names">
                            <span className="client-head-firm">{customer.clientFirm || customer.clientName || "Customer"}</span>
                            <span className="client-head-sub">
                                {customer.clientName && customer.clientName !== customer.clientFirm && (
                                    <span>{customer.clientName}</span>
                                )}
                                {customer.clientPhone && <span className="client-head-phone">{customer.clientPhone}</span>}
                            </span>
                        </span>
                    </span>

                    <span className="client-head-money">
                        {/* Was labelled "Pending Amount", but it is the ledger balance - money
                            already invoiced and still unpaid. Pending means the genuinely
                            un-billed work in the figure beside it. */}
                        <span className={`client-head-figure${due > 0 ? " is-due" : ""}`}>
                            <span className="client-head-label">Due</span>
                            <span className="client-head-value">&#8377;{RoundOff(due)}</span>
                        </span>
                        <span
                            className="client-head-figure"
                            title={pending.jobIds.length ? `Job ${pending.jobIds.join(", ")}` : undefined}
                        >
                            <span className="client-head-label">Not invoiced</span>
                            <span className="client-head-value">&#8377;{RoundOff(pending.pending)}</span>
                        </span>
                        <span className="client-head-figure">
                            <span className="client-head-label">Job-ids</span>
                            <span className="client-head-value">{jobs.length}</span>
                        </span>
                    </span>
                </header>

                    <ClientJobsTab
                        clientId={cid}
                        clientName={customer.clientFirm || customer.clientName}
                        entries={customer.entries}
                        onJobCreated={(job) => setJobs((prev) => (job && job._id ? [job, ...prev] : prev))}
                        onEntriesCreated={(newEntries) => {
                            const withChecked = newEntries.map((el) => ({ ...el, checked: false }));
                            setCustomer((prev) => ({ ...prev, entries: [...withChecked, ...prev.entries] }));
                        }}
                        onDownloadEntries={(entries) => downloadXLSX(entries, customer.clientFirm || customer.clientName)}
                        onInvoiceJobs={openInvoiceFromJobs}
                    />
                <GenrateInvoiceNumber modal={invoiceModal} onCancelHandler={() => setInvoiceModal(false)} onGenerateInvoice={onGenerateInvoice} />
                <ConfirmModal modal={confirmSaveModal} onCancelHandler={() => setConfirmSaveModal(false)} onSubmitHandler={onSaveInvoice} />
            </Container>
            <InvoicePreviewModal
                isOpen={dataForInvoice.view}
                toggle={() => setDataForInvoice({ ...dataForInvoice, view: false })}
                invoice={dataForInvoice}
                onSave={dataForInvoice.ready ? () => setConfirmSaveModal(true) : undefined}
            />
            <Container fluid style={{ flexShrink: 0 }}>
                <Footer darkModeFlag={"false"} style={{ background: "transparent" }} />
            </Container>
        </div>
    );
};

export default ClientComponents;
