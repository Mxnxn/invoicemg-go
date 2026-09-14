# Route inventory

**6 of 209 verified.**

Every route the Node API declares, generated from its source by
`invoice-mg-api/Docs/routeInventory.js` - not hand-listed, so it cannot drift from reality.

A route is ticked only when its answer has been diffed byte-for-byte against Node with
`scripts/parity.sh` for a real session. "Implemented" is not "done".

## Lifecycle (26)

- [ ] `POST /lifecycle/history/list`
- [ ] `POST /lifecycle/jobs/alert`
- [ ] `POST /lifecycle/jobs/assign`
- [ ] `POST /lifecycle/jobs/by-entry`
- [ ] `POST /lifecycle/jobs/convert-to-entries`
- [ ] `POST /lifecycle/jobs/create`
- [ ] `POST /lifecycle/jobs/delete`
- [ ] `POST /lifecycle/jobs/list`
- [ ] `POST /lifecycle/jobs/next-challan-number`
- [ ] `POST /lifecycle/jobs/progress`
- [ ] `POST /lifecycle/jobs/queue`
- [ ] `POST /lifecycle/jobs/queue/default`
- [ ] `POST /lifecycle/jobs/rows/assign`
- [ ] `POST /lifecycle/jobs/rows/complete-all`
- [ ] `POST /lifecycle/jobs/rows/progress`
- [ ] `POST /lifecycle/jobs/rows/queue`
- [ ] `POST /lifecycle/jobs/rows/queue-order`
- [ ] `POST /lifecycle/jobs/set-queue`
- [ ] `POST /lifecycle/jobs/unlock`
- [ ] `POST /lifecycle/jobs/update`
- [ ] `POST /lifecycle/lookups/clients`
- [ ] `POST /lifecycle/lookups/materials`
- [ ] `POST /lifecycle/lookups/people`
- [ ] `POST /lifecycle/notes/create`
- [ ] `POST /lifecycle/notes/list`
- [ ] `POST /lifecycle/notes/update`

## DevAdmin (16)

- [ ] `POST /dev/admin/company-limit`
- [ ] `POST /dev/admin/export`
- [ ] `POST /dev/admin/footprint`
- [ ] `POST /dev/admin/restore`
- [ ] `POST /dev/admin/status`
- [ ] `POST /dev/admin/wipe`
- [ ] `POST /dev/admins`
- [ ] `POST /dev/conversations`
- [ ] `POST /dev/conversations/messages`
- [ ] `POST /dev/conversations/send`
- [ ] `POST /dev/enquiries`
- [ ] `POST /dev/enquiries/handled`
- [ ] `POST /dev/password-requests`
- [ ] `POST /dev/password-requests/resolve`
- [ ] `POST /dev/registration-token/create`
- [ ] `POST /dev/registration-token/list`

## Analytics (14)

- [ ] `POST /analytics/aging`
- [ ] `POST /analytics/avg-payment-time`
- [ ] `POST /analytics/avg-pending-time`
- [ ] `POST /analytics/cashflow`
- [ ] `POST /analytics/payables`
- [ ] `POST /analytics/payout-weekday`
- [ ] `POST /analytics/production/throughput`
- [ ] `POST /analytics/production/wip`
- [ ] `POST /analytics/revenue`
- [ ] `POST /analytics/reviews`
- [ ] `POST /analytics/top-credits`
- [ ] `POST /analytics/top-paid`
- [ ] `POST /analytics/top-sales`
- [ ] `POST /analytics/unbilled`

## PurchaseOrder (14)

- [ ] `POST /purchase-order/approvals/dismiss`
- [ ] `POST /purchase-order/approve`
- [ ] `POST /purchase-order/confirm`
- [ ] `POST /purchase-order/convert`
- [ ] `POST /purchase-order/create`
- [ ] `POST /purchase-order/detail`
- [ ] `POST /purchase-order/list`
- [ ] `POST /purchase-order/next-number`
- [ ] `POST /purchase-order/note`
- [ ] `POST /purchase-order/note/edit`
- [ ] `POST /purchase-order/pending-approvals`
- [ ] `POST /purchase-order/revoke`
- [ ] `POST /purchase-order/share`
- [ ] `POST /purchase-order/update`

## Invoice (13)

- [ ] `GET /invoice/download/:fname`
- [ ] `POST /invoice/entries-jobs`
- [ ] `GET /invoice/export/:cid/:uid`
- [ ] `POST /invoice/get`
- [ ] `POST /invoice/getAll`
- [ ] `POST /invoice/getClientInvoices`
- [ ] `POST /invoice/getReceived`
- [ ] `POST /invoice/history`
- [ ] `POST /invoice/invoiceable-jobs`
- [ ] `POST /invoice/next-invoice-number`
- [ ] `POST /invoice/paid`
- [ ] `POST /invoice/remove`
- [ ] `POST /invoice/save`

## Client (11)

- [ ] `POST /client/add`
- [ ] `POST /client/batchReceiveDelete`
- [ ] `POST /client/batchReceiveUpdate`
- [ ] `POST /client/batchUpdate`
- [ ] `POST /client/get`
- [ ] `POST /client/getall`
- [ ] `POST /client/notify-preference`
- [ ] `GET /client/notify-preferences`
- [ ] `POST /client/only`
- [ ] `POST /client/remove`
- [ ] `POST /client/update`

## User (10)

