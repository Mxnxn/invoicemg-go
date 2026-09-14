import axios from "axios";

// One shape for every call, matching the other per-domain backends: SESSION-TOKEN per call,
// TAB-ID never set here (src/api/errorInterceptor.js attaches it globally), and no error
// toast - the global interceptor raises those.
const post = (path, formData, stoken) =>
    new Promise(async (resolve, reject) => {
        try {
            const res = await axios.post(`${import.meta.env.VITE_API_URL}/purchase-order/${path}`, formData, {
                headers: { "SESSION-TOKEN": stoken },
            });
            if (res.data.code !== 200) throw res.data;
            resolve(res.data);
        } catch (error) {
            reject(error);
        }
    });

class PurchaseOrderBackend {
    nextNumber(formData, stoken) { return post("next-number", formData, stoken); }
    list(formData, stoken) { return post("list", formData, stoken); }
    detail(formData, stoken) { return post("detail", formData, stoken); }
    create(formData, stoken) { return post("create", formData, stoken); }
    update(formData, stoken) { return post("update", formData, stoken); }
    approve(formData, stoken) { return post("approve", formData, stoken); }
    revoke(formData, stoken) { return post("revoke", formData, stoken); }
    addNote(formData, stoken) { return post("note", formData, stoken); }
    editNote(formData, stoken) { return post("note/edit", formData, stoken); }
    convert(formData, stoken) { return post("convert", formData, stoken); }
    pendingApprovals(formData, stoken) { return post("pending-approvals", formData, stoken); }
    dismissApproval(formData, stoken) { return post("approvals/dismiss", formData, stoken); }
    share(formData, stoken) { return post("share", formData, stoken); }
    confirm(formData, stoken) { return post("confirm", formData, stoken); }
}

export const purchaseOrderBackend = new PurchaseOrderBackend();
