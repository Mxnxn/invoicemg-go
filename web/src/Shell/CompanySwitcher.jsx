import React, { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Check, ChevronUp, Plus, Settings, User } from "react-feather";
import { companyBackend } from "../Common/company_backend";
import CompanyFormModal from "./CompanyFormModal";
import "./shell.css";
import { can } from "../Common/access";

// Drop-up profile switcher pinned to the sidebar footer. Switching is a full page reload by
// design: dozens of views cache fetched data in useState, and a reload is the only honest way
// to guarantee none of the previous company's data survives on screen.
const CompanySwitcher = ({ email }) => {
    // key:null in features.js means "admin only" - can(null) mirrors that.
    const isOwner = can(null);
    const navigate = useNavigate();
    const [open, setOpen] = useState(false);
    const [companies, setCompanies] = useState([]);
    const [activeId, setActiveId] = useState(null);
    const [formOpen, setFormOpen] = useState(false);
    // Whether this admin may own another company. The server decides and still refuses on
    // create (routes/Company.js) - this only stops the menu offering something that will be
    // rejected after the form has been filled in. Defaults to canAdd:false so a response
    // missing the field fails closed; failing open would offer a button the server refuses.
    const [allowance, setAllowance] = useState({ limit: 1, canAdd: false });
    const ref = useRef(null);

    const load = () => {
        companyBackend
            .listCompanies()
            .then((res) => {
                setCompanies(res.data.companies);
                setActiveId(res.data.active_company_id);
                setAllowance({
                    limit: Number(res.data?.company_limit) || 1,
                    canAdd: Boolean(res.data?.can_add_company),
                });
            })
            .catch(() => {});
    };

    useEffect(load, []);

    useEffect(() => {
        if (!open) return;
        const onOutside = (e) => {
            if (ref.current && !ref.current.contains(e.target)) setOpen(false);
        };
        const onKeyDown = (e) => {
            if (e.key === "Escape") setOpen(false);
        };
        document.addEventListener("mousedown", onOutside);
        document.addEventListener("keydown", onKeyDown);
        return () => {
            document.removeEventListener("mousedown", onOutside);
            document.removeEventListener("keydown", onKeyDown);
        };
    }, [open]);

    const active = companies.find((c) => String(c._id) === String(activeId));

    const onSwitch = (companyId) => {
        if (String(companyId) === String(activeId)) {
            setOpen(false);
            return;
        }
        const formData = new FormData();
        formData.set("company_id", companyId);
        companyBackend
            .switchCompany(formData)
            .then(() => {
                // Drop the cached lookups the previous company populated, then reload.
                window.localStorage.removeItem("redux");
                window.location.assign("/admin/dashboard");
            })
            .catch(() => {});
    };

    // Creating a company immediately switches to it - you made it to work in it.
    const onCreate = (formData) =>
        companyBackend.createCompany(formData).then((res) => {
            load();
            onSwitch(res.data._id);
        });

    const openCreateForm = () => {
        setFormOpen(true);
        setOpen(false);
    };

    // Editing the active company's details lives on the Accounts page (same Company record,
    // same /userinfo/add write path) - no separate edit modal here anymore.
    const goToAccounts = () => {
        setOpen(false);
        navigate("/admin/accounts");
    };

    return (
        <div className="shell-company-switcher" ref={ref}>
            <button type="button" className="shell-company-trigger" onClick={() => setOpen((v) => !v)} aria-expanded={open}>
                <span className="shell-company-avatar">
                    <User size={15} />
                </span>
                <span className="shell-company-labels">
                    <span className="shell-user-company">{active ? active.name : "No company"}</span>
                    <span className="shell-user-email">{email}</span>
                </span>
                <ChevronUp size={14} className={open ? "shell-company-chevron open" : "shell-company-chevron"} />
            </button>

            {open && (
                <div className="shell-company-menu" role="menu">
                    {companies.map((company) => (
                        <button
                            key={company._id}
                            type="button"
                            role="menuitem"
                            className="shell-company-item"
                            onClick={() => onSwitch(company._id)}
                        >
                            <span className="shell-company-item-name">{company.name}</span>
                            {String(company._id) === String(activeId) && <Check size={14} />}
                        </button>
                    ))}
                    {/* Company profiles and account identity are owner-level concerns, so an
                        employee sees neither control rather than a button that 403s. */}
                    {isOwner && <div className="shell-company-divider" />}
                    {isOwner && active && (
                        <button type="button" role="menuitem" className="shell-company-item" onClick={goToAccounts}>
                            <Settings size={14} />
                            <span className="shell-company-item-name">Account details</span>
                        </button>
                    )}
                    {isOwner && (
                        // Disabled rather than hidden: an admin at their cap should be able to
                        // see that adding a company is a thing that exists and read why they
                        // cannot, instead of the option silently not being there. Same
                        // treatment as the Account settings carousel, so the two agree.
                        <button
                            type="button"
                            role="menuitem"
                            className="shell-company-item"
                            onClick={openCreateForm}
                            disabled={!allowance.canAdd}
                            title={
                                allowance.canAdd
                                    ? "Add a company"
                                    : `Your account is limited to ${allowance.limit} ${
                                          allowance.limit === 1 ? "company" : "companies"
                                      }. Ask an administrator to raise it.`
                            }
                        >
                            <Plus size={14} />
                            <span className="shell-company-item-name">Add company</span>
                        </button>
                    )}
                </div>
            )}

            <CompanyFormModal
                isOpen={formOpen}
                toggle={() => setFormOpen(false)}
                company={null}
                onSubmit={onCreate}
            />
        </div>
    );
};

export default CompanySwitcher;
