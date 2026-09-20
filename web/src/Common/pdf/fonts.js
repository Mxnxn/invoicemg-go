import { Font } from "@react-pdf/renderer";

import Lato400 from "./fonts/Lato-Regular.ttf";
import Lato700 from "./fonts/Lato-Bold.ttf";
import PTSerif400 from "./fonts/PTSerif-Regular.ttf";
import PTSerif700 from "./fonts/PTSerif-Bold.ttf";
import Poppins400 from "./fonts/Poppins-Regular.ttf";
import Poppins700 from "./fonts/Poppins-Bold.ttf";
import Roboto400 from "./fonts/Roboto-Regular.ttf";
import Roboto700 from "./fonts/Roboto-Bold.ttf";
import Josefin400 from "./fonts/JosefinSans-Regular.ttf";
import Josefin700 from "./fonts/JosefinSans-Bold.ttf";
import Geist400 from "./fonts/Geist-Regular.ttf";
import Geist700 from "./fonts/Geist-Bold.ttf";
import Archivo400 from "./fonts/Archivo-Regular.ttf";
import Archivo700 from "./fonts/Archivo-Bold.ttf";
import Fira400 from "./fonts/FiraSans-Regular.ttf";
import Fira700 from "./fonts/FiraSans-Bold.ttf";
import Pliant400 from "./fonts/Pliant-Regular.ttf";
import Pliant700 from "./fonts/Pliant-Bold.ttf";

// The families a company can set its documents in (Configure > Templates, one choice for
// invoices, quotations and ledgers alike - Company.documentFont).
//
// `rupee` is not decoration. react-pdf 1.x has no font-fallback chain: a codepoint the
// registered family does not carry renders as .notdef - an empty box - rather than borrowing
// the glyph from another font. A family without U+20B9 therefore cannot print "₹1,200" without
// a box, so only families that carry the sign are offered here at all: the four that did not
// (Open Sans, Open Sans Condensed, Sora, DM Mono) were removed, and every remaining family
// draws a true rupee. money() still branches on the flag defensively, but it now always
// resolves to "₹".
//
// Verified against each file's cmap, not assumed. Re-check with the cmap when adding a family,
// and only add one that carries U+20B9.
const font = (key, label, blurb, family, regular, bold, rupee) => ({
    key,
    label,
    blurb,
    family,
    regular,
    bold,
    rupee,
});

export const FONTS = [
    font("lato", "Lato", "Warm and highly legible, with a true rupee sign.", "Lato", Lato400, Lato700, true),
    font("pt-serif", "PT Serif", "A serif, for documents meant to read as formal.", "PTSerif", PTSerif400, PTSerif700, true),
    font("poppins", "Poppins", "Geometric and round - modern without being loud.", "Poppins", Poppins400, Poppins700, true),
    font("roboto", "Roboto", "Neutral workhorse. Reads the same at every size.", "Roboto", Roboto400, Roboto700, true),
    font("archivo", "Archivo", "Slightly condensed grotesque - good where columns are tight.", "Archivo", Archivo400, Archivo700, true),
    font("fira-sans", "Fira Sans", "Humanist, with unusually clear numerals.", "FiraSans", Fira400, Fira700, true),
    font("geist", "Geist", "Contemporary and tight-set, built for interfaces and tables.", "Geist", Geist400, Geist700, true),
    font("pliant", "Pliant", "Soft grotesque - a touch friendlier than Roboto.", "Pliant", Pliant400, Pliant700, true),
    font("josefin-sans", "Josefin Sans", "Light geometric with a vintage air. Best on short documents.", "JosefinSans", Josefin400, Josefin700, true),
];

export const FONT_KEYS = FONTS.map((f) => f.key);

// Lato is the new default now that Open Sans (no ₹ glyph) has been removed: it is the closest
// remaining family to it and carries a true rupee sign. A company still storing "open-sans" (or
// one of the other removed keys) resolves here through resolveFont's fallback, so no stored
// value breaks - it simply renders in Lato with a real ₹ instead of "Rs ".
export const DEFAULT_FONT_KEY = "lato";

export const resolveFont = (key) => FONTS.find((f) => f.key === key) || FONTS.find((f) => f.key === DEFAULT_FONT_KEY);

// Registers the chosen family under its own name and hands the font back, so a renderer can
// do `const font = registerDocumentFont(key)` and then use `font.family` in its styles.
// Font.register is idempotent per family in react-pdf, so calling this on every render is safe.
export const registerDocumentFont = (key) => {
    const chosen = resolveFont(key);
    Font.register({
        family: chosen.family,
        fonts: [
            { src: chosen.regular, fontWeight: 400 },
            { src: chosen.bold, fontWeight: 700 },
        ],
    });
    return chosen;
};

// Indian grouping: the last three digits, then pairs. 1234567 -> 12,34,567.
const groupIndian = (whole) => {
    if (whole.length <= 3) return whole;
    const last3 = whole.slice(-3);
    const rest = whole.slice(0, -3);
    return `${rest.replace(/\B(?=(\d{2})+(?!\d))/g, ",")},${last3}`;
};

// One place that decides how an amount is written on a document, so the three renderers
// cannot disagree - and so the rupee-glyph question above is answered once.
export const money = (amount, fontOrKey) => {
    const chosen = typeof fontOrKey === "string" || fontOrKey == null ? resolveFont(fontOrKey) : fontOrKey;
    const symbol = chosen.rupee ? "₹" : "Rs ";
    const value = Number(amount);
    const safe = Number.isFinite(value) ? value : 0;
    const [whole, decimals] = Math.abs(safe).toFixed(2).split(".");
    return `${safe < 0 ? "-" : ""}${symbol}${groupIndian(whole)}.${decimals}`;
};
