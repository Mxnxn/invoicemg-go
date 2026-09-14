const ONES = [
    "",
    "One",
    "Two",
    "Three",
    "Four",
    "Five",
    "Six",
    "Seven",
    "Eight",
    "Nine",
    "Ten",
    "Eleven",
    "Twelve",
    "Thirteen",
    "Fourteen",
    "Fifteen",
    "Sixteen",
    "Seventeen",
    "Eighteen",
    "Nineteen",
];
const TENS = ["", "", "Twenty", "Thirty", "Forty", "Fifty", "Sixty", "Seventy", "Eighty", "Ninety"];

const twoDigits = (n) => (n < 20 ? ONES[n] : TENS[Math.floor(n / 10)] + (n % 10 ? " " + ONES[n % 10] : ""));
const threeDigits = (n) => {
    const hundred = Math.floor(n / 100);
    const rest = n % 100;
    return (hundred ? ONES[hundred] + " Hundred" + (rest ? " " : "") : "") + (rest ? twoDigits(rest) : "");
};

// Indian numbering (crore/lakh/thousand), used for the "Amount Chargeable (in words)" /
// "Tax Amount (in words)" lines on the GST-detailed invoice template.
export const numberToWordsIndian = (num) => {
    num = Math.round(Math.abs(Number(num) || 0));
    if (num === 0) return "Zero";
    const crore = Math.floor(num / 10000000);
    num %= 10000000;
    const lakh = Math.floor(num / 100000);
    num %= 100000;
    const thousand = Math.floor(num / 1000);
    num %= 1000;
    const hundred = num;
    const parts = [];
    if (crore) parts.push(threeDigits(crore) + " Crore");
    if (lakh) parts.push(threeDigits(lakh) + " Lakh");
    if (thousand) parts.push(threeDigits(thousand) + " Thousand");
    if (hundred) parts.push(threeDigits(hundred));
    return parts.join(" ").trim();
};

export const amountInWords = (amount) => `${numberToWordsIndian(amount)} Rupees Only`;
