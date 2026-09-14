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

// Same visual language as Views/Ledger/LedgerDoc.jsx - identical page padding, font sizes,
// firm header, summary strip and bordered table - so the two reports read as one family
// when printed side by side. Only the columns differ.
const styles = StyleSheet.create({
    page: { fontFamily: "OS", fontSize: 8.5, padding: 24, color: "#222" },
    firmName: { fontSize: 13, fontWeight: 700 },
    small: { fontSize: 8, marginTop: 1, color: "#555" },
    title: { fontSize: 10, fontWeight: 700, marginTop: 12, marginBottom: 6 },
    bankName: { fontSize: 9.5, fontWeight: 700, marginTop: 10, marginBottom: 4 },
    summaryRow: { flexDirection: "row", marginBottom: 8 },
    summaryItem: { marginRight: 24 },
    summaryLabel: { fontSize: 7.5, color: "#888", textTransform: "uppercase" },
    summaryValue: { fontSize: 10, fontWeight: 700 },
    table: { borderWidth: 1, borderColor: "#000" },
    thRow: { flexDirection: "row", backgroundColor: "#eee", borderBottomWidth: 1, borderColor: "#000" },
    trRow: { flexDirection: "row", borderBottomWidth: 0.5, borderColor: "#999", minHeight: 16 },
    th: { fontWeight: 700, fontSize: 8, padding: 3 },
    td: { fontSize: 8, padding: 3 },
    cDate: { width: "16%" },
    cType: { width: "22%" },
    cNote: { width: "22%" },
    cCredit: { width: "12%", textAlign: "right" },
    cExpense: { width: "12%", textAlign: "right" },
    cBalance: { width: "16%", textAlign: "right" },
    empty: { fontSize: 8, padding: 4, color: "#777" },
});

const BankReportPDF = ({ report, user, from, to }) => (
    <Document>
        <Page size="A4" style={styles.page}>
            <Text style={styles.firmName}>{user?.firm}</Text>
            {user?.address ? <Text style={styles.small}>{user.address}</Text> : null}
            {user?.gst ? <Text style={styles.small}>GST No: {user.gst}</Text> : null}

            <Text style={styles.title}>
                Bank Report
                {from || to ? ` (${from ? getDate(from) : "Start"} to ${to ? getDate(to) : "Today"})` : ""}
            </Text>

            <View style={styles.summaryRow}>
                <View style={styles.summaryItem}>
                    <Text style={styles.summaryLabel}>Opening</Text>
                    <Text style={styles.summaryValue}>₹{RoundOff(report.totals?.opening)}</Text>
                </View>
                <View style={styles.summaryItem}>
                    <Text style={styles.summaryLabel}>In</Text>
                    <Text style={styles.summaryValue}>₹{RoundOff(report.totals?.credits)}</Text>
                </View>
                <View style={styles.summaryItem}>
                    <Text style={styles.summaryLabel}>Out</Text>
                    <Text style={styles.summaryValue}>₹{RoundOff(report.totals?.debits)}</Text>
                </View>
                <View style={styles.summaryItem}>
                    <Text style={styles.summaryLabel}>Current</Text>
                    <Text style={styles.summaryValue}>₹{RoundOff(report.totals?.current)}</Text>
                </View>
            </View>

            {(report.rows || []).map((bank) => (
                <View key={bank.bankId} wrap={false}>
                    <Text style={styles.bankName}>
                        {bank.bankName} — Opening ₹{RoundOff(bank.opening)} · In ₹{RoundOff(bank.credits)} · Out ₹
                        {RoundOff(bank.debits)} · Current ₹{RoundOff(bank.current)}
                    </Text>
                    <View style={styles.table}>
                        <View style={styles.thRow}>
                            <Text style={[styles.th, styles.cDate]}>Date</Text>
                            <Text style={[styles.th, styles.cType]}>Type</Text>
                            <Text style={[styles.th, styles.cNote]}>Notes</Text>
                            <Text style={[styles.th, styles.cCredit]}>Credit</Text>
                            <Text style={[styles.th, styles.cExpense]}>Expense</Text>
                            <Text style={[styles.th, styles.cBalance]}>Balance</Text>
                        </View>
                        {bank.rows.length === 0 ? (
                            <Text style={styles.empty}>Nothing in this range.</Text>
                        ) : (
                            bank.rows.map((tx, i) => (
                                <View key={`${bank.bankId}-${i}`} style={styles.trRow}>
                                    <Text style={[styles.td, styles.cDate]}>{tx.date ? getDate(tx.date) : "-"}</Text>
                                    <Text style={[styles.td, styles.cType]}>{tx.type}</Text>
                                    <Text style={[styles.td, styles.cNote]}>{tx.note || "-"}</Text>
                                    {/* The opening balance is where the list starts, not money
                                        that moved, so it fills neither money column. */}
                                    <Text style={[styles.td, styles.cCredit]}>
                                        {!tx.opening && tx.amount >= 0 ? RoundOff(tx.amount) : ""}
                                    </Text>
                                    <Text style={[styles.td, styles.cExpense]}>
                                        {!tx.opening && tx.amount < 0 ? RoundOff(Math.abs(tx.amount)) : ""}
                                    </Text>
                                    <Text style={[styles.td, styles.cBalance]}>{RoundOff(tx.balance)}</Text>
                                </View>
                            ))
                        )}
                    </View>
                </View>
            ))}
        </Page>
    </Document>
);

export default BankReportPDF;
