import React from "react";
import { Page, Document, Text, View, Image } from "@react-pdf/renderer";

import { resolveDesignScaled } from "../../../Common/pdf/designs";
import { registerDocumentFont, money } from "../../../Common/pdf/fonts";
import { makeStyles, Letterhead, Party, DocTable, Totals } from "../../../Common/pdf/parts";
import { getDateForInvoice } from "../../../Common/DateAndTime/getDate";
import { amountInWords } from "../../../Common/numberToWords";
import { describeGoods, hsnTaxRows } from "../invoiceFormat";
import { invoiceTotals } from "../invoiceTotals";

// The one invoice renderer. Which of the six designs it draws is decided by `designKey`
// (Company.invoiceTemplate); the font by `fontKey` (Company.documentFont).
//
// There used to be four components here - Invoice, InvoiceGstDetailed, InvoiceModern,
// InvoiceCompact - each with its own copy of the totals arithmetic, one of which taxed
// everything at a hardcoded 18%. The arithmetic now lives in invoiceTotals.js and the look
// in designs.js, so a new design cannot bring a new rounding bug with it.

const COLUMNS = (detailed) =>
    [
        { key: "sr", label: "#", width: "5%", align: "center" },
        { key: "desc", label: "Description of Goods", width: detailed ? "33%" : "40%" },
        detailed ? { key: "hsn", label: "HSN", width: "10%" } : null,
        { key: "qty", label: "Qty", width: "10%", align: "center" },
        { key: "rate", label: "Rate", width: "14%", align: "right" },
        detailed ? { key: "gst", label: "GST", width: "8%", align: "right" } : null,
        { key: "amount", label: "Amount", width: detailed ? "20%" : "31%", align: "right" },
    ].filter(Boolean);

const InvoiceDoc = ({ invoice, designKey, fontKey, scaleId }) => {
    // Scaled here rather than in each renderer: the size preference is one setting
    // across every design, so it belongs where a design is resolved.
    const design = resolveDesignScaled(designKey, scaleId);
    const spec = design.spec;
    const font = registerDocumentFont(fontKey);
    const styles = makeStyles(spec, font);
    const amount = (value) => money(value, font);

    const entries = invoice?.entries || [];
    const totals = invoiceTotals(entries, invoice);
    // The Detailed design is the one that shows per-line HSN and GST and the HSN summary
    // table below the totals - that is what makes it detailed rather than merely bordered.
    const detailed = design.key === "detailed";

    const rows = entries.map((item, index) => ({
        sr: index + 1,
        desc: describeGoods(item),
        hsn: item.hsn || "-",
        qty: item.qty,
        rate: amount(item.rate),
        gst: `${(Number(item.cgst) || 0) + (Number(item.sgst) || 0) + (Number(item.igst) || 0)}%`,
        amount: amount(
            (Number(item.qty) || 0) * (Number(item.rate) || 0) - (Number(item.discount) || 0) + (Number(item.charges) || 0)
        ),
    }));

    const hsnRows = detailed ? hsnTaxRows(entries) : [];

    return (
        <Document>
            <Page size="A4" style={styles.page}>
                <Letterhead
                    spec={spec}
                    styles={styles}
                    title="Tax Invoice"
                    firm={invoice?.firm}
                    firmLines={[invoice?.address, invoice?.phone, invoice?.email, invoice?.gst && `GST ${invoice.gst}`]}
                    logo={invoice?.url}
                    meta={[
                        { label: "Invoice No", value: invoice?.invoiceNumber },
                        { label: "Date", value: getDateForInvoice(invoice?.date) },
                    ]}
                />

                <View style={styles.body}>
                    <View style={styles.partiesRow}>
                        <View style={styles.partyCol}>
                            <Party
                                styles={styles}
                                label="Bill To"
                                name={invoice?.clientFirm}
                                lines={[
                                    invoice?.clientAddress,
                                    invoice?.clientPhone,
                                    invoice?.clientGST && `GST ${invoice.clientGST}`,
                                ]}
                            />
                        </View>
                        <View style={styles.partyCol}>
                            {/* Bank details default to "" rather than undefined on Company, and
                                a bare "" child crashes react-pdf's layout engine - hence the
                                Boolean() guards rather than a plain && chain. */}
                            {Boolean(invoice?.bank_name) && (
                                <Party
                                    styles={styles}
                                    label="Bank Details"
                                    name={invoice.bank_name}
                                    lines={[
                                        invoice.account && `A/c ${invoice.account}`,
                                        invoice.ifsc && `IFSC ${invoice.ifsc}`,
                                    ]}
                                />
                            )}
                            {Boolean(invoice?.upiQr) && (
                                <View style={{ width: 64, marginTop: 6 }}>
                                    <Image
                                        style={{ width: 64, height: 64, objectFit: "contain" }}
                                        src={{ uri: `${import.meta.env.VITE_API_URL}/uploads/${invoice.upiQr}` }}
                                    />
                                    <Text style={[styles.small, { textAlign: "center" }]}>Scan to pay</Text>
                                </View>
                            )}
                        </View>
                    </View>

                    <DocTable styles={styles} columns={COLUMNS(detailed)} rows={rows} />

                    <Totals
                        styles={styles}
                        lines={[
                            { label: "Subtotal", value: amount(totals.subtotal) },
                            totals.showDiscount ? { label: "Discount", value: `-${amount(totals.discount)}` } : null,
                            totals.showCharges ? { label: "Charges", value: amount(totals.charges) } : null,
                            ...totals.taxRows.map((line) => ({ label: line.label, value: amount(line.amount) })),
                            // Through amount() like every line around it. Raw, it printed
                            // "0.0" in a column of "12,105.00" - the one figure on the
                            // document that looked like a typing error.
                            { label: "Round Off", value: amount(totals.roundOff) },
                        ]}
                        grand={{ label: "Grand Total", value: amount(totals.grandTotal) }}
                        after={
                            totals.showBalance
                                ? [
                                      { label: "Received", value: amount(totals.received) },
                                      { label: "Balance Due", value: amount(totals.balance), strong: true },
                                  ]
                                : []
                        }
                    />

                    <Text style={styles.note}>
                        Amount in words: {amountInWords(totals.grandTotal)}
                    </Text>

                    {/* hsnTaxRows groups by HSN and rate and reports the taxable value; the
                        rate and the tax it produces are derived below, so the grouping helper
                        stays about grouping. */}
                    {detailed && hsnRows.length > 0 && (
                        <DocTable
                            styles={styles}
                            columns={[
                                { key: "hsn", label: "HSN", width: "28%" },
                                { key: "taxable", label: "Taxable Value", width: "24%", align: "right" },
                                { key: "rate", label: "Rate", width: "16%", align: "right" },
                                { key: "tax", label: "Tax Amount", width: "32%", align: "right" },
                            ]}
                            rows={hsnRows.map((r) => {
                                const rate = (Number(r.cgst) || 0) + (Number(r.sgst) || 0);
                                return {
                                    hsn: r.hsn || "-",
                                    taxable: amount(r.taxable),
                                    rate: `${rate}%`,
                                    tax: amount((r.taxable * rate) / 100),
                                };
                            })}
                        />
                    )}

                    <View style={styles.footer}>
                        <Text style={styles.small}>This is a computer generated invoice.</Text>
                        <Text style={styles.small}>For {invoice?.firm}</Text>
                    </View>
                </View>
            </Page>
        </Document>
    );
};

export default InvoiceDoc;
