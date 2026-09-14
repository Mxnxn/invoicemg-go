import { useState } from "react";
import { Container } from "reactstrap";

import LiteHeader from "../../../Common/Header/LiteHeader";
import ProfitFlow from "./ProfitFlow";
import RevenueTab from "../tabs/RevenueTab";
import CustomersTab from "../tabs/CustomersTab";
import CashflowTab from "../tabs/CashflowTab";
import OperationsTab from "../tabs/OperationsTab";
import ProductionTab from "../tabs/ProductionTab";
import "../analytics.css";

// One tab per question someone actually arrives with. Charts do not travel between them:
// a revenue view with a customer chart in it is why the old flat grid was hard to read.
const TABS = [
    { id: "revenue", label: "Revenue", Panel: RevenueTab },
    { id: "customers", label: "Customers", Panel: CustomersTab },
    { id: "cashflow", label: "Cashflow", Panel: CashflowTab },
    { id: "operations", label: "Operations", Panel: OperationsTab },
    { id: "production", label: "Production", Panel: ProductionTab },
];

/**
 * The analytics shell. It owns the active tab and nothing else.
 *
 * `source` is the navbar's existing Invoiced/All switch, held in AdminLayout. It arrives
 * here so a whole tab can honour it - previously it reached RevenueCard alone, which meant
 * flipping it changed one panel and silently left the rest on invoiced figures.
 */
const AnalyticsIndex = ({ uid, source = "invoiced" }) => {
    const stoken = window.localStorage.getItem("session_token");
    const [tab, setTab] = useState("revenue");
    const Active = (TABS.find((t) => t.id === tab) || TABS[0]).Panel;

    return (
        <>
            <LiteHeader bg="primary" />
            <Container fluid className="analytics-page">
                {/* Above the tabs and outside them on purpose: the bottom line is true
                    whichever question you arrived with, and it owns its own month range so it
                    stays a summary in its own right. Mounted once here rather than per tab, so
                    switching tabs does not refetch it. */}
                <ProfitFlow stoken={stoken} />

                <div className="segmented" role="tablist" aria-label="Analytics sections">
                    {TABS.map((t) => (
                        <button
                            key={t.id}
                            type="button"
                            role="tab"
                            aria-selected={tab === t.id}
                            className={["segmented-option", tab === t.id ? "active" : ""].filter(Boolean).join(" ")}
                            onClick={() => setTab(t.id)}
                        >
                            <span>{t.label}</span>
                        </button>
                    ))}
                </div>

                <Active uid={uid} stoken={stoken} source={source} />
            </Container>
        </>
    );
};

export default AnalyticsIndex;
