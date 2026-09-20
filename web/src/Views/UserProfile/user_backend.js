import axios from "axios";
class UserBackend {
    getUserInfo(formData, stoken) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/userinfo/get`, formData, {
                    headers: {
                        "SESSION-TOKEN": stoken,
                    },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }
    addUserInfo(formData, stoken) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/userinfo/add`, formData, {
                    headers: {
                        "SESSION-TOKEN": stoken,
                    },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    // `docType` is "invoice" | "quotation" | "ledger", or "all" to set the same design on
    // every one of them in a single write. `documentFont` is company-wide, so it can be sent
    // on its own with no docType and no template at all.
    setTemplate(docType, template, stoken, documentFont, documentScale) {
        return new Promise(async (resolve, reject) => {
            try {
                const formData = new FormData();
                if (docType) formData.set("docType", docType);
                if (template) formData.set("template", template);
                if (documentFont) formData.set("documentFont", documentFont);
                // Company-wide like the font: a size-only save carries neither docType nor
                // template, which is why the route treats all three as independently optional.
                if (documentScale) formData.set("documentScale", documentScale);
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/userinfo/set-template`, formData, {
                    headers: {
                        "SESSION-TOKEN": stoken,
                    },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    // The invoice Units / Size display toggles, persisted through the same /set-template write.
    // Their own methods because the value is a boolean that can legitimately be false, and the
    // route reads presence (not truthiness) to tell a deliberate off from a field never sent.
    setShowUnits(on, stoken) {
        return this.#setToggle("documentShowUnits", on, stoken);
    }

    setShowSize(on, stoken) {
        return this.#setToggle("documentShowSize", on, stoken);
    }

    #setToggle(field, on, stoken) {
        return new Promise(async (resolve, reject) => {
            try {
                const formData = new FormData();
                formData.set(field, on ? "true" : "false");
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/userinfo/set-template`, formData, {
                    headers: { "SESSION-TOKEN": stoken },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    addUserImage(formData, setProgressCount, stoken) {
        return new Promise(async (resolve, reject) => {
            try {
                // onUploadProgress was passed as a FOURTH argument. axios.post takes three,
                // so it was silently dropped and the progress bar never moved - it belongs in
                // the config object alongside the headers.
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/userinfo/upload`, formData, {
                    headers: {
                        "SESSION-TOKEN": stoken,
                    },
                    onUploadProgress: (progressEvent) => {
                        if (typeof setProgressCount !== "function") return;
                        // total is absent when the server does not send a length; report
                        // indeterminate rather than dividing by undefined and showing NaN%.
                        const total = progressEvent.total || 0;
                        setProgressCount(total ? Math.round((progressEvent.loaded / total) * 100) : -1);
                    },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    totpStatus() {
        return this.#post("/user/totp/status");
    }

    totpSetup() {
        return this.#post("/user/totp/setup");
    }

    totpEnable(code) {
        return this.#post("/user/totp/enable", code);
    }

    totpDisable(code) {
        return this.#post("/user/totp/disable", code);
    }

    // The key that is already enrolled, so a second phone can be added to the same account.
    // Takes the account password, not a code - see routes/User.js for why.
    totpReveal(password) {
        return new Promise(async (resolve, reject) => {
            try {
                const formData = new FormData();
                formData.set("password", password);
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/user/totp/reveal`, formData, {
                    headers: { "SESSION-TOKEN": window.localStorage.getItem("session_token") },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    // Ask the team to reset this account's password, for someone who is signed in but cannot
    // remember it - so cannot use changePassword, which requires the current one.
    //
    // The same endpoint the sign-in screen uses. It answers identically whether or not the
    // address exists, to stop it being used to find out who has an account; here we already
    // know it does, so the caller reports a definite message rather than relaying that one.
    requestPasswordReset(email) {
        return new Promise(async (resolve, reject) => {
            try {
                const formData = new FormData();
                formData.set("email", email);
                const res = await axios.post(
                    `${import.meta.env.VITE_API_URL}/user/password/request-reset`,
                    formData
                );
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    // One shape for all four: session token in the header, optional code in the body.
    #post(path, code) {
        return new Promise(async (resolve, reject) => {
            try {
                const formData = new FormData();
                if (code) formData.set("code", code);
                const res = await axios.post(`${import.meta.env.VITE_API_URL}${path}`, formData, {
                    headers: { "SESSION-TOKEN": window.localStorage.getItem("session_token") },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }

    changePassword(currentPassword, newPassword) {
        return new Promise(async (resolve, reject) => {
            try {
                const formData = new FormData();
                formData.set("currentPassword", currentPassword);
                formData.set("newPassword", newPassword);
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/user/password/change`, formData, {
                    headers: { "SESSION-TOKEN": window.localStorage.getItem("session_token") },
                });
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }
}

export let userBackend = new UserBackend();
