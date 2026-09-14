import React, { useState, useEffect, useCallback } from "react";
import ClientComponents from "./ClientComponents";

import { userBackend } from "../../UserProfile/user_backend";
import Loader from "../../../global/Loader/Loader";
import { clientsBackend } from "../client_backend";
import ClientMenuPanel from "./ClientMenuPanel";

const PreClientComponents = ({ uid }) => {
      const [loading, setLoading] = useState(false);
      const [user, setUser] = useState({});
      const [client_id] = useState(window.location.pathname.split("/")[2]);
      const [open, setOpen] = useState({ menuPanel: false });
      const stoken = window.localStorage.getItem("session_token");

      const getUserDetail = useCallback(async () => {
            try {
                  const formData = new FormData();
                  formData.set("uid", uid);
                  const res = await userBackend.getUserInfo(formData, stoken);
                  setUser(res.data);
            } catch (error) {
                  console.log(error);
            }
      }, [uid, stoken]);

      const [customer, setCustomer] = useState({});

      const getClientDetail = useCallback(async () => {
            try {
                  const formData = new FormData();
                  formData.set("client_id", client_id);
                  const res = await clientsBackend.getClientDetail(formData, stoken);
                  const customer = res.data;
                  // adding a flag `checked` to entries for invoicing feature
                  const modifiedEntries = customer.entries.map((el) => {
                        return { ...el, checked: false };
                  });
                  // newest first
                  modifiedEntries.sort((a, b) => new Date(b.date) - new Date(a.date));
                  customer.entries = modifiedEntries;
                  setCustomer(customer);
                  setLoading(true);
            } catch (error) {
                  console.log(error);
            }
      }, [client_id, stoken]);

      useEffect(() => {
            getUserDetail();
            getClientDetail();
      }, [getUserDetail, getClientDetail]);

      // data-shell, like AdminLayout and AuthLayout. Everything the Appearance preferences set -
      // the chosen font family, the text scales, the accent, the scoped resets - is applied
      // under [data-shell] in tokens.css, so a page rendered outside one silently falls back to
      // Argon's own body font and fixed sizes. This page is routed OUTSIDE ProtectiveRoute for
      // customer-facing links, which is how it came to be the one screen without it.
      return loading ? (
            <div className="rltive" data-shell>
                  {open.menuPanel && (
                        <>
                              <ClientMenuPanel
                                    onClose={() => setOpen((prev) => ({ ...prev, menuPanel: false }))}
                                    cname={customer.clientFirm}
                                    uid={uid}
                                    clientId={client_id}
                                    batchRecieved={customer.batchUpdates}
                                    user={user}
                                    customer={customer}
                                    setCustomer={setCustomer}
                              />
                        </>
                  )}
                  <ClientComponents user={user} customer={customer} setOpen={setOpen} setCustomer={setCustomer} cid={client_id} uid={uid} />
            </div>
      ) : (
            <Loader />
      );
};

export default PreClientComponents;
