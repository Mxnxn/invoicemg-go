import React, { useEffect, useState } from "react";
import { DollarSign, Layers, AlertTriangle, Percent, TrendingUp, TrendingDown } from "react-feather";
import MonthRangePicker from "../../../Common/MonthRangePicker";
import { defaultMonthRange } from "../../../Common/monthRange";
import { RoundOff } from "../../../Common/DateAndTime/RoundOff";
import { analyticsBackend } from "../analytics_backend";
import "./cashflow.css";
import "./profitFlow.css";

// The headline arithmetic, above the tabs and common to all of them:
//
//     Revenue  −  Operations  −  Waste & Material  −  Tax  =  Net Profit
//
// It sits in the shell rather than in a tab because it is the one figure that is true
// whichever question you arrived with. Before the tab rewrite this lived inside CashflowCard,
// which meant three of the four tabs showed charts with no bottom line anywhere on screen.
//
// Two things it is careful to be honest about:
//
//   TAX IS SUBTRACTED because revenue is tax-INCLUSIVE. Model/Entry.js records
//   total = (amount - discount + charges) * (1 + tax%) - advance, and revenue sums
//   Invoice.totalAmount. The GST inside that is collected for the government and remitted, so
//   counting it as earnings would overstate profit by the whole tax bill.
//
//   OPERATIONS IS COGS - the purchase cost of what actually sold. "Waste & Material" is then
//   wastage alone (material written off, valued at what it cost). Splitting it that way is
//   what keeps material from being counted twice.

const money = (n) => `₹${RoundOff(n)}`;
const sign = (n) => (Number(n) >= 0 ? "cashflow-pos" : "cashflow-neg");

const Tile = ({ label, value, sub, icon: Icon, tone, variant }) => (
    <div className={`cashflow-tile${variant ? ` is-${variant}` : ""}`}>
        <div className="cashflow-tile-label">
            {Icon && <Icon size={11} />}
            {label}
        </div>
        <div className={`cashflow-tile-value${tone ? ` ${tone}` : ""}`} title={value}>
            {value}
        </div>
        {sub && <div className="cashflow-tile-sub">{sub}</div>}
    </div>
);

const Op = ({ children }) => (
    <div className="cashflow-op" aria-hidden="true">
        {children}
    </div>
);

const ProfitFlow = ({ stoken }) => {
    const [data, setData] = useState(null);
    const [range, setRange] = useState(defaultMonthRange());
    const [loading, setLoading] = useState(true);
    const [failed, setFailed] = useState(false);

    useEffect(() => {
        let live = true;
        setLoading(true);
        setFailed(false);
        const formData = new FormData();
        // The picker yields YYYY-MM; the route takes YYYY-MM-DD. 31 is safe as an upper bound
        // because the comparison is a string prefix, not a real date.
        if (range.from) formData.set("from", `${range.from}-01`);
        if (range.to) formData.set("to", `${range.to}-31`);
        analyticsBackend
            .cashflow(formData, stoken)
            .then((res) => {
                if (live) setData(res.data);
            })
            .catch(() => {
                // The global interceptor toasts the reason. This only decides whether to show
                // a strip of zeros - which would read as "you earned nothing" - or say plainly
                // that the figures could not be loaded.
                if (live) setFailed(true);
            })
            .finally(() => {
                if (live) setLoading(false);
            });
        return () => {
            live = false;
        };
    }, [range, stoken]);

    const t = data?.totals;

    return (
        <div className="profit-flow">
            <div className="profit-flow-head">
                <span className="cashflow-lane-title">Revenue → net profit</span>
                <MonthRangePicker value={range} onChange={setRange} />
            </div>

            {failed ? (
                <p className="profit-flow-note text-body-small">
                    Couldn’t load the figures for this range.
                </p>
            ) : loading && !t ? (
                <p className="profit-flow-note text-body-small">Loading…</p>
            ) : !t ? (
                <p className="profit-flow-note text-body-small">No figures for this range.</p>
            ) : (
                <div className="cashflow-lane">
                    <Tile label="Revenue" value={money(t.revenue)} sub={`${t.collectionRate}% collected`} icon={DollarSign} />
                    <Op>−</Op>
                    <Tile
                        variant="deduction"
                        label="Operations"
                        value={money(t.cogs)}
                        sub="cost of what sold"
                        icon={Layers}
                    />
                    <Op>−</Op>
                    <Tile
                        variant="deduction"
                        label="Waste & Material"
                        value={money(t.wastage)}
                        sub="material written off"
                        icon={AlertTriangle}
                    />
                    <Op>−</Op>
                    <Tile variant="deduction" label="Tax" value={money(t.tax)} sub="GST inside revenue" icon={Percent} />
                    <Op>=</Op>
                    <Tile
                        variant="result"
                        label="Net Profit"
                        value={money(t.netProfit)}
                        tone={sign(t.netProfit)}
                        sub={`${t.netMargin}% margin · operating ${t.operatingMargin}%`}
                        icon={Number(t.netProfit) >= 0 ? TrendingUp : TrendingDown}
                    />
                </div>
            )}
        </div>
    );
};

export default ProfitFlow;
