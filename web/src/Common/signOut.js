import axios from "axios";

// Everything login writes (see Views/Auth/components/AuthForm). Listed once so a key added
// at sign-in cannot be forgotten at sign-out - which is exactly how `role` and `permissions`
// came to survive a logout in AppNavbar, leaving a signed-out browser still claiming to be
// an admin or a superadmin.
export const SESSION_KEYS = ["uid", "session_token", "email", "role", "permissions"];

// Where a signed-out person lands. Not "/" - that is the public marketing page now, so
// logging out dropped you on a brochure with no sign of having signed out. Any protected
// path renders AuthLayout once `uid` is gone (Views/Auth/ProtectiveRoute), so /admin IS the
// login screen for someone with no session.
export const LOGIN_PATH = "/admin";

export function clearSession() {
    SESSION_KEYS.forEach((key) => {
        try {
            window.localStorage.removeItem(key);
        } catch (error) {
            // Private mode, or storage disabled. Nothing to remove and nothing to do about
            // it - the redirect below still ends the session for this tab.
        }
    });
}

/**
 * Sign out.
 *
 * THE LOCAL SESSION ALWAYS ENDS. Telling the server is best-effort and deliberately cannot
 * block it: the previous version awaited the request with no try/catch and only cleared
 * storage on `code === 200`, so
 *
 *   - a dropped network threw, nothing was caught, and the button did nothing at all
 *   - an already-expired session answered 401, which hit the `else` branch and alert()ed,
 *     leaving the user unable to log out precisely when they most needed to
 *
 * A logout a network error can refuse is not a logout. The request is still worth making -
 * it deletes the UserSession server-side so the token cannot be replayed - but its outcome
 * only affects that, never whether this browser forgets who it was.
 */
export async function signOut({ redirect = true } = {}) {
    const token = (() => {
        try {
            return window.localStorage.getItem("session_token");
        } catch (error) {
            return null;
        }
    })();

    if (token) {
        try {
            await axios.post(
                `${import.meta.env.VITE_API_URL}/user/logout`,
                {},
                { headers: { "SESSION-TOKEN": token } }
            );
        } catch (error) {
            // Already gone, or unreachable. Either way this browser is signing out.
        }
    }

    clearSession();

    if (redirect) {
        // assign, not reload: reload keeps you on the admin path you were on, which renders
        // the login form but leaves a deep URL in the bar that behaves oddly on back.
        window.location.assign(LOGIN_PATH);
    }
}

export default signOut;
