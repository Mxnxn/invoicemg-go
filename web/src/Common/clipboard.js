// Copy text to the clipboard, and say honestly whether it worked.
//
// navigator.clipboard only exists in a SECURE CONTEXT. The droplet serves plain HTTP on
// port 80 (see docker-compose.prod.yml), so in production the whole API is undefined and
// calling it throws - while localhost, being treated as secure, works fine. That is how a
// copy button ships broken: it works everywhere it gets tested.
//
// The execCommand fallback is deprecated but is the only thing available over http, and it
// is what every "copy" button on a non-TLS page still runs on.
export async function copyText(text) {
    const value = String(text ?? "");
    if (!value) return false;

    if (navigator.clipboard && window.isSecureContext) {
        try {
            await navigator.clipboard.writeText(value);
            return true;
        } catch (error) {
            // Permission refused, or the document was not focused. Fall through and try the
            // older path rather than giving up on it.
        }
    }

    try {
        const area = document.createElement("textarea");
        area.value = value;
        // Off-screen rather than hidden: display:none and visibility:hidden are not
        // selectable, and execCommand("copy") copies the SELECTION.
        area.setAttribute("readonly", "");
        area.style.position = "fixed";
        area.style.top = "-1000px";
        area.style.opacity = "0";
        document.body.appendChild(area);
        area.select();
        area.setSelectionRange(0, value.length);
        const ok = document.execCommand("copy");
        document.body.removeChild(area);
        return ok;
    } catch (error) {
        return false;
    }
}
