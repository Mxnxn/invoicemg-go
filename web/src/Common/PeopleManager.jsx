import PasswordInput from "./PasswordInput";
import React, { useEffect, useRef, useState } from "react";
import { FormGroup, Input, Button } from "reactstrap";
import RowCheckbox from "./DataTable/RowCheckbox";
import { Trash2, Edit } from "react-feather";
import DataTable from "./DataTable/DataTable";
import StatusBadge from "./DataTable/StatusBadge";
import RowActionMenu, { RowActionMenuItem } from "./DataTable/RowActionMenu";
import { personBackend } from "./person_backend";
import { ACTIONS } from "./access";
import { FEATURES } from "./features";
import { triggerFormAttention } from "./attention";
import useRequiredFields from "./useRequiredFields";
import { normalizePhone, phoneRule } from "./phone";

// Supplier is deliberately excluded - suppliers get their own dedicated manager/table
// (see Views/Supplier/SuppliersPage.jsx) instead of living in this general People list.
const PERSON_TYPES = ["Employee", "Vendor"];

const TYPE_STATUS = {
    Employee: "paid",
    Vendor: "pending",
    Designer: "neutral",
    Fabricator: "neutral",
    Transporter: "overdue",
};

// Permissions are stored as "feature:action" strings; these keep the form working in that
// shape without scattering string juggling through the JSX.
const hasPerm = (permissions, feature, action) =>
    permissions.some((entry) => {
        const [f, a] = String(entry).split(":");
        return f === feature && (!a || a === action);
    });

const withoutFeature = (permissions, feature) =>
    permissions.filter((entry) => String(entry).split(":")[0] !== feature);

const initialForm = { person_id: "", name: "", type: PERSON_TYPES[0], email: "", password: "", phone: "", permissions: [] };

