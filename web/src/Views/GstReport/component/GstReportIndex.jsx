import React, { useEffect, useState } from "react";
import ReportDownloads from "../../../Common/reports/ReportDownloads";
import defaultDateRange from "../../../Common/DateAndTime/defaultRange";
import DataTable from "../../../Common/DataTable/DataTable";
import Pagination from "../../../Shell/Pagination";
import usePagedRows from "../../../Common/useRowsPerPage";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { getDate } from "../../../Common/DateAndTime/getDate";
import { gstReportBackend } from "../gst_report_backend";
import { notifyError } from "../../../global/toast";
import DateField from "../../../Common/DateField";

const slabAmount = (perSlab, slab, key) => (perSlab[slab] ? perSlab[slab][key] : 0);

// Every GST slab contributes four columns (CGST/SGST/IGST/GST), but a given invoice only sits
// in one slab and only one of intrastate/interstate - so most cells on any row are 0.00.
// Printing them all at full contrast buries the handful of figures that carry the tax. Dimming
// the zeros keeps the grid complete (the columns still line up for reading down a slab)
// while letting the real numbers come forward.
const Amount = ({ value, bold }) => {
    const amount = Number(value) || 0;
    return (
        <td
            className="cell-mono"
            style={{
                fontWeight: bold ? 700 : undefined,
                color: amount === 0 ? "var(--text-tertiary)" : undefined,
            }}
        >
            ₹{RoundOff(amount)}
        </td>
    );
};

// Sr | Invoice No | Invoice Date | Customer Name | GST No | Bill Amount | per-slab
// CGST/SGST/IGST/GST columns | Final Bill - shared shape for both Sales and Purchase tables.
// Columns are dynamic here - one group of four per tax slab in the data - so the export is
// built from the same `slabs` array the table renders from rather than a fixed list. Widths
// are shared out evenly because the slab count is not known ahead of time.
const GST_KINDS = [
    { key: "cgst", label: "CGST", half: true },
    { key: "sgst", label: "SGST", half: true },
    { key: "igst", label: "IGST", half: false },
    { key: "gst", label: "GST", half: false },
];

// Which (slab, kind) pairs actually carry a figure. A sale is either intrastate (CGST+SGST)
// or interstate (IGST), never both, so printing four columns per slab means at least two
// all-zero columns per slab - and with three slabs that is twelve columns of which half are
// noise. The screen can afford them (it scrolls, and zeros are dimmed); a sheet of paper
// cannot. Columns with nothing in them are dropped from the PDF entirely.
const usedSlabColumns = (rows, totals, slabs) => {
    const used = [];
    slabs.forEach((slab) => {
        GST_KINDS.forEach((kind) => {
            const inRows = rows.some((row) => Number(slabAmount(row.perSlab, slab, kind.key)) !== 0);
            const inTotal = Number(slabAmount(totals?.perSlab, slab, kind.key)) !== 0;
            if (inRows || inTotal) used.push({ slab, kind });
        });
    });
    return used;
};

// Sr | Invoice No | Invoice Date | Customer Name | GST No | Bill Amount | per-slab
// CGST/SGST/IGST/GST columns | Final Bill - shared shape for both Sales and Purchase tables.
//
// Widths are WEIGHTS, not percentages, and normalised to exactly 100% at the end. They used
// to be hardcoded percentages plus a per-slab share, which summed past 100 in every case
// (102% at one slab, 122% at three) - and a react-pdf row whose children are wider than it
// overlaps them rather than wrapping, which is what made the printed report unreadable once
// more than one GST rate was in play.
const gstExport = (rows, totals, slabs, party) => {
    const used = usedSlabColumns(rows, totals, slabs);

    const columns = [
        { label: "Sr", weight: 3.5 },
        { label: "Invoice No", weight: 11 },
        { label: "Date", weight: 7 },
        { label: party, weight: 13 },
        { label: "GST No", weight: 11 },
        { label: "Bill", weight: 8, align: "right" },
    ];
    used.forEach(({ slab, kind }) => {
        columns.push({
            label: `${kind.label}@${kind.half ? RoundOff(slab / 2) : slab}%`,
            weight: 8,
            align: "right",
        });
    });
    columns.push({ label: "Final Bill", weight: 8.5, align: "right" });

    const totalWeight = columns.reduce((sum, c) => sum + c.weight, 0);
    const sized = columns.map(({ weight, ...rest }) => ({
        ...rest,
        width: `${((weight / totalWeight) * 100).toFixed(3)}%`,
    }));

    const body = rows.map((row) => {
        const cells = [row.sr, row.invoiceNo, getDate(row.invoiceDate), row.customerName, row.gstNo || "", RoundOff(row.billAmount)];
        used.forEach(({ slab, kind }) => cells.push(RoundOff(slabAmount(row.perSlab, slab, kind.key))));
        cells.push(RoundOff(row.finalBill));
        return cells;
    });

    const foot = ["Total", "", "", "", "", RoundOff(totals?.billAmount)];
    used.forEach(({ slab, kind }) => foot.push(RoundOff(slabAmount(totals?.perSlab, slab, kind.key))));
    foot.push(RoundOff(totals?.finalBill));

    return { columns: sized, rows: body, foot };
};

