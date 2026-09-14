import axios from "axios";

// The public job-status read behind the WhatsApp link. No SESSION-TOKEN and no TAB-ID:
// the caller is a customer with no login and no company context - the pair of ids in the
// URL is what identifies the job. See invoice-mg-api/routes/Alert.js.
class AlertBackend {
    async jobCard(jobId, jobcardId) {
        const url = `${import.meta.env.VITE_API_URL}/alert/${encodeURIComponent(jobId)}/job/${encodeURIComponent(jobcardId)}`;
        const res = await axios.get(url);
        return res.data;
    }

    // Whether this job already carries a review, so the page shows what the customer said
    // rather than an empty form they would fill in twice.
    async reviewStatus(jobId, jobcardId) {
        const url = `${import.meta.env.VITE_API_URL}/alert/${encodeURIComponent(jobId)}/job/${encodeURIComponent(jobcardId)}/review`;
        const res = await axios.get(url);
        return res.data;
    }

    // Unauthenticated, like the read: the pair of ids in the URL is what authorises it.
    async submitReview(jobId, jobcardId, formData) {
        const url = `${import.meta.env.VITE_API_URL}/alert/${encodeURIComponent(jobId)}/job/${encodeURIComponent(jobcardId)}/review`;
        const res = await axios.post(url, formData);
        if (res.data.code !== 200) throw res.data;
        return res.data;
    }
}

export const alertBackend = new AlertBackend();
export default alertBackend;
