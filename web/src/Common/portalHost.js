/**
 * Where a portalled surface should mount.
 *
 * Everything portalled used to go to document.body, which is OUTSIDE [data-shell] - and
 * [data-shell] is where tokens.css applies the Appearance preferences: the chosen font family,
 * the text scales, the accent, the scoped box-sizing reset. So a panel or a menu that portalled
 * to the body rendered in the browser's default font rather than the one the user picked. The
 * job board's card panel had been doing it since it was written; nobody noticed because the
 * panels were checked for geometry and for the values they wrote, never for what they looked
 * like next to the page behind them.
 *
 * The shell root carries no transform, filter or perspective - checked, because any of the
 * three would make it a containing block for `position: fixed` and every fixed overlay inside
 * it would start positioning against the div instead of the viewport.
 *
 * Falls back to the body when there is no shell, which is the pre-auth pages and every test:
 * the old behaviour, for the cases that never had the preferences to inherit in the first
 * place.
 */
export const portalHost = () =>
    (typeof document !== "undefined" && document.querySelector("[data-shell]")) || document.body;

export default portalHost;
