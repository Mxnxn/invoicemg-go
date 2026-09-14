import PasswordInput from "../../../Common/PasswordInput";
import React, { useState } from "react";
import { Mail, Lock, User, AlertOctagon, Key, Briefcase, MapPin, Phone, Hash, Home, CreditCard, Shield } from "react-feather";
import { authBackend } from "../auth_backend";
import { companyBackend } from "../../../Common/company_backend";
import { notifySuccess } from "../../../global/toast";
import { pullAppearance } from "../../../Common/appearanceSync";
import { pullTableSettings } from "../../../Common/tableSettings";
import { landingPathFor } from "../landingPath";

const initialRegisterForm = { registrationToken: "", name: "", email: "", password: "" };
const initialCompanyForm = { name: "", firm: "", address: "", phone: "", gst: "", account_no: "", ifsc: "", bank_name: "" };

// Single merged Login/Register surface, styled against the shell design tokens (see
// src/styles/tokens.css) instead of the old Argon/reactstrap auth card, so it matches the
// rest of the app in both themes rather than looking like a separate product.

const AuthForm = () => {
    const [mode, setMode] = useState("LOGIN");
    const [error, setError] = useState("");
    const [busy, setBusy] = useState(false);

    const [loginEmail, setLoginEmail] = useState("");
    const [loginPassword, setLoginPassword] = useState("");
    // Second factor, only for accounts that have it on: the server answers the first attempt
    // with totpRequired and no session, and the form asks for a code.
    const [totpRequired, setTotpRequired] = useState(false);
    const [totpCode, setTotpCode] = useState("");
    const [remember, setRemember] = useState(false);

    const [registerForm, setRegisterForm] = useState(initialRegisterForm);

    // Registration is two steps on this same surface: create the account, then create the
    // Company it needs. /register returns no session and /company/create sits behind
    // TokenHelper + requireAdmin, so step 1 logs in before handing over to step 2.
    const [registerStep, setRegisterStep] = useState(1);
    const [companyForm, setCompanyForm] = useState(initialCompanyForm);
    const [logoFile, setLogoFile] = useState(null);
    const [upiQrFile, setUpiQrFile] = useState(null);
    const [logoPreview, setLogoPreview] = useState("");
    const [upiQrPreview, setUpiQrPreview] = useState("");

    const pickFile = (event, setFile, setPreview) => {
        const file = event.target.files && event.target.files[0];
        if (!file) return;
        setFile(file);
        setPreview(URL.createObjectURL(file));
    };

    const switchMode = (next) => {
        setRegisterStep(1);
        setMode(next);
        setError("");
    };

    const onLogin = async (e) => {
        e.preventDefault();
        if (!loginEmail || !loginEmail.includes("@")) return setError("Invalid email.");
        if (!loginPassword) return setError("Invalid password.");

        if (totpRequired && !/^\d{6}$/.test(totpCode.trim())) return setError("Enter the 6-digit code from your authenticator app.");

        const formData = new FormData();
        formData.set("email", loginEmail);
        formData.set("password", loginPassword);
        formData.set("remember", String(remember));
        if (totpCode.trim()) formData.set("totp", totpCode.trim());
        setBusy(true);
        setError("");
        try {
            let res;
            try {
                res = await authBackend.loginWithEmailAndPassword(formData);
            } catch (adminError) {
                // The employee endpoint is a fallback for accounts that are not admins - but
                // a rejected TOTP code is a definite answer from the admin login, and falling
                // through would replace "that code isn't right" with "email doesn't exist".
                if (adminError && adminError.totpRequired) throw adminError;
                res = await authBackend.loginAsEmployee(formData);
            }
            // Password accepted, but the account wants a code. No session exists yet - this
            // is a prompt, not a failure, so the form steps forward rather than showing red.
            if (res.totpRequired && !res.data) {
                setTotpRequired(true);
                setBusy(false);
                setError("");
                return;
            }
            window.localStorage.setItem("uid", res.data.uid);
            window.localStorage.setItem("session_token", res.data.token);
            window.localStorage.setItem("email", res.data.email);
            window.localStorage.setItem("role", res.data.role);
            window.localStorage.setItem("permissions", JSON.stringify(res.data.permissions || []));
            // Before the redirect, so this person's saved font, sizes and row count are
            // already in localStorage when the next page paints - otherwise the first screen
            // after logging in on a new browser flashes the defaults. It cannot fail the
            // login: pullAppearance swallows its own errors and returns null.
            // Both together: the appearance paints the shell, the table settings restore the
            // column arrangement - and a session expiring must not lose either.
            await Promise.all([pullAppearance(), pullTableSettings()]);
            // Land directly on a route this account can open. Previously this went to "/",
            // which redirects to /admin, which an employee usually can't access - so the
            // guard had to bounce them again and the first paint showed nothing until a
            // manual reload.
            window.location.href = landingPathFor(res.data.role, res.data.permissions || []);
        } catch (error) {
            // A wrong code is still a prompt for a code - keep the field on screen.
            if (error && error.totpRequired) setTotpRequired(true);
            setError(error.message || "Couldn't log in.");
        } finally {
            setBusy(false);
        }
    };

    const onRegister = async (e) => {
        e.preventDefault();
        if (!registerForm.name) return setError("Name is required.");
        if (!registerForm.email || !registerForm.email.includes("@")) return setError("Invalid email.");
        if (!registerForm.password || registerForm.password.length < 8) return setError("Min. 8 character password.");

        const formData = new FormData();
        formData.set("email", registerForm.email);
        formData.set("password", registerForm.password);
        formData.set("name", registerForm.name);
        formData.set("registrationToken", registerForm.registrationToken);
        setBusy(true);
        setError("");
        try {
            await authBackend.registerWithEmailAndPassword(formData);

            // Obtain the session step 2 needs. /user/login stamps role "admin" on every
            // UserSession, so this satisfies requireAdmin on /company/create.
            const login = new FormData();
            login.set("email", registerForm.email);
            login.set("password", registerForm.password);
            const res = await authBackend.loginWithEmailAndPassword(login);
            window.localStorage.setItem("uid", res.data.uid);
            window.localStorage.setItem("session_token", res.data.token);
            window.localStorage.setItem("email", res.data.email);
            window.localStorage.setItem("role", res.data.role);

            setLoginEmail(registerForm.email);
            setCompanyForm({ name: registerForm.name, firm: "" });
            setRegisterStep(2);
        } catch (error) {
            setError(error.message || "Couldn't create the account.");
        } finally {
            setBusy(false);
        }
    };

    // Step 2 collects identity, step 3 the details that print on documents. Bank fields are
    // deliberately optional - plenty of firms invoice without printing bank details.
    const onCompanyDetails = (e) => {
        e.preventDefault();
        if (!companyForm.name) return setError("Company name is required.");
        setError("");
        setRegisterStep(3);
    };

    const onCreateCompany = async (e) => {
        e.preventDefault();
        if (!companyForm.name) return setError("Company name is required.");
        if (!String(companyForm.phone || "").trim()) return setError("Phone number is required.");

        const formData = new FormData();
        formData.set("name", companyForm.name);
        formData.set("firm", companyForm.firm || companyForm.name);
        formData.set("address", companyForm.address);
        formData.set("phone", companyForm.phone);
        formData.set("gst", companyForm.gst);
        formData.set("account_no", companyForm.account_no);
        formData.set("ifsc", companyForm.ifsc);
        formData.set("bank_name", companyForm.bank_name);
        // Both uploads are optional - only append what was actually chosen, because the
        // API decides by the presence of req.files rather than by empty values.
        if (logoFile) formData.append("logo", logoFile);
        if (upiQrFile) formData.append("upiQr", upiQrFile);

        setBusy(true);
        setError("");
        try {
            await companyBackend.createCompany(formData);
            window.location.href = "/admin";
        } catch (error) {
            setError(error.message || "Couldn't create the company.");
        } finally {
            setBusy(false);
        }
    };

    return (
        <div className="auth-card">
            <div className="auth-tabs" role="tablist">
                <button
                    type="button"
                    role="tab"
                    aria-selected={mode === "LOGIN"}
                    className={["auth-tab", mode === "LOGIN" ? "active" : ""].filter(Boolean).join(" ")}
                    onClick={() => switchMode("LOGIN")}
                >
                    Login
                </button>
                <button
                    type="button"
                    role="tab"
                    aria-selected={mode === "REGISTER"}
                    className={["auth-tab", mode === "REGISTER" ? "active" : ""].filter(Boolean).join(" ")}
                    onClick={() => switchMode("REGISTER")}
                >
                    Register
                </button>
            </div>

            {error && (
                <div className="auth-error">
                    <AlertOctagon size={16} />
                    <span>{error}</span>
                </div>
            )}

            {mode === "LOGIN" ? (
                <form onSubmit={onLogin}>
                    <div className="auth-field">
                        <Mail size={16} className="auth-field-icon" />
                        <input
                            className="auth-input"
                            type="email"
                            placeholder="Email"
                            autoComplete="email"
                            value={loginEmail}
                            onChange={(e) => setLoginEmail(e.target.value)}
                        />
                    </div>
                    <div className="auth-field">
                        <Lock size={16} className="auth-field-icon" />
                        <PasswordInput
                            inputClassName="auth-input"
                                                        placeholder="Password"
                            autoComplete="current-password"
                            value={loginPassword}
                            onChange={(e) => setLoginPassword(e.target.value)}
                        />
                    </div>

                    {/* Only after the password has been accepted and the account turns out to
                        have a second factor - asking every account for a code it does not have
                        would be nonsense, and showing the field up front would leak which
                        accounts have 2FA on. */}
                    {totpRequired && (
                        <div className="auth-field">
                            <Shield size={16} className="auth-field-icon" />
                            <input
                                className="auth-input"
                                placeholder="6-digit code"
                                // one-time-code lets a phone offer the code from its keyboard.
                                autoComplete="one-time-code"
                                inputMode="numeric"
                                maxLength={6}
                                autoFocus
                                value={totpCode}
                                onChange={(e) => setTotpCode(e.target.value.replace(/\D/g, ""))}
                            />
                        </div>
                    )}

                    {/* No email sender exists in this app, so a reset is a person handing over
                        a password. This raises the request; the developer team sees it in the
                        Dev panel. */}
                    <button
                        type="button"
                        className="auth-forgot"
                        onClick={async () => {
                            if (!loginEmail || !loginEmail.includes("@")) return setError("Enter your email address first.");
                            try {
                                const res = await authBackend.requestPasswordReset(loginEmail);
                                setError("");
                                notifySuccess(res.message);
                            } catch (err) {
                                setError(err.message || "Couldn't send that request.");
                            }
                        }}
                    >
                        Forgot password?
                    </button>

                    <label className="auth-remember">
                        <input type="checkbox" checked={remember} onChange={(e) => setRemember(e.target.checked)} />
                        <span>Keep me signed in for 7 days</span>
                    </label>

                    <button type="submit" className="shell-btn shell-btn-primary auth-submit" disabled={busy}>
                        {busy ? "Logging in…" : totpRequired ? "Verify code" : "Login"}
                    </button>
                </form>
            ) : registerStep === 1 ? (
                <form onSubmit={onRegister} className="auth-panel" key="reg-step-1">
                    <div className="auth-stepper" aria-hidden="true">
                        <span className="auth-step-dot" data-active="true" />
                        <span className="auth-step-dot" data-active="false" />
                        <span className="auth-step-dot" data-active="false" />
                    </div>
                    <div className="auth-field">
                        <Key size={16} className="auth-field-icon" />
                        <input
                            className="auth-input"
                            type="text"
                            placeholder="Registration token"
                            value={registerForm.registrationToken}
                            onChange={(e) => setRegisterForm({ ...registerForm, registrationToken: e.target.value })}
                        />
                    </div>
                    <div className="auth-field">
                        <User size={16} className="auth-field-icon" />
                        <input
                            className="auth-input"
                            type="text"
                            placeholder="Name"
                            autoComplete="name"
                            value={registerForm.name}
                            onChange={(e) => setRegisterForm({ ...registerForm, name: e.target.value })}
                        />
                    </div>
                    <div className="auth-field">
                        <Mail size={16} className="auth-field-icon" />
                        <input
                            className="auth-input"
                            type="email"
                            placeholder="Email"
                            autoComplete="email"
                            value={registerForm.email}
                            onChange={(e) => setRegisterForm({ ...registerForm, email: e.target.value })}
                        />
                    </div>
                    <div className="auth-field">
                        <Lock size={16} className="auth-field-icon" />
                        <PasswordInput
                            inputClassName="auth-input"
                            placeholder="Password (min. 8 characters)"
                            autoComplete="new-password"
                            value={registerForm.password}
                            onChange={(e) => setRegisterForm({ ...registerForm, password: e.target.value })}
                        />
                    </div>
                    <button type="submit" className="shell-btn shell-btn-primary auth-submit" disabled={busy}>
                        {busy ? "Creating account…" : "Continue"}
                    </button>
                </form>
            ) : registerStep === 2 ? (
                <form onSubmit={onCompanyDetails} className="auth-panel" key="reg-step-2">
                    <div className="auth-stepper" aria-hidden="true">
                        <span className="auth-step-dot" data-active="true" />
                        <span className="auth-step-dot" data-active="true" />
                        <span className="auth-step-dot" data-active="false" />
                    </div>
                    <p className="auth-step-hint">Your company appears on every invoice. You can change it later.</p>
                    <div className="auth-field">
                        <Briefcase size={16} className="auth-field-icon" />
                        <input
                            className="auth-input"
                            type="text"
                            placeholder="Company name"
                            value={companyForm.name}
                            onChange={(e) => setCompanyForm({ ...companyForm, name: e.target.value })}
                        />
                    </div>
                    <div className="auth-field">
                        <Briefcase size={16} className="auth-field-icon" />
                        <input
                            className="auth-input"
                            type="text"
                            placeholder="Firm (optional)"
                            value={companyForm.firm}
                            onChange={(e) => setCompanyForm({ ...companyForm, firm: e.target.value })}
                        />
                    </div>

                    <label className="auth-drop" htmlFor="auth-logo">
                        {logoPreview ? <img className="auth-drop-preview" src={logoPreview} alt="Logo preview" /> : null}
                        <span className="auth-drop-title">Logo <span className="auth-optional">optional</span></span>
                        <span className="auth-drop-hint">{logoFile ? logoFile.name : "PNG or JPG"}</span>
                        <input
                            id="auth-logo"
                            type="file"
                            accept="image/png,image/jpeg"
                            onChange={(e) => pickFile(e, setLogoFile, setLogoPreview)}
                        />
                    </label>

                    <label className="auth-drop" htmlFor="auth-upiqr">
                        {upiQrPreview ? <img className="auth-drop-preview" src={upiQrPreview} alt="UPI QR preview" /> : null}
                        <span className="auth-drop-title">UPI QR code <span className="auth-optional">optional</span></span>
                        <span className="auth-drop-hint">{upiQrFile ? upiQrFile.name : "Printed on your invoices"}</span>
                        <input
                            id="auth-upiqr"
                            type="file"
                            accept="image/png,image/jpeg"
                            onChange={(e) => pickFile(e, setUpiQrFile, setUpiQrPreview)}
                        />
                    </label>

                    <button type="submit" className="shell-btn shell-btn-primary auth-submit" disabled={busy}>
                        {busy ? "Setting up…" : "Continue"}
                    </button>
                </form>
            ) : (
                <form onSubmit={onCreateCompany} className="auth-panel" key="reg-step-3">
                    <div className="auth-stepper" aria-hidden="true">
                        <span className="auth-step-dot" data-active="true" />
                        <span className="auth-step-dot" data-active="true" />
                        <span className="auth-step-dot" data-active="true" />
                    </div>
                    <p className="auth-step-hint">These print on your invoices. Bank details are optional.</p>

                    <div className="auth-field">
                        <MapPin size={16} className="auth-field-icon" />
                        <input
                            className="auth-input"
                            type="text"
                            placeholder="Company address"
                            value={companyForm.address}
                            onChange={(e) => setCompanyForm({ ...companyForm, address: e.target.value })}
                        />
                    </div>
                    <div className="auth-field">
                        <Phone size={16} className="auth-field-icon" />
                        <input
                            className="auth-input"
                            type="text"
                            placeholder="Phone"
                            value={companyForm.phone}
                            onChange={(e) => setCompanyForm({ ...companyForm, phone: e.target.value })}
                        />
                    </div>
                    <div className="auth-field">
                        <Hash size={16} className="auth-field-icon" />
                        <input
                            className="auth-input"
                            type="text"
                            placeholder="GST number"
                            value={companyForm.gst}
                            onChange={(e) => setCompanyForm({ ...companyForm, gst: e.target.value.toUpperCase() })}
                        />
                    </div>

                    <p className="auth-section-label">
                        Bank details <span className="auth-optional">optional</span>
                    </p>
                    <div className="auth-field">
                        <Home size={16} className="auth-field-icon" />
                        <input
                            className="auth-input"
                            type="text"
                            placeholder="Bank name"
                            value={companyForm.bank_name}
                            onChange={(e) => setCompanyForm({ ...companyForm, bank_name: e.target.value })}
                        />
                    </div>
                    <div className="auth-field">
                        <CreditCard size={16} className="auth-field-icon" />
                        <input
                            className="auth-input"
                            type="text"
                            placeholder="Account number"
                            value={companyForm.account_no}
                            onChange={(e) => setCompanyForm({ ...companyForm, account_no: e.target.value })}
                        />
                    </div>
                    <div className="auth-field">
                        <Hash size={16} className="auth-field-icon" />
                        <input
                            className="auth-input"
                            type="text"
                            placeholder="IFSC"
                            value={companyForm.ifsc}
                            onChange={(e) => setCompanyForm({ ...companyForm, ifsc: e.target.value.toUpperCase() })}
                        />
                    </div>

                    <div className="auth-step-actions">
                        <button
                            type="button"
                            className="shell-btn shell-btn-secondary"
                            onClick={() => setRegisterStep(2)}
                            disabled={busy}
                        >
                            Back
                        </button>
                        <button type="submit" className="shell-btn shell-btn-primary auth-submit" disabled={busy}>
                            {busy ? "Setting up…" : "Finish"}
                        </button>
                    </div>
                </form>
            )}

            <div className="auth-switch" hidden={mode === "REGISTER" && registerStep > 1}>
                {mode === "LOGIN" ? (
                    <span>
                        No account?{" "}
                        <button type="button" className="auth-switch-link" onClick={() => switchMode("REGISTER")}>
                            Create one
                        </button>
                    </span>
                ) : (
                    <span>
                        Already have an account?{" "}
                        <button type="button" className="auth-switch-link" onClick={() => switchMode("LOGIN")}>
                            Log in
                        </button>
                    </span>
                )}
            </div>
        </div>
    );
};

export default AuthForm;
