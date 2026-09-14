import React from "react";
import { Page, Document, Text, View, StyleSheet, Font, Image } from "@react-pdf/renderer";
import OSR400 from "../../Views/Invoice/font/OSR400.ttf";
import OS700 from "../../Views/Invoice/font/OS700.ttf";
import { headerLinesFor, showsSubtitle } from "./letterhead";

Font.register({
    family: "OS",
    fonts: [
        { src: OSR400, fontWeight: 400 },
        { src: OS700, fontWeight: 700 },
    ],
});

// One printed-report layout for every report in Reports.
//
// Views/Ledger/LedgerDoc.jsx set the house look - page padding, letterhead, uppercase summary
// labels, bordered table. Copying that four more times would guarantee they drift, so this
// takes the parts that differ (title, summary figures, columns, rows) and keeps the rest
// fixed. Column widths come from the caller because only it knows which column holds a long
// client name and which holds a four-digit amount.
const styles = StyleSheet.create({
    page: { fontFamily: "OS", fontSize: 8.5, padding: 24, color: "#222" },
    // Text left, logo right - the same side the spreadsheet puts it on, so the two formats of
    // one report do not mirror each other.
    letterhead: { flexDirection: "row", justifyContent: "space-between", alignItems: "flex-start" },
    letterheadText: { flexGrow: 1, paddingRight: 12 },
    // Height fixed, width free, objectFit contain: a tall logo and a wide one both land inside
    // the same band instead of one of them setting the header's height.
    logo: { height: 34, maxWidth: 130, objectFit: "contain" },
    firmName: { fontSize: 13, fontWeight: 700 },
    small: { fontSize: 8, marginTop: 1, color: "#555" },
    title: { fontSize: 10, fontWeight: 700, marginTop: 12, marginBottom: 6 },
    summaryRow: { flexDirection: "row", marginBottom: 8, flexWrap: "wrap" },
    summaryItem: { marginRight: 24, marginBottom: 4 },
    summaryLabel: { fontSize: 7.5, color: "#888", textTransform: "uppercase" },
    summaryValue: { fontSize: 10, fontWeight: 700 },
    table: { borderWidth: 1, borderColor: "#000" },
    thRow: { flexDirection: "row", backgroundColor: "#eee", borderBottomWidth: 1, borderColor: "#000" },
    trRow: { flexDirection: "row", borderBottomWidth: 0.5, borderColor: "#999", minHeight: 16 },
    footRow: { flexDirection: "row", borderTopWidth: 1, borderColor: "#000", minHeight: 16 },
    th: { fontWeight: 700, fontSize: 8, padding: 3 },
    td: { fontSize: 8, padding: 3 },
    empty: { fontSize: 8, padding: 6, color: "#777" },
});

// `columns`: [{ label, width, align }]  ·  `rows`: array of arrays, one per column
// `summary`: [{ label, value }]  ·  `foot`: an optional totals row, same shape as a row
const ReportPDF = ({ title, subtitle, user, summary = [], columns = [], rows = [], foot = null, template = {}, logoUrl = "" }) => {
    // The same lines the spreadsheet prints, from the same function. Before this the page
    // ignored Configure > Exports entirely: it always showed the firm and GST whatever the
    // toggles said, and never showed the phone or the logo at all.
    const lines = headerLinesFor(user, template);
    // The FIRST line is the firm and is set larger; the rest are the small grey block. If the
    // firm is toggled off, whatever remains simply starts at the top - the layout does not
    // depend on which line is which.
    const [firstLine, ...restLines] = lines;
    // A GST report with several tax slabs runs to a lot of narrow numeric columns. Landscape
    // alone stops being enough past a dozen or so, and a figure that does not fit its column
    // does not wrap - it spills over its neighbour. Tighten the type and padding instead, so
    // the extra columns come out of the whitespace rather than out of the numbers.
    const dense = columns.length > 12;
    const cell = dense ? { fontSize: 6.8, padding: 1.8 } : {};

    return (
    <Document>
        <Page size="A4" style={styles.page} orientation={columns.length > 7 ? "landscape" : "portrait"}>
            <View style={styles.letterhead}>
                <View style={styles.letterheadText}>
                    {firstLine ? <Text style={styles.firmName}>{firstLine}</Text> : null}
                    {restLines.map((line) => (
                        <Text key={line} style={styles.small}>
                            {line}
                        </Text>
                    ))}
                </View>
                {/* logoUrl is only passed once the caller has confirmed the image actually
                    loads - see ReportDownloads. @react-pdf throws on a src it cannot fetch,
                    which would fail the whole download over a missing logo file. */}
                {logoUrl ? <Image src={logoUrl} style={styles.logo} /> : null}
            </View>

            <Text style={styles.title}>
                {title}
                {showsSubtitle(subtitle, template) ? ` (${subtitle})` : ""}
            </Text>

            {summary.length > 0 && (
                <View style={styles.summaryRow}>
                    {summary.map((item) => (
                        <View key={item.label} style={styles.summaryItem}>
                            <Text style={styles.summaryLabel}>{item.label}</Text>
                            <Text style={styles.summaryValue}>{item.value}</Text>
                        </View>
                    ))}
                </View>
            )}

            <View style={styles.table}>
                <View style={styles.thRow}>
                    {columns.map((c) => (
                        <Text key={c.label} style={[styles.th, cell, { width: c.width, textAlign: c.align || "left" }]}>
                            {c.label}
                        </Text>
                    ))}
                </View>
                {rows.length === 0 ? (
                    <Text style={styles.empty}>Nothing to report for this selection.</Text>
                ) : (
                    rows.map((row, i) => (
                        <View key={i} style={styles.trRow}>
                            {columns.map((c, j) => (
                                <Text key={c.label} style={[styles.td, cell, { width: c.width, textAlign: c.align || "left" }]}>
                                    {row[j] === null || row[j] === undefined ? "" : String(row[j])}
                                </Text>
                            ))}
                        </View>
                    ))
                )}
                {foot && (
                    <View style={styles.footRow}>
                        {columns.map((c, j) => (
                            <Text
                                key={c.label}
                                style={[styles.td, cell, { width: c.width, textAlign: c.align || "left", fontWeight: 700 }]}
                            >
                                {foot[j] === null || foot[j] === undefined ? "" : String(foot[j])}
                            </Text>
                        ))}
                    </View>
                )}
            </View>
        </Page>
    </Document>
    );
};

export default ReportPDF;
