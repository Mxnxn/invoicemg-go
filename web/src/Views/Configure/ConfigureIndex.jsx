import React, { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Users, Package, Briefcase, Truck, FilePlus, Layout, ArrowLeft, Hash, CreditCard, Share2 } from "react-feather";
import WhatsAppIcon from "../../Common/WhatsAppIcon";
import LiteHeader from "../../Common/Header/LiteHeader";
import SpotlightCard from "../../Common/DataTable/SpotlightCard";
import { hasAccess } from "../../Common/access";
import { CONFIGURE_SECTIONS } from "../../Common/features";
import { clientsBackend } from "../Client/client_backend";
import { materialsBackend } from "../Material/material_backend";
import { personBackend } from "../../Common/person_backend";
import { wastageBackend } from "../Challan/wastage_backend";
import AddClient from "../Client/component/AddClient";
import AddMaterial from "../Material/component/AddMaterial";
import PeopleManager from "../../Common/PeopleManager";
import SuppliersPage from "../Supplier/SuppliersPage";
import BanksManager from "./BanksManager";
import ChallanIndex from "../Challan/components/Index";
import TemplatesManager from "./TemplatesManager";
import WhatsAppManager from "./WhatsAppManager";
import SharedRecordsIndex from "../Shared/component/SharedRecordsIndex";

const ICONS = { customers: Users, products: Package, people: Briefcase, suppliers: Truck, challan: FilePlus, templates: Layout, whatsapp: WhatsAppIcon, "across-company": Share2, banks: CreditCard };
const BODIES = {
    customers: AddClient,
    products: AddMaterial,
    people: PeopleManager,
    suppliers: SuppliersPage,
    banks: BanksManager,
    challan: ChallanIndex,
    templates: TemplatesManager,
    whatsapp: WhatsAppManager,
    "across-company": SharedRecordsIndex,
};
const ACCENTS = {
    blue: { fg: "var(--xan-blue)", bg: "var(--xan-blue-bg)" },
    emerald: { fg: "var(--xan-emerald)", bg: "var(--xan-emerald-bg)" },
    amber: { fg: "var(--xan-amber)", bg: "var(--xan-amber-bg)" },
    rose: { fg: "var(--xan-rose)", bg: "var(--xan-rose-bg)" },
    violet: { fg: "var(--xan-violet)", bg: "var(--xan-violet-bg)" },
};

// Landing hub for the five record managers (Customers/Products/People/Suppliers/Logs) that
// used to each have their own sidebar entry. Shows one thing at a time instead of all five at
// once: a picker grid of stat-cards, then - once a section is picked - that section's full
// manager view with a slim color-coded tab strip to jump sideways without going back to the
// grid. Each card only renders/fetches if the current session has that feature's permission
// (People/Suppliers are admin-only, same as Accounts).
const ConfigureIndex = () => {
    const { section } = useParams();
    const navigate = useNavigate();
    const [counts, setCounts] = useState({});

    const visibleSections = useMemo(() => CONFIGURE_SECTIONS.filter((s) => hasAccess(s.key)), []);
    // Exports moved into Templates and Numbering as its "Export configs" tab. Without this,
    // a bookmark or an old link to /admin/configure/exports would quietly land on the section
    // grid, which reads as the feature having been removed.
    useEffect(() => {
        if (section === "exports") navigate("/admin/configure/templates", { replace: true });
    }, [section, navigate]);

    const activeSection = visibleSections.find((s) => s.id === section) || null;

    useEffect(() => {
        const uid = window.localStorage.getItem("uid");
        const stoken = window.localStorage.getItem("session_token");

        if (visibleSections.some((s) => s.id === "customers")) {
            const formData = new FormData();
            formData.set("uid", uid);
            clientsBackend.getOnlyClients(formData).then((res) => setCounts((c) => ({ ...c, customers: res.data.length })));
        }
        if (visibleSections.some((s) => s.id === "products")) {
            materialsBackend.getAllMaterials(new FormData(), stoken).then((res) => setCounts((c) => ({ ...c, products: res.data.length })));
        }
        if (visibleSections.some((s) => s.id === "people" || s.id === "suppliers")) {
            personBackend.list().then((res) => {
                setCounts((c) => ({
                    ...c,
                    people: res.data.filter((p) => p.type !== "Supplier").length,
                    suppliers: res.data.filter((p) => p.type === "Supplier").length,
                }));
            });
        }
        if (visibleSections.some((s) => s.id === "challan")) {
            const formData = new FormData();
            formData.set("uid", uid);
            wastageBackend.getWastages(formData).then((res) => setCounts((c) => ({ ...c, challan: res.data.length })));
        }
        // Re-run whenever the active section changes, which includes navigating back to the
        // card grid. Fetching once on mount meant adding a supplier (either in People or via
        // the "Also a supplier" box on a customer) left the count showing its old value until
        // a full page reload.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [section]);

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
                            const count = counts[s.id];
                            return (
                                <div key={s.id} className={["bento-card", s.wide ? "bento-card--wide" : "", s.tall ? "bento-card--tall" : ""].filter(Boolean).join(" ")}>
                                    <SpotlightCard
                                        className="configure-card"
                                        style={{
                                            "--spotlight-color": accent.bg,
                                            "--card-accent": accent.fg,
                                            "--card-accent-tint": accent.bg,
                                            animationDelay: `${index * 60}ms`,
                                        }}
                                        onClick={() => navigate(`/admin/configure/${s.id}`)}
                                    >
                                        <div className="bento-card-top">
                                            <div className="configure-card-icon" style={{ background: accent.bg, color: accent.fg }}>
                                                <Icon size={22} />
                                            </div>
                                            {count !== undefined && <span className="bento-card-count">{count}</span>}
                                        </div>
                                        <div className="bento-card-body">
                                            <div className="text-body-regular" style={{ color: "var(--text-primary)", fontWeight: 600 }}>
                                                {s.label}
                                            </div>
                                            {s.description && <div className="bento-card-desc">{s.description}</div>}
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
                                onClick={() => navigate("/admin/configure")}
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
                                            onClick={() => navigate(`/admin/configure/${s.id}`)}
                                        >
                                            {s.label}
                                            {counts[s.id] !== undefined ? ` (${counts[s.id]})` : ""}
                                        </button>
                                    );
                                })}
                            </div>
                        </div>
                        {ActiveBody && <ActiveBody />}
                    </>
                )}
            </div>
        </>
    );
};

export default ConfigureIndex;
