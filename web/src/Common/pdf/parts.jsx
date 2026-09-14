import React from "react";
import { Text, View, Image, StyleSheet } from "@react-pdf/renderer";

// The pieces every document is built from, drawn according to a design spec (see designs.js).
//
// Invoice, quotation and ledger each have their own renderer because their content genuinely
// differs - a ledger has a running balance, an invoice has tax. What does NOT differ is what
// a band header or a boxed table looks like, so that lives here once. Without this file the
// three renderers would each carry their own idea of a band header and the "same design in
// all three tabs" promise would quietly stop being true.

const uploaded = (name) => ({ uri: `${import.meta.env.VITE_API_URL}/uploads/${name}` });

// Built per (design, font) pair and handed to every part below. A StyleSheet rather than
// inline objects so react-pdf can cache the resolved styles.
export const makeStyles = (spec, font) => {
    const t = spec.tokens;
    const family = font.family;
    const boxed = spec.table === "boxed";

    return StyleSheet.create({
        page: {
            fontFamily: family,
            fontSize: t.baseSize,
            color: t.ink,
            // A band header is full-bleed, so the page cannot carry side padding - the band
            // draws to the edge and the body below supplies its own inset instead.
            padding: spec.header === "band" ? 0 : t.pagePadding,
            paddingBottom: t.pagePadding,
        },
        body: spec.header === "band" ? { paddingHorizontal: t.pagePadding, paddingTop: t.pagePadding * 0.6 } : {},

        band: {
            backgroundColor: t.accent,
            paddingHorizontal: t.pagePadding,
            paddingVertical: t.pagePadding * 0.65,
            flexDirection: "row",
            justifyContent: "space-between",
            alignItems: "center",
        },
        bandFirm: { color: "#FFFFFF", fontSize: t.titleSize, fontWeight: 700 },
        bandSub: { color: "#FFFFFF", fontSize: t.labelSize + 0.5, marginTop: 3, opacity: 0.85 },
        bandMeta: { alignItems: "flex-end" },
        bandTitle: { color: "#FFFFFF", fontSize: t.baseSize + 2, fontWeight: 700, textTransform: "uppercase", letterSpacing: 1 },

        headRow: { flexDirection: "row", justifyContent: "space-between", alignItems: "flex-start" },
        splitBox: { flexDirection: "row", borderWidth: 1, borderColor: t.rule },
        splitCell: { width: "50%", padding: 8 },
        splitDivider: { borderLeftWidth: 1, borderColor: t.rule },

        firmName: { fontSize: t.titleSize * 0.6 + 4, fontWeight: 700, color: t.ink },
        docTitle: {
            fontSize: t.titleSize,
            fontWeight: 700,
            color: t.accent,
            letterSpacing: spec.header === "plain" ? 1.5 : 0.5,
            textTransform: "uppercase",
        },
        // Height fixed, width free. It was a fixed 92x52 box with objectFit: contain - a
        // 1.77:1 frame that almost no logo matches, so a wide wordmark floated in a band of
        // empty space and a square mark was stranded in the middle of it. Constraining ONE
        // axis lets the image keep its own proportions; maxWidth stops a very wide wordmark
        // running into the document meta beside it.
        logo: {
            height: spec.header === "band" ? 34 : 44,
            maxWidth: 170,
            objectFit: "contain",
        },

        label: {
            fontSize: t.labelSize,
            color: t.muted,
            textTransform: "uppercase",
            letterSpacing: 0.6,
            marginBottom: 2,
        },
        value: { fontSize: t.baseSize + 1, fontWeight: 700, marginBottom: 5 },
        small: { fontSize: t.baseSize - 0.5, color: t.muted, marginTop: 1 },
        strong: { fontSize: t.baseSize + 1, fontWeight: 700, marginBottom: 2, color: t.ink },

        partiesRow: { flexDirection: "row", justifyContent: "space-between", marginTop: t.rowGap * 3 },
        partyCol: { width: "48%" },

        table: {
            marginTop: t.rowGap * 3,
            ...(boxed ? { borderWidth: 1, borderColor: t.rule } : {}),
        },
        // No backgroundColor key at all when the design has no fill. react-pdf 1.x does not
        // understand the "transparent" keyword and paints it solid black, which turned the
        // header row, every second row and the grand total into black bars.
        thRow: {
            flexDirection: "row",
            ...(spec.table === "ruled" ? {} : { backgroundColor: t.fill }),
            borderBottomWidth: boxed ? 1 : 1.2,
            borderColor: boxed ? t.rule : t.accent,
            paddingVertical: t.rowGap,
        },
        th: {
            fontSize: t.labelSize,
            fontWeight: 700,
            color: spec.table === "ruled" ? t.muted : t.ink,
            textTransform: "uppercase",
            letterSpacing: 0.4,
            paddingHorizontal: 3,
        },
        trRow: {
            flexDirection: "row",
            paddingVertical: t.rowGap,
            // A zebra design carries no rules at all - the fill is what separates the rows.
            borderBottomWidth: spec.table === "zebra" ? 0 : boxed ? 1 : 0.5,
            borderColor: t.rule,
        },
        trAlt: spec.table === "zebra" ? { backgroundColor: t.fill } : {},
        td: { fontSize: t.baseSize, paddingHorizontal: 3, color: t.ink },

        totalsWrap: { width: "48%", marginLeft: "auto", marginTop: t.rowGap * 2 },
        totalsBox: spec.totals === "boxed" ? { borderWidth: 1, borderColor: t.rule, padding: 6 } : {},
        totalsRow: { flexDirection: "row", paddingVertical: 2.5 },
        totalsLabel: { width: "58%", textAlign: "right", paddingRight: 8, fontSize: t.baseSize - 0.5, color: t.muted },
        totalsValue: { width: "42%", textAlign: "right", fontSize: t.baseSize - 0.5 },
        grandRow: {
            flexDirection: "row",
            alignItems: "center",
            marginTop: 4,
            paddingVertical: spec.totals === "bar" ? 7 : 5,
            paddingHorizontal: spec.totals === "bar" ? 9 : 0,
            ...(spec.totals === "bar" ? { backgroundColor: t.accent, borderRadius: 3 } : {}),
            borderTopWidth: spec.totals === "bar" ? 0 : 1,
            borderColor: t.accent,
        },
        grandLabel: {
            width: "58%",
            textAlign: "right",
            paddingRight: 8,
            fontWeight: 700,
            fontSize: t.baseSize + 1.5,
            color: spec.totals === "bar" ? "#FFFFFF" : t.ink,
        },
        grandValue: {
            width: "42%",
            textAlign: "right",
            fontWeight: 700,
            fontSize: t.baseSize + 1.5,
            color: spec.totals === "bar" ? "#FFFFFF" : t.accent,
        },

        note: { marginTop: t.rowGap * 4, fontSize: t.baseSize - 0.5, color: t.muted },
        noteBox: { marginTop: t.rowGap * 4, backgroundColor: t.fill, borderRadius: 3, padding: 10 },
        footer: {
            flexDirection: "row",
            justifyContent: "space-between",
            marginTop: t.rowGap * 5,
            paddingTop: t.rowGap * 2,
            borderTopWidth: 0.5,
            borderColor: t.rule,
        },
    });
};