- [ ] `POST /user/login`
- [ ] `POST /user/logout`
- [ ] `POST /user/password/change`
- [ ] `POST /user/password/request-reset`
- [ ] `POST /user/register`
- [ ] `POST /user/totp/disable`
- [ ] `POST /user/totp/enable`
- [ ] `POST /user/totp/reveal`
- [ ] `POST /user/totp/setup`
- [ ] `POST /user/totp/status`

## Company (9)

- [ ] `POST /company/active`
- [ ] `POST /company/create`
- [ ] `POST /company/deactivate`
- [ ] `POST /company/list`
- [ ] `POST /company/numbering`
- [ ] `POST /company/numbering/update`
- [ ] `POST /company/sharing`
- [ ] `POST /company/switch`
- [ ] `POST /company/update`

## Quotation (8)

- [ ] `POST /quotation/create`
- [ ] `POST /quotation/delete`
- [ ] `POST /quotation/get`
- [ ] `POST /quotation/list`
- [ ] `POST /quotation/next-quotation-number`
- [ ] `POST /quotation/row/add-to-job`
- [ ] `POST /quotation/row/delete`
- [ ] `POST /quotation/update`

## Material (6)

- [ ] `POST /material/add`
- [ ] `POST /material/get`
- [ ] `POST /material/getall`
- [ ] `POST /material/remove`
- [ ] `POST /material/set-unit`
- [ ] `POST /material/update`

## Person (6)

- [ ] `POST /person/create`
- [ ] `POST /person/delete`
- [ ] `POST /person/list`
- [ ] `POST /person/login`
- [ ] `POST /person/notify-preference`
- [ ] `POST /person/update`

## Bank (5)

- [ ] `POST /bank/create`
- [ ] `POST /bank/list`
- [ ] `POST /bank/remove`
- [ ] `POST /bank/report`
- [ ] `POST /bank/update`

## PurchaseInvoice (5)

- [ ] `POST /purchase-invoice/create`
- [ ] `POST /purchase-invoice/delete`
- [ ] `POST /purchase-invoice/list`
- [ ] `POST /purchase-invoice/next-number`
- [ ] `POST /purchase-invoice/update`

## UserInfo (5)

- [ ] `POST /userinfo/add`
- [ ] `POST /userinfo/get`
- [ ] `POST /userinfo/set-template`
- [ ] `POST /userinfo/update`
- [ ] `POST /userinfo/upload`

## BatchReceive (4)

- [ ] `POST /batch-receive/create`
- [ ] `POST /batch-receive/delete`
- [ ] `POST /batch-receive/list`
- [ ] `POST /batch-receive/lookups/open-jobs`

## Settings (4)

- [ ] `POST /settings/appearance`
- [ ] `POST /settings/appearance/update`
- [ ] `POST /settings/tables`
- [ ] `POST /settings/tables/update`

## Statistics (4)

- [ ] `GET /stats/clients`
- [ ] `GET /stats/download/:fname`
- [ ] `GET /stats/exports/:uid/:mode`
- [ ] `POST /stats/get`

## SupplierPayment (4)

- [ ] `POST /supplier-payment/create`
- [ ] `POST /supplier-payment/delete`
- [ ] `POST /supplier-payment/list`
- [ ] `POST /supplier-payment/lookups/open-invoices`

## Unit (4)

- [x] `POST /unit/create`
- [x] `POST /unit/delete`
- [x] `POST /unit/list`
- [x] `POST /unit/update`

## Wastage (4)

- [ ] `POST /wastage/add`
- [ ] `POST /wastage/getall`
- [ ] `POST /wastage/materials`
- [ ] `POST /wastage/remove`

## WhatsApp (4)

- [ ] `POST /whatsapp/config`
- [ ] `POST /whatsapp/config/update`
- [ ] `POST /whatsapp/send`
- [ ] `POST /whatsapp/templates`

## Alert (3)

- [ ] `GET /alert/:job_id/job/:jobcard_id`
- [ ] `GET /alert/:job_id/job/:jobcard_id/review`
- [ ] `POST /alert/:job_id/job/:jobcard_id/review`

## Docs (3)

- [ ] `GET /docs`
- [ ] `GET /docs/coverage.json`
- [ ] `GET /docs/openapi.json`

## Expense (3)

- [ ] `POST /expense/create`
- [ ] `POST /expense/list`
- [ ] `POST /expense/remove`

## Ledger (3)

- [ ] `POST /ledger/client`
- [ ] `POST /ledger/dues`
- [ ] `POST /ledger/dues/remind`

## Shared (3)

- [ ] `GET /shared/customers`
- [ ] `GET /shared/duplicate-materials`
- [ ] `GET /shared/materials`

## Sheet (3)

- [ ] `POST /sheet/get`
- [x] `POST /sheet/only`
- [x] `POST /sheet/open-jobs`

## Challan (2)

- [ ] `POST /challan/getAll`
- [ ] `POST /challan/new`

## PurchaseReport (2)

- [ ] `POST /purchase-report/dues`
- [ ] `POST /purchase-report/supplier`

## Sharing (2)

- [ ] `POST /sharing/preview`
- [ ] `POST /sharing/set`

## Trash (2)

- [ ] `POST /trash/get`
- [ ] `POST /trash/setPassword`

## WhatsAppWebhook (2)

- [ ] `GET /whatsapp/webhook`
- [ ] `POST /whatsapp/webhook`

## DevToken (1)

- [ ] `GET /dev/registration-token`

## Enquiry (1)

- [ ] `POST /enquiry`

## GstReport (1)

- [ ] `POST /gst-report`

## Inventory (1)

- [ ] `POST /inventory/report`

## PurchaseOrderPublic (1)

- [ ] `GET /po-public/:supplier_id/:po_id`

