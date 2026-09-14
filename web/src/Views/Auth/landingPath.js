import { PATH_FEATURES } from "../../Common/features";

// First /admin route the account has access to.
//
// Admins go to "/admin", whose index route redirects to analytics (see AdminLayout). NOT "/":
// that used to bounce to /admin, but "/" is the marketing landing page now
// (src/Landing, added later), so logging in dropped an admin back onto the public website
// with no sign anything had happened.
const landingPathFor = (role, permissions) => {
    if (role !== "employee") return "/admin";
    const allowed = PATH_FEATURES.find((el) => el.prefix.startsWith("/admin/") && permissions.includes(el.key));
    return allowed ? allowed.prefix : "/admin/lifecycle";
};

export { landingPathFor };
export default landingPathFor;