// A stack of "LABEL / value" lines - the From block, the Bill To block, the document meta.
export const Party = ({ styles, label, name, lines = [] }) => (
    <View>
        {Boolean(label) && <Text style={styles.label}>{label}</Text>}
        {Boolean(name) && <Text style={styles.strong}>{name}</Text>}
        {lines.filter(Boolean).map((line, i) => (
            <Text key={i} style={styles.small}>
                {line}
            </Text>
        ))}
    </View>
);

export const Meta = ({ styles, items = [] }) => (
    <View>
        {items
            .filter((i) => i && i.value)
            .map((item, i) => (
                <View key={i}>
                    <Text style={styles.label}>{item.label}</Text>
                    <Text style={styles.value}>{item.value}</Text>
                </View>
            ))}
    </View>
);

// The top of the page, in whichever of the three shapes the design calls for.
//
// `logo` is the uploaded filename or falsy. It is rendered through an Image with a uri src;
// react-pdf throws on a src it cannot fetch, so callers pass a falsy value rather than a
// filename they are unsure about.
export const Letterhead = ({ spec, styles, title, firm, firmLines = [], logo, meta = [] }) => {
    if (spec.header === "band") {
        return (
            <View style={styles.band}>
                <View>
                    <Text style={styles.bandFirm}>{firm}</Text>
                    {firmLines.filter(Boolean).map((line, i) => (
                        <Text key={i} style={styles.bandSub}>
                            {line}
                        </Text>
                    ))}
                </View>
                {/* The document title and its number live in the band too. Drawing only the
                    firm here shipped an invoice with no invoice number on it. */}
                <View style={styles.bandMeta}>
                    {Boolean(logo) && <Image style={[styles.logo, { marginBottom: 5 }]} src={uploaded(logo)} />}
                    <Text style={styles.bandTitle}>{title}</Text>
                    {meta
                        .filter((i) => i && i.value)
                        .map((item, i) => (
                            <Text key={i} style={styles.bandSub}>
                                {item.label}: {item.value}
                            </Text>
                        ))}
                </View>
            </View>
        );
    }

    if (spec.header === "split") {
        return (
            <View>
                <Text style={[styles.docTitle, { textAlign: "center", marginBottom: 6 }]}>{title}</Text>
                <View style={styles.splitBox}>
                    <View style={styles.splitCell}>
                        {Boolean(logo) && <Image style={[styles.logo, { marginBottom: 5 }]} src={uploaded(logo)} />}
                        <Text style={styles.firmName}>{firm}</Text>
                        {firmLines.filter(Boolean).map((line, i) => (
                            <Text key={i} style={styles.small}>
                                {line}
                            </Text>
                        ))}
                    </View>
                    <View style={[styles.splitCell, styles.splitDivider]}>
                        <Meta styles={styles} items={meta} />
                    </View>
                </View>
            </View>
        );
    }

    return (
        <View style={styles.headRow}>
            <View>
                {Boolean(logo) && <Image style={[styles.logo, { marginBottom: 6 }]} src={uploaded(logo)} />}
                <Text style={styles.docTitle}>{title}</Text>
                <Text style={[styles.firmName, { marginTop: 4 }]}>{firm}</Text>
                {firmLines.filter(Boolean).map((line, i) => (
                    <Text key={i} style={styles.small}>
                        {line}
                    </Text>
                ))}
            </View>
            <Meta styles={styles} items={meta} />
        </View>
    );
};

