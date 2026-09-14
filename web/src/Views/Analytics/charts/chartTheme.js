// The categorical palette lives in tokens.css so both themes resolve without any
// JavaScript running on a theme change - the custom property simply changes value.
//
// The eight hues were validated with the dataviz skill's validate_palette.js against this
// app's own surfaces (#ffffff light, #161616 dark). Do not substitute one without
// re-running it: the checks cover colourblind separation, a normal-vision floor and
// contrast, none of which survive being eyeballed.

export const SERIES_SLOTS = 8;

/** The CSS custom property for a series slot, wrapping at the palette's end. */
export function seriesVar(index) {
    return `var(--series-${(Number(index) % SERIES_SLOTS) + 1})`;
}

/**
 * Keep the top `max - 1` items and fold the rest into one "Other" row.
 *
 * A ninth series never gets a generated hue: the palette is eight validated slots, and
 * inventing a ninth breaks both the colourblind guarantees and the rule that a colour
 * identifies an entity rather than its rank.
 */
export function foldSeries(items, max = SERIES_SLOTS) {
    const list = Array.isArray(items) ? items : [];
    if (list.length <= max) return { shown: list, other: null };

    const shown = list.slice(0, max - 1);
    const rest = list.slice(max - 1);
    const value = rest.reduce((sum, item) => sum + (Number(item.value) || 0), 0);
    return { shown, other: { name: "Other", value } };
}
