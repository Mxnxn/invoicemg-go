import { useEffect, useState } from "react";
import { bankBackend } from "./bank_backend";

// One bank list shared by every picker on the page.
//
// Each picker used to fetch its own copy on mount and keep it in local state, so a bank
// created inline on Bank Transfers > Received existed only inside that card. The three tabs
// are three different components, so "create a bank on Received, switch to Paid" showed a
// list that did not have it until something happened to remount and refetch.
//
// Module-level rather than a context: switching company is a full page reload (see
// Shell/CompanySwitcher.jsx), so this cache cannot outlive the company it was fetched for.

let banks = null;
let inFlight = null;
const listeners = new Set();

const byName = (a, b) => String(a.name || "").localeCompare(String(b.name || ""));

const emit = () => listeners.forEach((listener) => listener(banks));

function load() {
    if (banks) return Promise.resolve(banks);
    // Share one request between pickers that mount together, rather than firing three.
    if (!inFlight) {
        inFlight = bankBackend
            .list()
            .then((res) => {
                banks = (res.data || []).slice().sort(byName);
                emit();
                return banks;
            })
            .catch(() => {
                // The global interceptor toasts; an empty list keeps the picker usable.
                banks = [];
                emit();
                return banks;
            })
            .finally(() => {
                inFlight = null;
            });
    }
    return inFlight;
}

// Call after creating a bank so every mounted picker sees it immediately - no refetch, and
// no waiting for a remount.
export const addBank = (bank) => {
    if (!bank) return;
    banks = [...(banks || []), bank].sort(byName);
    emit();
};

// Re-fetch after a rename or removal - addBank only covers additions, and a stale name in
// every picker is exactly the drift this store exists to prevent.
export function refreshBanks() {
    banks = null;
    inFlight = null;
    return load();
}

export function useBanks() {
    const [list, setList] = useState(banks || []);
    useEffect(() => {
        listeners.add(setList);
        load().then(setList);
        return () => listeners.delete(setList);
    }, []);
    return list;
}
