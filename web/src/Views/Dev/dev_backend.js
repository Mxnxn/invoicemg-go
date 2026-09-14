import axios from "axios";

// Developer-team endpoints. Every call is gated server-side by requireSuperAdmin - the
// sidebar hiding this section is convenience, not security.
const getHeader = () => ({
    headers: {
        "SESSION-TOKEN": window.localStorage.getItem("session_token"),
    },
});

const post = (path, formData) =>
    new Promise(async (resolve, reject) => {
        try {
            const res = await axios.post(`${import.meta.env.VITE_API_URL}/dev${path}`, formData, getHeader());
            if (res.data.code !== 200) throw res.data;
            resolve(res.data);
        } catch (error) {
            reject(error);
        }
    });

const withUid = (uid) => {
    const fd = new FormData();
    fd.set("uid", uid);
    return fd;
};

class DevBackend {
    listAdmins() {
        return post("/admins", new FormData());
    }

    // How much a wipe would move, so the dialog can state it before anything happens.
    footprint(uid) {
        return post("/admin/footprint", withUid(uid));
    }

    // How many companies an admin may own. A grant, not a lifecycle switch, so it has its own
    // endpoint rather than riding along with setStatus.
    setCompanyLimit(uid, companyLimit) {
        const fd = new FormData();
        fd.set("uid", uid);
        fd.set("companyLimit", String(companyLimit));
        return post("/admin/company-limit", fd);
    }

    setStatus(uid, { is_active, activeUntil }) {
        const fd = withUid(uid);
        if (typeof is_active !== "undefined") fd.set("is_active", String(is_active));
        if (typeof activeUntil !== "undefined") fd.set("activeUntil", activeUntil || "");
        return post("/admin/status", fd);
    }

    // confirmEmail is verified server-side too - the typed confirmation is not just a
    // client-side speed bump.
    wipe(uid, confirmEmail) {
        const fd = withUid(uid);
        fd.set("confirmEmail", confirmEmail);
        return post("/admin/wipe", fd);
    }

    restore(uid, batchId) {
        const fd = withUid(uid);
        fd.set("batchId", batchId);
        return post("/admin/restore", fd);
    }

    exportData(uid) {
        return post("/admin/export", withUid(uid));
    }

    // `expiry` is yyyymmdd, or blank for the API's 24-hour default.
    createRegistrationToken(expiry = "") {
        const fd = new FormData();
        if (expiry) fd.set("expiry", expiry);
        return post("/registration-token/create", fd);
    }

    listRegistrationTokens() {
        return post("/registration-token/list", new FormData());
    }

    listPasswordRequests() {
        return post("/password-requests", new FormData());
    }

    listConversations() {
        return post("/conversations", new FormData());
    }

    listMessages(conversationId) {
        const fd = new FormData();
        fd.set("conversation_id", conversationId);
        return post("/conversations/messages", fd);
    }

    sendMessage(conversationId, body) {
        const fd = new FormData();
        fd.set("conversation_id", conversationId);
        fd.set("body", body);
        return post("/conversations/send", fd);
    }

    listEnquiries() {
        return post("/enquiries", new FormData());
    }

    setEnquiryHandled(enquiryId, handled) {
        const fd = new FormData();
        fd.set("enquiry_id", enquiryId);
        fd.set("handled", String(handled));
        return post("/enquiries/handled", fd);
    }

    resolvePasswordRequest(requestId, action) {
        const fd = new FormData();
        fd.set("request_id", requestId);
        if (action) fd.set("action", action);
        return post("/password-requests/resolve", fd);
    }
}

export const devBackend = new DevBackend();
