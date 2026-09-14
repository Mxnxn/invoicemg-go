import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import axios from "axios";

import { triggerDownload } from "../BankReport/downloads";
import { rowAmount, rowTotal, purchaseInvoiceGrandTotal } from "../PurchaseInvoice/purchaseInvoiceMath";
import "./prePurchaseOrder.css";

const money = (n) => `₹${Number(n || 0).toLocaleString("en-IN")}`;

const fileStem = (poNumber) => String(poNumber || "purchase-order").replace(/\//g, "-");

/**
 * The purchase order a supplier opens from a link.
 *
 * Unauthenticated: the reader is a supplier, not a user. It shows one order and offers it as
 * a PDF or a spreadsheet, and once the order has become a purchase invoice it shows only that
 * the order is closed - see Helpers/PoPublic.js on the server for what is and is not served.
 *
 * Seen by people with no session and no stored theme, so it has to read correctly under the
 * default theme without one being set.
 */
export default function PrePurchaseOrder() {
    const { supplier_id, po_id } = useParams();
    const [state, setState] = useState({ loading: true, error: "", notFound: false, data: null });
    const [busy, setBusy] = useState("");

    useEffect(() => {
        axios
            .get(`${import.meta.env.VITE_API_URL}/po-public/${supplier_id}/${po_id}`)
            .then((res) => {
                if (res.data.code === 404) {
                    setState({ loading: false, error: "", notFound: true, data: null });
                    return;
                }
                if (res.data.code !== 200) throw res.data;
                setState({ loading: false, error: "", notFound: false, data: res.data.data });
            })
            .catch(() => setState({ loading: false, error: "Could not load this purchase order.", notFound: false, data: null }));
    }, [supplier_id, po_id]);

    const order = state.data?.order;
    const company = state.data?.company || {};

    // The logo lives behind the API's /uploads mount, not at the bare stored filename - the
    // same URL the report exports build.
    const logoUrl = company.url ? `${import.meta.env.VITE_API_URL}/uploads/${company.url}` : "";

    const onXlsx = async () => {
        if (!order) return;
        setBusy("xlsx");
        try {
            // Lazily imported at download time: ExcelJS is large and most visitors only read.
            const { exportWorkbook } = await import("../../Common/reports/exportWorkbook");
            const blob = await exportWorkbook({
                title: `Purchase Order ${order.poNumber}`,
                subtitle: `${company.firm || ""} · ${order.date}`,
                columns: [
                    { key: "sr", label: "#" },
                    { key: "desc", label: "Description" },
                    { key: "hsn", label: "HSN" },
                    { key: "qty", label: "Qty" },
                    { key: "rate", label: "Rate", numeric: true },
                    { key: "total", label: "Total", numeric: true },
                ],
                // Positional arrays, in column order - exportWorkbook writes cells by index
                // (row?.[i]), not by key.
                rows: order.rows.map((row, i) => [
                    i + 1,
                    [row.material, row.description].filter(Boolean).join(": "),
                    row.hsn,
                    `${row.qty} ${row.unit}`.trim(),
                    rowAmount(row),
                    rowTotal(row),
                ]),
                foot: ["", "", "", "", "Order Total", purchaseInvoiceGrandTotal(order.rows)],
                company,
                template: company.exportTemplate || {},
                logoUrl,
            });
            triggerDownload(blob, `${fileStem(order.poNumber)}.xlsx`);
        } catch (error) {
            setState((p) => ({ ...p, error: "" }));
            window.alert("Couldn't generate the spreadsheet - please try again.");
        } finally {
            setBusy("");
        }
    };

    const onPdf = async () => {
        if (!order) return;
        setBusy("pdf");
        try {
            // @react-pdf has a top-level require() that is invalid in a browser ESM bundle, so
            // it is only pulled in when a download is actually asked for.
            const [{ pdf }, { default: PurchaseOrderDoc }] = await Promise.all([
                import("@react-pdf/renderer"),
                import("./Template/PurchaseOrderDoc"),
            ]);
            const render = (logo) =>
                pdf(
                    <PurchaseOrderDoc
                        order={order}
                        company={{ ...company, url: logo }}
                        designKey={company.exportTemplate?.design}
                        fontKey={company.documentFont}
                        scaleId={company.documentScale}
                    />
                ).toBlob();

            // @react-pdf throws on an <Image src> it cannot fetch, and that failure takes the
            // whole document with it - a company whose logo file has gone missing would lose
            // the PDF entirely rather than lose the logo. Retrying without it costs nothing in
            // the normal case and keeps the order available in the broken one.
            let blob;
            try {
                blob = await render(logoUrl);
            } catch (logoError) {
                if (!logoUrl) throw logoError;
                blob = await render("");
            }
            triggerDownload(blob, `${fileStem(order.poNumber)}.pdf`);
        } catch (error) {
            window.alert("Couldn't generate the PDF - please try again.");
        } finally {
            setBusy("");
        }
    };

    if (state.loading) return <p className="po-public-state text-body-regular">Loading…</p>;
    if (state.error) return <p className="po-public-state text-body-regular">{state.error}</p>;
    if (state.notFound) return <p className="po-public-state text-body-regular">This purchase order was not found.</p>;

    if (state.data?.closed) {
        return (
            <div className="po-public">
                <div className="po-public-card">
                    <h1 className="text-heading-page">This order is no longer current</h1>
                    <p className="text-body-regular po-public-closed">
                        It has been closed and is no longer available here. Please get in touch if you need a copy.
                    </p>
                </div>
            </div>
        );
    }

    return (
        <div className="po-public">
            <div className="po-public-card">
                <header className="po-public-head">
                    <div>
                        <p className="text-heading-page">{company.firm}</p>
                        <p className="text-body-small">
                            {[company.address, company.phone, company.gst && `GST ${company.gst}`].filter(Boolean).join(" · ")}
                        </p>
                    </div>
                    <div className="po-public-meta">
                        <p className="text-body-medium">{order.poNumber}</p>
                        <p className="text-body-small">{order.date}</p>
                    </div>
                </header>

                <p className="text-body-small po-public-party">
                    Ordered from <strong>{order.supplier.firm || order.supplier.name}</strong>
                </p>

                <table className="po-public-table">
                    <thead>
                        <tr>
                            <th scope="col">Description</th>
                            <th scope="col">HSN</th>
                            <th scope="col">Qty</th>
                            <th scope="col">Rate</th>
                            <th scope="col">Total</th>
                        </tr>
                    </thead>
                    <tbody>
                        {order.rows.map((row, i) => (
                            <tr key={i}>
                                <td>{[row.material, row.description].filter(Boolean).join(": ")}</td>
                                <td>{row.hsn}</td>
                                <td>
                                    {row.qty} {row.unit}
                                </td>
                                <td>{money(row.rate)}</td>
                                <td>{money(rowTotal(row))}</td>
                            </tr>
                        ))}
                    </tbody>
                </table>

                <p className="po-public-total text-body-medium">Order Total {money(purchaseInvoiceGrandTotal(order.rows))}</p>

                <footer className="po-public-foot">
                    <button type="button" className="shell-btn shell-btn-primary" onClick={onPdf} disabled={busy === "pdf"}>
                        {busy === "pdf" ? "Preparing PDF…" : "Download PDF"}
                    </button>
                    <button type="button" className="shell-btn shell-btn-secondary" onClick={onXlsx} disabled={busy === "xlsx"}>
                        {busy === "xlsx" ? "Preparing Excel…" : "Download Excel"}
                    </button>
                </footer>
            </div>
        </div>
    );
}
