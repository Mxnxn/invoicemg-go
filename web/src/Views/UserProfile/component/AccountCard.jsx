import React, { useEffect, useState } from "react";
import { userBackend } from "../user_backend";
import { expiryLevel, expiryLabel } from "../../../Common/accountExpiry";
import { initials } from "../../dashboardCards";
import "../account.css";

const LEVEL_STYLE = {
    ok: { color: "var(--xan-emerald)", background: "var(--xan-emerald-bg)" },
    warn: { color: "var(--xan-amber)", background: "var(--xan-amber-bg)" },
    urgent: { color: "var(--text-danger)", background: "var(--status-red-bg-subtle)" },
    expired: { color: "var(--text-danger)", background: "var(--status-red-bg-subtle)" },
};

const fmt = (iso) => {
    if (!iso) return "";
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return "";
    return d.toLocaleDateString(undefined, { day: "2-digit", month: "short", year: "numeric" });
};

/**
 * Who you are signed in as, across the top of the settings page.
 *
 * It used to be a card, sitting in a column beside Appearance as if "your name" and "table row
 * height" were comparable things. They are not: this is the answer to "whose settings am I
 * looking at", which every section below it needs and none of them repeat. So it is a strip,
 * once, above the lot.
 *
 * activeUntil is what actually locks a login out (the API's Helpers/Tenancy.js), but it only
 * existed in the superadmin Dev panel - the person it affects could not see it, and found out
 * by failing to sign in one morning. Inside a month it changes colour so it is noticed before
 * then rather than after.
 *
 * --text-danger, not --status-red-text: the latter is half of a chip pair, legible only on
 * --status-red-bg and 2.6:1 read on a dark surface.
 */
const AccountCard = () => {
    const [profile, setProfile] = useState(null);

    useEffect(() => {
        const formData = new FormData();
        formData.set("uid", window.localStorage.getItem("uid"));
        userBackend
            .getUserInfo(formData, window.localStorage.getItem("session_token"))
            .then((res) => setProfile(res.data || {}))
            .catch(() => {});
    }, []);

    const level = profile ? expiryLevel(profile.activeUntil) : null;
    const style = level ? LEVEL_STYLE[level] : null;
    const pressing = level === "warn" || level === "urgent" || level === "expired";

    return (
        <header className="account-identity">
            <span className="account-identity-who">
                <span className="account-monogram" aria-hidden="true">
                    {initials(profile?.name || profile?.email || "?")}
                </span>
                <span className="account-identity-names">
                    <span className="account-identity-name">{profile?.name || "Account"}</span>
                    <span className="account-identity-email">{profile?.email || ""}</span>
                </span>
            </span>

            <span className="account-identity-meta">
                {profile?.role && <span className="xan-status-badge status-neutral">{profile.role}</span>}
                <span className="account-identity-expiry">
                    <span className="account-identity-label">Active until</span>
                    <span className="account-identity-value">
                        {profile?.activeUntil ? fmt(profile.activeUntil) : "No expiry set"}
                    </span>
                </span>
                {style && (
                    // Spelled out for a screen reader, which cannot see the colour that carries
                    // the urgency.
                    <span className="xan-status-badge" style={style} title={expiryLabel(profile.activeUntil)}>
                        {expiryLabel(profile.activeUntil)}
                    </span>
                )}
            </span>

            {pressing && (
                <p className="account-identity-note text-body-small">
                    Contact your administrator to extend it — the account stops signing in on this date.
                </p>
            )}
        </header>
    );
};

export default AccountCard;
