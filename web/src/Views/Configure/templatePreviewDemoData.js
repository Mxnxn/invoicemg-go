// Realistic-but-fake data for the live template preview in Configure > Templates - covers
// two different tax slabs (18% and 12%) so a preview actually shows off each template's
// multi-rate tax breakdown, not just a single flat line.
export const DEMO_INVOICE = {
    date: new Date().toISOString().slice(0, 10),
    invoiceNumber: "DEMO/26-27/00001",
    firm: "Your Firm Name",
    address: "123 Business Street, Industrial Area",
    phone: "+91 98765 43210",
    email: "billing@yourfirm.com",
    gst: "22AAAAA0000A1Z5",
    url: null,
    clientFirm: "Sample Client Pvt. Ltd.",
    clientAddress: "456 Client Avenue, Business District",
    clientPhone: "+91 91234 56789",
    clientGST: "22BBBBB0000B1Z5",
    bank_name: "Sample Bank",
    account: "1234567890",
    ifsc: "SAMP0001234",
    total: 0,
    entries: [
        { material: "Flex Banner", description: "Polished, Grade A", hsn: "4901", qty: 2, length: 8, width: 4, rate: 120, cgst: 9, sgst: 9, igst: 0 },
        { material: "Vinyl Sticker", description: "Matte finish", hsn: "3919", qty: 10, length: 2, width: 2, rate: 60, cgst: 6, sgst: 6, igst: 0 },
        { material: "Foam Board", description: "Rough cut", hsn: "4823", qty: 5, length: 3, width: 3, rate: 45, cgst: 9, sgst: 9, igst: 0 },
    ],
};

export const DEMO_USER = {
    firm: DEMO_INVOICE.firm,
    address: DEMO_INVOICE.address,
    phone: DEMO_INVOICE.phone,
    email: DEMO_INVOICE.email,
    gst: DEMO_INVOICE.gst,
    url: null,
};

export const DEMO_QUOTATION = {
    date: DEMO_INVOICE.date,
    quotationNumber: "DEMO-QT-0001",
    client_id: {
        clientFirm: DEMO_INVOICE.clientFirm,
        clientAddress: DEMO_INVOICE.clientAddress,
        clientPhone: DEMO_INVOICE.clientPhone,
        clientGST: DEMO_INVOICE.clientGST,
    },
    rows: DEMO_INVOICE.entries,
};

export const DEMO_LEDGER = {
    clientFirm: DEMO_INVOICE.clientFirm,
    clientName: DEMO_INVOICE.clientFirm,
    openingBalance: 12500,
    currentBalance: 8340,
    rows: [
        { sr: 1, date: "2026-06-01", type: "Sales Invoice", invoiceNo: "DEMO/26-27/00001", bill: 15340, receipt: null, balance: 27840 },
        { sr: 2, date: "2026-06-18", type: "Receipts", invoiceNo: "", bill: null, receipt: 19500, balance: 8340 },
    ],
};
