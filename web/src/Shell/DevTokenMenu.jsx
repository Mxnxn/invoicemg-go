import { useCallback, useEffect, useRef, useState } from "react";
import { Key, Copy, Loader } from "react-feather";

import DateField from "../Common/DateField";
import { devBackend } from "../Views/Dev/dev_backend";
import { copyText } from "../Common/clipboard";
import { notifySuccess, notifyError } from "../global/toast";

/** The API takes yyyymmdd; DateField speaks ISO. "" stays "" so the default TTL applies. */
export function toApiDate(iso) {
    return String(iso || "").replace(/-/g, "");
}

/** Redeemable means unused and unexpired - the same rule Model/RegistrationToken enforces. */
export function newestRedeemable(tokens, now = Date.now()) {
    return (
        (tokens || [])
            .filter((t) => t && t.token && !t.usedAt && new Date(t.expiresAt).getTime() > now)
            // The list arrives newest first, but do not depend on that.
            .sort((a, b) => new Date(b.expiresAt) - new Date(a.expiresAt))[0] || null
    );
}

/**
 * The registration token, reachable from anywhere in the dev panel rather than only from
 * the bottom of one page.
 *
 * Opening shows the token that is currently live; it does not mint one. A token is
 * single-use, so issuing a fresh one every time someone glanced at this would leave a
 * trail of tokens nobody redeemed.
 */
export default function DevTokenMenu() {
    const [open, setOpen] = useState(false);
    const [token, setToken] = useState(null);
    const [loading, setLoading] = useState(false);
    const [minting, setMinting] = useState(false);
    // Blank means "use the API's 24-hour default" rather than a date we invented here.
    const [expiry, setExpiry] = useState("");
    const wrapRef = useRef(null);

    const load = useCallback(() => {
        setLoading(true);
        return devBackend
            .listRegistrationTokens()
            .then((res) => setToken(newestRedeemable(res.data)))
            .catch(() => {})
            .finally(() => setLoading(false));
    }, []);

    useEffect(() => {
        if (open) load();
    }, [open, load]);

    // A popover that only closes via its own button is a popover people leave open.
    useEffect(() => {
        if (!open) return undefined;
        const onDown = (event) => {
            if (wrapRef.current && !wrapRef.current.contains(event.target)) setOpen(false);
        };
        const onKey = (event) => {
            if (event.key === "Escape") setOpen(false);
        };
        document.addEventListener("mousedown", onDown);
        document.addEventListener("keydown", onKey);
        return () => {
            document.removeEventListener("mousedown", onDown);
            document.removeEventListener("keydown", onKey);
        };
    }, [open]);

    async function mint() {
        setMinting(true);
        try {
            const res = await devBackend.createRegistrationToken(toApiDate(expiry));
            if (res.data) setToken(res.data);
        } catch (error) {
            notifyError(error.message || "Could not issue a token.");
        } finally {
            setMinting(false);
        }
    }

    async function copy() {
        // Awaited, and the toast follows the RESULT - announcing "Copied." regardless is
        // worse than a button that plainly fails.
        const ok = await copyText(token.token);
        if (ok) notifySuccess("Copied.");
        else notifyError("Couldn't copy - select the token and copy it manually.");
    }

    return (
        <div className="dev-token-menu" ref={wrapRef}>
            <button
                type="button"
                className="shell-btn shell-btn-secondary dev-token-trigger"
                aria-haspopup="dialog"
                aria-expanded={open}
                onClick={() => setOpen((v) => !v)}
            >
                <Key size={14} aria-hidden="true" />
                Token
            </button>

            {open && (
                <div className="dev-token-pop" role="dialog" aria-label="Registration token">
                    <p className="dev-token-pop-title">Registration token</p>
                    <p className="dev-muted dev-token-pop-note">
                        Single use. Pick an expiry below, or leave it blank for 24 hours. No authenticator code needed — this session is already the gate.
                    </p>

                    <label className="dev-token-pop-label" htmlFor="dev-token-expiry">
                        Expires on <span className="dev-muted">(blank = 24 hours)</span>
                    </label>
                    <DateField
                        name="dev-token-expiry"
                        value={expiry}
                        onChange={(e) => setExpiry(e.target.value)}
                    />

                    {loading ? (
                        <p className="dev-muted">Loading…</p>
                    ) : token ? (
                        <>
                            <code className="dev-token-pop-value">{token.token}</code>
                            <p className="dev-muted dev-token-pop-expiry">
                                expires {new Date(token.expiresAt).toLocaleString()}
                            </p>
                            <div className="dev-token-pop-actions">
                                <button type="button" className="shell-btn shell-btn-secondary" onClick={copy}>
                                    <Copy size={14} aria-hidden="true" /> Copy
                                </button>
                                <button type="button" className="shell-btn" onClick={mint} disabled={minting}>
                                    {minting ? <Loader size={14} aria-hidden="true" /> : null}
                                    {minting ? "Issuing" : "New token"}
                                </button>
                            </div>
                        </>
                    ) : (
                        <>
                            <p className="dev-muted">No live token right now.</p>
                            <button type="button" className="shell-btn" onClick={mint} disabled={minting}>
                                {minting ? <Loader size={14} aria-hidden="true" /> : <Key size={14} aria-hidden="true" />}
                                {minting ? "Issuing" : "Generate token"}
                            </button>
                        </>
                    )}
                </div>
            )}
        </div>
    );
}
