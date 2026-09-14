import React from "react";
import { Page, Document, Text, View } from "@react-pdf/renderer";

import { resolveDesignScaled } from "../../Common/pdf/designs";
import { registerDocumentFont, money } from "../../Common/pdf/fonts";
import { makeStyles, Letterhead, Party, DocTable, Totals } from "../../Common/pdf/parts";
import { getDate } from "../../Common/DateAndTime/getDate";

// The one client-ledger renderer.
//
// This is the document the whole change was for: the ledger offered a single design while
// invoices and quotations offered four, because a design meant a hand-written component per
// document type and nobody wrote the ledger's three times. Reading the design from designs.js
// means the Ledger tab now lists exactly what the other two tabs list.

const COLUMNS = [
    { key: "sr", label: "#", width: "6%", align: "center" },
    { key: "date", label: "Date", width: "13%" },
    { key: "type", label: "Type", width: "17%" },
    { key: "invoiceNo", label: "Invoice No", width: "16%" },
    { key: "bill", label: "Bill Amount", width: "16%", align: "right" },
    { key: "receipt", label: "Receipt", width: "16%", align: "right" },
    { key: "balance", label: "Balance", width: "16%", align: "right" },
];

const LedgerDoc = ({ ledger, user, from, to, designKey, fontKey, scaleId }) => {
    // Scaled here rather than in each renderer: the size preference is one setting
    // across every design, so it belongs where a design is resolved.
    const design = resolveDesignScaled(designKey, scaleId);
    const spec = design.spec;
    const font = registerDocumentFont(fontKey);
    const styles = makeStyles(spec, font);
    const amount = (value) => money(value, font);

    const rows = ledger?.rows || [];
    const period = from || to ? `${from ? getDate(from) : "Start"} to ${to ? getDate(to) : "Today"}` : "All time";

    const bodyRows = rows.map((row) => ({
        sr: row.sr,
        date: getDate(row.date),
        type: row.type,
        invoiceNo: row.invoiceNo || "-",
        // A blank cell where there is no figure, rather than a zero that would read as an
        // amount of nothing having been billed or received on that line.
        bill: row.bill != null ? amount(row.bill) : "",
        receipt: row.receipt != null ? amount(row.receipt) : "",
        balance: amount(row.balance),
    }));

    return (
        <Document>
            <Page size="A4" style={styles.page}>
                <Letterhead
                    spec={spec}
                    styles={styles}
                    title="Statement of Account"
                    firm={user?.firm}
                    firmLines={[user?.address, user?.phone, user?.gst && `GST ${user.gst}`]}
                    logo={user?.url}
                    meta={[{ label: "Period", value: period }]}
                />

                <View style={styles.body}>
                    <View style={styles.partiesRow}>
                        <View style={styles.partyCol}>
                            <Party styles={styles} label="Account Of" name={ledger?.clientFirm || ledger?.clientName} />
                        </View>
                        <View style={styles.partyCol}>
                            <Party
                                styles={styles}
                                label="Opening Balance"
                                name={amount(ledger?.openingBalance)}
                                lines={[`Period: ${period}`]}
                            />
                        </View>
                    </View>

                    <DocTable styles={styles} columns={COLUMNS} rows={bodyRows} />

                    <Totals
                        styles={styles}
                        lines={[{ label: "Opening Balance", value: amount(ledger?.openingBalance) }]}
                        grand={{ label: "Balance Due", value: amount(ledger?.currentBalance) }}
                    />

                    <Text style={styles.note}>
                        Balance as of {getDate(to || new Date())}. This is a computer generated statement.
                    </Text>
                </View>
            </Page>
        </Document>
    );
};

export default LedgerDoc;
