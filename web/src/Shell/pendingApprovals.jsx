import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";

import { hasAccess } from "../Common/access";
import { purchaseOrderBackend } from "../Views/PurchaseOrder/purchaseOrder_backend";

// The approval queue, for anything in the shell that needs it.
//
// Not Redux: that store persists to localStorage for cached lookups, and a live count must
// not survive a session. Not props from AdminLayout: that threads a refresh callback through
// route elements with no interest in it.
//
// The default value matters - suites that render AppNavbar without the provider get a quiet
// zero rather than a crash.
const PendingApprovalsContext = createContext({
    count: 0,
    totalPending: 0,
    items: [],
    refresh: () => {},
    dismiss: () => {},
});

export const usePendingApprovals = () => useContext(PendingApprovalsContext);

export function PendingApprovalsProvider({ children }) {
    const [state, setState] = useState({ count: 0, totalPending: 0, items: [] });

    const refresh = useCallback(() => {
        // The endpoint is gated by the same key the bell is. Asking anyway would just be a
        // 403 per page load for every employee who cannot approve.
        if (!hasAccess("purchase_orders_approve")) return;
        const stoken = window.localStorage.getItem("session_token");
        purchaseOrderBackend
            .pendingApprovals(new FormData(), stoken)
            .then((res) =>
                setState({
                    count: res.data?.count || 0,
                    // What is still awaiting approval whether or not it was waved away. The
                    // panel needs both: "nothing new" and "nothing to do" are different facts.
                    totalPending: res.data?.totalPending || 0,
                    items: res.data?.items || [],
                })
            )
            // Deliberately silent. A failed count must not raise a toast on every page load;
            // the badge simply stays as it was, which is the least alarming wrong answer.
            .catch(() => {});
    }, []);

    // Mark one order as read, or everything currently showing.
    //
    // Refreshes from the server afterwards rather than dropping the row locally: the reply
    // carries the true counts, and an order that changed between the render and the click is
    // not dismissed at all - the route only ever dismisses what it can still see.
    const dismiss = useCallback(
        (poId) => {
            const stoken = window.localStorage.getItem("session_token");
            const formData = new FormData();
            if (poId) formData.set("po_id", poId);
            else formData.set("all", "true");
            return purchaseOrderBackend
                .dismissApproval(formData, stoken)
                .then(refresh)
                // The interceptor has already said what went wrong; the list simply stays as
                // it is, which is the least alarming wrong answer here too.
                .catch(() => {});
        },
        [refresh]
    );

    useEffect(() => {
        refresh();
    }, [refresh]);

    const value = useMemo(() => ({ ...state, refresh, dismiss }), [state, refresh, dismiss]);

    return <PendingApprovalsContext.Provider value={value}>{children}</PendingApprovalsContext.Provider>;
}
