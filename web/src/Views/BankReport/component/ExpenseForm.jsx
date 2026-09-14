import React, { useEffect, useState } from "react";
import { FormGroup, Input, Button, Row, Col } from "reactstrap";
import useRequiredFields from "../../../Common/useRequiredFields";
import AssigneeDropdown from "../../Lifecycle/component/AssigneeDropdown";
import { bankBackend } from "../../../Common/bank_backend";
import { useBanks, addBank } from "../../../Common/bankStore";
import { bankReportBackend } from "../bank_report_backend";
import ExpenseList from "./ExpenseList";
import DateField from "../../../Common/DateField";

const today = () => new Date().toISOString().slice(0, 10);
const emptyForm = () => ({ bank_id: "", bank_name: "", date: today(), amount: "", notes: "" });

// Money out that is not a supplier payment - rent, salaries, fuel. Every expense is
// attributed to a bank so it can be netted against that account in the Bank Report.
const ExpenseForm = ({ onSaved }) => {
    const [form, setForm] = useState(emptyForm);
    const banks = useBanks();
    const [saving, setSaving] = useState(false);
    // Bumped after a save so the list below reloads without either component owning the other.
    const [savedCount, setSavedCount] = useState(0);

    const required = useRequiredFields(form, {
        bank_id: "Bank",
        date: "Date",
        amount: {
            label: "Amount",
            validate: (v) => (Number(v) > 0 ? null : "Amount must be greater than zero."),
        },
    });

    const onChange = (e) => setForm((prev) => ({ ...prev, [e.target.name]: e.target.value }));

    const bankOptions = banks.map((b) => ({ id: b._id, name: b.name }));

    // Same inline "+ Create new" affordance the Paid/Received forms have - a bank that
    // doesn't exist yet shouldn't force you to leave the Expense form to add it.
    const onBankCreateNew = async (query) => {
        if (!query || !query.trim()) return;
        const formData = new FormData();
        formData.set("name", query.trim());
        const res = await bankBackend.create(formData);
        addBank(res.data);
        setForm((f) => ({ ...f, bank_id: res.data._id, bank_name: res.data.name }));
        required.markTouched("bank_id");
    };

    const onSubmit = async () => {
        if (!required.isComplete) {
            required.showAll();
            return;
        }
        setSaving(true);
        try {
            const formData = new FormData();
            formData.set("bank_id", form.bank_id);
            formData.set("date", form.date);
            formData.set("amount", form.amount);
            formData.set("notes", form.notes.trim());
            await bankReportBackend.createExpense(formData);
            // No notifySuccess here: /expense/create matches the mutation pattern in
            // api/errorInterceptor.js, which already toasts the API's message. Adding one
            // here showed the same confirmation twice on a single tap.
            setForm(emptyForm());
            setSavedCount((n) => n + 1);
            if (onSaved) onSaved();
        } catch (err) {
            // The global axios interceptor raises the toast - nothing to add here.
        } finally {
            setSaving(false);
        }
    };

    return (
        // Form on the left, the list of expenses on the right - the same two-column shape as
        // Received and Paid, instead of a full-width form with the list stacked underneath.
        <Row className="mt-2">
            <Col xl="6">
        <div className="shell-card">
            {/* Titled card with the form inside, the same shape as Received and Paid - this
                one was a bare padded box with the fields loose in it. */}
            <div className="shell-card-header">
                <span className="text-heading-brand">New expense</span>
            </div>
            <div style={{ padding: 20 }}>
            <Row>
                <Col md="6">
                    <FormGroup>
                        <label className="form-control-label pp fs-12">
                            Bank{required.errors.bank_id !== undefined && <span className="required-star">*</span>}
                        </label>
                        <AssigneeDropdown
                            value={form.bank_name}
                            placeholder="Select a bank"
                            options={bankOptions}
                            fullWidth
                            onSelect={(id, name) => {
                                setForm((f) => ({ ...f, bank_id: id || "", bank_name: name || "" }));
                                required.markTouched("bank_id");
                            }}
                            onCreateNew={onBankCreateNew}
                        />
                        {required.errorFor("bank_id") && <span className="field-error">{required.errorFor("bank_id")}</span>}
                    </FormGroup>
                </Col>
                <Col md="6">
                    <FormGroup>
                        <label className="form-control-label pp fs-12">
                            Date{required.errors.date !== undefined && <span className="required-star">*</span>}
                        </label>
                        <DateField className={required.errorFor("date") ? "is-required-missing" : undefined} name="date" value={form.date} onChange={onChange} onBlur={() => required.markTouched("date")} />
                        {required.errorFor("date") && <span className="field-error">{required.errorFor("date")}</span>}
                    </FormGroup>
                </Col>
                <Col md="6">
                    <FormGroup>
                        <label className="form-control-label pp fs-12">
                            Amount{required.errors.amount !== undefined && <span className="required-star">*</span>}
                        </label>
                        <Input
                            className={required.errorFor("amount") ? "is-required-missing" : undefined}
                            type="number"
                            step="0.01"
                            name="amount"
                            value={form.amount}
                            onChange={onChange}
                            onBlur={() => required.markTouched("amount")}
                        />
                        {required.errorFor("amount") && <span className="field-error">{required.errorFor("amount")}</span>}
                    </FormGroup>
                </Col>
                <Col md="6">
                    <FormGroup>
                        <label className="form-control-label pp fs-12">Notes</label>
                        <Input name="notes" placeholder="What was it for?" value={form.notes} onChange={onChange} />
                    </FormGroup>
                </Col>
            </Row>
            <div className="d-flex align-items-center" style={{ gap: 8 }}>
                <Button className="shell-btn shell-btn-primary" onClick={onSubmit} disabled={saving}>
                    {saving ? "Saving…" : "Save Expense"}
                </Button>
                <Button
                    type="button"
                    className="shell-btn shell-btn-secondary"
                    onClick={() => setForm(emptyForm())}
                    disabled={saving}
                >
                    Reset
                </Button>
            </div>
            </div>
        </div>
            </Col>
            <Col xl="6">
                <ExpenseList reloadKey={savedCount} />
            </Col>
        </Row>
    );
};

export default ExpenseForm;
