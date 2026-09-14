import React, { useCallback, useEffect, useRef, useState } from "react";
import { AlertTriangle, CheckCircle, Info, X, XCircle } from "react-feather";
import { subscribeToasts } from "./toast";
import "./toast.css";

// The app's own toasts, replacing notistack.
//
// notistack v1 is a Material-UI v4 component, and MUI was otherwise removed from this project
// (see CLAUDE.md). It pulled the whole @material-ui/core runtime back in for one widget, it
// could not be styled past its own DOM without !important on every rule, and it had nowhere to
// put the two things that actually help when something fails: what the server said, and which
// call said it.
//
// What this adds over that:
//   - a status code, shown as a chip. "422" answers "did I send something wrong or is the
//     server down" immediately; "Something went wrong" never did.
//   - a second line for detail (the endpoint, usually), so the message stays one sentence.
//   - a depleting track along the bottom edge, the same idea as the undo bar - the toast
//     stops vanishing without warning, and the remaining time is visible rather than felt.
//   - hover to hold. Reading a message should not be a race.

// At least 5s. Errors get longer: they are the ones worth reading twice, and they usually
// arrive while attention is elsewhere.
const DEFAULT_MS = 5000;
const ERROR_MS = 8000;

// Beyond this the stack is taller than it is useful; the oldest drops out. A burst of errors
// from one failed save should not bury the page.
const MAX_VISIBLE = 4;

const ICONS = {
    success: CheckCircle,
    error: XCircle,
    warning: AlertTriangle,
    info: Info,
};

const Toast = ({ toast, onDismiss }) => {
    const Icon = ICONS[toast.variant] || Info;
    const [paused, setPaused] = useState(false);
    const timerRef = useRef(null);
    const startedRef = useRef(0);
    const leftRef = useRef(toast.duration);

    // The timer is restarted rather than paused-in-place, because setTimeout has no pause.
    // Tracking what is left keeps hover from granting a full fresh window every time the
    // pointer crosses the toast.
    useEffect(() => {
        if (paused) {
            clearTimeout(timerRef.current);
            leftRef.current = Math.max(0, leftRef.current - (Date.now() - startedRef.current));
            return undefined;
        }
        startedRef.current = Date.now();
        timerRef.current = setTimeout(() => onDismiss(toast.id), leftRef.current);
        return () => clearTimeout(timerRef.current);
    }, [paused, toast.id, onDismiss]);

    return (
        <div
            className={`app-toast app-toast-${toast.variant}`}
            role={toast.variant === "error" ? "alert" : "status"}
            onMouseEnter={() => setPaused(true)}
            onMouseLeave={() => setPaused(false)}
        >
            <Icon size={16} className="app-toast-icon" aria-hidden="true" />
            <div className="app-toast-body">
                <div className="app-toast-head">
                    <span className="app-toast-message">{toast.message}</span>
                    {toast.code !== undefined && toast.code !== null && toast.code !== "" && (
                        <span className="app-toast-code" title="Status code">
                            {toast.code}
                        </span>
                    )}
                </div>
                {toast.description && <span className="app-toast-desc">{toast.description}</span>}
            </div>
            <button type="button" className="app-toast-close" aria-label="Dismiss" onClick={() => onDismiss(toast.id)}>
                <X size={14} strokeWidth={2.5} />
            </button>
            {/* animationPlayState, NOT a remount. Re-keying the element to restart the
                animation made the track snap back to full width the moment the pointer left,
                which reads as the timer resetting - the opposite of what hover-to-hold does.
                Pausing the animation in place leaves it exactly where the countdown is. */}
            <span
                className="app-toast-track"
                style={{
                    animationDuration: `${toast.duration}ms`,
                    animationPlayState: paused ? "paused" : "running",
                }}
                aria-hidden="true"
            />
        </div>
    );
};

const ToastProvider = () => {
    const [toasts, setToasts] = useState([]);

    const dismiss = useCallback((id) => {
        setToasts((prev) => prev.filter((t) => t.id !== id));
    }, []);

    useEffect(
        () =>
            subscribeToasts((toast) => {
                setToasts((prev) => {
                    const next = [
                        ...prev,
                        {
                            ...toast,
                            duration: toast.duration || (toast.variant === "error" ? ERROR_MS : DEFAULT_MS),
                        },
                    ];
                    return next.slice(-MAX_VISIBLE);
                });
            }),
        []
    );

    if (toasts.length === 0) return null;

    return (
        <div className="app-toast-stack" aria-live="polite" aria-relevant="additions">
            {toasts.map((toast) => (
                <Toast key={toast.id} toast={toast} onDismiss={dismiss} />
            ))}
        </div>
    );
};

export default ToastProvider;