// Embedded as the "People" section of /admin/configure (see Views/Configure/ConfigureIndex.jsx).
const PeopleManager = () => {
    const [people, setPeople] = useState([]);
    const [form, setForm] = useState(initialForm);

    const [saving, setSaving] = useState(false);
    const [moreMenu, setMoreMenu] = useState(-1);
    const formCardRef = useRef(null);

    const isEditing = !!form.person_id;
    // Name is the only always-required field; an Employee additionally needs credentials,
    // otherwise the account it creates can never be logged into.
    const required = useRequiredFields(form, {
        name: "Name",
        ...(form.type === "Employee" && !isEditing
            ? { email: "Email", password: "Password" }
            : {}),
        // Phone is optional - the API accepts a person without one, and making it mandatory
        // would disable submit on a flow that works today. But an empty rule always reports
        // "required", so it is only added once something has been typed: optional, yet
        // validated the moment it is filled in.
        ...(String(form.phone || "").trim()
            ? { phone: { label: "Phone", validate: (v) => phoneRule(v) } }
            : {}),
    });


    useEffect(() => {
        // Filters out any pre-existing Supplier-type records too, in case some were created
        // before Suppliers got their own dedicated manager.
        personBackend.list().then((res) => setPeople(res.data.filter((p) => p.type !== "Supplier")));
    }, []);

    const onChange = (e) => setForm({ ...form, [e.target.name]: e.target.value });

    const onTogglePermission = (key, action) => {
        setForm((prev) => {
            const granted = hasPerm(prev.permissions, key, action);
            // Expand any legacy flat entry for this feature into explicit actions first, so
            // unticking one action doesn't silently revoke the other two.
            const explicit = ACTIONS.filter((a) => hasPerm(prev.permissions, key, a)).map((a) => `${key}:${a}`);
            let next = granted ? explicit.filter((e) => e !== `${key}:${action}`) : [...new Set([...explicit, `${key}:${action}`])];
            // View is the gate - without it the other two are unusable, and granting create or
            // delete implies being able to see the screen.
            if (action === "view" && granted) next = [];
            if (action !== "view" && !granted) next = [...new Set([...next, `${key}:view`])];
            return { ...prev, permissions: [...withoutFeature(prev.permissions, key), ...next] };
        });
    };

    const onToggleFeatureRow = (key, grantAll) => {
        // Only the actions this feature actually offers. Granting create and delete on a
        // view-only capability would write permissions nothing ever reads, which then show up
        // in the stored set and invite the question of what they do.
        const actions = FEATURES.find((f) => f.key === key)?.actions || ACTIONS;
        setForm((prev) => ({
            ...prev,
            permissions: [...withoutFeature(prev.permissions, key), ...(grantAll ? actions.map((a) => `${key}:${a}`) : [])],
        }));
    };

    const onEditClick = (person) => {
        setForm({
            person_id: person._id,
            name: person.name,
            type: person.type,
            email: person.email || "",
            password: "",
            phone: person.phone || "",
            permissions: person.permissions || [],
        });
        setMoreMenu(-1);
        triggerFormAttention(formCardRef.current);
    };

    const onCancelEdit = () => setForm(initialForm);

    const onSubmit = async (e) => {
        e.preventDefault();
        // A slow save left the button live, so a second click created a second person - and
        // for an Employee that means a second login too. The guard is here as well as on the
        // button's disabled state: Enter in a text field submits the form without ever
        // touching the button.
        if (saving) return;
        if (!required.isComplete) {
            required.showAll();
            return;
        }
        setSaving(true);
        const formData = new FormData();
        formData.set("name", form.name.trim());
        formData.set("type", form.type);
        if (form.email) formData.set("email", form.email);
        if (form.password) formData.set("password", form.password);
        // Stored as the ten digits, the way every other phone in the app is - so "+91 98765
        // 43210" and "9876543210" are one number rather than two (see Common/phone.js).
        if (form.phone) formData.set("phone", normalizePhone(form.phone));
        if (form.type === "Employee") formData.set("permissions", JSON.stringify(form.permissions));

        try {
            if (isEditing) {
                formData.set("person_id", form.person_id);
                const res = await personBackend.update(formData);
                setPeople((prev) => prev.map((p) => (p._id === res.data._id ? res.data : p)));
            } else {
                const res = await personBackend.create(formData);
                setPeople((prev) => [res.data, ...prev]);
            }
            setForm(initialForm);
        } catch (error) {
            // The global interceptor toasts the API envelope. Catching it here is what keeps
            // the form filled in on failure - it used to reset regardless, so a rejected save
            // silently threw away everything that had been typed.
        } finally {
            setSaving(false);
        }
    };

    const onDelete = (id) => {
        const formData = new FormData();
        formData.set("person_id", id);
        personBackend.delete(formData).then(() => {
            setPeople((prev) => prev.filter((p) => p._id !== id));
            setMoreMenu(-1);
            if (form.person_id === id) setForm(initialForm);
        });
    };

    return (
        <div className="people-grid" style={{ marginTop: 24 }}>
            <div
                className={["shell-card", isEditing ? "shell-attention-active" : ""].filter(Boolean).join(" ")}
                ref={formCardRef}
                style={{
                    "--pulse-color": "rgba(251, 191, 36, 0.55)",
                    "--pulse-color-strong": "var(--xan-amber)",
                    border: isEditing ? "2px solid var(--xan-amber)" : undefined,
                }}
            >
                <div className="shell-card-header">
                    <span className="text-heading-brand">{isEditing ? "Edit Person" : "Add Person"}</span>
                </div>
                <form autoComplete="off" onSubmit={onSubmit} style={{ padding: "20px 24px" }}>
                    {/* Two per row: these are short fields, and one per line made the form
                        taller than the screen before the permission matrix even appeared. */}
                    <div className="people-form-grid">
                    <FormGroup>
                        <label className="form-control-label pp fs-12">
                            Name{required.errors.name !== undefined && <span className="required-star">*</span>}
                        </label>
                        <Input
                            className={required.errorFor("name") ? "is-required-missing" : undefined}
                            name="name"
                            placeholder="Person or company name"
                            value={form.name}
                            onChange={onChange}
                            onBlur={() => required.markTouched("name")}
                        />
                        {required.errorFor("name") && <span className="field-error">{required.errorFor("name")}</span>}
                    </FormGroup>
                    <FormGroup>
                        <label className="form-control-label pp fs-12">Type</label>
                        <Input type="select" name="type" value={form.type} onChange={onChange}>
                            {PERSON_TYPES.map((t) => (
                                <option key={t} value={t}>
                                    {t}
                                </option>
                            ))}
                        </Input>
                    </FormGroup>
                    <FormGroup>
                        <label className="form-control-label pp fs-12">
                            Email{required.errors.email !== undefined && <span className="required-star">*</span>}
                        </label>
                        <Input
                            className={required.errorFor("email") ? "is-required-missing" : undefined}
                            type="email"
                            name="email"
                            placeholder="name@example.com"
                            value={form.email}
                            onChange={onChange}
                            onBlur={() => required.markTouched("email")}
                        />
                        {required.errorFor("email") && <span className="field-error">{required.errorFor("email")}</span>}
                    </FormGroup>
                    <FormGroup>
                        <label className="form-control-label pp fs-12">
                            Password{required.errors.password !== undefined && <span className="required-star">*</span>}
                        </label>
                        <PasswordInput
                            inputClassName={`form-control${required.errorFor("password") ? " is-required-missing" : ""}`}
                            name="password"
                            placeholder={isEditing ? "Leave blank to keep current password" : "••••••••"}
                            value={form.password}
                            onChange={onChange}
                            onBlur={() => required.markTouched("password")}
                        />
                        {required.errorFor("password") && <span className="field-error">{required.errorFor("password")}</span>}
                    </FormGroup>
                    <FormGroup>
                        <label className="form-control-label pp fs-12">Phone Number</label>
                        <Input
                            className={required.errorFor("phone") ? "is-required-missing" : undefined}
                            type="tel"
                            name="phone"
                            placeholder="+91 98765 43210"
                            value={form.phone}
                            onChange={onChange}
                            onBlur={() => required.markTouched("phone")}
                        />
                        {required.errorFor("phone") && <span className="field-error">{required.errorFor("phone")}</span>}
                    </FormGroup>
                    </div>
                    {form.type === "Employee" && (
                        <FormGroup>
                            <label className="form-control-label pp fs-12">Feature access</label>
                            {/* One row per feature, one checkbox per action. "View" is the gate:
                                clearing it clears create and delete too, since an employee who
                                can't open a screen can't meaningfully create on it. */}
                            <div className="perm-matrix">
                                <div className="perm-matrix-head">
                                    <span>Feature</span>
                                    {ACTIONS.map((action) => (
                                        <span key={action}>{action}</span>
                                    ))}
                                </div>
                                {FEATURES.map((feature) => {
                                    // A feature may narrow its own actions. Approving and
                                    // sending are capabilities, not screens - requireFeature()
                                    // reads the view action and nothing else, so offering
                                    // create and delete there would be two cells that do
                                    // nothing whichever way they are set.
                                    const rowActions = feature.actions || ACTIONS;
                                    const rowGranted = rowActions.filter((a) => hasPerm(form.permissions, feature.key, a));
                                    return (
                                        <div className="perm-matrix-row" key={feature.key}>
                                            <button
                                                type="button"
                                                className="perm-matrix-label"
                                                title="Toggle every action for this feature"
                                                onClick={() => onToggleFeatureRow(feature.key, rowGranted.length !== rowActions.length)}
                                            >
                                                {feature.label}
                                            </button>
                                            {ACTIONS.map((action) =>
                                                rowActions.includes(action) ? (
                                                    <span key={action} className="perm-matrix-cell">
                                                        {/* The same control the tables use, so a
                                                            checkbox looks like a checkbox wherever
                                                            it appears. */}
                                                        <RowCheckbox
                                                            checked={hasPerm(form.permissions, feature.key, action)}
                                                            onChange={() => onTogglePermission(feature.key, action)}
                                                            ariaLabel={`${feature.label}: ${action}`}
                                                        />
                                                    </span>
                                                ) : (
                                                    // Holds the column, so the grid does not
                                                    // reflow and a narrowed row still lines up
                                                    // under the right heading.
                                                    <span key={action} className="perm-matrix-cell" aria-hidden="true" />
                                                )
                                            )}
                                        </div>
                                    );
                                })}
                            </div>
                        </FormGroup>
                    )}
                    <div style={{ display: "flex", gap: 8 }}>
                        <Button
                            type="submit"
                            className="shell-btn shell-btn-primary"
                            disabled={!required.isComplete || saving}
                            style={isEditing ? { "--btn-accent": "var(--xan-amber)", "--btn-accent-hover": "var(--xan-amber)" } : undefined}
                        >
                            {saving ? "Saving…" : isEditing ? "Update Person" : "Add Person"}
                        </Button>
                        {isEditing && (
                            <Button type="button" className="shell-btn shell-btn-secondary" onClick={onCancelEdit}>
                                Cancel
                            </Button>
                        )}
                    </div>
                </form>
            </div>

            <div className="shell-card">
                <div className="shell-card-header">
                    <span className="text-heading-brand">People</span>
                </div>
                <DataTable>
                    <thead>
                        <tr>
                            <th scope="col">#</th>
                            <th scope="col">Name</th>
                            <th scope="col">Type</th>
                            <th scope="col">Email</th>
                            <th scope="col">Phone</th>
                            <th scope="col" style={{ width: 44 }} />
                        </tr>
                    </thead>
                    <tbody>
                        {people.map((p, index) => (
                            <tr key={p._id}>
                                <td className="cell-mono">{index + 1}</td>
                                <td className="text-body-medium">{p.name}</td>
                                <td>
                                    <StatusBadge status={TYPE_STATUS[p.type] || "neutral"}>{p.type}</StatusBadge>
                                </td>
                                <td>{p.email || <span style={{ color: "var(--text-tertiary)" }}>—</span>}</td>
                                <td className="cell-mono">{p.phone || <span style={{ color: "var(--text-tertiary)" }}>—</span>}</td>
                                <td>
                                    <RowActionMenu open={moreMenu === index} onOpenChange={(next) => setMoreMenu(next ? index : -1)}>
                                        <RowActionMenuItem icon={Edit} variant="warning" onClick={() => onEditClick(p)}>
                                            Edit
                                        </RowActionMenuItem>
                                        <RowActionMenuItem icon={Trash2} variant="danger" onClick={() => onDelete(p._id)}>
                                            Delete
                                        </RowActionMenuItem>
                                    </RowActionMenu>
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </DataTable>
                <div className="shell-card-footer">
                    <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                        {people.length} people
                    </span>
                </div>
            </div>
        </div>
    );
};

export default PeopleManager;