// `columns` is [{ key, label, width, align }]; `rows` is an array of plain objects keyed by
// those column keys. Widths are the caller's business - only it knows which column holds a
// long description.
export const DocTable = ({ styles, columns, rows }) => {
    const cell = (col) => ({ width: col.width, textAlign: col.align || "left" });
    return (
        <View style={styles.table}>
            <View style={styles.thRow}>
                {columns.map((col) => (
                    <Text key={col.key} style={[styles.th, cell(col)]}>
                        {col.label}
                    </Text>
                ))}
            </View>
            {rows.map((row, index) => (
                <View key={index} style={[styles.trRow, index % 2 === 1 ? styles.trAlt : null]}>
                    {columns.map((col) => (
                        <Text key={col.key} style={[styles.td, cell(col)]}>
                            {/* An empty string would be rendered as a bare child and crash
                                the Yoga layout engine; a space keeps the cell a real node. */}
                            {row[col.key] === 0 ? "0" : row[col.key] || " "}
                        </Text>
                    ))}
                </View>
            ))}
        </View>
    );
};

// `lines` is [{ label, value }] above the rule; `grand` is { label, value }; `after` is
// anything that belongs below it (Received / Balance Due), each optionally strong.
export const Totals = ({ styles, lines = [], grand, after = [] }) => (
    <View style={styles.totalsWrap}>
        <View style={styles.totalsBox}>
            {lines.filter(Boolean).map((line, i) => (
                <View key={i} style={styles.totalsRow}>
                    <Text style={styles.totalsLabel}>{line.label}</Text>
                    <Text style={styles.totalsValue}>{line.value}</Text>
                </View>
            ))}
            {Boolean(grand) && (
                <View style={styles.grandRow}>
                    <Text style={styles.grandLabel}>{grand.label}</Text>
                    <Text style={styles.grandValue}>{grand.value}</Text>
                </View>
            )}
            {after.filter(Boolean).map((line, i) => (
                <View key={i} style={line.strong ? styles.grandRow : styles.totalsRow}>
                    <Text style={line.strong ? styles.grandLabel : styles.totalsLabel}>{line.label}</Text>
                    <Text style={line.strong ? styles.grandValue : styles.totalsValue}>{line.value}</Text>
                </View>
            ))}
        </View>
    </View>
);
