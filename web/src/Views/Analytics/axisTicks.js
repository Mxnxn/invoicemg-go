// Value-axis tick options shared by every analytics chart.
//
// The charts dropped from 300px to 150px tall and from half-width to a third, but Chart.js
// still auto-fits a tick roughly every ~28px - so a short chart ends up with eight or nine
// currency labels stacked on top of each other. The tick budget is derived from the axis's
// actual pixel length instead, and Chart.js's autoSkip drops the rest.
//
// Which axis this applies to depends on the chart: on a vertical Bar the values are on Y, on a
// HorizontalBar they're on X and Y holds category names - those must never be skipped, or
// clients disappear from their own rows.

// One label per ~44px of axis, clamped so a tall chart doesn't turn into a ruler and a very
// short one still shows a floor and a ceiling.
export const tickBudget = (lengthPx) => {
    const px = Number(lengthPx) || 0;
    if (px <= 0) return 3;
    return Math.max(3, Math.min(8, Math.round(px / 44)));
};

// Short money for axis ticks only - headline figures elsewhere stay exact.
export const compactMoney = (n) => {
    const v = Number(n) || 0;
    const abs = Math.abs(v);
    const sign = v < 0 ? "-" : "";
    if (abs >= 10000000) return `${sign}₹${(abs / 10000000).toFixed(2)}Cr`;
    if (abs >= 100000) return `${sign}₹${(abs / 100000).toFixed(2)}L`;
    if (abs >= 1000) return `${sign}₹${(abs / 1000).toFixed(1)}k`;
    return `${sign}₹${abs}`;
};

// `format` defaults to compact currency; pass a formatter for non-money axes (e.g. days).
export const valueAxisTicks = ({ lengthPx, fontColor, format = compactMoney }) => ({
    beginAtZero: true,
    fontColor,
    autoSkip: true,
    maxTicksLimit: tickBudget(lengthPx),
    callback: (value) => format(value),
});

// Category axes keep every label - a skipped one means a missing client, weekday or month.
export const categoryAxisTicks = ({ fontColor }) => ({
    fontColor,
    autoSkip: false,
});
