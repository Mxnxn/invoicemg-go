// Restarts the `.shell-attention-pulse` CSS animation (shell.css) on an element even if it's
// already mid-animation, by removing the class, forcing a reflow, then re-adding it - then
// scrolls it into view. Used to draw the eye back to a form after a row's "Edit" action
// populates it, since the row click and the form live in different parts of the layout.
export function triggerFormAttention(el) {
    if (!el) return;
    el.classList.remove("shell-attention-pulse");
    // eslint-disable-next-line no-void
    void el.offsetWidth;
    el.classList.add("shell-attention-pulse");
    el.scrollIntoView({ behavior: "smooth", block: "center" });
}
