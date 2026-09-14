// Toast API, callable from anywhere - including module-level code like the axios interceptor,
// which runs outside React and cannot use a hook or a context.
//
// A plain subscriber list rather than a ref to a provider's enqueue function: messages raised
// before the provider mounts (an interceptor firing on the very first request) are held and
// delivered once it does, instead of being dropped the way the old setSnackbarRef was.

const listeners = new Set();
// Raised before anyone was listening. Drained on the first subscribe.
let pending = [];
let nextId = 0;

export function subscribeToasts(listener) {
    listeners.add(listener);
    if (pending.length > 0) {
        const queued = pending;
        pending = [];
        queued.forEach((toast) => listener(toast));
    }
    return () => listeners.delete(listener);
}

// `options` is optional so every existing caller - notifyError("Saved") and friends - keeps
// working unchanged.
//   code        a status code worth showing (422, 500, a network failure's own status)
//   description a second line: which call, usually
//   duration    override the default; the provider decides when this is absent
function push(variant, message, options = {}) {
    const toast = {
        id: `${Date.now()}-${nextId++}`,
        variant,
        message: message || fallbackFor(variant),
        code: options.code,
        description: options.description,
        duration: options.duration,
    };
    if (listeners.size === 0) {
        pending.push(toast);
        return;
    }
    listeners.forEach((listener) => listener(toast));
}

const fallbackFor = (variant) => {
    if (variant === "error") return "Something went wrong";
    if (variant === "warning") return "Heads up";
    if (variant === "success") return "Done";
    return "";
};

export function notifyError(message, options) {
    push("error", message, options);
}

export function notifySuccess(message, options) {
    push("success", message, options);
}

// For a change that is allowed but has consequences worth stating - editing a job-id that is
// already billed on an invoice, say. Not an error: the action went through.
export function notifyWarning(message, options) {
    push("warning", message, options);
}

export function notifyInfo(message, options) {
    push("info", message, options);
}

// Test seam: drops any queued messages and every listener.
export function resetToasts() {
    listeners.clear();
    pending = [];
}
