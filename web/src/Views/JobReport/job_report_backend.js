import axios from "axios";

const getHeader = () => ({
    headers: {
        "SESSION-TOKEN": window.localStorage.getItem("session_token"),
    },
});

// The Job Report (More > Reports). Reads the lifecycle jobs report - one header per job-id with
// its line items nested - for a Job-Date range. Behind the same `lifecycle` permission the Jobs
// board uses, so an employee who can see jobs can read the report on them.
class JobReportBackend {
    get(formData) {
        return new Promise(async (resolve, reject) => {
            try {
                const res = await axios.post(`${import.meta.env.VITE_API_URL}/lifecycle/jobs/report`, formData, getHeader());
                if (res.data.code !== 200) throw res.data;
                resolve(res.data);
            } catch (error) {
                reject(error);
            }
        });
    }
}

export const jobReportBackend = new JobReportBackend();
