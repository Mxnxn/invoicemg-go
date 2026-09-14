// A blank/undefined/unparseable amount reads as 0.00, never the string "NaN".
//
// These results are rendered directly into table cells and PDF totals, so one missing
// field printed "₹NaN" on a document going to a customer. A missing figure is 0.
//
// Note the API has its own copy of this pair in Helpers/RoundOff.js, with the same guard -
// and note that RoundOffWithAmount below returns a [total, roundOff] PAIR here while the
// API's returns a single Number. Same name, different shape; do not port code between them
// without checking which one you are holding.
const money = (amt) => {
    const n = Number(amt);
    return Number.isFinite(n) ? n : 0;
};

export const RoundOff = (amt) => {
    return (Math.round(money(amt) * 100) / 100).toFixed(2);
};

export const RoundOffWithAmount = (amt) => {
    const rounded = (Math.round(money(amt) * 100) / 100).toFixed(2);
    const decimal = rounded.split(".")[1];
    if (parseInt(decimal) > 50) {
        return [`${parseInt(rounded.split(".")[0]) + 1}.00`, parseInt("0." + decimal)];
    }
    return [`${parseInt(rounded.split(".")[0])}.00`, "0." + parseInt(decimal)];
};
