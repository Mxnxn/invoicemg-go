// Where a floating menu should sit relative to its trigger.
//
// Written for the phone case, which the desktop layout hides. A picker's menu is
// position:fixed below its trigger. Tap one in the lower half of a phone screen and the
// on-screen keyboard covers the bottom ~40% of the viewport - so the menu renders behind the
// keyboard. It is open, it is focused, you can type into it, and you cannot see any of it.
// That reads as "the dropdown disappears when I tap search", which is how it was reported.
//
// So: measure against the VISUAL viewport (what the keyboard actually shrinks - on iOS the
// layout viewport does not change at all), flip above the trigger when the space below has
// gone, and shrink rather than overflow when neither side fits.

export const MENU_GAP = 6;

// Below this a menu is more frustrating than no menu - if neither side offers it, we take the
// roomier side and let the list scroll inside.
const MIN_USABLE_HEIGHT = 120;

export function placeMenu({ trigger, viewportHeight, viewportOffsetTop = 0, menuHeight, width }) {
    // The visible band, in the same coordinate space getBoundingClientRect reports in.
    const visibleTop = viewportOffsetTop;
    const visibleBottom = viewportOffsetTop + viewportHeight;

    const spaceBelow = visibleBottom - trigger.bottom - MENU_GAP;
    const spaceAbove = trigger.top - visibleTop - MENU_GAP;

    const fitsBelow = spaceBelow >= menuHeight;
    const fitsAbove = spaceAbove >= menuHeight;

    // Below is the default because it matches where the eye already is. Above only when below
    // genuinely cannot hold it and above can - or, failing both, whichever is roomier.
    const placement = fitsBelow ? "below" : fitsAbove ? "above" : spaceAbove > spaceBelow ? "above" : "below";

    const available = placement === "above" ? spaceAbove : spaceBelow;
    const height = Math.max(MIN_USABLE_HEIGHT, Math.min(menuHeight, available));

    const top = placement === "above" ? trigger.top - height - MENU_GAP : trigger.bottom + MENU_GAP;

    return {
        placement,
        // Never off the top edge, whichever way it went - a menu whose header is unreachable
        // cannot be closed by the person looking at it.
        top: Math.max(visibleTop, top),
        left: trigger.left,
        width: width ?? trigger.width,
        maxHeight: height,
    };
}
