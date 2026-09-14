import { RoundOffWithAmount } from "../../Common/DateAndTime/RoundOff";
import { dimensionFactor } from "../../Common/rowPricing";
import { rowNetAmount, taxLines } from "./invoiceFormat";

// Everything the totals block of an invoice states, computed once.
//
// This used to live inside InvoiceTableFooter, which meant each template that did not use
// that component carried its own copy of the arithmetic - and one of those copies taxed
// everything at a hardcoded 18%. With six designs sharing three renderers, a seventh copy
// was not an option. Pure and exported so the arithmetic is testable without rendering a PDF.
//
// Every factor is coerced the way rowNetAmount coerces it: a service line with a blank
// length or width once turned the whole subtotal into NaN, and "NaN" printed on a customer's
// invoice.
export const invoiceTotals = (entries, invoice) => {
    const items = Array.isArray(entries) ? entries : [];

    const subtotal = items.reduce(
        (sum, item) => sum + (Number(item.qty) || 0) * (Number(item.rate) || 0) * dimensionFactor(item, 1),
        0
    );
    const discount = items.reduce((sum, item) => sum + (Number(item.discount) || 0), 0);
    const charges = items.reduce((sum, item) => sum + (Number(item.charges) || 0), 0);
    const net = items.reduce((sum, item) => sum + rowNetAmount(item), 0);

    const taxRows = taxLines(items);
    const tax = taxRows.reduce((sum, line) => sum + line.amount, 0);

    const preRounding = net + tax;
    const [rounded, roundOff] = RoundOffWithAmount(preRounding);
    const grandTotal = Number(rounded) || 0;

    const received = Number(invoice?.receivedAmount) || 0;

    return {
        subtotal,
        discount,
        charges,
        net,
        taxRows,
        tax,
        roundOff,
        grandTotal,
        received,
        balance: grandTotal - received,
        showDiscount: discount > 0,
        showCharges: charges > 0,
        showBalance: received > 0,
    };
};
