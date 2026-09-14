import React, { useEffect, useRef, useState } from "react";
import { Shield, ShieldOff, Check, Eye } from "react-feather";
import QRCode from "qrcode";
import { userBackend } from "../user_backend";
import { notifySuccess } from "../../../global/toast";
import { copyText } from "../../../Common/clipboard";
import PasswordInput from "../../../Common/PasswordInput";

// Optional two-factor, per account.
//
// Three steps rather than one: /setup writes a secret but leaves it OFF, and only a verified
// code from that secret turns it on. Enabling in a single step is how people lock themselves
// out - a mistyped secret, or an authenticator they never actually opened, and the next login
// is unanswerable.
//
// The QR is rendered here from the otpauth:// URI rather than sent as an image, so the secret
// crosses the wire once, as text, in the response that created it.
const TwoFactorCard = () => {
    const [enabled, setEnabled] = useState(false);
    const [loading, setLoading] = useState(true);
    const [setup, setSetup] = useState(null); // { secret, otpauth } while enrolling
    const [qr, setQr] = useState("");
    const [code, setCode] = useState("");
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState("");
    // The already-enrolled key, shown again so a second phone can be added. Held only while
    // the panel is open: pressing Done drops it, and the password is asked for again next
    // time rather than the secret sitting in component state for the rest of the visit.
    const [revealed, setRevealed] = useState(null); // { secret, otpauth }
    const [revealPrompt, setRevealPrompt] = useState(false);
    const [revealPassword, setRevealPassword] = useState("");
    const [revealError, setRevealError] = useState("");
    const codeRef = useRef(null);
    const revealRef = useRef(null);
    // Generated once per mount rather than fixed, so Chrome has no saved credential whose
    // field name matches this one. Held in a ref because a name that changed on every render
    // would reset the field under a half-typed value.
    const revealFieldName = useRef(`totp-reveal-${Math.random().toString(36).slice(2)}`).current;

    // Either enrolment shows a key, or a re-reveal does. Same block of markup either way -
    // it is the same secret, and two near-identical panels would drift.
    const showingKey = setup || revealed;

    const load = () => {
        userBackend
            .totpStatus()
            .then((res) => setEnabled(Boolean(res.data?.enabled)))
            .catch(() => {})
            .finally(() => setLoading(false));
    };

    useEffect(load, []);

    // Redrawn whenever a new secret is issued. Errors are swallowed to the extent that the
    // QR simply does not appear - the secret is shown as text beside it and is enough on its
    // own, so a canvas failure must not block enrolment.
    useEffect(() => {
        const uri = setup?.otpauth || revealed?.otpauth;
        if (!uri) return setQr("");
        QRCode.toDataURL(uri, { width: 176, margin: 1 })
            .then(setQr)
            .catch(() => setQr(""));
    }, [setup, revealed]);

    const begin = async () => {
        setBusy(true);
        setError("");
        closeReveal();
        try {
            const res = await userBackend.totpSetup();
            setSetup(res.data);
            setCode("");
            requestAnimationFrame(() => codeRef.current?.focus());
        } catch (err) {
            setError(err.message || "Couldn't start setup.");
        } finally {
            setBusy(false);
        }
    };

    const confirm = async () => {
        if (!/^\d{6}$/.test(code.trim())) return setError("Enter the 6-digit code from your app.");
        setBusy(true);
        setError("");
        try {
            const res = await userBackend.totpEnable(code.trim());
            notifySuccess(res.message || "Two-factor is on.");
            setSetup(null);
            setCode("");
            setEnabled(true);
        } catch (err) {
            setError(err.message || "Couldn't turn two-factor on.");
        } finally {
            setBusy(false);
        }
    };

    // Forgets the secret as well as closing the panel: leaving it in state would mean the
    // password bought a reveal that lasts the rest of the visit, which is not what was asked
    // for and not what "close" looks like.
    const closeReveal = () => {
        setRevealed(null);
        setRevealPrompt(false);
        setRevealPassword("");
        setRevealError("");
    };

    const reveal = async () => {
        if (!revealPassword) return setRevealError("Enter your account password.");
        setBusy(true);
        setRevealError("");
        try {
            const res = await userBackend.totpReveal(revealPassword);
            setRevealed(res.data);
            setRevealPrompt(false);
            // Dropped the moment it has been used - there is no reason for the password to
            // outlive the request it was typed for.
            setRevealPassword("");
        } catch (err) {
            setRevealError(err.message || "Couldn't show the key.");
        } finally {
            setBusy(false);
        }
    };

    const turnOff = async () => {
        if (!/^\d{6}$/.test(code.trim())) return setError("Enter a current code to turn it off.");
        setBusy(true);
        setError("");
        try {
            const res = await userBackend.totpDisable(code.trim());
            notifySuccess(res.message || "Two-factor is off.");
            setCode("");
            setEnabled(false);
        } catch (err) {
            setError(err.message || "Couldn't turn two-factor off.");
        } finally {
            setBusy(false);
        }
    };

    return (
        <div className="shell-card">
            <div className="shell-card-header">
                <span className="text-heading-brand">Two-factor</span>
                <span className={enabled ? "xan-status-badge status-paid" : "xan-status-badge status-neutral"}>
                    {loading ? "…" : enabled ? "On" : "Off"}
                </span>
            </div>

            <div style={{ padding: 20, display: "grid", gap: 14 }}>
                {!enabled && !setup && (
                    <>
                        <p className="text-body-small" style={{ color: "var(--text-tertiary)", margin: 0 }}>
                            Ask for a code from your authenticator app as well as your password. Optional - your
                            password keeps working exactly as it does now until you turn this on.
                        </p>
                        <div>
                            <button type="button" className="shell-btn shell-btn-primary" onClick={begin} disabled={busy}>
                                <Shield size={14} /> {busy ? "Starting…" : "Set up two-factor"}
                            </button>
                        </div>
                    </>
                )}

                {/* Adding a second phone to an account that already has two-factor. Without
                    this the only route is to turn it off and re-enrol, which invalidates the
                    phone that already works. Offered only once it is on, and only when nothing
                    else is already showing a key. */}
                {enabled && !showingKey && !revealPrompt && (
                    <>
                        <p className="text-body-small" style={{ color: "var(--text-tertiary)", margin: 0 }}>
                            Setting up a second phone or another person on this account? Show the same key again
                            rather than turning two-factor off - turning it off would stop the phone that already
                            works.
                        </p>
                        <div>
                            <button
                                type="button"
                                className="shell-btn shell-btn-secondary"
                                onClick={() => {
                                    setRevealPrompt(true);
                                    setRevealError("");
                                    requestAnimationFrame(() => revealRef.current?.focus());
                                }}
                                disabled={busy}
                            >
                                <Eye size={14} /> Show key again
                            </button>
                        </div>
                    </>
                )}

                {revealPrompt && (
                    <div style={{ display: "grid", gap: 8 }}>
                        <label className="form-control-label pp fs-12" htmlFor="totp-reveal-password">
                            Your account password
                        </label>
                        <span className="text-body-small" style={{ color: "var(--text-tertiary)" }}>
                            Typed here, not filled in for you - whoever is at the keyboard has to know the account
                            password before the key is shown.
                        </span>
                        <PasswordInput
                            id="totp-reveal-password"
                            inputClassName="form-control"
                            wrapperStyle={{ maxWidth: 280 }}
                            innerRef={revealRef}
                            value={revealPassword}
                            onChange={(e) => setRevealPassword(e.target.value)}
                            onKeyDown={(e) => {
                                if (e.key === "Enter") {
                                    e.preventDefault();
                                    reveal();
                                }
                            }}
                            // Nothing may fill this in or carry it away. autoComplete="off" on
                            // its own is widely ignored by Chrome, so the field also carries a
                            // name no saved credential can match, plus the opt-outs 1Password
                            // and LastPass honour. Paste, copy and cut are refused outright:
                            // the point of the prompt is that the person present knows the
                            // password, not that a manager on the machine remembers it.
                            autoComplete="off"
                            name={revealFieldName}
                            data-1p-ignore="true"
                            data-lpignore="true"
                            data-form-type="other"
                            onPaste={(e) => e.preventDefault()}
                            onCopy={(e) => e.preventDefault()}
                            onCut={(e) => e.preventDefault()}
                            onDrop={(e) => e.preventDefault()}
                        />
                        <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                            <button type="button" className="shell-btn shell-btn-primary" onClick={reveal} disabled={busy}>
                                <Eye size={14} /> {busy ? "Checking..." : "Show key"}
                            </button>
                            <button type="button" className="shell-btn shell-btn-secondary" onClick={closeReveal} disabled={busy}>
                                Cancel
                            </button>
                        </div>
                        {revealError && (
                            <span className="text-body-small" style={{ color: "var(--status-red-text)" }}>
                                {revealError}
                            </span>
                        )}
                    </div>
                )}

                {showingKey && (
                    <>
                        <p className="text-body-small" style={{ color: "var(--text-tertiary)", margin: 0 }}>
                            {setup
                                ? "Scan this with Google Authenticator, Authy or 1Password, then enter the code it shows."
                                : "Scan this on the other phone. It is the key this account already uses, so the phone you set up first keeps working."}
                        </p>
                        <div style={{ display: "flex", gap: 16, flexWrap: "wrap", alignItems: "flex-start" }}>
                            {qr ? (
                                <img
                                    src={qr}
                                    alt="Two-factor setup QR code"
                                    width={176}
                                    height={176}
                                    // White plate behind it: a QR needs light quiet space to
                                    // scan, and in dark mode the card behind is nearly black.
                                    style={{ background: "#fff", padding: 8, borderRadius: 8 }}
                                />
                            ) : null}
                            <div style={{ display: "grid", gap: 8, minWidth: 200 }}>
                                <span className="text-label-caps" style={{ color: "var(--text-tertiary)" }}>
                                    Or enter this key by hand
                                </span>
                                <code className="cell-mono" style={{ wordBreak: "break-all", fontSize: 12.5 }}>
                                    {showingKey.secret}
                                </code>
                                <div>
                                    {/* Copy is offered while enrolling, where the key has just
                                        been minted and moving it onto a phone is the whole
                                        task. It is NOT offered on a re-reveal: that key is
                                        already protecting the account, and the reason for
                                        asking for a password was to keep it off the clipboard. */}
                                    {setup ? (
                                        <button
                                            type="button"
                                            className="shell-btn shell-btn-secondary"
                                            onClick={async () => {
                                                if (await copyText(setup.secret)) notifySuccess("Key copied.");
                                            }}
                                        >
                                            Copy key
                                        </button>
                                    ) : (
                                        <button type="button" className="shell-btn shell-btn-secondary" onClick={closeReveal}>
                                            Done
                                        </button>
                                    )}
                                </div>
                            </div>
                        </div>
                    </>
                )}

                {(setup || enabled) && (
                    <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
                        <input
                            ref={codeRef}
                            className="form-control"
                            style={{ width: 140 }}
                            placeholder="6-digit code"
                            inputMode="numeric"
                            maxLength={6}
                            autoComplete="one-time-code"
                            value={code}
                            onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
                        />
                        {setup ? (
                            <button type="button" className="shell-btn shell-btn-primary" onClick={confirm} disabled={busy}>
                                <Check size={14} /> {busy ? "Checking…" : "Turn on"}
                            </button>
                        ) : (
                            // Turning it off needs a live code, not just a live session -
                            // otherwise an unattended signed-in browser strips the second
                            // factor off the account.
                            <button type="button" className="shell-btn shell-btn-secondary" onClick={turnOff} disabled={busy}>
                                <ShieldOff size={14} /> {busy ? "Checking…" : "Turn off"}
                            </button>
                        )}
                        {setup && (
                            <button type="button" className="shell-btn shell-btn-secondary" onClick={() => setSetup(null)} disabled={busy}>
                                Cancel
                            </button>
                        )}
                    </div>
                )}

                {error && (
                    <span className="text-body-small" style={{ color: "var(--status-red-text)" }}>
                        {error}
                    </span>
                )}
            </div>
        </div>
    );
};

export default TwoFactorCard;
