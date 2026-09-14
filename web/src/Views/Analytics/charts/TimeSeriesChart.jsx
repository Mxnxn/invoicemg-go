import { CartesianGrid, Legend, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

import { seriesVar } from "./chartTheme";

// Recessive grid and axes; the data is the ink. Text wears text tokens so a value never
// inherits a series colour.
const AXIS = { stroke: "var(--text-tertiary)", fontSize: 12, tickLine: false };

const TOOLTIP_STYLE = {
    background: "var(--bg-raised)",
    border: "1px solid var(--border-default)",
    borderRadius: "var(--radius-control)",
    color: "var(--text-primary)",
};

/**
 * A line per series over a shared x axis.
 *
 * `series` is [{ key, label }]. Every series shares one y axis by design: two measures of
 * different scale become two charts, never a second axis.
 *
 * `onPointClick` is optional and receives the clicked x value. Only the Revenue tab uses it,
 * to drill from a year into its months; every other caller omits it and the chart stays
 * inert, with no pointer cursor implying a click that does nothing.
 */
export default function TimeSeriesChart({ data, xKey, series, onPointClick }) {
    return (
        <ResponsiveContainer width="100%" height={260}>
            <LineChart
                data={data}
                margin={{ top: 8, right: 12, bottom: 0, left: -8 }}
                onClick={onPointClick ? (e) => e && e.activeLabel && onPointClick(e.activeLabel) : undefined}
                style={onPointClick ? { cursor: "pointer" } : undefined}
            >
                <CartesianGrid stroke="var(--border-default)" vertical={false} />
                <XAxis dataKey={xKey} {...AXIS} axisLine={{ stroke: "var(--border-default)" }} />
                <YAxis {...AXIS} axisLine={false} width={64} />
                <Tooltip contentStyle={TOOLTIP_STYLE} cursor={{ stroke: "var(--border-strong)" }} />
                {/* A legend whenever there is more than one series - identity is never
                    colour alone. One series needs none: the card's title names it. */}
                {series.length > 1 && <Legend wrapperStyle={{ fontSize: 12, color: "var(--text-secondary)" }} />}
                {series.map((s, i) => (
                    <Line
                        key={s.key}
                        type="monotone"
                        dataKey={s.key}
                        name={s.label}
                        stroke={seriesVar(i)}
                        strokeWidth={2}
                        dot={false}
                        activeDot={{ r: 4 }}
                    />
                ))}
            </LineChart>
        </ResponsiveContainer>
    );
}