const GstTable = ({ title, rows, totals, slabs, party }) => {
    // Each table pages independently - Sales and Purchase are separate questions, and a
    // shared page number would move one while you were reading the other. Totals below
    // still come from `totals`, computed over every row, not the page.
    const { pageRows, page, setPage, perPage, total } = usePagedRows(rows);
    return (
    <div style={{ marginBottom: 24 }}>
        {/* A flex row so the download buttons sit hard right, level with the heading.
            .report-downloads already carries margin-left:auto for exactly this, but it was
            inert while this heading was a plain block, leaving the buttons stuck to the end
            of the title text. */}
        <div
            className="text-heading-brand"
            style={{ marginBottom: 8, display: "flex", alignItems: "center", gap: 12, flexWrap: "wrap" }}
        >
            {title}
            <ReportDownloads title={title} {...gstExport(rows, totals, slabs, party)} />
        </div>
        <div style={{ overflowX: "auto" }}>
            <DataTable>
                <thead>
                    <tr>
                        <th scope="col">Sr</th>
                        <th scope="col">Invoice No</th>
                        <th scope="col">Invoice Date</th>
                        <th scope="col">{party}</th>
                        <th scope="col">GST No</th>
                        <th scope="col">Bill Amount</th>
                        {slabs.map((slab) => (
                            <React.Fragment key={slab}>
                                <th scope="col">CGST@{RoundOff(slab / 2)}%</th>
                                <th scope="col">SGST@{RoundOff(slab / 2)}%</th>
                                <th scope="col">IGST@{slab}%</th>
                                <th scope="col">GST@{slab}%</th>
                            </React.Fragment>
                        ))}
                        <th scope="col">Final Bill</th>
                    </tr>
                </thead>
                <tbody>
                    {rows.length === 0 ? (
                        <tr>
                            <td colSpan={6 + slabs.length * 4 + 1} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 16 }}>
                                No {title.toLowerCase()} in this range.
                            </td>
                        </tr>
                    ) : (
                        pageRows.map((row) => (
                            <tr key={row.sr}>
                                <td className="cell-mono">{row.sr}</td>
                                <td className="cell-mono">{row.invoiceNo}</td>
                                <td className="cell-mono">{getDate(row.invoiceDate)}</td>
                                <td>{row.customerName}</td>
                                <td className="cell-mono">{row.gstNo || <span style={{ color: "var(--text-tertiary)" }}>—</span>}</td>
                                <Amount value={row.billAmount} />
                                {slabs.map((slab) => (
                                    <React.Fragment key={slab}>
                                        <Amount value={slabAmount(row.perSlab, slab, "cgst")} />
                                        <Amount value={slabAmount(row.perSlab, slab, "sgst")} />
                                        <Amount value={slabAmount(row.perSlab, slab, "igst")} />
                                        <Amount value={slabAmount(row.perSlab, slab, "gst")} />
                                    </React.Fragment>
                                ))}
                                <Amount value={row.finalBill} />
                            </tr>
                        ))
                    )}
                    <tr>
                        <td colSpan={5} style={{ fontWeight: 700 }}>
                            Total
                        </td>
                        <Amount value={totals.billAmount} bold />
                        {slabs.map((slab) => (
                            <React.Fragment key={slab}>
                                <Amount value={slabAmount(totals.perSlab, slab, "cgst")} bold />
                                <Amount value={slabAmount(totals.perSlab, slab, "sgst")} bold />
                                <Amount value={slabAmount(totals.perSlab, slab, "igst")} bold />
                                <Amount value={slabAmount(totals.perSlab, slab, "gst")} bold />
                            </React.Fragment>
                        ))}
                        <td className="cell-mono" style={{ fontWeight: 700 }}>
                            ₹{RoundOff(totals.finalBill)}
                        </td>
                    </tr>
                </tbody>
            </DataTable>
            <div className="table-foot">
                <span className="text-body-small table-foot-count">
                    {perPage > 0 && total > perPage ? `${pageRows.length} of ${total} rows` : `${total} rows`}
                </span>
                <Pagination totalItems={total} perPage={perPage} currentPage={page} setCurrentPage={setPage} />
            </div>
        </div>
    </div>
    );
};

