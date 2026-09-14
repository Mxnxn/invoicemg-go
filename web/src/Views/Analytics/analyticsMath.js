// Derived figures the API does not return.
//
// Pure functions over what the existing analytics endpoints already send, so they are
// testable without a database and without mounting a component - which is where the
// division-by-zero cases actually get exercised. A shop with no revenue this period is
// normal, and a KPI reading "NaN%" is how that normal case reaches the screen.

const round1 = (n) => Math.round(n * 10) / 10;

/** Share of revenue held by the biggest `n` customers, as a percent. */
export function concentration(topSales, n = 5) {
    const list = Array.isArray(topSales) ? topSales : [];
    const totals = list.map((row) => Number(row.total) || 0);
    const all = totals.reduce((a, b) => a + b, 0);
    if (all <= 0) return 0;

    const top = [...totals]
        .sort((a, b) => b - a)
        .slice(0, n)
        .reduce((a, b) => a + b, 0);
    return round1((top / all) * 100);
}

/** Jobs raised as a percent of quotations issued. */
export function conversionRate(quotations, jobs) {
    const q = Number(quotations) || 0;
    if (q <= 0) return 0;
    return round1(((Number(jobs) || 0) / q) * 100);
}

/** Collected as a percent of billed, across a revenue series. */
export function collectionRate(revenueData) {
    const list = Array.isArray(revenueData) ? revenueData : [];
    const billed = list.reduce((sum, row) => sum + (Number(row.billed) || 0), 0);
    if (billed <= 0) return 0;

    const collected = list.reduce((sum, row) => sum + (Number(row.collected) || 0), 0);
    return round1((collected / billed) * 100);
}

/** Receivables minus payables. Negative means we owe more than we are owed. */
export function netPosition(receivables, payables) {
    return Math.round(((Number(receivables) || 0) - (Number(payables) || 0)) * 100) / 100;
}
