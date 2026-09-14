import React, { useState } from "react";
import { Key, HelpCircle } from "react-feather";
import PasswordInput from "../../../Common/PasswordInput";
import { userBackend } from "../user_backend";
import { notifySuccess } from "../../../global/toast";

// Change your own password.
//
// The current one is required, not just a live session: an unattended signed-in browser must
// not be enough to take an account over. Someone who has genuinely forgotten it cannot use
// this at all - which is what the button beside it is for: it files the same request the
// sign-in screen files, so being signed in is not a reason to be stuck.
const PasswordCard = ({ email = "" }) => {
    const [current, setCurrent] = useState("");
    const [next, setNext] = useState("");
    const [confirm, setConfirm] = useState("");
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState("");
    // Asked for before filing, because this lands in a queue a person has to work through -
    // a button that silently raised a ticket on one stray click would fill it with noise.
    const [confirmingForgot, setConfirmingForgot] = useState(false);
    const [forgotSent, setForgotSent] = useState(false);

    const submit = async (e) => {
        e.preventDefault();
        setError("");
        if (!current) return setError("Enter your current password.");
        if (next.length < 8) return setError("Use at least 8 characters for the new password.");
        // Checked here as well as being a different field, because a typo in a masked box is
        // invisible and would lock the person out of their own account.
        if (next !== confirm) return setError("The two new passwords do not match.");

        setBusy(true);
        try {
            const res = await userBackend.changePassword(current, next);
            notifySuccess(res.message || "Password changed.");
            setCurrent("");
            setNext("");
            setConfirm("");
        } catch (err) {
            setError(err.message || "Couldn't change the password.");
        } finally {
            setBusy(false);
        }
    };

    // The same endpoint the sign-in screen uses. It answers identically whether or not the
    // address exists, so it cannot be used to find out who has an account - but this caller is
    // signed in and the account plainly exists, so the message here is definite rather than a
    // relay of that deliberately vague one, which would read as though nothing had happened.
    const forgot = async () => {
        setError("");
        setBusy(true);
        try {
            await userBackend.requestPasswordReset(email);
            setConfirmingForgot(false);
            setForgotSent(true);
            notifySuccess("Asked the team to reset your password.");
        } catch (err) {
            setError(err.message || "Couldn't send that request.");
        } finally {
            setBusy(false);
        }
    };

    return (
        <div className="shell-card">
            <div className="shell-card-header">
                <span className="text-heading-brand">Password</span>
            </div>
            <form onSubmit={submit} style={{ padding: 20, display: "grid", gap: 12 }} autoComplete="off">
                <div>
                    <label className="form-control-label pp fs-12">Current password</label>
                    <PasswordInput
                        inputClassName="form-control"
                        autoComplete="current-password"
                        value={current}
                        onChange={(e) => setCurrent(e.target.value)}
                    />
                </div>
                <div>
                    <label className="form-control-label pp fs-12">New password</label>
                    <PasswordInput
                        inputClassName="form-control"
                        autoComplete="new-password"
                        value={next}
                        onChange={(e) => setNext(e.target.value)}
                    />
                </div>
                <div>
                    <label className="form-control-label pp fs-12">Confirm new password</label>
                    <PasswordInput
                        inputClassName="form-control"
                        autoComplete="new-password"
                        value={confirm}
                        onChange={(e) => setConfirm(e.target.value)}
                    />
                </div>

                {error && (
                    <span className="text-body-small" style={{ color: "var(--status-red-text)" }}>
                        {error}
                    </span>
                )}

                <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
                    <button type="submit" className="shell-btn shell-btn-primary" disabled={busy}>
                        <Key size={14} /> {busy ? "Changing…" : "Change password"}
                    </button>
                    {/* Beside Change password, not on the sign-in screen only: the person who
                        cannot remember their password is usually already signed in somewhere,
                        and signing out to reach the link is exactly the move that strands
                        them. type="button" so it cannot submit the form it sits in. */}
                    {!forgotSent && !confirmingForgot && (
                        <button
                            type="button"
                            className="shell-btn shell-btn-secondary"
                            onClick={() => setConfirmingForgot(true)}
                            disabled={busy}
                        >
                            <HelpCircle size={14} /> I've forgotten it
                        </button>
                    )}
                </div>

                {confirmingForgot && (
                    <div style={{ display: "grid", gap: 8 }}>
                        <span className="text-body-small">
                            Ask the team to reset the password for <strong>{email}</strong>? They will be in touch
                            to set a new one — nothing changes until they do, and you stay signed in here.
                        </span>
                        <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                            <button type="button" className="shell-btn shell-btn-primary" onClick={forgot} disabled={busy}>
                                {busy ? "Sending…" : "Ask for a reset"}
                            </button>
                            <button
                                type="button"
                                className="shell-btn shell-btn-secondary"
                                onClick={() => setConfirmingForgot(false)}
                                disabled={busy}
                            >
                                Cancel
                            </button>
                        </div>
                    </div>
                )}

                {forgotSent && (
                    <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                        Reset requested. The team has it — one open request per account, so pressing it again
                        would not move it up the queue.
                    </span>
                )}

                <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                    Your other devices are signed out when the password changes.
                </span>
            </form>
        </div>
    );
};

export default PasswordCard;
