import React, { useCallback, useEffect, useRef, useState } from "react";
import { Send, MessageSquare, User, Loader } from "react-feather";

import { devBackend } from "../dev_backend";
import { notifyError } from "../../../global/toast";

const POLL_MS = 20000;

/** "2m", "3h", "5d" - a list needs the age, not the date. */
export function relativeTime(value, now = Date.now()) {
    if (!value) return "";
    const seconds = Math.max(0, Math.round((now - new Date(value).getTime()) / 1000));
    if (seconds < 60) return "now";
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
    if (seconds < 86400) return `${Math.floor(seconds / 3600)}h`;
    return `${Math.floor(seconds / 86400)}d`;
}

/**
 * The dev-admin WhatsApp inbox.
 *
 * Its own component rather than another card in DevIndex, which is already long. Polls
 * instead of subscribing: the app has no socket layer, and this is an internal tool a
 * handful of people have open at a time.
 */
export default function InboxPanel() {
    const [conversations, setConversations] = useState([]);
    const [activeId, setActiveId] = useState(null);
    const [messages, setMessages] = useState([]);
    const [draft, setDraft] = useState("");
    const [sending, setSending] = useState(false);
    const [loading, setLoading] = useState(true);
    const threadRef = useRef(null);

    const loadConversations = useCallback(
        () =>
            devBackend
                .listConversations()
                .then((res) => setConversations(res.data || []))
                .catch(() => {})
                .finally(() => setLoading(false)),
        []
    );

    const loadMessages = useCallback((conversationId) => {
        if (!conversationId) return Promise.resolve();
        return devBackend
            .listMessages(conversationId)
            .then((res) => setMessages(res.data || []))
            .catch(() => {});
    }, []);

    useEffect(() => {
        loadConversations();
        const id = setInterval(loadConversations, POLL_MS);
        return () => clearInterval(id);
    }, [loadConversations]);

    useEffect(() => {
        if (!activeId) return undefined;
        loadMessages(activeId);
        const id = setInterval(() => loadMessages(activeId), POLL_MS);
        return () => clearInterval(id);
    }, [activeId, loadMessages]);

    // A thread reads from the bottom, like every other messaging view.
    useEffect(() => {
        const el = threadRef.current;
        if (el) el.scrollTop = el.scrollHeight;
    }, [messages]);

    const active = conversations.find((c) => c._id === activeId) || null;

    async function onSend(event) {
        event.preventDefault();
        const body = draft.trim();
        if (!body || sending || !active) return;
        setSending(true);
        try {
            const res = await devBackend.sendMessage(active._id, body);
            if (res.code === 200) {
                setDraft("");
                await Promise.all([loadMessages(active._id), loadConversations()]);
            } else if (res.message) {
                notifyError(res.message);
            }
        } catch (error) {
            notifyError("Could not send that message.");
        } finally {
            setSending(false);
        }
    }

    return (
        <div className="dev-inbox">
            <div className="dev-inbox-list">
                {loading ? (
                    <p className="dev-muted dev-inbox-empty">Loading…</p>
                ) : conversations.length === 0 ? (
                    <p className="dev-muted dev-inbox-empty">
                        Nothing yet. Conversations appear here when someone messages the business WhatsApp number —
                        there is no history from before this was switched on.
                    </p>
                ) : (
                    conversations.map((c) => (
                        <button
                            key={c._id}
                            type="button"
                            className={["dev-inbox-row", c._id === activeId ? "is-active" : ""].filter(Boolean).join(" ")}
                            onClick={() => setActiveId(c._id)}
                        >
                            <span className="dev-inbox-row-top">
                                <span className="dev-inbox-name">{c.profileName || c.phone}</span>
                                <span className="dev-muted dev-inbox-time">{relativeTime(c.lastMessageAt)}</span>
                            </span>
                            <span className="dev-inbox-row-top">
                                <span className="cell-mono dev-inbox-phone">{c.phone}</span>
                                {/* The distinction this view exists to draw. */}
                                <span className={["dev-inbox-chip", c.connected ? "is-customer" : ""].filter(Boolean).join(" ")}>
                                    {c.connected ? c.clientFirm || "Customer" : "Not a customer"}
                                </span>
                            </span>
                            <span className="dev-inbox-preview dev-muted">
                                {c.lastDirection === "out" ? "You: " : ""}
                                {c.lastMessagePreview}
                            </span>
                            {c.unread > 0 && <span className="dev-badge-count dev-inbox-unread">{c.unread}</span>}
                        </button>
                    ))
                )}
            </div>

            <div className="dev-inbox-thread-wrap">
                {!active ? (
                    <p className="dev-muted dev-inbox-empty">
                        <MessageSquare size={16} aria-hidden="true" /> Pick a conversation.
                    </p>
                ) : (
                    <>
                        <div className="dev-inbox-head">
                            <span className="dev-inbox-head-icon" aria-hidden="true">
                                <User size={16} />
                            </span>
                            <span>
                                <span className="dev-inbox-name">{active.profileName || active.phone}</span>
                                <span className="dev-muted dev-inbox-head-sub">
                                    <span className="cell-mono">{active.phone}</span>
                                    {active.connected ? ` · ${active.clientFirm || "Customer"}` : " · Not a customer"}
                                    {active.company ? ` · ${active.company}` : ""}
                                </span>
                            </span>
                        </div>

                        <div className="dev-inbox-thread" ref={threadRef}>
                            {messages.map((m) => (
                                <div
                                    key={m._id}
                                    className={["dev-inbox-bubble", m.direction === "out" ? "is-out" : "is-in"].join(" ")}
                                >
                                    <span className="dev-inbox-bubble-body">{m.body}</span>
                                    <span className="dev-inbox-bubble-meta">
                                        {new Date(m.createdAt).toLocaleString()}
                                        {m.direction === "out" && m.status ? ` · ${m.status}` : ""}
                                    </span>
                                </div>
                            ))}
                        </div>

                        {active.windowOpen ? (
                            <form className="dev-inbox-composer" onSubmit={onSend}>
                                <input
                                    value={draft}
                                    onChange={(e) => setDraft(e.target.value)}
                                    placeholder="Write a reply…"
                                    aria-label="Reply"
                                    disabled={sending}
                                />
                                <button type="submit" className="shell-btn" disabled={sending || !draft.trim()}>
                                    {sending ? <Loader size={14} aria-hidden="true" /> : <Send size={14} aria-hidden="true" />}
                                    {sending ? "Sending" : "Send"}
                                </button>
                            </form>
                        ) : (
                            // Not a disabled-looking box that silently fails: WhatsApp forbids
                            // this send, so say which rule applies rather than let it be tried.
                            <p className="dev-muted dev-inbox-closed">
                                This conversation is older than 24 hours. WhatsApp only allows an approved template
                                now, which this inbox does not send yet.
                            </p>
                        )}
                    </>
                )}
            </div>
        </div>
    );
}
