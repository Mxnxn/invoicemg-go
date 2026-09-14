import React, { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Repeat, BookOpen, Percent, Clock, TrendingUp, ArrowLeft, Briefcase, Package } from "react-feather";
import LiteHeader from "../../Common/Header/LiteHeader";
import SpotlightCard from "../../Common/DataTable/SpotlightCard";
import { hasAccess } from "../../Common/access";
import { MORE_SECTIONS } from "../../Common/features";
import { batchReceiveBackend } from "../BatchReceive/batch_receive_backend";
import BatchReceiveForm from "../BatchReceive/component/BatchReceiveForm";
import { lifecycleBackend } from "../Lifecycle/lifecycle_backend";
import LedgerIndex from "../Ledger/component/LedgerIndex";
import GstReportIndex from "../GstReport/component/GstReportIndex";
import DuesIndex from "../Dues/component/DuesIndex";
import PurchaseDuesIndex from "../PurchaseDues/component/PurchaseDuesIndex";
import PurchaseReportIndex from "../PurchaseReport/component/PurchaseReportIndex";
import BankReportIndex from "../BankReport/component/BankReportIndex";
import InventoryIndex from "../Inventory/component/InventoryIndex";
import { purchaseReportBackend } from "../PurchaseReport/purchase_report_backend";
import { ledgerBackend } from "../Ledger/ledger_backend";

const ICONS = { "batch-receive": Repeat, ledger: BookOpen, "customer-dues": Clock, "gst-report": Percent, "purchase-report": TrendingUp, "purchase-dues": Clock, "bank-report": Briefcase, inventory: Package };
const BODIES = { "batch-receive": BatchReceiveForm, ledger: LedgerIndex, "customer-dues": DuesIndex, "gst-report": GstReportIndex, "purchase-report": PurchaseReportIndex, "purchase-dues": PurchaseDuesIndex, "bank-report": BankReportIndex, inventory: InventoryIndex };
const ACCENTS = {
    blue: { fg: "var(--xan-blue)", bg: "var(--xan-blue-bg)" },
    emerald: { fg: "var(--xan-emerald)", bg: "var(--xan-emerald-bg)" },
    amber: { fg: "var(--xan-amber)", bg: "var(--xan-amber-bg)" },
    rose: { fg: "var(--xan-rose)", bg: "var(--xan-rose-bg)" },
    violet: { fg: "var(--xan-violet)", bg: "var(--xan-violet-bg)" },
};

// Landing hub for the "More" tab (see Refactor.md "# NEW") - same card-grid pattern as
// Configure, just with a different, growable set of sections. Starts with just Batch
// Receive; future additions (see More: section of the spec) slot in the same way Configure's
// five managers do.
const MoreIndex = () => {
    const { section } = useParams();
    const navigate = useNavigate();
    const [counts, setCounts] = useState({});

    const visibleSections = useMemo(() => MORE_SECTIONS.filter((s) => hasAccess(s.key)), []);
    const activeSection = visibleSections.find((s) => s.id === section) || null;

    useEffect(() => {
        if (visibleSections.some((s) => s.id === "batch-receive")) {
            batchReceiveBackend.listReceives().then((res) => setCounts((c) => ({ ...c, "batch-receive": res.data.length })));
        }
        if (visibleSections.some((s) => s.id === "ledger")) {
            lifecycleBackend.lookupClients().then((res) => setCounts((c) => ({ ...c, ledger: res.data.length })));
        }
        // The card's number is how many clients actually owe money - a more useful glance
        // than a row count, and it's the same call the section body makes.
        if (visibleSections.some((s) => s.id === "customer-dues")) {
            ledgerBackend.dues().then((res) => setCounts((c) => ({ ...c, "customer-dues": res.data.totals.outstandingClients })));
        }
        if (visibleSections.some((s) => s.id === "purchase-report")) {
            purchaseReportBackend
                .dues()
                .then((res) => setCounts((c) => ({ ...c, "purchase-report": res.data.totals.outstandingSuppliers })));
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const ActiveBody = activeSection ? BODIES[activeSection.id] : null;

    return (
        <>
            <LiteHeader bg="primary" />
            <div style={{ padding: "20px 24px" }}>
                <div className="configure-hero-strip" />
                {!activeSection ? (
                    <div className="bento-grid">
                        {visibleSections.map((s, index) => {
                            const Icon = ICONS[s.id];
                            const accent = ACCENTS[s.accent];
                            return (
                                <div
                                    key={s.id}
                                    className={["bento-card", s.wide ? "bento-card--wide" : "", s.tall ? "bento-card--tall" : ""]
                                        .filter(Boolean)
                                        .join(" ")}
                                >
                                <SpotlightCard
                                    className="configure-card"
                                    style={{ "--spotlight-color": accent.bg, "--card-accent": accent.fg, animationDelay: `${index * 60}ms` }}
                                    onClick={() => navigate(`/admin/more/${s.id}`)}
                                >
                                    <div className="configure-card-icon" style={{ background: accent.bg, color: accent.fg }}>
                                        <Icon size={22} />
                                    </div>
                                    <div className="text-kpi-value">{counts[s.id] ?? "—"}</div>
                                    <div className="text-body-regular" style={{ color: "var(--text-secondary)" }}>
                                        {s.label}
                                    </div>
                                </SpotlightCard>
                                </div>
                            );
                        })}
                    </div>
                ) : (
                    <>
                        <div className="d-flex align-items-center" style={{ gap: 16, marginBottom: 20, flexWrap: "wrap" }}>
                            <button
                                type="button"
                                className="shell-btn shell-btn-sm shell-btn-secondary d-flex align-items-center"
                                style={{ gap: 6 }}
                                onClick={() => navigate("/admin/more")}
                            >
                                <ArrowLeft size={14} />
                                All
                            </button>
                            <div className="shell-segmented" role="tablist">
                                {visibleSections.map((s) => {
                                    const accent = ACCENTS[s.accent];
                                    const active = s.id === activeSection.id;
                                    return (
                                        <button
                                            key={s.id}
                                            type="button"
                                            role="tab"
                                            aria-selected={active}
                                            className="shell-segmented-btn"
                                            style={active ? { background: accent.bg, color: accent.fg } : undefined}
                                            onClick={() => navigate(`/admin/more/${s.id}`)}
                                        >
                                            {s.label}
                                            {counts[s.id] !== undefined ? ` (${counts[s.id]})` : ""}
                                        </button>
                                    );
                                })}
                            </div>
                        </div>
                        {/* isAdmin gates the cross-company sharing toggle. The server enforces the
                            same rule on POST /company/sharing; this only decides whether the
                            control is worth rendering. */}
                        {ActiveBody && <ActiveBody isAdmin={(window.localStorage.getItem("role") || "admin") !== "employee"} />}
                    </>
                )}
            </div>
        </>
    );
};

export default MoreIndex;
