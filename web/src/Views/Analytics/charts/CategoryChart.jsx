import { Bar, BarChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

import { seriesVar } from "./chartTheme";

const AXIS = { stroke: "var(--text-tertiary)", fontSize: 12, tickLine: false };

const TOOLTIP_STYLE = {
    background: "var(--bg-raised)",
    border: "1px solid var(--border-default)",
    borderRadius: "var(--radius-control)",
    color: "var(--text-primary)",
};

/**
 * Bars over a category axis.
 *
 * `layout="vertical"` puts the categories down the left, which is what long names such as
 * customer or product names need - rotated x-axis labels are unreadable at any width.
 */
export default function CategoryChart({ data, xKey, series, layout = "horizontal" }) {
    const vertical = layout === "vertical";

    return (
        <ResponsiveContainer width="100%" height={260}>
            <BarChart
                data={data}
                layout={layout}
                margin={{ top: 8, right: 12, bottom: 0, left: vertical ? 8 : -8 }}
            >
                <CartesianGrid stroke="var(--border-default)" vertical={vertical} horizontal={!vertical} />
                {vertical ? (
                    <>
                        <XAxis type="number" {...AXIS} axisLine={false} />
                        <YAxis type="category" dataKey={xKey} {...AXIS} axisLine={false} width={132} />
                    </>
                ) : (
                    <>
                        <XAxis dataKey={xKey} {...AXIS} axisLine={{ stroke: "var(--border-default)" }} />
                        <YAxis {...AXIS} axisLine={false} width={64} />
                    </>
                )}
                <Tooltip contentStyle={TOOLTIP_STYLE} cursor={{ fill: "var(--bg-field-on-canvas)" }} />
                {series.length > 1 && <Legend wrapperStyle={{ fontSize: 12, color: "var(--text-secondary)" }} />}
                {series.map((s, i) => (
                    // Rounded data-ends, anchored to the baseline.
                    <Bar
                        key={s.key}
                        dataKey={s.key}
                        name={s.label}
                        fill={seriesVar(i)}
                        radius={vertical ? [0, 4, 4, 0] : [4, 4, 0, 0]}
                    />
                ))}
            </BarChart>
        </ResponsiveContainer>
    );
}
