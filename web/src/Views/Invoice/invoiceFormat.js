import { hasDimensions, dimensionFactor } from "../../Common/rowPricing";

// Shared invoice presentation logic. Every template (classic, gst-detailed, modern, compact)
// used to carry its own copy of the net-amount formula and the tax grouping, so a change had
// to be made four times to keep the documents consistent.

// Discount and charges net against the line before tax - the same convention Entry, Job and
// Quotation rows use.
//
// The blank-counts-as-1 fallback is passed explicitly and must stay. Entries predate the
// by-quantity flag and legitimately carry "" dimensions on old flat-rate lines; those rows are
// hasDimensions:true (the default) and would price to zero under the job side's blank-is-zero
// rule, re-pricing invoices that have already gone to customers.
export const rowNetAmount = (item) =>
    (Number(item.qty) || 0) * (Number(item.rate) || 0) * dimensionFactor(item, 1) -
    (Number(item.discount) || 0) +
    (Number(item.charges) || 0);

// "Description of Goods" reads `<Product>: <job-card description> <length> x <width>`. On an
// Entry the product name is `material` ("Product 1") and `description` is the text typed on
// the job card ("Kanan"); `product` is a legacy field the older templates read, kept as a
// fallback since it's undefined on everything created through the Jobs flow.
//
// The size lives HERE, in the description, rather than in a Size column of its own. A single
// invoice now mixes rows that have a size with rows that do not, and a dedicated column for
// something most of the page may not have prints a run of empty cells - it reads as data that
// went missing rather than data that was never there. Folded into the description, a row
// simply says as much about itself as it has to say.
//
// Written unbracketed and separated from the note by a middle dot ("Vinyl: Banner · 2 x 3").
// Brackets read as an aside; a bare space ran the size into the end of the note so that
// "Banner 2 x 3" could be read as part of the description itself. The dot says these are two
// facts about one line, which is what they are.
// `includeSize` folds the size into the description ("Vinyl: Banner · 2 x 3"). The invoice
// passes false when its Size toggle is on, because then the size has a column of its own.
export const describeGoods = (item = {}, includeSize = true) => {
    const product = String(item.material || item.product || "").trim();
    const detail = String(item.description || "").trim();
    const base = product && detail ? `${product}: ${detail}` : product || detail || "";
    if (!includeSize) return base;
    const size = describeSize(item);
    if (!size) return base;
    return base ? `${base} · ${size}` : size;
};

// The size of one line, or "" when there is no size to state.
//
// Two different questions, two different answers, deliberately:
//
//   PRICING  - a blank side counts as 1 (see rowNetAmount above). That is how those legacy
//              flat-rate entries have always been billed and must not change.
//   PRINTING - a blank side prints NOTHING. Rendering "1 x 1" for a row that never recorded a
//              size states a measurement nobody took. The old Size column did exactly that,
//              and it is half of what this change is here to stop.
//
// So both sides must carry a real value before a size is printed. A row with "2" and "3" says
// so; a row with "" and "" says nothing and is still billed as 1 x 1.
export const describeSize = (item = {}) => {
    if (!hasDimensions(item)) return "";
    const side = (value) => String(value ?? "").trim();
    const l = side(item.length);
    const w = side(item.width);
    if (!l || !w) return "";
    return `${l} x ${w}`;
};

// One CGST line and one SGST line (and IGST when used), each the sum across every line of
// the invoice, with no percentage in the label - even when the invoice mixes rates. Mixed
// rates are still broken out per HSN and rate in the Tax Details table below the totals
// (hsnTaxRows), which is where the rate-level detail belongs on a GST invoice; repeating
// "@18%" in the totals block just duplicated it.
// Tax summary lines for the invoice footer.
//
// Two shapes, because one label cannot honestly describe both cases:
//
//   Every row on the same slab  ->  one line per component, carrying the rate:
//                                   "CGST 9%", "SGST 9%". CGST and SGST are always half the
//                                   combined rate, which is what the reader expects to see.
//
//   Rows on different slabs     ->  one line per component PER RATE:
//                                   "CGST 2.5%", "CGST 9%", ... Collapsing those into a single
//                                   "CGST" line hides that the invoice mixes slabs, and the
//                                   percentage shown would be wrong for some of the goods.
export const taxLines = (entries = []) => {
    const mixed = hasMixedRates(entries);

    if (!mixed) {
        const totals = { cgst: 0, sgst: 0, igst: 0 };
        const rates = { cgst: 0, sgst: 0, igst: 0 };

        for (const item of entries) {
            const base = rowNetAmount(item);
            ["cgst", "sgst", "igst"].forEach((kind) => {
                const pct = Number(item[kind]) || 0;
                totals[kind] += (base * pct) / 100;
                if (pct > 0) rates[kind] = pct;
            });
        }

        return ["cgst", "sgst", "igst"]
            .filter((kind) => totals[kind] > 0)
            .map((kind) => ({
                key: kind,
                label: `${kind.toUpperCase()} ${formatPct(rates[kind])}%`,
                rate: rates[kind],
                amount: totals[kind],
            }));
    }

    // Mixed: group by component AND rate so each slab is stated separately.
    const buckets = new Map();
    for (const item of entries) {
        const base = rowNetAmount(item);
        ["cgst", "sgst", "igst"].forEach((kind) => {
            const pct = Number(item[kind]) || 0;
            if (pct <= 0) return;
            const key = `${kind}@${pct}`;
            const prev = buckets.get(key) || { key, kind, rate: pct, amount: 0 };
            prev.amount += (base * pct) / 100;
            buckets.set(key, prev);
        });
    }

    const order = { cgst: 0, sgst: 1, igst: 2 };
    return [...buckets.values()]
        .filter((b) => b.amount > 0)
        .sort((a, b) => order[a.kind] - order[b.kind] || a.rate - b.rate)
        .map((b) => ({
            key: b.key,
            label: `${b.kind.toUpperCase()} ${formatPct(b.rate)}%`,
            rate: b.rate,
            amount: b.amount,
        }));
};

// 9 not 9.00, but 2.5 stays 2.5 - GST slabs are commonly halved to a single decimal.
const formatPct = (pct) => {
    const n = Number(pct) || 0;
    return Number.isInteger(n) ? String(n) : String(Number(n.toFixed(2)));
};

export const hsnTaxRows = (entries = []) => {
    const groups = new Map();
    for (const item of entries) {
        const hsn = item.hsn || "-";
        const cgst = Number(item.cgst) || 0;
        const sgst = Number(item.sgst) || 0;
        const key = `${hsn}|${cgst}|${sgst}`;
        if (!groups.has(key)) groups.set(key, { hsn, cgst, sgst, taxable: 0 });
        groups.get(key).taxable += rowNetAmount(item);
    }
    return [...groups.values()];
};

// A per-line GST column only earns its space when the invoice actually mixes rates.
export const hasMixedRates = (entries = []) =>
    new Set(entries.map((i) => `${Number(i.cgst) || 0}|${Number(i.sgst) || 0}|${Number(i.igst) || 0}`)).size > 1;

export const rowGstPct = (item = {}) => (Number(item.cgst) || 0) + (Number(item.sgst) || 0) + (Number(item.igst) || 0);

export const totalTax = (entries = []) => taxLines(entries).reduce((sum, line) => sum + line.amount, 0);
