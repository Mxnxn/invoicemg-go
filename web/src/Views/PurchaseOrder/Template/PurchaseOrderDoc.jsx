import React from "react";
import { Page, Document, Text, View } from "@react-pdf/renderer";

import { resolveDesignScaled } from "../../../Common/pdf/designs";
import { registerDocumentFont, money } from "../../../Common/pdf/fonts";
import { makeStyles, Letterhead, Party, DocTable, Totals } from "../../../Common/pdf/parts";
import { getDateForInvoice } from "../../../Common/DateAndTime/getDate";
import { describeSize } from "../../Invoice/invoiceFormat";
import { rowAmount, rowTotal, purchaseInvoiceGrandTotal } from "../../PurchaseInvoice/purchaseInvoiceMath";

// The purchase order a supplier receives - the same six designs, the same parts kit and the
// same typeface as the firm's invoices and quotations, so a supplier and a customer get
// paperwork that plainly comes from one business.
//
// registerDocumentFont is what makes Company.documentFont apply here, and it is also why
// money() takes the font: react-pdf 1.x has no fallback chain, so a family with no U+20B9
// renders the rupee sign as an empty box. money() writes "Rs " for those instead.
//
// The arithmetic is purchaseInvoiceMath, shared with the purchase invoice this order becomes
// - and with the server's Helpers/PurchaseRowTotal.js - so the PDF, the screen and the
// database cannot show three different totals.

const COLUMNS = [
    { key: "sr", label: "#", width: "5%", align: "center" },
    { key: "desc", label: "Description", width: "47%" },
    { key: "qty", label: "Qty", width: "10%", align: "center" },
    { key: "rate", label: "Rate", width: "13%", align: "right" },
    { key: "amount", label: "Amount", width: "12%", align: "right" },
    { key: "total", label: "Total", width: "13%", align: "right" },
];

const describeRow = (row) => {
    const parts = [row.material, row.description].map((p) => String(p || "").trim()).filter(Boolean);
    const base = parts.join(": ");
    const size = describeSize(row);
    if (!size) return base;
    return base ? `${base} · ${size}` : size;
};

const PurchaseOrderDoc = ({ order, company, designKey, fontKey, scaleId }) => {
    // Scaled here rather than in each renderer: the size preference is one setting
    // across every design, so it belongs where a design is resolved.
    const design = resolveDesignScaled(designKey, scaleId);
    const spec = design.spec;
    const font = registerDocumentFont(fontKey);
    const styles = makeStyles(spec, font);
    const amount = (value) => money(value, font);

    const rows = order?.rows || [];
    const supplier = order?.supplier || {};

    return (
        <Document>
            <Page size="A4" style={styles.page}>
                <Letterhead
                    spec={spec}
                    styles={styles}
                    title="Purchase Order"
                    firm={company?.firm}
                    firmLines={[company?.address, company?.phone, company?.gst && `GST ${company.gst}`]}
                    logo={company?.url}
                    meta={[
                        { label: "Order No", value: order?.poNumber },
                        { label: "Date", value: getDateForInvoice(order?.date) },
                    ]}
                />

                <View style={styles.body}>
                    <View style={styles.partiesRow}>
                        <View style={styles.partyCol}>
                            {/* "Ordered From", not "Billed To" - the supplier is the seller here,
                                which is the mirror image of every other document in this app. */}
                            <Party styles={styles} label="Ordered From" name={supplier.firm || supplier.name} lines={[]} />
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

                    <Totals styles={styles} grand={{ label: "Order Total", value: amount(purchaseInvoiceGrandTotal(rows)) }} />

                    <View style={styles.noteBox}>
                        <Text style={styles.label}>Notes</Text>
                        <Text style={styles.small}>
                            Please confirm receipt of this order and advise the expected delivery date.
                        </Text>
                    </View>
                </View>
            </Page>
        </Document>
    );
};

export default PurchaseOrderDoc;
