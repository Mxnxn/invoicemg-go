import React, { useEffect, useState, useRef, useCallback } from "react";
import LiteHeader from "../../../Common/Header/LiteHeader";
import { Briefcase, Shield, Sliders } from "react-feather";
import "../account.css";
import { Users } from "react-feather";

import { userBackend } from "../user_backend";

import { Row, Container, Col, Button } from "reactstrap";
import AppearanceSettings from "./AppearanceSettings";
import TwoFactorCard from "./TwoFactorCard";
import AccountCard from "./AccountCard";
import PasswordCard from "./PasswordCard";
import CompanyCarousel from "./CompanyCarousel";
import AddDetail from "./AddDetail";
import UploadModal from "./UploadModal";
import Loader from "../../../global/Loader/Loader";
import { can } from "../../../Common/access";

// What the page is about, in the order somebody needs it: the business first (it is what the
// paperwork carries), then the way in, then the look of it.
const SECTIONS = [
      { key: "company", label: "Company", hint: "Name, GST, bank, logo", Icon: Briefcase },
      { key: "security", label: "Security", hint: "Password and two-factor", Icon: Shield },
      { key: "appearance", label: "Appearance", hint: "Font, size, colour", Icon: Sliders },
];

const UserProfileIndex = (props) => {
      const inputRef = useRef(null);
      const qrInputRef = useRef(null);
      const [uid] = useState(window.localStorage.getItem("uid"));
      const [form, setForm] = useState({
            name: "",
            email: "",
            firm: "",
            phone: "",
            address: "",
            gst: "",
            bank_name: "",
            account_no: "",
            ifsc: "",
      });
      const isOwner = can(null);
      const [userPhoto, setUserPhoto] = useState(null);
      // The UPI QR is a second, independent image on the same Company. It reuses
      // UploadModal and /userinfo/upload, which now accepts either field on its own.
      const [qrPhoto, setQrPhoto] = useState(null);
      const [qrModal, setQrModal] = useState(false);
      const [error, setError] = useState(false);
      const [loading, setLoading] = useState(false);
      const [urlx, setUrl] = useState(false);

      const [progressVar, setProgressVar] = useState(0);
      const [modal, setModal] = useState(false);
      const [modalNew, setModalNew] = useState(false);
      const [section, setSection] = useState("company");

      const qrToggle = (e) => {
            setQrModal(!qrModal);
      };

      const handleQrPhoto = (e) => {
            setError(false);
            try {
                  const file = e.target.files[0];
                  const validFileType = ["image/jpg", "image/jpeg", "image/png"];
                  if (validFileType.includes(file["type"])) {
                        setQrPhoto(file);
                  } else {
                        setError(true);
                  }
            } catch (err) {
                  alert(err);
            }
      };

      const uploadQrImage = async () => {
            const formData = new FormData();
            formData.append("upiQr", qrPhoto);
            formData.set("uid", uid);
            try {
                  await userBackend.addUserImage(formData, setProgressVar, window.localStorage.getItem("session_token"));
                  setQrModal(false);
            } catch (error) {}
      };

      const toggle = (e) => {
            try {
                  setModal(!modal);
            } catch (err) {
                  setModal(!modal);
            }
      };

      const userInfo = useCallback(async () => {
            try {
                  const formData = new FormData();
                  formData.set("uid", uid);
                  const res = await userBackend.getUserInfo(formData, window.localStorage.getItem("session_token"));

                  setForm({
                        ...form,
                        name: res.data.name,
                        email: res.data.email,
                        firm: res.data.firm,
                        address: res.data.address,
                        phone: res.data.phone,
                        gst: res.data.gst,
                        bank_name: res.data.bank_name,
                        account_no: res.data.account,
                        ifsc: res.data.ifsc,
                  });
                  setUrl(`${import.meta.env.VITE_API_URL}/uploads/${res.data.url}`);
                  setLoading(true);
            } catch (error) {
                  console.log(error.message);
            }
            // eslint-disable-next-line react-hooks/exhaustive-deps
      }, [uid]);

      useEffect(() => {
            userInfo();
      }, [userInfo]);

      const onCancelHandler = () => {
            setModalNew(false);
            setModal(false);
      };

      const uploadImage = async () => {
            const formData = new FormData();
            if (userPhoto) formData.append("userDP", userPhoto);
            formData.set("uid", uid);
            try {
                  const res = await userBackend.addUserImage(
                        formData,
                        setProgressVar,
                        window.localStorage.getItem("session_token")
                  );
                  console.log(res.data.url);
                  setUrl(`${import.meta.env.VITE_API_URL}/uploads/${res.data.url}`);
                  setModal(false);
            } catch (error) {}
      };

      const handlePhoto = (e) => {
            setError(false);
            try {
                  const file = e.target.files[0];
                  const validFileType = ["image/jpg", "image/jpeg", "image/png"];
                  if (validFileType.includes(file["type"])) {
                        setUserPhoto(file);
                  } else {
                        setError(true);
                  }
            } catch (err) {
                  alert(err);
            }
      };

      const detailSubmit = async () => {
            try {
                  const formData = new FormData();
                  for (let key in form) {
                        if (form[key] === "") {
                              return setError(`Invalid ${form[key]}.`);
                        }
                        formData.set(key, form[key]);
                  }
                  formData.set("uid", uid);
                  const res = await userBackend.addUserInfo(formData, window.localStorage.getItem("session_token"));
                  console.log(res.data);
                  setForm({
                        ...form,
                        name: res.data.name,
                        email: res.data.email,
                        firm: res.data.firm,
                        address: res.data.address,
                        phone: res.data.phone,
                        gst: res.data.gst,
                        bank_name: res.data.bank_name,
                        account_no: res.data.account_no,
                        ifsc: res.data.ifsc,
                  });
                  setModalNew(false);
            } catch (error) {
                  console.log(error);
                  setModalNew(false);
            }
      };

      const details = [
            { label: "Address", value: form.address },
            { label: "Phone", value: form.phone },
            { label: "GST", value: form.gst },
            { label: "Bank", value: form.bank_name },
            { label: "Account No", value: form.account_no },
            { label: "IFSC", value: form.ifsc },
      ];

      return (
            <>
                  <LiteHeader bg={"primary"} />
                  <Container className="account-page" fluid>
                        {loading ? (
                              <>
                                    <AccountCard />

                                    {/* Three destinations, not five cards in two columns.
                                        This page used to show Company Profiles, Two-Factor,
                                        Password, Account and Appearance all at once, masonried
                                        into two columns - a three-line card sitting beside a
                                        fifteen-control panel as though they were peers, with
                                        company identity next to a security switch next to font
                                        size. Nothing had priority because everything was
                                        visible, and the columns ended ragged because their
                                        contents have nothing to do with each other.

                                        Grouped by what they are ABOUT instead: the business,
                                        the way in, and how it looks. One at a time. */}
                                    <div className="account-layout">
                                          <nav className="account-rail" role="tablist" aria-label="Settings sections">
                                                {SECTIONS.map((s) => (
                                                      <button
                                                            key={s.key}
                                                            type="button"
                                                            role="tab"
                                                            aria-selected={section === s.key}
                                                            className={`account-rail-item${section === s.key ? " is-active" : ""}`}
                                                            onClick={() => setSection(s.key)}
                                                      >
                                                            <s.Icon size={16} aria-hidden="true" />
                                                            <span className="account-rail-text">
                                                                  <span className="account-rail-label">{s.label}</span>
                                                                  <span className="account-rail-hint">{s.hint}</span>
                                                            </span>
                                                      </button>
                                                ))}
                                          </nav>

                                          <div className="account-panel" key={section}>
                                                {section === "company" && <CompanyCarousel />}
                                                {section === "security" && (
                                                      <>
                                                            <PasswordCard email={form.email} />
                                                            <TwoFactorCard />
                                                      </>
                                                )}
                                                {section === "appearance" && <AppearanceSettings />}
                                          </div>
                                    </div>
                              </>
                        ) : (
                              <Loader />
                        )}
                  </Container>
                  <UploadModal
                        error={error}
                        modal={modal}
                        toggle={toggle}
                        uploadImage={uploadImage}
                        inputRef={inputRef}
                        userPhoto={userPhoto}
                        progressVar={progressVar}
                        handlePhoto={handlePhoto}
                  />
                  <UploadModal
                        error={error}
                        modal={qrModal}
                        toggle={qrToggle}
                        uploadImage={uploadQrImage}
                        inputRef={qrInputRef}
                        userPhoto={qrPhoto}
                        progressVar={progressVar}
                        handlePhoto={handleQrPhoto}
                  />
                  <AddDetail
                        modalNew={modalNew}
                        form={form}
                        setForm={setForm}
                        onSubmitHandler={detailSubmit}
                        onCancelHandler={onCancelHandler}
                  />
            </>
      );
};

export default UserProfileIndex;