const GstReportIndex = () => {
    // Today, back to the same day last month - the same opening range every report now
    // uses. This one started on the 1st, so early in a month it showed almost nothing.
    const [from, setFrom] = useState(() => defaultDateRange().from);
    const [to, setTo] = useState(() => defaultDateRange().to);
    const [report, setReport] = useState(null);
    const [loading, setLoading] = useState(false);

    // `live` drops the response of a request the user has already moved on from. Typing in
    // a date field fires a request per keystroke-ish change, and they do not come back in
    // the order they were sent - without this, a slower earlier request can land last and
    // repaint the table with the range the user just left.
    useEffect(() => {
        let live = true;
        setLoading(true);
        const formData = new FormData();
        if (from) formData.set("from", from);
        if (to) formData.set("to", to);
        gstReportBackend
            .get(formData)
            .then((res) => live && setReport(res.data))
            .catch((err) => live && notifyError(err.message || "Couldn't load the GST report."))
            .finally(() => live && setLoading(false));
        return () => {
            live = false;
        };
    }, [from, to]);

    return (
        <div className="shell-card">
            <div className="shell-card-header">
                <span className="text-heading-brand">GST Report</span>
            </div>
            <div style={{ padding: 20 }}>
                <div className="d-flex align-items-end" style={{ gap: 16, marginBottom: 20 }}>
                    <div>
                        <label className="form-control-label pp fs-12" style={{ display: "block" }}>
                            From
                        </label>
                        <DateField value={from} onChange={(e) => setFrom(e.target.value)} />
                    </div>
                    <div>
                        <label className="form-control-label pp fs-12" style={{ display: "block" }}>
                            To
                        </label>
                        <DateField value={to} onChange={(e) => setTo(e.target.value)} />
                    </div>
                </div>

                {loading && (
                    <p className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                        Loading...
                    </p>
                )}

                {!loading && report && (
                    <>
                        <GstTable title="Sales GST Report" rows={report.sales.rows} totals={report.sales.totals} slabs={report.slabs} party="Customer Name" />
                        <GstTable
                            title="Purchase GST Report"
                            rows={report.purchase.rows}
                            totals={report.purchase.totals}
                            slabs={report.slabs}
                            party="Supplier Name"
                        />

                        <div className="text-heading-brand" style={{ marginBottom: 8 }}>
                            GST Summary
                        </div>
                        <DataTable>
                            <thead>
                                <tr>
                                    <th scope="col">Tax Name</th>
                                    <th scope="col">Tax Collected</th>
                                    <th scope="col">Tax Paid</th>
                                    <th scope="col">Tax to Pay/Collect</th>
                                </tr>
                            </thead>
                            <tbody>
                                {report.summary.length === 0 ? (
                                    <tr>
                                        <td colSpan={4} className="text-body-small" style={{ color: "var(--text-tertiary)", padding: 16 }}>
                                            No taxable transactions in this range.
                                        </td>
                                    </tr>
                                ) : (
                                    report.summary.map((row) => (
                                        <tr key={row.slab}>
                                            <td>GST@{row.slab}%</td>
                                            <td>
                                                <div className="cell-mono" style={{ fontWeight: 700 }}>
                                                    ₹{RoundOff(row.collected.total)}
                                                </div>
                                                <div className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                                    CGST ({RoundOff(row.collected.cgst)}) + SGST ({RoundOff(row.collected.sgst)}) + IGST ({RoundOff(row.collected.igst)})
                                                </div>
                                            </td>
                                            <td>
                                                <div className="cell-mono" style={{ fontWeight: 700 }}>
                                                    ₹{RoundOff(row.paid.total)}
                                                </div>
                                                <div className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                                                    CGST ({RoundOff(row.paid.cgst)}) + SGST ({RoundOff(row.paid.sgst)}) + IGST ({RoundOff(row.paid.igst)})
                                                </div>
                                            </td>
                                            <td className="cell-mono" style={{ fontWeight: 700 }}>
                                                ₹{RoundOff(row.toPayOrCollect)}
                                            </td>
                                        </tr>
                                    ))
                                )}
                                <tr>
                                    <td style={{ fontWeight: 700 }}>Total</td>
                                    <td className="cell-mono" style={{ fontWeight: 700 }}>
                                        ₹{RoundOff(report.summaryTotal.collected)}
                                    </td>
                                    <td className="cell-mono" style={{ fontWeight: 700 }}>
                                        ₹{RoundOff(report.summaryTotal.paid)}
                                    </td>
                                    <td className="cell-mono" style={{ fontWeight: 700 }}>
                                        ₹{RoundOff(report.summaryTotal.toPayOrCollect)}
                                    </td>
                                </tr>
                            </tbody>
                        </DataTable>
                    </>
                )}
            </div>
        </div>
    );
};

export default GstReportIndex;
