import { Font } from "@react-pdf/renderer";

import OSR400 from "../../Views/Invoice/font/OSR400.ttf";
import OS700 from "../../Views/Invoice/font/OS700.ttf";
import OSC400 from "../../Views/Invoice/font/OS400.ttf";
import Lato400 from "./fonts/Lato-Regular.ttf";
import Lato700 from "./fonts/Lato-Bold.ttf";
import PTSerif400 from "./fonts/PTSerif-Regular.ttf";
import PTSerif700 from "./fonts/PTSerif-Bold.ttf";
import Poppins400 from "./fonts/Poppins-Regular.ttf";
import Poppins700 from "./fonts/Poppins-Bold.ttf";
import Roboto400 from "./fonts/Roboto-Regular.ttf";
import Roboto700 from "./fonts/Roboto-Bold.ttf";
import Sora400 from "./fonts/Sora-Regular.ttf";
import Sora700 from "./fonts/Sora-Bold.ttf";
import Josefin400 from "./fonts/JosefinSans-Regular.ttf";
import Josefin700 from "./fonts/JosefinSans-Bold.ttf";
import Geist400 from "./fonts/Geist-Regular.ttf";
import Geist700 from "./fonts/Geist-Bold.ttf";
import DMMono400 from "./fonts/DMMono-Regular.ttf";
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
// the glyph from another font. Open Sans, which every document in this app has been set in
// since the beginning, has no U+20B9, so "₹1,200" has been printing as a box followed by the
// number on quotations and ledgers. money() below reads this flag and writes "Rs " for a
// family that cannot draw the sign, which is why the default can stay Open Sans without
// anybody's existing paperwork changing shape.
//
// Verified against each file's cmap, not assumed. Re-check with the cmap when adding a family.
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
    font("open-sans", "Open Sans", "The current default - plain, familiar, widely legible.", "OS", OSR400, OS700, false),
    font("lato", "Lato", "Slightly warmer than Open Sans, with a true rupee sign.", "Lato", Lato400, Lato700, true),
    font("pt-serif", "PT Serif", "A serif, for documents meant to read as formal.", "PTSerif", PTSerif400, PTSerif700, true),
    // One weight only - the condensed cut ships without a bold, so bold text is drawn in the
    // regular cut and differs by size and colour rather than by weight.
    font("open-sans-condensed", "Open Sans Condensed", "Narrow - fits more columns before a table has to wrap.", "OSC", OSC400, OSC400, false),
    // Every one of these was checked against its own cmap with fontkit, not assumed - the
    // `rupee` flag below is what money() reads to decide between "₹" and "Rs ", and getting it
    // wrong prints an empty box on a customer-facing document. Sora and DM Mono genuinely have
    // no U+20B9; the other seven do.
    font("poppins", "Poppins", "Geometric and round - modern without being loud.", "Poppins", Poppins400, Poppins700, true),
    font("roboto", "Roboto", "Neutral workhorse. Reads the same at every size.", "Roboto", Roboto400, Roboto700, true),
    font("archivo", "Archivo", "Slightly condensed grotesque - good where columns are tight.", "Archivo", Archivo400, Archivo700, true),
    font("fira-sans", "Fira Sans", "Humanist, with unusually clear numerals.", "FiraSans", Fira400, Fira700, true),
    font("geist", "Geist", "Contemporary and tight-set, built for interfaces and tables.", "Geist", Geist400, Geist700, true),
    font("pliant", "Pliant", "Soft grotesque - a touch friendlier than Roboto.", "Pliant", Pliant400, Pliant700, true),
    font("josefin-sans", "Josefin Sans", "Light geometric with a vintage air. Best on short documents.", "JosefinSans", Josefin400, Josefin700, true),
    font("sora", "Sora", "Technical and squared-off.", "Sora", Sora400, Sora700, false),
    // One weight only, like Open Sans Condensed above: DM Mono ships no bold on Google Fonts,
    // so bold text is drawn in the regular cut and differs by size and colour instead.
    font("dm-mono", "DM Mono", "Monospaced - every figure occupies the same width, so columns of numbers line up exactly.", "DMMono", DMMono400, DMMono400, false),
];

export const FONT_KEYS = FONTS.map((f) => f.key);

// Deliberately the family documents are already set in. Changing this line restyles every
// invoice, quotation and ledger of every company that has not explicitly chosen a font.
export const DEFAULT_FONT_KEY = "open-sans";

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
