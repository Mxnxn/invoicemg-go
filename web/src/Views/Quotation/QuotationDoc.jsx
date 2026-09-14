import React from "react";
import { Page, Document, Text, View } from "@react-pdf/renderer";

import { resolveDesignScaled } from "../../Common/pdf/designs";
import { registerDocumentFont, money } from "../../Common/pdf/fonts";
import { makeStyles, Letterhead, Party, DocTable, Totals } from "../../Common/pdf/parts";
import { getDateForInvoice } from "../../Common/DateAndTime/getDate";
import { describeSize } from "../Invoice/invoiceFormat";
import { rowAmount, rowTotal, quotationGrandTotal } from "./quotationMath";

// The one quotation renderer - same six designs as the invoice and the ledger, drawn from the
// same spec. See Views/Invoice/Template/InvoiceDoc.jsx for why this replaced four components.

const COLUMNS = [
    { key: "sr", label: "#", width: "5%", align: "center" },
    { key: "desc", label: "Description", width: "47%" },
    { key: "qty", label: "Qty", width: "10%", align: "center" },
    { key: "rate", label: "Rate", width: "13%", align: "right" },
    { key: "amount", label: "Amount", width: "12%", align: "right" },
    { key: "total", label: "Total", width: "13%", align: "right" },
];

// The product, the note typed on the job card, and the size if the row has one - the same
// sentence the invoice builds, so a quotation and the invoice that follows it read alike.
const describeRow = (row) => {
    const parts = [row.material, row.description].map((p) => String(p || "").trim()).filter(Boolean);
    const base = parts.join(": ");
    const size = describeSize(row);
    if (!size) return base;
    return base ? `${base} · ${size}` : size;
};

const QuotationDoc = ({ quotation, user, designKey, fontKey, scaleId }) => {
    // Scaled here rather than in each renderer: the size preference is one setting
    // across every design, so it belongs where a design is resolved.
    const design = resolveDesignScaled(designKey, scaleId);
    const spec = design.spec;
    const font = registerDocumentFont(fontKey);
    const styles = makeStyles(spec, font);
    const amount = (value) => money(value, font);

    const rows = quotation?.rows || [];
    const client = quotation?.client_id || {};

    return (
        <Document>
            <Page size="A4" style={styles.page}>
                <Letterhead
                    spec={spec}
                    styles={styles}
                    title="Quotation"
                    firm={user?.firm}
                    firmLines={[user?.address, user?.phone, user?.gst && `GST ${user.gst}`]}
                    logo={user?.url}
                    meta={[
                        { label: "Quotation No", value: quotation?.quotationNumber },
                        { label: "Date", value: getDateForInvoice(quotation?.date) },
                    ]}
                />

                <View style={styles.body}>
                    <View style={styles.partiesRow}>
                        <View style={styles.partyCol}>
                            <Party
                                styles={styles}
                                label="Quoted To"
                                name={client.clientFirm}
                                lines={[client.clientAddress, client.clientPhone, client.clientGST && `GST ${client.clientGST}`]}
                            />
                        </View>
                    </View>

                    <DocTable
                        styles={styles}
                        columns={COLUMNS}
                        rows={rows.map((row, index) => ({
                            sr: index + 1,
                            desc: describeRow(row),
                            qty: row.qty,
                            rate: amount(row.rate),
                            amount: amount(rowAmount(row)),
                            total: amount(rowTotal(row)),
                        }))}
                    />

                    <Totals styles={styles} grand={{ label: "Grand Total", value: amount(quotationGrandTotal(rows)) }} />

                    <View style={styles.noteBox}>
                        <Text style={styles.label}>Notes</Text>
                        <Text style={styles.small}>This quotation is valid for 15 days from the date above.</Text>
                    </View>
                </View>
            </Page>
        </Document>
    );
};

export default QuotationDoc;
