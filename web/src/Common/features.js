// Feature keys mirror the backend's requireFeature(key) guards (see routes/*.js) and the
// nav items in Shell/SideMenu.jsx. This is the single list an admin can toggle per-Employee.
export const FEATURES = [
    { key: "analytics", label: "Analytics" },
    { key: "dashboard", label: "Home" },
    // The key stays "lifecycle" - it is persisted in Person.permissions in the database, so
    // renaming it would silently revoke every employee's access. Only the label changes.
    { key: "lifecycle", label: "Jobs" },
    { key: "invoices", label: "Invoices" },
    { key: "purchase_invoices", label: "Purchase Invoices" },
    { key: "purchase_orders", label: "Purchase Orders" },
    // Two capabilities rather than actions on purchase_orders, and that is not a style choice.
    // normalisePermissions expands a flat legacy key to EVERY action on its feature, so folding
    // approval in as an action would hand it to everyone already holding "purchase_orders" -
    // which is the whole set of people it exists to be separate from.
    //
    // `actions: ["view"]` because requireFeature() checks the view action and nothing else:
    // create and delete on a capability would be two cells that do nothing.
    { key: "purchase_orders_approve", label: "Approve purchase orders", actions: ["view"] },
    { key: "purchase_orders_send", label: "Send orders to suppliers", actions: ["view"] },
    { key: "quotations", label: "Quotations" },
    { key: "batch_receive", label: "Bank Transfers" },
    { key: "ledger", label: "Client Ledger" },
    { key: "gst_report", label: "GST Report" },
    { key: "whatsapp", label: "WhatsApp Messages" },
    { key: "challan", label: "Logs" },
    { key: "customers", label: "Customers" },
    { key: "products", label: "Products" },
    { key: "trash", label: "Trash" },
];

// Longest-prefix-first path -> feature key map, used by ProtectiveRoute to gate/redirect
// employee sessions. Includes the client-facing detail routes (/customer, /sheet, /invoice)
// alongside the /admin/* nav routes since they carry the same underlying data.
export const PATH_FEATURES = [
    { prefix: "/admin/analytics", key: "analytics" },
    { prefix: "/admin/dashboard", key: "dashboard" },
    { prefix: "/admin/lifecycle", key: "lifecycle" },
    { prefix: "/admin/invoices", key: "invoices" },
    { prefix: "/admin/purchase-invoices", key: "purchase_invoices" },
    { prefix: "/admin/purchase-orders", key: "purchase_orders" },
    { prefix: "/admin/quotations", key: "quotations" },
    { prefix: "/admin/bank-transfers", key: "batch_receive" },
    { prefix: "/admin/more/job-report", key: "lifecycle" },
    { prefix: "/admin/more/ledger", key: "ledger" },
    { prefix: "/admin/more/customer-dues", key: "ledger" },
    { prefix: "/admin/more/gst-report", key: "gst_report" },
    { prefix: "/admin/more/inventory", key: "products" },
    { prefix: "/admin/configure/across-company", key: "customers" },
    { prefix: "/admin/more/purchase-report", key: "purchase_invoices" },
    { prefix: "/admin/more/purchase-dues", key: "purchase_invoices" },
    { prefix: "/admin/configure/customers", key: "customers" },
    { prefix: "/admin/configure/products", key: "products" },
    { prefix: "/admin/configure/challan", key: "challan" },
    { prefix: "/admin/trash", key: "trash" },
    { prefix: "/customer", key: "customers" },
    { prefix: "/sheet", key: "customers" },
    { prefix: "/invoice", key: "invoices" },
];

// The five record managers consolidated under /admin/configure (see Views/Configure/ConfigureIndex.jsx).
// `key: null` (People, Suppliers) mirrors the admin-only nav items in SideMenu.jsx - hasAccess()
// treats a null key as "employees never get this", same as Accounts.
// `wide` marks the two or three cards worth a 2-column span in Configure's bento grid
// (ConfigureIndex.jsx) - purely a layout hint, ignored anywhere else this list is read.
export const CONFIGURE_SECTIONS = [
    { id: "customers", key: "customers", label: "Customers", accent: "blue", description: "Manage client records and contact details.", wide: true },
    { id: "products", key: "products", label: "Products", accent: "emerald", description: "Materials, rates, and HSN codes.", tall: true },
    { id: "people", key: null, label: "People", accent: "amber", description: "Employees and permissions." },
    { id: "suppliers", key: null, label: "Suppliers", accent: "rose", description: "Vendors for purchase invoices." },
    { id: "challan", key: "challan", label: "Logs", accent: "violet", description: "Wastage and challan records." },
    { id: "banks", key: "batch_receive", label: "Banks", accent: "emerald", description: "Accounts money moves through." },
    // Admin-only (key: null, same convention as People/Suppliers) - which document template
    // is used is a business-identity decision, not a per-employee permission.
    { id: "templates", key: null, label: "Templates and Numbering", accent: "blue", description: "How documents look, how they are numbered, and what exports carry.", wide: true },
    { id: "whatsapp", key: null, label: "WhatsApp", accent: "emerald", description: "Connect WhatsApp Business to message clients directly.", wide: true },
    // Admin-only (key: null), like Templates and Exports: this owns the cross-company sharing
    // switch, which decides what every other company can see. It lives in Configure rather
    // than More because it is a setting that changes data visibility, not a report.
    { id: "across-company", key: null, label: "Across Company", accent: "violet", description: "Share customers, products and inventory between the companies you own.", wide: true },
];

// Sections under /admin/more (see Views/More/MoreIndex.jsx) - same card-grid hub pattern as
// Configure. Batch Receive (now Bank Transfers) moved to its own top-level sidebar item.
export const MORE_SECTIONS = [
    // Behind the lifecycle permission - a report ON jobs, so anyone who can see the Jobs board.
    { id: "job-report", key: "lifecycle", label: "Job Report", accent: "violet", wide: true },
    { id: "ledger", key: "ledger", label: "Client Ledger", accent: "blue", wide: true },
    // Shares the `ledger` permission rather than adding a key of its own - it's the same
    // receivables data the Client Ledger already exposes, just every client at once.
    { id: "customer-dues", key: "ledger", label: "Customer Dues", accent: "rose", tall: true },
    { id: "gst-report", key: "gst_report", label: "GST Report", accent: "amber" },
    // Shares the `products` permission rather than adding a key of its own: this is a
    // figure per product, built from the rates and names Products already exposes, and a
    // new key would silently lock out every employee who already has that access.
    { id: "inventory", key: "products", label: "Inventory", accent: "emerald", wide: true },
    // Payables. Shares the `purchase_invoices` permission with the invoices themselves and
    // with recording payments - seeing what's owed is part of managing purchases.
    // Distinct from Purchase Report on purpose: that one is a ledger you read one supplier at
    // a time, this is the chase list - who needs paying, and how much - without picking anyone
    // first. Both read /purchase-report/dues, so the figures cannot drift apart.
    { id: "purchase-dues", key: "purchase_invoices", label: "Purchase Dues", accent: "amber", tall: true },
    { id: "purchase-report", key: "purchase_invoices", label: "Purchase Report", accent: "violet", wide: true },
    // Shares the batch_receive permission with Bank Transfers - both are the same "money
    // through our accounts" surface, and the expense form lives inside this report.
    { id: "bank-report", key: "batch_receive", label: "Bank Report", accent: "emerald", wide: true },
];
