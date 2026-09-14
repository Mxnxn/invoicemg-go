import axios from "axios";
import { notifyError, notifySuccess } from "../global/toast";
import { getTabId } from "./tabIdentity";
import { clearSession, LOGIN_PATH } from "../Common/signOut";
import { hasAccess } from "../Common/access";
import { PATH_FEATURES } from "../Common/features";

// 401 = the session itself is invalid/expired (TokenHelper) - nothing to do but log out.
// Shares clearSession with the Logout button so ONE list of keys serves both. The two lists
// had already drifted: the navbar's was missing `role` and `permissions`, so a signed-out
// browser still claimed to be an admin.
//
// It lands on the login screen rather than "/", which is the public marketing page now - an
// expired session used to dump you on a brochure with no sign of what had happened. No
// server call here: the session it would delete is the one that just answered 401.
function logOutAndRedirect() {
    clearSession();
    if (window.location.pathname !== LOGIN_PATH) window.location.assign(LOGIN_PATH);
}

// 403 = authenticated but lacking the feature's permission (requireFeature/requireAdmin) -
// send an employee to the first feature they DO have access to, rather than leaving them
// stuck on a page that will keep 403ing. Admin 403s are a misconfiguration, not something
// we can route around, so just leave the toast.
function redirectToAuthorizedFeature() {
    const role = window.localStorage.getItem("role") || "admin";
    if (role !== "employee") return;
    const fallback = PATH_FEATURES.find((el) => el.prefix.startsWith("/admin/") && hasAccess(el.key));
    const target = fallback ? fallback.prefix : "/admin/lifecycle";
    if (window.location.pathname !== target) window.location.href = target;
}

// Which calls are worth confirming out loud. This API uses POST for reads as well as writes
// (/client/get, /invoice/list ...), so "toast every POST" would fire on every page load.
// Matching the verb in the path is the only signal available, and it is a good one: the
// route names are consistent across all sixteen backends.
const MUTATION_PATH = /\/(add|create|new|update|edit|save|delete|remove|restore|convert|switch|deactivate|activate|set-|upload|wipe|reorder|assign|batch)/i;

// Reads that happen to match the pattern above but are not user-initiated writes.
const NEVER_ANNOUNCE = /\/(list|get|getAll|getall|lookups|active|footprint|admins)\b/i;

// The login endpoints, whose failures the form reports itself.
//
// A wrong password produces TWO failing requests, not one: AuthForm falls back from the
// admin login to the employee login, because an employee account is a different endpoint and
// the client cannot know which kind it is holding until one of them answers. Toasting both
// put two identical errors on screen for a single mistake - and the form already prints the
// message inline, under the field, where someone typing a password is actually looking.
const SILENT_ERROR_PATH = /\/(user|person)\/login(?:\?|$)/i;

export function isSilentError(url = "") {
    return Boolean(url) && SILENT_ERROR_PATH.test(url);
}

// Writes whose result is already on the screen.
//
// A toast exists to tell you something you could not otherwise see. Dragging a card from one
// column to another and being told "Job updated" says nothing the card did not already say by
// moving, and a toast that fires on every drag is a toast people learn to ignore - which costs
// us the failures too, since those use the same channel.
//
// Errors are NOT silenced here. A refused move must still say why.
const SILENT_SUCCESS_PATH = /\/lifecycle\/jobs\/rows\/queue(?:\?|$)/i;

export function isSilentSuccess(url = "") {
    return Boolean(url) && SILENT_SUCCESS_PATH.test(url);
}

// The endpoint, without the host or the query string - "/invoice/save", not the whole URL.
// Shown as the toast's second line so a failure says WHICH call failed. The host is noise (it
// is the same one every time) and a query string can carry ids that do not belong on screen.
export function endpointOf(url = "") {
    if (!url) return "";
    try {
        return new URL(url, window.location.origin).pathname;
    } catch (error) {
        return String(url).split("?")[0];
    }
}

export function isMutation(url = "") {
    if (!url) return false;
    if (NEVER_ANNOUNCE.test(url)) return false;
    return MUTATION_PATH.test(url);
}

// The session and permission consequences of a failure, in one place.
//
// They have to be, because the SAME failure arrives down two different paths depending on
// which API style the server is running:
//
//   legacy  HTTP 200 with {code: 401} in the body  -> axios RESOLVES -> onFulfilled
//   rest    HTTP 401                               -> axios REJECTS  -> onRejected
//
// The Go service can serve either (API_STYLE), and the Node API only serves the first, so the
// client has to understand both. Before this was shared, a 401 under rest style toasted and
// then did nothing: the dead session stayed in localStorage and the app kept making calls
// with it, looking broken rather than logged out.
function applySessionEffects(code) {
    if (code === 401) logOutAndRedirect();
    else if (code === 403) redirectToAuthorizedFeature();
}

export function onFulfilled(response) {
    const code = response?.data?.code;
    if (code !== undefined && code !== 200) {
        // Only the toast is suppressed - the session and permission side effects below still
        // run. A dead session left in place would be a worse bug than a missing message.
        if (!isSilentError(response?.config?.url)) {
            // This API returns its business errors as a `code` inside a 200 body, so the
            // envelope's code is the one worth showing - the HTTP status is 200 either way
            // and would tell the reader nothing.
            notifyError(response.data.message || "Something went wrong", {
                code,
                description: endpointOf(response?.config?.url),
            });
        }
        applySessionEffects(code);
        return response;
    }

    // Successful writes confirm themselves. Reads stay silent - a toast on every page load
    // would train the user to ignore toasts entirely, which costs us the failures too.
    if (code === 200 && isMutation(response?.config?.url) && !isSilentSuccess(response?.config?.url)) {
        const message = response?.data?.message;
        if (message) notifySuccess(message);
    }
    return response;
}

export function onRejected(error) {
    // Under rest style this is where every business failure lands, so the status has to be
    // read before the early return - a silenced login error must still not leave a dead
    // session in place.
    //
    // The body's `code` is preferred over the HTTP status because the server sends both and
    // they agree; the status is the fallback for a failure that never reached a handler at
    // all (a proxy 502, a dropped connection).
    const code = error?.response?.data?.code ?? error?.response?.status;
    applySessionEffects(code);

    if (isSilentError(error?.config?.url)) return Promise.reject(error);
    // When there is no response at all - the request never left - say so rather than showing
    // a blank chip: "Network" is the distinction between "the server refused" and "nothing
    // answered", which is the first thing worth knowing.
    notifyError(error?.response?.data?.message || error?.message || "Network error", {
        code: code || "Network",
        description: endpointOf(error?.config?.url),
    });
    return Promise.reject(error);
}

// Every request carries this tab's identity so the API can resolve which company this tab is
// acting as. Injected globally instead of in each of the sixteen *_backend.js header helpers.
export function attachTabId(config) {
    config.headers = config.headers || {};
    config.headers["TAB-ID"] = getTabId();
    return config;
}

axios.interceptors.request.use(attachTabId);
axios.interceptors.response.use(onFulfilled, onRejected);
