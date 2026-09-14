import React from "react";
import { Page, Document, Text, View, StyleSheet, Font } from "@react-pdf/renderer";
import { getDate } from "../../Common/DateAndTime/getDate";
import { RoundOff } from "../../Common/DateAndTime/RoundOff";
import OSR400 from "../Invoice/font/OSR400.ttf";
import OS700 from "../Invoice/font/OS700.ttf";

Font.register({
    family: "OS",
    fonts: [
        { src: OSR400, fontWeight: 400 },
        { src: OS700, fontWeight: 700 },
    ],
});

// Same template family as Views/Ledger/LedgerDoc.jsx and BankReportPDF.jsx - identical page
// padding, letterhead, summary strip and bordered table, so every printed report matches.
const styles = StyleSheet.create({
    page: { fontFamily: "OS", fontSize: 8.5, padding: 24, color: "#222" },
    firmName: { fontSize: 13, fontWeight: 700 },
    small: { fontSize: 8, marginTop: 1, color: "#555" },
    title: { fontSize: 10, fontWeight: 700, marginTop: 12, marginBottom: 6 },
    summaryRow: { flexDirection: "row", marginBottom: 8 },
    summaryItem: { marginRight: 24 },
    summaryLabel: { fontSize: 7.5, color: "#888", textTransform: "uppercase" },
    summaryValue: { fontSize: 10, fontWeight: 700 },
    table: { borderWidth: 1, borderColor: "#000" },
    thRow: { flexDirection: "row", backgroundColor: "#eee", borderBottomWidth: 1, borderColor: "#000" },
    trRow: { flexDirection: "row", borderBottomWidth: 0.5, borderColor: "#999", minHeight: 16 },
    th: { fontWeight: 700, fontSize: 8, padding: 3 },
    td: { fontSize: 8, padding: 3 },
    cSr: { width: "7%" },
    cDate: { width: "17%" },
    cBank: { width: "22%" },
    cNote: { width: "26%" },
    // Two money columns, matching the on-screen list and the xlsx export.
    cCredit: { width: "14%", textAlign: "right" },
    cExpense: { width: "14%", textAlign: "right" },
});

const ExpenseReportPDF = ({ rows = [], total = 0, user, from, to }) => (
    <Document>
        <Page size="A4" style={styles.page}>
            <Text style={styles.firmName}>{user?.firm}</Text>
            {user?.address ? <Text style={styles.small}>{user.address}</Text> : null}
            {user?.gst ? <Text style={styles.small}>GST No: {user.gst}</Text> : null}

            <Text style={styles.title}>
                Expense Report
                {from || to ? ` (${from ? getDate(from) : "Start"} to ${to ? getDate(to) : "Today"})` : ""}
            </Text>

            <View style={styles.summaryRow}>
                <View style={styles.summaryItem}>
                    <Text style={styles.summaryLabel}>Entries</Text>
                    <Text style={styles.summaryValue}>{rows.length}</Text>
                </View>
                <View style={styles.summaryItem}>
                    <Text style={styles.summaryLabel}>Total Spent</Text>
                    <Text style={styles.summaryValue}>₹{RoundOff(total)}</Text>
                </View>
            </View>

            <View style={styles.table}>
                <View style={styles.thRow}>
                    <Text style={[styles.th, styles.cSr]}>Sr</Text>
                    <Text style={[styles.th, styles.cDate]}>Date</Text>
                    <Text style={[styles.th, styles.cBank]}>Bank</Text>
                    <Text style={[styles.th, styles.cNote]}>Notes</Text>
                    <Text style={[styles.th, styles.cCredit]}>Credit</Text>
                    <Text style={[styles.th, styles.cExpense]}>Expense</Text>
                </View>
                {rows.map((row, i) => (
                    <View key={row._id || i} style={styles.trRow}>
                        <Text style={[styles.td, styles.cSr]}>{i + 1}</Text>
                        <Text style={[styles.td, styles.cDate]}>{row.date ? getDate(row.date) : "-"}</Text>
                        <Text style={[styles.td, styles.cBank]}>{row.bank_id?.name || "-"}</Text>
                        <Text style={[styles.td, styles.cNote]}>{row.notes || "-"}</Text>
                        <Text style={[styles.td, styles.cCredit]}>-</Text>
                        <Text style={[styles.td, styles.cExpense]}>{RoundOff(row.amount)}</Text>
                    </View>
                ))}
                <View style={styles.trRow}>
                    <Text style={[styles.td, styles.cSr]} />
                    <Text style={[styles.td, styles.cDate]} />
                    <Text style={[styles.td, styles.cBank]} />
                    <Text style={[styles.td, styles.cNote, { fontWeight: 700 }]}>Total</Text>
                    <Text style={[styles.td, styles.cCredit]} />
                    <Text style={[styles.td, styles.cExpense, { fontWeight: 700 }]}>{RoundOff(total)}</Text>
                </View>
            </View>
        </Page>
    </Document>
);

export default ExpenseReportPDF;
